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

const LocalGoogleToken = "cailab-google-local"

type GoogleRuntimeConfig struct {
	NativeRuntimeControl
	Provider scenario.GoogleProvider `json:"provider"`
}

type googleFacade struct {
	mu           sync.RWMutex
	provider     scenario.GoogleProvider
	baseline     scenario.GoogleProvider
	statePath    string
	runID        string
	controlToken string
	shutdown     func()
}

// ServeGoogleRuntime runs the private child-process entrypoint used by the native facade manager.
func ServeGoogleRuntime(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read Google runtime config: %w", err)
	}
	var config GoogleRuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("decode Google runtime config: %w", err)
	}
	if config.RunID == "" || config.StatePath == "" || config.ReadyPath == "" || config.ControlToken == "" {
		return errors.New("Google runtime config is incomplete")
	}
	if config.Listen == "" {
		config.Listen = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp4", config.Listen)
	if err != nil {
		return fmt.Errorf("listen for Google facade: %w", err)
	}
	defer listener.Close()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return fmt.Errorf("resolve Google facade address: %w", err)
	}
	if host != "127.0.0.1" {
		return fmt.Errorf("Google facade must bind to IPv4 loopback, got %q", host)
	}
	endpoint := "http://127.0.0.1:" + port

	runtimeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	baseline, err := cloneProviderState(config.Provider)
	if err != nil {
		return fmt.Errorf("prepare Google baseline: %w", err)
	}
	providerState, err := cloneProviderState(config.Provider)
	if err != nil {
		return fmt.Errorf("prepare Google state: %w", err)
	}
	facade := &googleFacade{
		provider: providerState, baseline: baseline, statePath: config.StatePath,
		runID: config.RunID, controlToken: config.ControlToken, shutdown: cancel,
	}
	if err := facade.persist(); err != nil {
		return err
	}
	server := &http.Server{
		Handler: facade, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
	}
	readyData, err := json.Marshal(nativeReady{RunID: config.RunID, Endpoint: endpoint, PID: os.Getpid()})
	if err != nil {
		return fmt.Errorf("encode Google readiness: %w", err)
	}
	if err := os.WriteFile(config.ReadyPath, readyData, 0o600); err != nil {
		return fmt.Errorf("write Google readiness: %w", err)
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
			return fmt.Errorf("shut down Google facade: %w", err)
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}

func (f *googleFacade) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if f.handleControl(w, r) {
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+LocalGoogleToken {
		w.Header().Set("WWW-Authenticate", `Bearer realm="CloudAILab"`)
		writeGoogleError(w, http.StatusUnauthorized, "authError", "A valid CloudAILab local bearer token is required.")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin/directory/v1/") {
		f.handleDirectory(w, r, strings.TrimPrefix(r.URL.Path, "/admin/directory/v1/"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/drive/v3/") {
		f.handleDrive(w, r, strings.TrimPrefix(r.URL.Path, "/drive/v3/"))
		return
	}
	writeGoogleError(w, http.StatusNotFound, "notFound", "The requested resource was not found.")
}

func (f *googleFacade) handleControl(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/_cailab/health":
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"ready": true, "runId": f.runID})
		return true
	case "/_cailab/shutdown":
		if r.Method != http.MethodPost {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return true
		}
		if r.Header.Get("Authorization") != "Bearer "+f.controlToken || r.Header.Get("X-CloudAILab-Run") != f.runID {
			writeGoogleError(w, http.StatusForbidden, "forbidden", "Invalid runtime control credentials.")
			return true
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "stopping"})
		go f.shutdown()
		return true
	case "/_cailab/reset":
		if r.Method != http.MethodPost {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return true
		}
		if r.Header.Get("Authorization") != "Bearer "+f.controlToken || r.Header.Get("X-CloudAILab-Run") != f.runID {
			writeGoogleError(w, http.StatusForbidden, "forbidden", "Invalid runtime control credentials.")
			return true
		}
		if err := f.restoreBaseline(); err != nil {
			writeGoogleError(w, http.StatusInternalServerError, "internalError", "The local facade could not restore its baseline.")
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "restored"})
		return true
	default:
		return false
	}
}

