package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/provider"
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
	runtimes := []provider.Instance{{Provider: "aws", Engine: "floci", Name: "runtime", Endpoint: "http://127.0.0.1:4566", Status: "ready"}}
	if err := store.SetRuntimes(ctx, run.ID, runtimes); err != nil {
		t.Fatal(err)
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
	if len(active.Runtimes) != 1 || active.Runtimes[0].Endpoint != runtimes[0].Endpoint {
		t.Fatalf("active runtimes = %+v", active.Runtimes)
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

func TestMigratesVersionOneDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT INTO schema_migrations(version, applied_at) VALUES(1, '2026-07-11T00:00:00Z');
CREATE TABLE runs (
    id TEXT PRIMARY KEY,
    scenario_name TEXT NOT NULL,
    scenario_version TEXT NOT NULL,
    seed INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'stopped')),
    compiled_json BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX one_active_run ON runs(status) WHERE status = 'active';`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	compiledData, err := json.Marshal(scenario.Compiled{
		SchemaVersion: scenario.APIVersion, ScenarioName: "legacy", ScenarioVersion: "0.1.0", Digest: strings.Repeat("b", 64),
	})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO runs(id, scenario_name, scenario_version, seed, status, compiled_json, created_at, updated_at)
VALUES(?, 'legacy', '0.1.0', 1, 'active', ?, '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z')`, "legacy-run", compiledData); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}
	var defaultValue string
	if err := store.db.QueryRowContext(ctx, `SELECT dflt_value FROM pragma_table_info('runs') WHERE name = 'runtimes_json'`).Scan(&defaultValue); err != nil {
		t.Fatal(err)
	}
	if defaultValue != "'[]'" {
		t.Fatalf("runtimes_json default = %q", defaultValue)
	}
	var eventColumns int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('agent_decision_events')`).Scan(&eventColumns); err != nil {
		t.Fatal(err)
	}
	if eventColumns != 8 {
		t.Fatalf("agent_decision_events columns = %d, want 8", eventColumns)
	}
	var outcomeColumns int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('agent_tool_outcomes')`).Scan(&outcomeColumns); err != nil {
		t.Fatal(err)
	}
	if outcomeColumns != 10 {
		t.Fatalf("agent_tool_outcomes columns = %d, want 10", outcomeColumns)
	}
	var approvalColumns int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('agent_approval_resolutions')`).Scan(&approvalColumns); err != nil {
		t.Fatal(err)
	}
	if approvalColumns != 9 {
		t.Fatalf("agent_approval_resolutions columns = %d, want 9", approvalColumns)
	}
	var agentRunColumns int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('agent_runs')`).Scan(&agentRunColumns); err != nil {
		t.Fatal(err)
	}
	if agentRunColumns != 6 {
		t.Fatalf("agent_runs columns = %d, want 6", agentRunColumns)
	}
	active, err := store.ActiveRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != "legacy-run" || len(active.Runtimes) != 0 {
		t.Fatalf("migrated active run = %+v", active)
	}
}
