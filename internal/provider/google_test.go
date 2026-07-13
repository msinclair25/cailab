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

	"github.com/msinclair25/cailab/internal/scenario"
)

func TestGoogleFacadeContractAndSnapshot(t *testing.T) {
	compiled := loadGoogleScenario(t)
	statePath := filepath.Join(t.TempDir(), "google-state.json")
	facade := &googleFacade{provider: *compiled.Providers.Google, statePath: statePath, runID: "google-test", controlToken: "control"}
	if err := facade.persist(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(facade)
	defer server.Close()

	response := googleRequest(t, http.DefaultClient, http.MethodGet, server.URL+"/admin/directory/v1/users?customer=my_customer", "")
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", response.StatusCode)
	}

	response = googleRequest(t, http.DefaultClient, http.MethodGet, server.URL+"/admin/directory/v1/users?customer=my_customer&maxResults=1", LocalGoogleToken)
	var usersPage struct {
		Users         []map[string]any `json:"users"`
		NextPageToken string           `json:"nextPageToken"`
	}
	decodeResponse(t, response, http.StatusOK, &usersPage)
	if len(usersPage.Users) != 1 || usersPage.NextPageToken == "" {
		t.Fatalf("users page = %+v", usersPage)
	}

	response = googleRequest(t, http.DefaultClient, http.MethodGet, server.URL+"/admin/directory/v1/groups/group_records_team/members?maxResults=200", LocalGoogleToken)
	var membersPage struct {
		Members []scenario.GoogleGroupMember `json:"members"`
	}
	decodeResponse(t, response, http.StatusOK, &membersPage)
	if len(membersPage.Members) != 1 || membersPage.Members[0].Email != "records.admin@example.test" {
		t.Fatalf("members page = %+v", membersPage)
	}

	response = googleRequest(t, http.DefaultClient, http.MethodGet, server.URL+"/drive/v3/files/file_retention_plan?alt=media", LocalGoogleToken)
	content, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(content), "Synthetic restricted") {
		t.Fatalf("media response status=%d body=%q err=%v", response.StatusCode, content, err)
	}

	snapshot, err := snapshotGoogleWithClient(context.Background(), server.URL, compiled, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "principal:contractor", "resource:retention-plan", true)
	assertPath(t, snapshot, "principal:records-admin", "resource:retention-plan", true)

	response = googleRequest(t, http.DefaultClient, http.MethodDelete, server.URL+"/drive/v3/files/file_retention_plan/permissions/permission_contractor", LocalGoogleToken)
	var deleted map[string]any
	decodeResponse(t, response, http.StatusOK, &deleted)
	snapshot, err = snapshotGoogleWithClient(context.Background(), server.URL, compiled, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "principal:contractor", "resource:retention-plan", false)
	assertPath(t, snapshot, "principal:records-admin", "resource:retention-plan", true)
	if len(snapshot.Providers.Google.DrivePermissions) != 1 || snapshot.Providers.Google.DrivePermissions[0].ID != "permission_records_team" {
		t.Fatalf("snapshot provider permissions = %+v", snapshot.Providers.Google.DrivePermissions)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted scenario.GoogleProvider
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.DrivePermissions) != 1 || persisted.DrivePermissions[0].ID != "permission_records_team" {
		t.Fatalf("persisted permissions = %+v", persisted.DrivePermissions)
	}
}

func TestGoogleFacadeRejectsUnsupportedQuery(t *testing.T) {
	compiled := loadGoogleScenario(t)
	facade := &googleFacade{provider: *compiled.Providers.Google, statePath: filepath.Join(t.TempDir(), "state.json"), runID: "google-query", controlToken: "control"}
	server := httptest.NewServer(facade)
	defer server.Close()
	response := googleRequest(t, http.DefaultClient, http.MethodGet, server.URL+"/drive/v3/files?q=trashed%3Dfalse", LocalGoogleToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsupported query status = %d", response.StatusCode)
	}
	var envelope struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || envelope.Error.Code != http.StatusBadRequest {
		t.Fatalf("Google error = %+v, err=%v", envelope, err)
	}
}

