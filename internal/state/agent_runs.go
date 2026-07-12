package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/msinclair25/cailab/internal/agent"
)

var (
	ErrNoActiveAgentRun    = errors.New("no active agent run")
	ErrDuplicateAgentRun   = errors.New("agent run trial already exists")
	ErrAgentRunIntegrity   = errors.New("agent run integrity check failed")
	ErrAgentRunNotTerminal = errors.New("agent run completion must be terminal")
	ErrAgentRunTransition  = errors.New("invalid agent run transition")
)

func (s *Store) BeginAgentRun(ctx context.Context, run agent.AgentRun) error {
	if err := agent.ValidateAgentRun(run); err != nil {
		return err
	}
	if run.Status != "running" || run.EndedAt != nil {
		return fmt.Errorf("%w: initial status must be running", ErrAgentRunTransition)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent run: %w", err)
	}
	defer tx.Rollback()

	var scenarioName, scenarioVersion, status string
	var seed int64
	var compiledJSON []byte
	err = tx.QueryRowContext(ctx, `
SELECT scenario_name, scenario_version, seed, status, compiled_json
FROM runs WHERE id = ?`, run.RunID).Scan(&scenarioName, &scenarioVersion, &seed, &status, &compiledJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoActiveRun
	} else if err != nil {
		return fmt.Errorf("read range run for agent trial: %w", err)
	}
	if status != "active" {
		return ErrNoActiveRun
	}
	var compiled struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(compiledJSON, &compiled); err != nil {
		return fmt.Errorf("decode range run for agent trial: %w", err)
	}
	if run.Scenario.Name != scenarioName || run.Scenario.Version != scenarioVersion || run.Scenario.Seed != seed || run.Scenario.Digest != compiled.Digest {
		return fmt.Errorf("%w: agent scenario reference does not match active range", ErrAgentRunIntegrity)
	}
	canonical, hash, err := canonicalAgentRun(run)
	if err != nil {
		return err
	}
	var duplicate int
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_runs WHERE run_id = ? AND trial_id = ?`, run.RunID, run.TrialID).Scan(&duplicate); err == nil {
		return ErrDuplicateAgentRun
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check duplicate agent run: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_runs(run_id, trial_id, start_json, start_hash)
VALUES(?, ?, ?, ?)`, run.RunID, run.TrialID, canonical, hash); err != nil {
		return fmt.Errorf("insert agent run %q: %w", run.TrialID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit agent run %q: %w", run.TrialID, err)
	}
	return nil
}

func (s *Store) CompleteAgentRun(ctx context.Context, terminal agent.AgentRun) error {
	if err := agent.ValidateAgentRun(terminal); err != nil {
		return err
	}
	if terminal.Status != "completed" && terminal.Status != "failed" && terminal.Status != "canceled" {
		return ErrAgentRunNotTerminal
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent run completion: %w", err)
	}
	defer tx.Rollback()
	var startJSON []byte
	var storedStartHash string
	var existingTerminal []byte
	var existingTerminalHash sql.NullString
	if err := tx.QueryRowContext(ctx, `
SELECT start_json, start_hash, terminal_json, terminal_hash
FROM agent_runs WHERE run_id = ? AND trial_id = ?`, terminal.RunID, terminal.TrialID).Scan(&startJSON, &storedStartHash, &existingTerminal, &existingTerminalHash); errors.Is(err, sql.ErrNoRows) {
		return ErrNoActiveAgentRun
	} else if err != nil {
		return fmt.Errorf("read agent run completion: %w", err)
	}
	if len(existingTerminal) == 0 && existingTerminalHash.Valid {
		return fmt.Errorf("%w: terminal hash exists without record", ErrAgentRunIntegrity)
	}
	if len(existingTerminal) != 0 {
		return ErrNoActiveAgentRun
	}
	start, err := decodeStoredAgentRun(startJSON, storedStartHash)
	if err != nil {
		return err
	}
	expected := start
	expected.Status = terminal.Status
	expected.EndedAt = terminal.EndedAt
	if !reflect.DeepEqual(expected, terminal) {
		return fmt.Errorf("%w: terminal record changes immutable run metadata", ErrAgentRunIntegrity)
	}
	canonical, hash, err := canonicalAgentRun(terminal)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE agent_runs SET terminal_json = ?, terminal_hash = ?
WHERE run_id = ? AND trial_id = ? AND terminal_json IS NULL`, canonical, hash, terminal.RunID, terminal.TrialID)
	if err != nil {
		return fmt.Errorf("complete agent run %q: %w", terminal.TrialID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirm agent run completion: %w", err)
	}
	if rows != 1 {
		return ErrNoActiveAgentRun
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit agent run completion: %w", err)
	}
	return nil
}

func (s *Store) AgentRun(ctx context.Context, runID, trialID string) (agent.AgentRun, error) {
	var startJSON, terminalJSON []byte
	var startHash string
	var terminalHash sql.NullString
	if err := s.db.QueryRowContext(ctx, `
SELECT start_json, start_hash, terminal_json, terminal_hash
FROM agent_runs WHERE run_id = ? AND trial_id = ?`, runID, trialID).Scan(&startJSON, &startHash, &terminalJSON, &terminalHash); errors.Is(err, sql.ErrNoRows) {
		return agent.AgentRun{}, ErrNoActiveAgentRun
	} else if err != nil {
		return agent.AgentRun{}, fmt.Errorf("read agent run: %w", err)
	}
	if len(terminalJSON) != 0 {
		if !terminalHash.Valid {
			return agent.AgentRun{}, fmt.Errorf("%w: terminal hash is missing", ErrAgentRunIntegrity)
		}
		return decodeStoredAgentRun(terminalJSON, terminalHash.String)
	}
	if terminalHash.Valid {
		return agent.AgentRun{}, fmt.Errorf("%w: terminal hash exists without record", ErrAgentRunIntegrity)
	}
	return decodeStoredAgentRun(startJSON, startHash)
}

func canonicalAgentRun(run agent.AgentRun) ([]byte, string, error) {
	encoded, err := json.Marshal(run)
	if err != nil {
		return nil, "", fmt.Errorf("encode agent run: %w", err)
	}
	canonical, err := agent.CanonicalJSON(encoded)
	if err != nil {
		return nil, "", fmt.Errorf("canonicalize agent run: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

func decodeStoredAgentRun(encoded []byte, storedHash string) (agent.AgentRun, error) {
	digest := sha256.Sum256(encoded)
	if hex.EncodeToString(digest[:]) != storedHash {
		return agent.AgentRun{}, fmt.Errorf("%w: stored record hash mismatch", ErrAgentRunIntegrity)
	}
	run, err := agent.DecodeAgentRun(encoded)
	if err != nil {
		return agent.AgentRun{}, fmt.Errorf("%w: %v", ErrAgentRunIntegrity, err)
	}
	canonical, _, err := canonicalAgentRun(run)
	if err != nil || !reflect.DeepEqual(canonical, encoded) {
		return agent.AgentRun{}, fmt.Errorf("%w: stored record is not canonical", ErrAgentRunIntegrity)
	}
	return run, nil
}
