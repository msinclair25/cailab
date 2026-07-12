package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/graph"
	"github.com/msinclair25/cailab/internal/scenario"
)

const (
	riskyGrantID = "77777777-7777-4777-8777-777777777777"
	adminGrantID = "88888888-8888-4888-8888-888888888888"
)

func TestMicrosoftFacadeAuthPaginationAndGrantDeletion(t *testing.T) {
	t.Parallel()
	compiled := loadMicrosoftScenario(t)
	statePath := filepath.Join(t.TempDir(), "state.json")
	facade := &microsoftFacade{
		provider: *compiled.Providers.Microsoft, statePath: statePath,
		runID: "test-run", controlToken: "control", shutdown: func() {},
	}
	if err := facade.persist(); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: handlerRoundTripper{handler: facade}}
	endpoint := "http://cailab.test"
	facade.baseURL = endpoint

	response := graphRequest(t, client, http.MethodGet, endpoint+"/v1.0/oauth2PermissionGrants", "")
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", response.StatusCode)
	}
	response.Body.Close()

	response = graphRequest(t, client, http.MethodGet, endpoint+"/v1.0/oauth2PermissionGrants?$top=1", LocalGraphToken)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("first page status = %d", response.StatusCode)
	}
	var page struct {
		Value    []scenario.MicrosoftPermissionGrant `json:"value"`
		NextLink string                              `json:"@odata.nextLink"`
	}
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(page.Value) != 1 || page.NextLink == "" {
		t.Fatalf("first page = %+v", page)
	}
	response = graphRequest(t, client, http.MethodGet, page.NextLink, LocalGraphToken)
	var secondPage struct {
		Value []scenario.MicrosoftPermissionGrant `json:"value"`
	}
	if err := json.NewDecoder(response.Body).Decode(&secondPage); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(secondPage.Value) != 1 || secondPage.Value[0].ID == page.Value[0].ID {
		t.Fatalf("second page = %+v", secondPage)
	}

	response = graphRequest(t, client, http.MethodGet, endpoint+"/v1.0/users?$filter=accountEnabled%20eq%20true", LocalGraphToken)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsupported query status = %d, want 400", response.StatusCode)
	}
	response.Body.Close()

	response = graphRequest(t, client, http.MethodDelete, endpoint+"/v1.0/oauth2PermissionGrants/"+riskyGrantID, LocalGraphToken)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", response.StatusCode)
	}
	response.Body.Close()
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), riskyGrantID) || !strings.Contains(string(data), adminGrantID) {
		t.Fatalf("persisted state did not preserve only the intended grant: %s", data)
	}
	response = graphRequest(t, client, http.MethodDelete, endpoint+"/v1.0/oauth2PermissionGrants/"+adminGrantID, LocalGraphToken)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("second delete status = %d, want 204", response.StatusCode)
	}
	response.Body.Close()
	response = graphRequest(t, client, http.MethodGet, endpoint+"/v1.0/oauth2PermissionGrants", LocalGraphToken)
	emptyBody, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(emptyBody), `"value":[]`) {
		t.Fatalf("empty collection response = %s", emptyBody)
	}
}

func TestMicrosoftSnapshotTracksLivePermissionGrants(t *testing.T) {
	t.Parallel()
	compiled := loadMicrosoftScenario(t)
	facade := &microsoftFacade{
		provider: *compiled.Providers.Microsoft, statePath: filepath.Join(t.TempDir(), "state.json"),
		runID: "test-run", controlToken: "control", shutdown: func() {},
	}
	if err := facade.persist(); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: handlerRoundTripper{handler: facade}}
	endpoint := "http://cailab.test"
	facade.baseURL = endpoint

	snapshot, err := snapshotMicrosoftWithClient(context.Background(), endpoint, compiled, client)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "microsoft:analyst", "microsoft:directory-data", true)
	assertPath(t, snapshot, "microsoft:security-admin", "microsoft:directory-data", true)

	response := graphRequest(t, client, http.MethodDelete, endpoint+"/v1.0/oauth2PermissionGrants/"+riskyGrantID, LocalGraphToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.StatusCode)
	}
	snapshot, err = snapshotMicrosoftWithClient(context.Background(), endpoint, compiled, client)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "microsoft:analyst", "microsoft:directory-data", false)
	assertPath(t, snapshot, "microsoft:security-admin", "microsoft:directory-data", true)
}