func TestGoogleControlResetRestoresBaselineWithoutChangingEndpoint(t *testing.T) {
	compiled := loadGoogleScenario(t)
	baseline, err := cloneProviderState(*compiled.Providers.Google)
	if err != nil {
		t.Fatal(err)
	}
	current, err := cloneProviderState(*compiled.Providers.Google)
	if err != nil {
		t.Fatal(err)
	}
	facade := &googleFacade{
		provider: current, baseline: baseline, statePath: filepath.Join(t.TempDir(), "state.json"),
		runID: "google-test", controlToken: "control",
	}
	server := httptest.NewServer(facade)
	defer server.Close()
	response := googleRequest(t, http.DefaultClient, http.MethodDelete, server.URL+"/drive/v3/files/file_retention_plan/permissions/permission_contractor", LocalGoogleToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", response.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/_cailab/reset", nil)
	request.Header.Set("Authorization", "Bearer control")
	request.Header.Set("X-CloudAILab-Run", "google-test")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reset status = %d", response.StatusCode)
	}
	snapshot, err := snapshotGoogleWithClient(context.Background(), server.URL, compiled, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Providers.Google.DrivePermissions) != len(compiled.Providers.Google.DrivePermissions) {
		t.Fatalf("restored permissions = %+v", snapshot.Providers.Google.DrivePermissions)
	}
}

func TestNativeStartupFailureRemovesIncompleteRunDirectory(t *testing.T) {
	compiled := loadGoogleScenario(t)
	manager := NewNativeProcessManager(t.TempDir())
	manager.command = func(_, _, _ string) *exec.Cmd {
		return exec.Command(filepath.Join(t.TempDir(), "missing-cailab"))
	}
	runID := "google-startup-failure"
	_, err := manager.Start(context.Background(), runID, compiled)
	if err == nil {
		t.Fatal("Start() error = nil, want command startup failure")
	}
	if _, statErr := os.Stat(manager.runtimeDir(runID, "google")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("incomplete runtime directory still exists: %v", statErr)
	}
}

func TestGoogleNativeIntegration(t *testing.T) {
	if os.Getenv("CAILAB_NATIVE_INTEGRATION") != "1" {
		t.Skip("set CAILAB_NATIVE_INTEGRATION=1 to run the native facade lifecycle")
	}
	compiled := loadGoogleScenario(t)
	manager := NewNativeProcessManager(t.TempDir())
	manager.command = func(executable, providerName, configPath string) *exec.Cmd {
		if providerName != "google" {
			t.Fatalf("providerName = %q, want google", providerName)
		}
		command := exec.Command(executable, "-test.run=^TestGoogleRuntimeHelper$", "--", configPath)
		command.Env = append(os.Environ(), "CAILAB_GOOGLE_RUNTIME_HELPER=1")
		return command
	}
	runID := "google-native-integration"
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
	if len(instances) != 1 || instances[0].Provider != "google" {
		t.Fatalf("instances = %+v", instances)
	}
	snapshot, err := manager.Snapshot(context.Background(), instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "principal:contractor", "resource:retention-plan", true)
	response := googleRequest(t, http.DefaultClient, http.MethodDelete, instances[0].Endpoint+"/drive/v3/files/file_retention_plan/permissions/permission_contractor", LocalGoogleToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", response.StatusCode)
	}
	snapshot, err = manager.Snapshot(context.Background(), instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "principal:contractor", "resource:retention-plan", false)
	assertPath(t, snapshot, "principal:records-admin", "resource:retention-plan", true)
	if _, err := manager.Restore(context.Background(), runID, instances, compiled); err != nil {
		t.Fatal(err)
	}
	snapshot, err = manager.Snapshot(context.Background(), instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "principal:contractor", "resource:retention-plan", true)
	if err := manager.Stop(context.Background(), runID, instances, compiled); err != nil {
		t.Fatal(err)
	}
	stopped = true
	if _, err := os.Stat(manager.runtimeDir(runID, "google")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime directory still exists: %v", err)
	}
}

func TestGoogleRuntimeHelper(t *testing.T) {
	if os.Getenv("CAILAB_GOOGLE_RUNTIME_HELPER") != "1" {
		return
	}
	configPath := os.Args[len(os.Args)-1]
	if err := ServeGoogleRuntime(context.Background(), configPath); err != nil {
		t.Fatal(err)
	}
}

func googleRequest(t *testing.T, client *http.Client, method, target, token string) *http.Response {
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

func decodeResponse(t *testing.T, response *http.Response, status int, target any) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != status {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("response status = %d, want %d; body=%s", response.StatusCode, status, body)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func loadGoogleScenario(t *testing.T) scenario.Compiled {
	t.Helper()
	definition, err := scenario.Load(filepath.Join("..", "..", "scenarios", "google-drive-sharing", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := scenario.Compile(definition, definition.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}
