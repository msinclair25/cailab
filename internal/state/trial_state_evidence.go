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
	ErrDuplicateTrialStateEvidence = errors.New("trial state evidence phase already exists")
	ErrTrialStateEvidenceIntegrity = errors.New("trial state evidence integrity check failed")
)

type trialStateQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func ensureTrialStateOpen(ctx context.Context, queryer trialStateQueryer, runID, trialID string) error {
	var closed int
	err := queryer.QueryRowContext(ctx, `
SELECT 1 FROM agent_trial_state_evidence
WHERE run_id = ? AND trial_id = ? AND phase = 'after'`, runID, trialID).Scan(&closed)
	if err == nil {
		return fmt.Errorf("%w: after evidence already closed the trial trace", ErrTrialStateEvidenceIntegrity)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("inspect trial state closure: %w", err)
	}
	return nil
}

func (s *Store) AppendTrialStateEvidence(ctx context.Context, evidence agent.TrialStateEvidence) error {
	if err := agent.ValidateTrialStateEvidence(evidence); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin trial state evidence append: %w", err)
	}
	defer tx.Rollback()

	var startJSON []byte
	var startHash string
	if err := tx.QueryRowContext(ctx, `
SELECT start_json, start_hash FROM agent_runs
WHERE run_id = ? AND trial_id = ? AND terminal_json IS NULL`, evidence.RunID, evidence.TrialID).Scan(&startJSON, &startHash); errors.Is(err, sql.ErrNoRows) {
		return ErrNoActiveAgentRun
	} else if err != nil {
		return fmt.Errorf("read active agent run for state evidence: %w", err)
	}
	run, err := decodeStoredAgentRun(startJSON, startHash)
	if err != nil {
		return err
	}
	if run.State == nil || evidence.Verification.Scenario != run.Scenario.Name ||
		evidence.Verification.ScenarioVersion != run.Scenario.Version || evidence.Verification.Digest != run.Scenario.Digest {
		return fmt.Errorf("%w: evidence does not match agent run configuration", ErrTrialStateEvidenceIntegrity)
	}
	if evidence.Phase == "before" {
		if evidence.FixtureRestored != run.State.Restore {
			return fmt.Errorf("%w: fixture restoration flag does not match agent run", ErrTrialStateEvidenceIntegrity)
		}
		if run.State.Restore && evidence.SnapshotDigest != run.State.BaselineDigest {
			return fmt.Errorf("%w: restored fixture digest does not match baseline", ErrTrialStateEvidenceIntegrity)
		}
		var decisions int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_decision_events WHERE run_id = ? AND trial_id = ?`, evidence.RunID, evidence.TrialID).Scan(&decisions); err != nil {
			return fmt.Errorf("inspect decisions before state evidence: %w", err)
		}
		if decisions != 0 {
			return fmt.Errorf("%w: before evidence must precede decisions", ErrTrialStateEvidenceIntegrity)
		}
	} else {
		var before int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM agent_trial_state_evidence WHERE run_id = ? AND trial_id = ? AND phase = 'before'`, evidence.RunID, evidence.TrialID).Scan(&before); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: after evidence requires before evidence", ErrTrialStateEvidenceIntegrity)
		} else if err != nil {
			return fmt.Errorf("read before state evidence: %w", err)
		}
	}
	var duplicate int
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_trial_state_evidence
WHERE run_id = ? AND trial_id = ? AND phase = ?`, evidence.RunID, evidence.TrialID, evidence.Phase).Scan(&duplicate); err == nil {
		return ErrDuplicateTrialStateEvidence
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check duplicate trial state evidence: %w", err)
	}
	canonical, hash, err := canonicalTrialStateEvidence(evidence)
	if err != nil {
		return err
	}
	if len(canonical) > agent.MaxFrameBytes {
		return fmt.Errorf("trial state evidence exceeds %d bytes", agent.MaxFrameBytes)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_trial_state_evidence(run_id, trial_id, phase, evidence_json, record_hash)
VALUES(?, ?, ?, ?, ?)`, evidence.RunID, evidence.TrialID, evidence.Phase, canonical, hash); err != nil {
		return fmt.Errorf("insert %s trial state evidence: %w", evidence.Phase, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s trial state evidence: %w", evidence.Phase, err)
	}
	return nil
}

func (s *Store) TrialStateEvidence(ctx context.Context, runID, trialID string) ([]agent.TrialStateEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT phase, evidence_json, record_hash FROM agent_trial_state_evidence
WHERE run_id = ? AND trial_id = ? ORDER BY CASE phase WHEN 'before' THEN 1 ELSE 2 END`, runID, trialID)
	if err != nil {
		return nil, fmt.Errorf("query trial state evidence: %w", err)
	}
	defer rows.Close()
	result := make([]agent.TrialStateEvidence, 0, 2)
	for rows.Next() {
		var phase, storedHash string
		var encoded []byte
		if err := rows.Scan(&phase, &encoded, &storedHash); err != nil {
			return nil, fmt.Errorf("scan trial state evidence: %w", err)
		}
		evidence, err := decodeStoredTrialStateEvidence(encoded, storedHash)
		if err != nil {
			return nil, err
		}
		if evidence.RunID != runID || evidence.TrialID != trialID || evidence.Phase != phase {
			return nil, fmt.Errorf("%w: stored evidence identity is inconsistent", ErrTrialStateEvidenceIntegrity)
		}
		result = append(result, evidence)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trial state evidence: %w", err)
	}
	return result, nil
}

func canonicalTrialStateEvidence(evidence agent.TrialStateEvidence) ([]byte, string, error) {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return nil, "", fmt.Errorf("encode trial state evidence: %w", err)
	}
	canonical, err := agent.CanonicalJSON(encoded)
	if err != nil {
		return nil, "", fmt.Errorf("canonicalize trial state evidence: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

func decodeStoredTrialStateEvidence(encoded []byte, storedHash string) (agent.TrialStateEvidence, error) {
	digest := sha256.Sum256(encoded)
	if hex.EncodeToString(digest[:]) != storedHash {
		return agent.TrialStateEvidence{}, fmt.Errorf("%w: stored record hash mismatch", ErrTrialStateEvidenceIntegrity)
	}
	evidence, err := agent.DecodeTrialStateEvidence(encoded)
	if err != nil {
		return agent.TrialStateEvidence{}, fmt.Errorf("%w: %v", ErrTrialStateEvidenceIntegrity, err)
	}
	canonical, _, err := canonicalTrialStateEvidence(evidence)
	if err != nil || !reflect.DeepEqual(canonical, encoded) {
		return agent.TrialStateEvidence{}, fmt.Errorf("%w: stored record is not canonical", ErrTrialStateEvidenceIntegrity)
	}
	return evidence, nil
}
