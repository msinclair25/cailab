package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
)

type fakeProviderManager struct {
	startErr         error
	started          bool
	stopped          bool
	rotated          bool
	restored         bool
	restoreErr       error
	restoredRuntimes []provider.Instance
	snapshots        []scenario.Compiled
	snapshotIndex    int
}

func (f *fakeProviderManager) Start(_ context.Context, _ string, _ scenario.Compiled) ([]provider.Instance, error) {
	f.started = true
	if f.startErr != nil {
		return nil, f.startErr
	}
	return []provider.Instance{{Provider: "aws", Engine: "floci", Name: "test-runtime", Endpoint: "http://127.0.0.1:4566", Status: "ready"}}, nil
}

func (f *fakeProviderManager) Stop(_ context.Context, _ string, _ []provider.Instance, _ scenario.Compiled) error {
	f.stopped = true
	return nil
}

func (f *fakeProviderManager) Snapshot(_ context.Context, _ []provider.Instance, compiled scenario.Compiled) (scenario.Compiled, error) {
	if f.snapshotIndex < len(f.snapshots) {
		result := f.snapshots[f.snapshotIndex]
		f.snapshotIndex++
		return result, nil
	}
	return compiled, nil
}

func (f *fakeProviderManager) Restore(_ context.Context, _ string, instances []provider.Instance, _ scenario.Compiled) ([]provider.Instance, error) {
	f.restored = true
	if f.restoredRuntimes != nil {
		return append([]provider.Instance(nil), f.restoredRuntimes...), f.restoreErr
	}
	return append([]provider.Instance(nil), instances...), f.restoreErr
}

func (f *fakeProviderManager) RotateIdentity(_ context.Context, _ string, _ []provider.Instance) (provider.OIDCJWKSet, error) {
	f.rotated = true
	return provider.OIDCJWKSet{}, nil
}

func TestServicePersistsAndStopsProviderRuntime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := state.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := &fakeProviderManager{}
	service := New(store, manager)
	run, err := service.Up(ctx, UpOptions{ScenarioPath: writeAppScenario(t)})
	if err != nil {
		t.Fatal(err)
	}
	if !manager.started || len(run.Runtimes) != 1 {
		t.Fatalf("started = %v, runtimes = %+v", manager.started, run.Runtimes)
	}
	persisted, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Runtimes) != 1 || persisted.Runtimes[0].Name != "test-runtime" {
		t.Fatalf("persisted runtimes = %+v", persisted.Runtimes)
	}
	if _, err := service.RotateIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	if !manager.rotated {
		t.Fatal("identity signing key was not rotated")
	}
	if _, err := service.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if !manager.stopped {
		t.Fatal("provider runtime was not stopped")
	}
}

func TestServiceRollsBackRunWhenProviderStartFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := state.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := &fakeProviderManager{startErr: errors.New("runtime unavailable")}
	service := New(store, manager)
	if _, err := service.Up(ctx, UpOptions{ScenarioPath: writeAppScenario(t)}); err == nil {
		t.Fatal("Up() succeeded with provider start failure")
	}
	if _, err := service.Status(ctx); !errors.Is(err, state.ErrNoActiveRun) {
		t.Fatalf("Status() error = %v, want ErrNoActiveRun", err)
	}
}

func writeAppScenario(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	data := `apiVersion: cloudailab.dev/v1alpha1
kind: Scenario
metadata:
  name: app-test
  version: 0.1.0
  title: App Test
spec:
  seed: 1
  briefing: Test service lifecycle behavior.
  objectives:
    - id: inspect
      description: Inspect the path.
  tenants:
    - id: tenant-a
      name: Tenant A
      providers: [local]
  principals:
    - id: principal:a
      tenant: tenant-a
      type: human
      displayName: Principal A
  resources:
    - id: resource:a
      tenant: tenant-a
      type: test_resource
      displayName: Resource A
      classification: internal
  relationships:
    - id: edge:a
      from: principal:a
      to: resource:a
      type: can_access
  verification:
    invariants:
      - id: path-visible
        type: path_exists
        from: principal:a
        to: resource:a
        severity: low
        description: The path exists.
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
