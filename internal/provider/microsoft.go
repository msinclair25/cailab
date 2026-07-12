package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/msinclair25/cailab/internal/scenario"
)

const LocalGraphToken = "cailab-local"

type MicrosoftRuntimeConfig struct {
	NativeRuntimeControl
	Provider scenario.MicrosoftProvider `json:"provider"`
}

type microsoftFacade struct {
	mu           sync.RWMutex
	provider     scenario.MicrosoftProvider
	statePath    string
	runID        string
	controlToken string
	baseURL      string
	shutdown     func()
}

// ServeMicrosoftRuntime runs the private child-process entrypoint used by the native facade manager.
func ServeMicrosoftRuntime(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read Microsoft runtime config: %w", err)
	}
	var config MicrosoftRuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("decode Microsoft runtime config: %w", err)
	}
	if config.RunID == "" || config.StatePath == "" || config.ReadyPath == "" || config.ControlToken == "" {
		return errors.New("Microsoft runtime config is incomplete")
	}
	if config.Listen == "" {
		config.Listen = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp4", config.Listen)
	if err != nil {
		return fmt.Errorf("listen for Microsoft facade: %w", err)
	}
	defer listener.Close()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return fmt.Errorf("resolve Microsoft facade address: %w", err)
	}
	if host != "127.0.0.1" {
		return fmt.Errorf("Microsoft facade must bind to IPv4 loopback, got %q", host)
	}
	endpoint := "http://127.0.0.1:" + port

	runtimeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	facade := &microsoftFacade{
		provider: config.Provider, statePath: config.StatePath,
		runID: config.RunID, controlToken: config.ControlToken, baseURL: endpoint, shutdown: cancel,
	}
	if err := facade.persist(); err != nil {
		return err
	}
	server := &http.Server{
		Handler:           facade,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	readyData, err := json.Marshal(nativeReady{RunID: config.RunID, Endpoint: endpoint, PID: os.Getpid()})
	if err != nil {
		return fmt.Errorf("encode Microsoft readiness: %w", err)
	}
	if err := os.WriteFile(config.ReadyPath, readyData, 0o600); err != nil {
		return fmt.Errorf("write Microsoft readiness: %w", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()
	select {
	case <-runtimeCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down Microsoft facade: %w", err)
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}

func (f *microsoftFacade) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.URL.Path {
	case "/_cailab/health":
		if r.Method != http.MethodGet {
			writeGraphError(w, http.StatusMethodNotAllowed, "Request_BadRequest", "Method not allowed.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ready": true, "runId": f.runID})
		return
	case "/_cailab/shutdown":
		if r.Method != http.MethodPost {
			writeGraphError(w, http.StatusMethodNotAllowed, "Request_BadRequest", "Method not allowed.")
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+f.controlToken || r.Header.Get("X-CloudAILab-Run") != f.runID {
			writeGraphError(w, http.StatusForbidden, "Authorization_RequestDenied", "Invalid runtime control credentials.")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "stopping"})
		go f.shutdown()
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/v1.0/") {
		writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested resource was not found.")
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+LocalGraphToken {
		w.Header().Set("WWW-Authenticate", `Bearer realm="CloudAILab"`)
		writeGraphError(w, http.StatusUnauthorized, "InvalidAuthenticationToken", "A valid CloudAILab local bearer token is required.")
		return
	}

	resource := strings.TrimPrefix(r.URL.Path, "/v1.0/")
	if strings.Contains(resource, "/") {
		parts := strings.Split(resource, "/")
		if len(parts) == 3 && parts[0] == "servicePrincipals" && parts[2] == "appRoleAssignedTo" && r.Method == http.MethodGet {
			f.listAppRoleAssignments(w, r, "resource", parts[1])
			return
		}
		if len(parts) == 3 && parts[0] == "groups" && parts[2] == "appRoleAssignments" && r.Method == http.MethodGet {
			f.listAppRoleAssignments(w, r, "principal", parts[1])
			return
		}
		if len(parts) == 4 && parts[0] == "servicePrincipals" && parts[2] == "appRoleAssignedTo" && r.Method == http.MethodDelete {
			f.deleteAppRoleAssignment(w, parts[1], parts[3])
			return
		}
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested resource was not found.")
			return
		}
		if parts[0] == "oauth2PermissionGrants" && r.Method == http.MethodDelete {
			f.deleteGrant(w, parts[1])
			return
		}
		if r.Method == http.MethodGet {
			f.getObject(w, parts[0], parts[1])
			return
		}
		writeGraphError(w, http.StatusMethodNotAllowed, "Request_BadRequest", "Method not allowed.")
		return
	}
	if r.Method != http.MethodGet {
		writeGraphError(w, http.StatusMethodNotAllowed, "Request_BadRequest", "Method not allowed.")
		return
	}
	f.listObjects(w, r, resource)
}

func (f *microsoftFacade) listObjects(w http.ResponseWriter, r *http.Request, resource string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	values := make([]map[string]any, 0)
	switch resource {
	case "users":
		for _, user := range f.provider.Users {
			values = append(values, map[string]any{"id": user.ID, "displayName": user.DisplayName, "userPrincipalName": user.UserPrincipalName})
		}
	case "applications":
		for _, application := range f.provider.Applications {
			values = append(values, map[string]any{"id": application.ID, "appId": application.AppID, "displayName": application.DisplayName})
		}
	case "groups":
		for _, group := range f.provider.Groups {
			values = append(values, map[string]any{"id": group.ID, "displayName": group.DisplayName})
		}
	case "servicePrincipals":
		for _, servicePrincipal := range f.provider.ServicePrincipals {
			values = append(values, servicePrincipalObject(servicePrincipal))
		}
	case "oauth2PermissionGrants":
		for _, grant := range f.provider.OAuth2PermissionGrants {
			values = append(values, grantObject(grant))
		}
	default:
		writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested resource was not found.")
		return
	}
	page, nextLink, err := paginateGraph(r, values, f.baseURL)
	if err != nil {
		writeGraphError(w, http.StatusBadRequest, "Request_UnsupportedQuery", err.Error())
		return
	}
	response := map[string]any{
		"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#" + resource,
		"value":          page,
	}
	if nextLink != "" {
		response["@odata.nextLink"] = nextLink
	}
	writeJSON(w, http.StatusOK, response)
}

func (f *microsoftFacade) getObject(w http.ResponseWriter, resource, id string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var value map[string]any
	switch resource {
	case "users":
		for _, user := range f.provider.Users {
			if user.ID == id || strings.EqualFold(user.UserPrincipalName, id) {
				value = map[string]any{"id": user.ID, "displayName": user.DisplayName, "userPrincipalName": user.UserPrincipalName}
				break
			}
		}
	case "applications":
		for _, application := range f.provider.Applications {
			if application.ID == id {
				value = map[string]any{"id": application.ID, "appId": application.AppID, "displayName": application.DisplayName}
				break
			}
		}
	case "groups":
		for _, group := range f.provider.Groups {
			if group.ID == id {
				value = map[string]any{"id": group.ID, "displayName": group.DisplayName}
				break
			}
		}
	case "servicePrincipals":
		for _, servicePrincipal := range f.provider.ServicePrincipals {
			if servicePrincipal.ID == id {
				value = servicePrincipalObject(servicePrincipal)
				break
			}
		}
	case "oauth2PermissionGrants":
		for _, grant := range f.provider.OAuth2PermissionGrants {
			if grant.ID == id {
				value = grantObject(grant)
				break
			}
		}
	default:
		writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested resource was not found.")
		return
	}
	if value == nil {
		writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested object was not found.")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (f *microsoftFacade) listAppRoleAssignments(w http.ResponseWriter, r *http.Request, filter, objectID string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	values := make([]map[string]any, 0)
	for _, assignment := range f.provider.AppRoleAssignments {
		if filter == "resource" && assignment.ResourceID != objectID {
			continue
		}
		if filter == "principal" && assignment.PrincipalID != objectID {
			continue
		}
		values = append(values, appRoleAssignmentObject(assignment))
	}
	page, nextLink, err := paginateGraph(r, values, f.baseURL)
	if err != nil {
		writeGraphError(w, http.StatusBadRequest, "Request_UnsupportedQuery", err.Error())
		return
	}
	response := map[string]any{"@odata.context": "https://graph.microsoft.com/v1.0/$metadata#appRoleAssignments", "value": page}
	if nextLink != "" {
		response["@odata.nextLink"] = nextLink
	}
	writeJSON(w, http.StatusOK, response)
}

func (f *microsoftFacade) deleteAppRoleAssignment(w http.ResponseWriter, resourceID, assignmentID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := -1
	for i, assignment := range f.provider.AppRoleAssignments {
		if assignment.ResourceID == resourceID && assignment.ID == assignmentID {
			index = i
			break
		}
	}
	if index < 0 {
		writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested app-role assignment was not found.")
		return
	}
	previous := append([]scenario.MicrosoftAppRoleAssignment(nil), f.provider.AppRoleAssignments...)
	f.provider.AppRoleAssignments = append(f.provider.AppRoleAssignments[:index], f.provider.AppRoleAssignments[index+1:]...)
	if err := f.persistLocked(); err != nil {
		f.provider.AppRoleAssignments = previous
		writeGraphError(w, http.StatusInternalServerError, "InternalServerError", "The local facade could not persist the mutation.")
		return
	}
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func (f *microsoftFacade) deleteGrant(w http.ResponseWriter, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := -1
	for i, grant := range f.provider.OAuth2PermissionGrants {
		if grant.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		writeGraphError(w, http.StatusNotFound, "Request_ResourceNotFound", "The requested permission grant was not found.")
		return
	}
	previous := append([]scenario.MicrosoftPermissionGrant(nil), f.provider.OAuth2PermissionGrants...)
	f.provider.OAuth2PermissionGrants = append(f.provider.OAuth2PermissionGrants[:index], f.provider.OAuth2PermissionGrants[index+1:]...)
	if err := f.persistLocked(); err != nil {
		f.provider.OAuth2PermissionGrants = previous
		writeGraphError(w, http.StatusInternalServerError, "InternalServerError", "The local facade could not persist the mutation.")
		return
	}
	w.Header().Del("Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func (f *microsoftFacade) persist() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.persistLocked()
}

func (f *microsoftFacade) persistLocked() error {
	data, err := json.MarshalIndent(f.provider, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Microsoft facade state: %w", err)
	}
	if err := os.WriteFile(f.statePath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("persist Microsoft facade state: %w", err)
	}
	return nil
}

func grantObject(grant scenario.MicrosoftPermissionGrant) map[string]any {
	return map[string]any{
		"id": grant.ID, "clientId": grant.ClientID, "consentType": grant.ConsentType,
		"principalId": grant.PrincipalID, "resourceId": grant.ResourceID, "scope": grant.Scope,
	}
}

func servicePrincipalObject(servicePrincipal scenario.MicrosoftServicePrincipal) map[string]any {
	appRoles := make([]map[string]any, 0, len(servicePrincipal.AppRoles))
	for _, appRole := range servicePrincipal.AppRoles {
		appRoles = append(appRoles, map[string]any{"id": appRole.ID, "value": appRole.Value, "displayName": appRole.DisplayName})
	}
	return map[string]any{"id": servicePrincipal.ID, "appId": servicePrincipal.AppID, "displayName": servicePrincipal.DisplayName, "appRoles": appRoles}
}

func appRoleAssignmentObject(assignment scenario.MicrosoftAppRoleAssignment) map[string]any {
	return map[string]any{"id": assignment.ID, "principalId": assignment.PrincipalID, "resourceId": assignment.ResourceID, "appRoleId": assignment.AppRoleID}
}

func paginateGraph(r *http.Request, values []map[string]any, baseURL string) ([]map[string]any, string, error) {
	for key := range r.URL.Query() {
		if key != "$top" && key != "$skiptoken" && key != "$select" {
			return nil, "", fmt.Errorf("query option %q is not supported by this CloudAILab facade", key)
		}
	}
	top := 100
	if raw := r.URL.Query().Get("$top"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 999 {
			return nil, "", errors.New("$top must be an integer from 1 through 999")
		}
		top = parsed
	}
	offset := 0
	if raw := r.URL.Query().Get("$skiptoken"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > len(values) {
			return nil, "", errors.New("$skiptoken is invalid")
		}
		offset = parsed
	}
	end := offset + top
	if end > len(values) {
		end = len(values)
	}
	page := values[offset:end]
	if end == len(values) {
		return page, "", nil
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme != "http" || base.Host == "" {
		return nil, "", errors.New("facade base URL is invalid")
	}
	next := base.ResolveReference(&url.URL{Path: r.URL.Path})
	query := url.Values{"$top": {strconv.Itoa(top)}, "$skiptoken": {strconv.Itoa(end)}}
	if selected := r.URL.Query().Get("$select"); selected != "" {
		query.Set("$select", selected)
	}
	next.RawQuery = query.Encode()
	return page, next.String(), nil
}

func writeGraphError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func snapshotMicrosoft(ctx context.Context, endpoint string, compiled scenario.Compiled) (scenario.Compiled, error) {
	return snapshotMicrosoftWithClient(ctx, endpoint, compiled, &http.Client{Timeout: 5 * time.Second})
}

func snapshotMicrosoftWithClient(ctx context.Context, endpoint string, compiled scenario.Compiled, client *http.Client) (scenario.Compiled, error) {
	if compiled.Providers.Microsoft == nil {
		return compiled, nil
	}
	grants, err := fetchMicrosoftGrants(ctx, endpoint, client)
	if err != nil {
		return scenario.Compiled{}, err
	}
	assignments, err := fetchMicrosoftAppRoleAssignments(ctx, endpoint, compiled.Providers.Microsoft.ServicePrincipals, client)
	if err != nil {
		return scenario.Compiled{}, err
	}
	users := make(map[string]string)
	groups := make(map[string]string)
	clients := make(map[string]scenario.MicrosoftServicePrincipal)
	resources := make(map[string]string)
	appRoleValues := make(map[string]map[string]string)
	for _, user := range compiled.Providers.Microsoft.Users {
		users[user.ID] = user.Node
	}
	for _, group := range compiled.Providers.Microsoft.Groups {
		groups[group.ID] = group.Node
	}
	for _, servicePrincipal := range compiled.Providers.Microsoft.ServicePrincipals {
		if servicePrincipal.Node != "" {
			clients[servicePrincipal.ID] = servicePrincipal
		}
		if servicePrincipal.ResourceNode != "" {
			resources[servicePrincipal.ID] = servicePrincipal.ResourceNode
		}
		roles := make(map[string]string)
		for _, appRole := range servicePrincipal.AppRoles {
			roles[appRole.ID] = appRole.Value
		}
		appRoleValues[servicePrincipal.ID] = roles
	}
	nodes := make([]scenario.Node, 0, len(compiled.Nodes)+len(grants)+len(assignments))
	for _, node := range compiled.Nodes {
		if !strings.HasPrefix(node.ID, "microsoft:grant:") && !strings.HasPrefix(node.ID, "microsoft:app-role-assignment:") {
			nodes = append(nodes, node)
		}
	}
	edges := make([]scenario.Relationship, 0, len(compiled.Edges)+2*len(grants)+2*len(assignments))
	for _, edge := range compiled.Edges {
		if !strings.HasPrefix(edge.ID, "microsoft:grant:") && !strings.HasPrefix(edge.ID, "microsoft:app-role-assignment:") {
			edges = append(edges, edge)
		}
	}
	for _, grant := range grants {
		principal, principalOK := users[grant.PrincipalID]
		client, clientOK := clients[grant.ClientID]
		resource, resourceOK := resources[grant.ResourceID]
		if !principalOK || !clientOK || !resourceOK {
			return scenario.Compiled{}, fmt.Errorf("Microsoft grant %q references an object outside the compiled scenario", grant.ID)
		}
		actions := strings.Fields(grant.Scope)
		sort.Strings(actions)
		grantNode := "microsoft:grant:" + grant.ID
		nodes = append(nodes, scenario.Node{
			ID: grantNode, Kind: "authorization", Tenant: compiled.Providers.Microsoft.Tenant,
			Type: "oauth2_permission_grant", DisplayName: "Delegated grant to " + client.DisplayName,
		})
		edges = append(edges,
			scenario.Relationship{ID: grantNode + ":subject", From: principal, To: grantNode, Type: "assigned_to", Actions: append([]string(nil), actions...)},
			scenario.Relationship{ID: grantNode + ":resource", From: grantNode, To: resource, Type: "can_access", Actions: append([]string(nil), actions...)},
		)
	}
	for _, assignment := range assignments {
		principal, principalOK := groups[assignment.PrincipalID]
		resource, resourceOK := clients[assignment.ResourceID]
		roleValue, roleOK := appRoleValues[assignment.ResourceID][assignment.AppRoleID]
		if !principalOK || !resourceOK || !roleOK {
			return scenario.Compiled{}, fmt.Errorf("Microsoft app-role assignment %q references an object outside the compiled scenario", assignment.ID)
		}
		assignmentNode := "microsoft:app-role-assignment:" + assignment.ID
		actions := []string{roleValue}
		nodes = append(nodes, scenario.Node{
			ID: assignmentNode, Kind: "authorization", Tenant: compiled.Providers.Microsoft.Tenant,
			Type: "app_role_assignment", DisplayName: roleValue + " assignment to " + resource.DisplayName,
		})
		edges = append(edges,
			scenario.Relationship{ID: assignmentNode + ":principal", From: principal, To: assignmentNode, Type: "assigned_to", Actions: actions},
			scenario.Relationship{ID: assignmentNode + ":resource", From: assignmentNode, To: resource.Node, Type: "can_access", Actions: actions},
		)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	compiled.Nodes = nodes
	compiled.Edges = edges
	liveProvider := *compiled.Providers.Microsoft
	liveProvider.OAuth2PermissionGrants = append([]scenario.MicrosoftPermissionGrant(nil), grants...)
	liveProvider.AppRoleAssignments = append([]scenario.MicrosoftAppRoleAssignment(nil), assignments...)
	compiled.Providers.Microsoft = &liveProvider
	return compiled, nil
}

func fetchMicrosoftGrants(ctx context.Context, endpoint string, client *http.Client) ([]scenario.MicrosoftPermissionGrant, error) {
	next := strings.TrimRight(endpoint, "/") + "/v1.0/oauth2PermissionGrants?$top=100"
	var grants []scenario.MicrosoftPermissionGrant
	for next != "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, fmt.Errorf("build Microsoft snapshot request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+LocalGraphToken)
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("list Microsoft permission grants: %w", err)
		}
		var page struct {
			Value    []scenario.MicrosoftPermissionGrant `json:"value"`
			NextLink string                              `json:"@odata.nextLink"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&page)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("list Microsoft permission grants: status %d", response.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode Microsoft permission grants: %w", decodeErr)
		}
		grants = append(grants, page.Value...)
		next = page.NextLink
	}
	return grants, nil
}

func fetchMicrosoftAppRoleAssignments(ctx context.Context, endpoint string, servicePrincipals []scenario.MicrosoftServicePrincipal, client *http.Client) ([]scenario.MicrosoftAppRoleAssignment, error) {
	var assignments []scenario.MicrosoftAppRoleAssignment
	for _, servicePrincipal := range servicePrincipals {
		if servicePrincipal.Node == "" || len(servicePrincipal.AppRoles) == 0 {
			continue
		}
		next := strings.TrimRight(endpoint, "/") + "/v1.0/servicePrincipals/" + url.PathEscape(servicePrincipal.ID) + "/appRoleAssignedTo?$top=100"
		for next != "" {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
			if err != nil {
				return nil, fmt.Errorf("build Microsoft app-role assignment snapshot request: %w", err)
			}
			request.Header.Set("Authorization", "Bearer "+LocalGraphToken)
			response, err := client.Do(request)
			if err != nil {
				return nil, fmt.Errorf("list Microsoft app-role assignments: %w", err)
			}
			var page struct {
				Value    []scenario.MicrosoftAppRoleAssignment `json:"value"`
				NextLink string                                `json:"@odata.nextLink"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&page)
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("list Microsoft app-role assignments: status %d", response.StatusCode)
			}
			if decodeErr != nil {
				return nil, fmt.Errorf("decode Microsoft app-role assignments: %w", decodeErr)
			}
			assignments = append(assignments, page.Value...)
			next = page.NextLink
		}
	}
	return assignments, nil
}