func TestMicrosoftNativeIntegration(t *testing.T) {
	if os.Getenv("CAILAB_NATIVE_INTEGRATION") != "1" {
		t.Skip("set CAILAB_NATIVE_INTEGRATION=1 to run the native facade lifecycle")
	}
	compiled := loadMicrosoftScenario(t)
	manager := NewMicrosoftProcessManager(t.TempDir())
	manager.command = func(executable, providerName, configPath string) *exec.Cmd {
		if providerName != "microsoft" {
			t.Fatalf("providerName = %q, want microsoft", providerName)
		}
		command := exec.Command(executable, "-test.run=^TestMicrosoftRuntimeHelper$", "--", configPath)
		command.Env = append(os.Environ(), "CAILAB_MICROSOFT_RUNTIME_HELPER=1")
		return command
	}
	runID := "microsoft-native-integration"
	instances, err := manager.Start(context.Background(), runID, compiled)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = manager.Stop(context.Background(), runID, instances, compiled)
		}
	}()
	if len(instances) != 1 || instances[0].Provider != "microsoft" {
		t.Fatalf("instances = %+v", instances)
	}
	snapshot, err := manager.Snapshot(context.Background(), instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "microsoft:analyst", "microsoft:directory-data", true)
	response := graphRequest(t, http.DefaultClient, http.MethodDelete, instances[0].Endpoint+"/v1.0/oauth2PermissionGrants/"+riskyGrantID, LocalGraphToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.StatusCode)
	}
	snapshot, err = manager.Snapshot(context.Background(), instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "microsoft:analyst", "microsoft:directory-data", false)
	assertPath(t, snapshot, "microsoft:security-admin", "microsoft:directory-data", true)
	if err := manager.Stop(context.Background(), runID, instances, compiled); err != nil {
		t.Fatal(err)
	}
	stopped = true
	if _, err := os.Stat(manager.runtimeDir(runID, "microsoft")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime directory still exists: %v", err)
	}
}

func TestMicrosoftRuntimeHelper(t *testing.T) {
	if os.Getenv("CAILAB_MICROSOFT_RUNTIME_HELPER") != "1" {
		return
	}
	configPath := os.Args[len(os.Args)-1]
	if err := ServeMicrosoftRuntime(context.Background(), configPath); err != nil {
		t.Fatal(err)
	}
}

func TestMicrosoftControlEndpointMustBeIPv4Loopback(t *testing.T) {
	t.Parallel()
	tests := map[string]bool{
		"http://127.0.0.1:4567":        true,
		"http://localhost:4567":        false,
		"http://127.0.0.1:4567/path":   false,
		"https://127.0.0.1:4567":       false,
		"http://192.0.2.10:4567":       false,
		"http://127.0.0.1:4567?x=true": false,
	}
	for endpoint, want := range tests {
		if got := isIPv4LoopbackEndpoint(endpoint); got != want {
			t.Errorf("isIPv4LoopbackEndpoint(%q) = %v, want %v", endpoint, got, want)
		}
	}
}

func loadMicrosoftScenario(t *testing.T) scenario.Compiled {
	t.Helper()
	definition, err := scenario.Load(filepath.Join("..", "..", "scenarios", "microsoft-consent", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := scenario.Compile(definition, definition.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func graphRequest(t *testing.T, client *http.Client, method, target, token string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (h handlerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	h.handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	response.Request = request
	if response.Body == nil {
		response.Body = io.NopCloser(strings.NewReader(""))
	}
	return response, nil
}

func assertPath(t *testing.T, compiled scenario.Compiled, from, to string, want bool) {
	t.Helper()
	g, err := graph.New(compiled.Nodes, compiled.Edges)
	if err != nil {
		t.Fatal(err)
	}
	_, got := g.FindPath(from, to)
	if got != want {
		t.Fatalf("path %s -> %s = %v, want %v; edges = %+v", from, to, got, want, compiled.Edges)
	}
}