func (f *googleFacade) handleDirectory(w http.ResponseWriter, r *http.Request, resource string) {
	parts := strings.Split(strings.Trim(resource, "/"), "/")
	switch {
	case len(parts) == 1 && (parts[0] == "users" || parts[0] == "groups"):
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.listDirectory(w, r, parts[0])
	case len(parts) == 2 && (parts[0] == "users" || parts[0] == "groups"):
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.getDirectoryObject(w, parts[0], parts[1])
	case len(parts) == 3 && parts[0] == "groups" && parts[2] == "members":
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.listMembers(w, r, parts[1])
	default:
		writeGoogleError(w, http.StatusNotFound, "notFound", "The requested Directory resource was not found.")
	}
}

func (f *googleFacade) listDirectory(w http.ResponseWriter, r *http.Request, resource string) {
	allowed := map[string]bool{"customer": true, "domain": true, "maxResults": true, "pageToken": true}
	if err := validateGoogleQuery(r.URL.Query(), allowed); err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	customer, domain := r.URL.Query().Get("customer"), r.URL.Query().Get("domain")
	if (customer == "") == (domain == "") {
		writeGoogleError(w, http.StatusBadRequest, "required", "Exactly one of customer or domain is required.")
		return
	}
	if customer != "" && customer != "my_customer" && customer != f.provider.CustomerID {
		writeGoogleError(w, http.StatusBadRequest, "invalid", "The customer identifier is not available in this scenario.")
		return
	}
	if domain != "" && !f.hasDomain(domain) {
		writeGoogleError(w, http.StatusBadRequest, "invalid", "The domain is not available in this scenario.")
		return
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	values := make([]any, 0)
	if resource == "users" {
		for _, user := range f.provider.Users {
			values = append(values, googleUserObject(user))
		}
	} else {
		for _, group := range f.provider.Groups {
			values = append(values, googleGroupObject(group))
		}
	}
	maximum := 500
	if resource == "groups" {
		maximum = 200
	}
	page, next, err := paginateGoogle(r.URL.Query(), values, "maxResults", maximum)
	if err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	response := map[string]any{"kind": "admin#directory#" + resource, resource: page}
	if next != "" {
		response["nextPageToken"] = next
	}
	writeJSON(w, http.StatusOK, response)
}

func (f *googleFacade) hasDomain(domain string) bool {
	suffix := "@" + strings.ToLower(domain)
	for _, user := range f.provider.Users {
		if strings.HasSuffix(strings.ToLower(user.PrimaryEmail), suffix) {
			return true
		}
	}
	for _, group := range f.provider.Groups {
		if strings.HasSuffix(strings.ToLower(group.Email), suffix) {
			return true
		}
	}
	return false
}

func (f *googleFacade) getDirectoryObject(w http.ResponseWriter, resource, key string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if resource == "users" {
		for _, user := range f.provider.Users {
			if user.ID == key || strings.EqualFold(user.PrimaryEmail, key) {
				writeJSON(w, http.StatusOK, googleUserObject(user))
				return
			}
		}
	} else {
		for _, group := range f.provider.Groups {
			if group.ID == key || strings.EqualFold(group.Email, key) {
				writeJSON(w, http.StatusOK, googleGroupObject(group))
				return
			}
		}
	}
	writeGoogleError(w, http.StatusNotFound, "notFound", "The requested Directory object was not found.")
}

func (f *googleFacade) listMembers(w http.ResponseWriter, r *http.Request, groupKey string) {
	if err := validateGoogleQuery(r.URL.Query(), map[string]bool{"maxResults": true, "pageToken": true}); err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, group := range f.provider.Groups {
		if group.ID != groupKey && !strings.EqualFold(group.Email, groupKey) {
			continue
		}
		values := make([]any, 0, len(group.Members))
		for _, member := range group.Members {
			values = append(values, map[string]any{"kind": "admin#directory#member", "id": member.ID, "email": member.Email, "role": member.Role, "type": member.Type, "status": "ACTIVE"})
		}
		page, next, err := paginateGoogle(r.URL.Query(), values, "maxResults", 200)
		if err != nil {
			writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
			return
		}
		response := map[string]any{"kind": "admin#directory#members", "members": page}
		if next != "" {
			response["nextPageToken"] = next
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	writeGoogleError(w, http.StatusNotFound, "notFound", "The requested group was not found.")
}

func (f *googleFacade) handleDrive(w http.ResponseWriter, r *http.Request, resource string) {
	parts := strings.Split(strings.Trim(resource, "/"), "/")
	switch {
	case len(parts) == 1 && parts[0] == "files":
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.listFiles(w, r)
	case len(parts) == 2 && parts[0] == "files":
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.getFile(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "files" && parts[2] == "permissions":
		if r.Method != http.MethodGet {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.listPermissions(w, r, parts[1])
	case len(parts) == 4 && parts[0] == "files" && parts[2] == "permissions":
		if r.Method != http.MethodDelete {
			writeGoogleError(w, http.StatusMethodNotAllowed, "badRequest", "Method not allowed.")
			return
		}
		f.deletePermission(w, r, parts[1], parts[3])
	default:
		writeGoogleError(w, http.StatusNotFound, "notFound", "The requested Drive resource was not found.")
	}
}

func (f *googleFacade) listFiles(w http.ResponseWriter, r *http.Request) {
	if err := validateGoogleQuery(r.URL.Query(), map[string]bool{"pageSize": true, "pageToken": true, "fields": true}); err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	values := make([]any, 0, len(f.provider.DriveFiles))
	for _, file := range f.provider.DriveFiles {
		values = append(values, googleFileObject(file))
	}
	page, next, err := paginateGoogle(r.URL.Query(), values, "pageSize", 1000)
	if err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	response := map[string]any{"kind": "drive#fileList", "files": page, "incompleteSearch": false}
	if next != "" {
		response["nextPageToken"] = next
	}
	writeJSON(w, http.StatusOK, response)
}

func (f *googleFacade) getFile(w http.ResponseWriter, r *http.Request, fileID string) {
	if err := validateGoogleQuery(r.URL.Query(), map[string]bool{"alt": true, "fields": true}); err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, file := range f.provider.DriveFiles {
		if file.ID != fileID {
			continue
		}
		if r.URL.Query().Get("alt") == "media" {
			w.Header().Set("Content-Type", file.MimeType)
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, file.Content)
			return
		}
		if alt := r.URL.Query().Get("alt"); alt != "" && alt != "json" {
			writeGoogleError(w, http.StatusBadRequest, "invalid", "alt must be json or media.")
			return
		}
		writeJSON(w, http.StatusOK, googleFileObject(file))
		return
	}
	writeGoogleError(w, http.StatusNotFound, "notFound", "The requested Drive file was not found.")
}

func (f *googleFacade) listPermissions(w http.ResponseWriter, r *http.Request, fileID string) {
	if err := validateGoogleQuery(r.URL.Query(), map[string]bool{"pageSize": true, "pageToken": true, "fields": true}); err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.hasFile(fileID) {
		writeGoogleError(w, http.StatusNotFound, "notFound", "The requested Drive file was not found.")
		return
	}
	values := make([]any, 0)
	for _, permission := range f.provider.DrivePermissions {
		if permission.FileID == fileID {
			values = append(values, googlePermissionObject(permission))
		}
	}
	page, next, err := paginateGoogle(r.URL.Query(), values, "pageSize", 100)
	if err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	response := map[string]any{"kind": "drive#permissionList", "permissions": page}
	if next != "" {
		response["nextPageToken"] = next
	}
	writeJSON(w, http.StatusOK, response)
}

func (f *googleFacade) deletePermission(w http.ResponseWriter, r *http.Request, fileID, permissionID string) {
	if err := validateGoogleQuery(r.URL.Query(), map[string]bool{}); err != nil {
		writeGoogleError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	index := -1
	for i, permission := range f.provider.DrivePermissions {
		if permission.FileID == fileID && permission.ID == permissionID {
			index = i
			break
		}
	}
	if index < 0 {
		writeGoogleError(w, http.StatusNotFound, "notFound", "The requested Drive permission was not found.")
		return
	}
	previous := append([]scenario.GoogleDrivePermission(nil), f.provider.DrivePermissions...)
	f.provider.DrivePermissions = append(f.provider.DrivePermissions[:index], f.provider.DrivePermissions[index+1:]...)
	if err := f.persistLocked(); err != nil {
		f.provider.DrivePermissions = previous
		writeGoogleError(w, http.StatusInternalServerError, "internalError", "The local facade could not persist the mutation.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (f *googleFacade) hasFile(fileID string) bool {
	for _, file := range f.provider.DriveFiles {
		if file.ID == fileID {
			return true
		}
	}
	return false
}

func (f *googleFacade) persist() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.persistLocked()
}

func (f *googleFacade) persistLocked() error {
	data, err := json.MarshalIndent(f.provider, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Google facade state: %w", err)
	}
	if err := os.WriteFile(f.statePath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("persist Google facade state: %w", err)
	}
	return nil
}

func (f *googleFacade) restoreBaseline() error {
	restored, err := cloneProviderState(f.baseline)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	previous := f.provider
	f.provider = restored
	if err := f.persistLocked(); err != nil {
		f.provider = previous
		return err
	}
	return nil
}

func googleUserObject(user scenario.GoogleUser) map[string]any {
	return map[string]any{"kind": "admin#directory#user", "id": user.ID, "primaryEmail": user.PrimaryEmail, "name": map[string]any{"fullName": user.DisplayName}}
}

func googleGroupObject(group scenario.GoogleGroup) map[string]any {
	return map[string]any{"kind": "admin#directory#group", "id": group.ID, "email": group.Email, "name": group.Name, "description": group.Description, "directMembersCount": strconv.Itoa(len(group.Members))}
}

func googleFileObject(file scenario.GoogleDriveFile) map[string]any {
	return map[string]any{"kind": "drive#file", "id": file.ID, "name": file.Name, "mimeType": file.MimeType}
}

func googlePermissionObject(permission scenario.GoogleDrivePermission) map[string]any {
	return map[string]any{"kind": "drive#permission", "id": permission.ID, "type": permission.Type, "emailAddress": permission.EmailAddress, "role": permission.Role}
}

func validateGoogleQuery(query url.Values, allowed map[string]bool) error {
	for key := range query {
		if !allowed[key] {
			return fmt.Errorf("query option %q is not supported by this CloudAILab facade", key)
		}
	}
	return nil
}

func paginateGoogle(query url.Values, values []any, sizeKey string, maximum int) ([]any, string, error) {
	size := maximum
	if raw := query.Get(sizeKey); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maximum {
			return nil, "", fmt.Errorf("%s must be an integer from 1 through %d", sizeKey, maximum)
		}
		size = parsed
	}
	offset := 0
	if raw := query.Get("pageToken"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > len(values) {
			return nil, "", errors.New("pageToken is invalid")
		}
		offset = parsed
	}
	end := offset + size
	if end > len(values) {
		end = len(values)
	}
	if end == len(values) {
		return values[offset:end], "", nil
	}
	return values[offset:end], strconv.Itoa(end), nil
}

func writeGoogleError(w http.ResponseWriter, status int, reason, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": status, "message": message,
		"errors": []map[string]any{{"message": message, "domain": "global", "reason": reason}},
	}})
}

func snapshotGoogle(ctx context.Context, endpoint string, compiled scenario.Compiled) (scenario.Compiled, error) {
	return snapshotGoogleWithClient(ctx, endpoint, compiled, &http.Client{Timeout: 5 * time.Second})
}

func snapshotGoogleWithClient(ctx context.Context, endpoint string, compiled scenario.Compiled, client *http.Client) (scenario.Compiled, error) {
	if compiled.Providers.Google == nil {
		return compiled, nil
	}
	provider := compiled.Providers.Google
	liveProvider := *provider
	liveProvider.Users = append([]scenario.GoogleUser(nil), provider.Users...)
	liveProvider.Groups = make([]scenario.GoogleGroup, len(provider.Groups))
	for i, group := range provider.Groups {
		liveProvider.Groups[i] = group
		liveProvider.Groups[i].Members = nil
	}
	liveProvider.DriveFiles = append([]scenario.GoogleDriveFile(nil), provider.DriveFiles...)
	liveProvider.DrivePermissions = nil
	users := make(map[string]string)
	groups := make(map[string]string)
	files := make(map[string]string)
	for _, user := range provider.Users {
		users[strings.ToLower(user.PrimaryEmail)] = user.Node
	}
	for _, group := range provider.Groups {
		groups[strings.ToLower(group.Email)] = group.Node
	}
	for _, file := range provider.DriveFiles {
		files[file.ID] = file.Node
	}

	nodes := make([]scenario.Node, 0, len(compiled.Nodes)+len(provider.DrivePermissions))
	for _, node := range compiled.Nodes {
		if !strings.HasPrefix(node.ID, "google:permission:") {
			nodes = append(nodes, node)
		}
	}
	edges := make([]scenario.Relationship, 0, len(compiled.Edges)+len(provider.Groups)+2*len(provider.DrivePermissions))
	for _, edge := range compiled.Edges {
		if !strings.HasPrefix(edge.ID, "google:membership:") && !strings.HasPrefix(edge.ID, "google:permission:") {
			edges = append(edges, edge)
		}
	}

	for groupIndex, group := range provider.Groups {
		members, err := fetchGoogleMembers(ctx, endpoint, group.ID, client)
		if err != nil {
			return scenario.Compiled{}, err
		}
		liveProvider.Groups[groupIndex].Members = append([]scenario.GoogleGroupMember(nil), members...)
		for _, member := range members {
			principal, ok := users[strings.ToLower(member.Email)]
			if !ok {
				return scenario.Compiled{}, fmt.Errorf("Google group %q contains a member outside the compiled scenario", group.ID)
			}
			edges = append(edges, scenario.Relationship{ID: "google:membership:" + group.ID + ":" + member.ID, From: principal, To: group.Node, Type: "member_of", Actions: []string{"member"}})
		}
	}
	for _, file := range provider.DriveFiles {
		permissions, err := fetchGooglePermissions(ctx, endpoint, file.ID, client)
		if err != nil {
			return scenario.Compiled{}, err
		}
		for _, permission := range permissions {
			liveProvider.DrivePermissions = append(liveProvider.DrivePermissions, permission)
			var principal string
			if permission.Type == "user" {
				principal = users[strings.ToLower(permission.EmailAddress)]
			} else if permission.Type == "group" {
				principal = groups[strings.ToLower(permission.EmailAddress)]
			}
			resource := files[file.ID]
			if principal == "" || resource == "" {
				return scenario.Compiled{}, fmt.Errorf("Google permission %q references an object outside the compiled scenario", permission.ID)
			}
			permissionNode := "google:permission:" + file.ID + ":" + permission.ID
			nodes = append(nodes, scenario.Node{ID: permissionNode, Kind: "authorization", Tenant: provider.Tenant, Type: "drive_permission", DisplayName: permission.Role + " Drive permission"})
			actions := []string{permission.Role}
			edges = append(edges,
				scenario.Relationship{ID: permissionNode + ":subject", From: principal, To: permissionNode, Type: "assigned_to", Actions: actions},
				scenario.Relationship{ID: permissionNode + ":resource", From: permissionNode, To: resource, Type: "can_access", Actions: actions},
			)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	compiled.Nodes = nodes
	compiled.Edges = edges
	compiled.Providers.Google = &liveProvider
	return compiled, nil
}

func fetchGoogleMembers(ctx context.Context, endpoint, groupID string, client *http.Client) ([]scenario.GoogleGroupMember, error) {
	target := strings.TrimRight(endpoint, "/") + "/admin/directory/v1/groups/" + url.PathEscape(groupID) + "/members?maxResults=200"
	var members []scenario.GoogleGroupMember
	for target != "" {
		var page struct {
			Members       []scenario.GoogleGroupMember `json:"members"`
			NextPageToken string                       `json:"nextPageToken"`
		}
		next, err := fetchGooglePage(ctx, target, client, &page)
		if err != nil {
			return nil, fmt.Errorf("list Google group members: %w", err)
		}
		members = append(members, page.Members...)
		target = nextGooglePageURL(target, next)
	}
	return members, nil
}

func fetchGooglePermissions(ctx context.Context, endpoint, fileID string, client *http.Client) ([]scenario.GoogleDrivePermission, error) {
	target := strings.TrimRight(endpoint, "/") + "/drive/v3/files/" + url.PathEscape(fileID) + "/permissions?pageSize=100"
	var permissions []scenario.GoogleDrivePermission
	for target != "" {
		var page struct {
			Permissions   []scenario.GoogleDrivePermission `json:"permissions"`
			NextPageToken string                           `json:"nextPageToken"`
		}
		next, err := fetchGooglePage(ctx, target, client, &page)
		if err != nil {
			return nil, fmt.Errorf("list Google Drive permissions: %w", err)
		}
		for i := range page.Permissions {
			page.Permissions[i].FileID = fileID
		}
		permissions = append(permissions, page.Permissions...)
		target = nextGooglePageURL(target, next)
	}
	return permissions, nil
}

func fetchGooglePage(ctx context.Context, target string, client *http.Client, page any) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("build snapshot request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+LocalGoogleToken)
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(page)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", response.StatusCode)
	}
	if decodeErr != nil {
		return "", decodeErr
	}
	data, err := json.Marshal(page)
	if err != nil {
		return "", err
	}
	var envelope struct {
		NextPageToken string `json:"nextPageToken"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	return envelope.NextPageToken, nil
}

func nextGooglePageURL(current, token string) string {
	if token == "" {
		return ""
	}
	parsed, err := url.Parse(current)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set("pageToken", token)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
