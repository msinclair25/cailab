package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/scenario"
)

func TestRunLifecyclePersists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	compiled := scenario.Compiled{
		ScenarioName: "test", ScenarioVersion: "0.1.0", Seed: 42,
		Digest: strings.Repeat("a", 64),
	}
	run, err := store.CreateRun(ctx, compiled)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRun(ctx, compiled); !errors.Is(err, ErrActiveRun) {
		t.Fatalf("second CreateRun() error = %v, want ErrActiveRun", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	active, err := store.ActiveRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != run.ID || active.Compiled.Digest != compiled.Digest {
		t.Fatalf("active run = %+v, want %q", active, run.ID)
	}
	stopped, err := store.StopActiveRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != "stopped" {
		t.Fatalf("status = %q, want stopped", stopped.Status)
	}
	if _, err := store.ActiveRun(ctx); !errors.Is(err, ErrNoActiveRun) {
		t.Fatalf("ActiveRun() after stop error = %v, want ErrNoActiveRun", err)
	}
	next, err := store.CreateRun(ctx, compiled)
	if err != nil {
		t.Fatalf("CreateRun() after stop error = %v", err)
	}
	if next.ID == run.ID {
		t.Fatalf("CreateRun() reused run ID %q", next.ID)
	}
}
