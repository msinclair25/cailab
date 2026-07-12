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

	"github.com/msinclair25/cailab/internal/scenario"
	_ "modernc.org/sqlite"
)

var (
	ErrNoActiveRun = errors.New("no active run")
	ErrActiveRun   = errors.New("an active run already exists")
)

const currentSchemaVersion = 1

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
SELECT id, scenario_name, scenario_version, seed, status, compiled_json, created_at, updated_at
FROM runs WHERE status = 'active' LIMIT 1`)
}

func (s *Store) queryRun(ctx context.Context, query string, args ...any) (Run, error) {
	var run Run
	var data []byte
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&run.ID, &run.ScenarioName, &run.ScenarioVersion, &run.Seed, &run.Status,
		&data, &createdAt, &updatedAt,
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
