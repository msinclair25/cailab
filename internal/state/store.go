package state

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/scenario"
	_ "modernc.org/sqlite"
)

var (
	ErrNoActiveRun = errors.New("no active run")
	ErrActiveRun   = errors.New("an active run already exists")
)

const currentSchemaVersion = 4

type Store struct {
	db *sql.DB
}

type Run struct {
	ID              string
	ScenarioName    string
	ScenarioVersion string
	Seed            int64
	Status          string
	Compiled        scenario.Compiled
	Runtimes        []provider.Instance
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("state database path must not be empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create state directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open state database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin state migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	var version int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("state schema version %d is newer than supported version %d", version, currentSchemaVersion)
	}
	if version < 1 {
		if _, err := tx.ExecContext(ctx, `
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
CREATE UNIQUE INDEX one_active_run ON runs(status) WHERE status = 'active';
`); err != nil {
			return fmt.Errorf("apply state migration 1: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record state migration 1: %w", err)
		}
		version = 1
	}
	if version < 2 {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE runs ADD COLUMN runtimes_json TEXT NOT NULL DEFAULT '[]'`); err != nil {
			return fmt.Errorf("apply state migration 2: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(2, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record state migration 2: %w", err)
		}
		version = 2
	}
	if version < 3 {
		if _, err := tx.ExecContext(ctx, `
CREATE TABLE agent_event_heads (
    run_id TEXT NOT NULL,
    trial_id TEXT NOT NULL,
    next_sequence INTEGER NOT NULL CHECK (next_sequence > 0),
    last_hash TEXT NOT NULL,
    PRIMARY KEY (run_id, trial_id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);
CREATE TABLE agent_decision_events (
    run_id TEXT NOT NULL,
    trial_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    event_id TEXT NOT NULL UNIQUE,
    correlation_id TEXT NOT NULL,
    event_json BLOB NOT NULL,
    previous_hash TEXT NOT NULL,
    record_hash TEXT NOT NULL,
    PRIMARY KEY (run_id, trial_id, sequence),
    UNIQUE (run_id, trial_id, correlation_id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);`); err != nil {
			return fmt.Errorf("apply state migration 3: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(3, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record state migration 3: %w", err)
		}
		version = 3
	}
	if version < 4 {
		if _, err := tx.ExecContext(ctx, `
CREATE TABLE agent_tool_outcomes (
    run_id TEXT NOT NULL,
    trial_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    event_id TEXT NOT NULL UNIQUE,
    decision_event_id TEXT NOT NULL UNIQUE,
    event_json BLOB NOT NULL,
    decision_record_hash TEXT NOT NULL,
    record_hash TEXT NOT NULL,
    PRIMARY KEY (run_id, trial_id, correlation_id),
    FOREIGN KEY (run_id) REFERENCES runs(id)
);`); err != nil {
			return fmt.Errorf("apply state migration 4: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(4, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record state migration 4: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit state migration: %w", err)
	}
	return nil
}

func (s *Store) CreateRun(ctx context.Context, compiled scenario.Compiled) (Run, error) {
	if _, err := s.ActiveRun(ctx); err == nil {
		return Run{}, ErrActiveRun
	} else if !errors.Is(err, ErrNoActiveRun) {
		return Run{}, err
	}

	data, err := json.Marshal(compiled)
	if err != nil {
		return Run{}, fmt.Errorf("marshal compiled scenario: %w", err)
	}
	now := time.Now().UTC()
	runID, err := newRunID(compiled.ScenarioName)
	if err != nil {
		return Run{}, err
	}
	run := Run{
		ID: runID, ScenarioName: compiled.ScenarioName,
		ScenarioVersion: compiled.ScenarioVersion, Seed: compiled.Seed,
		Status: "active", Compiled: compiled, CreatedAt: now, UpdatedAt: now,
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO runs(id, scenario_name, scenario_version, seed, status, compiled_json, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ScenarioName, run.ScenarioVersion, run.Seed, run.Status, data,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Run{}, fmt.Errorf("create run %q: %w", run.ID, err)
	}
	return run, nil
}

func (s *Store) ActiveRun(ctx context.Context) (Run, error) {
	return s.queryRun(ctx, `
SELECT id, scenario_name, scenario_version, seed, status, compiled_json, runtimes_json, created_at, updated_at
FROM runs WHERE status = 'active' LIMIT 1`)
}

func (s *Store) queryRun(ctx context.Context, query string, args ...any) (Run, error) {
	var run Run
	var data, runtimeData []byte
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&run.ID, &run.ScenarioName, &run.ScenarioVersion, &run.Seed, &run.Status,
		&data, &runtimeData, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNoActiveRun
	}
	if err != nil {
		return Run{}, fmt.Errorf("query run: %w", err)
	}
	if err := json.Unmarshal(data, &run.Compiled); err != nil {
		return Run{}, fmt.Errorf("decode run %q state: %w", run.ID, err)
	}
	if err := json.Unmarshal(runtimeData, &run.Runtimes); err != nil {
		return Run{}, fmt.Errorf("decode run %q runtimes: %w", run.ID, err)
	}
	run.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Run{}, fmt.Errorf("parse run %q creation time: %w", run.ID, err)
	}
	run.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Run{}, fmt.Errorf("parse run %q update time: %w", run.ID, err)
	}
	return run, nil
}

func (s *Store) SetRuntimes(ctx context.Context, runID string, runtimes []provider.Instance) error {
	data, err := json.Marshal(runtimes)
	if err != nil {
		return fmt.Errorf("encode run %q runtimes: %w", runID, err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx,
		`UPDATE runs SET runtimes_json = ?, updated_at = ? WHERE id = ? AND status = 'active'`, data, now, runID)
	if err != nil {
		return fmt.Errorf("save run %q runtimes: %w", runID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirm run %q runtimes: %w", runID, err)
	}
	if rows != 1 {
		return ErrNoActiveRun
	}
	return nil
}

func newRunID(scenarioName string) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return scenarioName + "-" + hex.EncodeToString(random), nil
}

func (s *Store) StopActiveRun(ctx context.Context) (Run, error) {
	run, err := s.ActiveRun(ctx)
	if err != nil {
		return Run{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE runs SET status = 'stopped', updated_at = ? WHERE id = ? AND status = 'active'`,
		now.Format(time.RFC3339Nano), run.ID)
	if err != nil {
		return Run{}, fmt.Errorf("stop run %q: %w", run.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Run{}, fmt.Errorf("confirm stopped run %q: %w", run.ID, err)
	}
	if rows != 1 {
		return Run{}, ErrNoActiveRun
	}
	run.Status = "stopped"
	run.UpdatedAt = now
	return run, nil
}

func (s *Store) ResetActiveRun(ctx context.Context) (Run, error) {
	run, err := s.ActiveRun(ctx)
	if err != nil {
		return Run{}, err
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE runs SET updated_at = ? WHERE id = ?`, now.Format(time.RFC3339Nano), run.ID); err != nil {
		return Run{}, fmt.Errorf("reset run %q: %w", run.ID, err)
	}
	run.UpdatedAt = now
	return run, nil
}
