package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/msinclair25/cailab/internal/scenario"
)

func TestNativeProcessManagerRestoreUsesOwnedEndpoint(t *testing.T) {
	runID := "native-restore"
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/_cailab/reset" || request.Method != http.MethodPost ||
			request.Header.Get("Authorization") != "Bearer control" || request.Header.Get("X-CloudAILab-Run") != runID {
			t.Fatalf("restore request = %+v", request)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	manager := NewNativeProcessManager(t.TempDir())
	runtimeDir := manager.runtimeDir(runID, "google")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	controlPath := filepath.Join(runtimeDir, "control.json")
	controlData, _ := json.Marshal(NativeRuntimeControl{RunID: runID, ControlToken: "control"})
	if err := os.WriteFile(controlPath, controlData, 0o600); err != nil {
		t.Fatal(err)
	}
	readyData, _ := json.Marshal(nativeReady{RunID: runID, Endpoint: server.URL, PID: os.Getpid()})
	if err := os.WriteFile(filepath.Join(runtimeDir, "ready.json"), readyData, 0o600); err != nil {
		t.Fatal(err)
	}
	compiled := scenario.Compiled{Runtimes: scenario.Runtimes{Google: &scenario.GoogleRuntime{Engine: "native"}}}
	instances := []Instance{{Provider: "google", Engine: "native", Endpoint: server.URL, ControlPath: controlPath}}
	restored, err := manager.Restore(context.Background(), runID, instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0].Endpoint != server.URL {
		t.Fatalf("restored = %+v", restored)
	}
	if !called {
		t.Fatal("restore endpoint was not called")
	}
}
