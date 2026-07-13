package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/msinclair25/cailab/internal/agent"
)

var ErrDuplicateApprovalResolution = errors.New("approval resolution already exists")

func (s *Store) AppendApprovalResolutionEvent(ctx context.Context, draft agent.ApprovalResolutionEventDraft) (agent.ApprovalResolutionEvent, error) {
	if _, err := agent.BuildApprovalResolutionEvent(draft, "approval-event:pending"); err != nil {
		return agent.ApprovalResolutionEvent{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("begin approval resolution append: %w", err)
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM runs WHERE id = ? AND status = 'active'`, draft.RunID).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		return agent.ApprovalResolutionEvent{}, ErrNoActiveRun
	} else if err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("verify approval range run: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_runs
WHERE run_id = ? AND trial_id = ? AND terminal_json IS NULL`, draft.RunID, draft.TrialID).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		return agent.ApprovalResolutionEvent{}, ErrNoActiveAgentRun
	} else if err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("verify approval agent trial: %w", err)
	}
	if err := ensureTrialStateOpen(ctx, tx, draft.RunID, draft.TrialID); err != nil {
		return agent.ApprovalResolutionEvent{}, err
	}
	var nextSequence int64
	var headHash string
	if err := tx.QueryRowContext(ctx, `
SELECT next_sequence, last_hash FROM agent_event_heads WHERE run_id = ? AND trial_id = ?`,
		draft.RunID, draft.TrialID).Scan(&nextSequence, &headHash); err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("%w: read decision event head: %v", ErrDecisionEventIntegrity, err)
	}
	if err := verifyDecisionEventChain(ctx, tx, draft.RunID, draft.TrialID, nextSequence, headHash); err != nil {
		return agent.ApprovalResolutionEvent{}, err
	}
	var decisionJSON []byte
	var decisionHash string
	err = tx.QueryRowContext(ctx, `
SELECT event_json, record_hash FROM agent_decision_events
WHERE run_id = ? AND trial_id = ? AND correlation_id = ? AND event_id = ?`,
		draft.RunID, draft.TrialID, draft.CorrelationID, draft.DecisionEventID).Scan(&decisionJSON, &decisionHash)
	if errors.Is(err, sql.ErrNoRows) {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("%w: matching approval decision is missing", ErrDecisionEventIntegrity)
	}
	if err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("read approval decision: %w", err)
	}
	decision, err := agent.DecodeDecisionEvent(decisionJSON)
	if err != nil || decision.Decision.Effect != "require_approval" || decision.Outcome.Status != "not_executed" ||
		decision.Decision.ApprovalID != draft.ApprovalID || decision.Tool != draft.Tool ||
		decision.Action != draft.Action || decision.Resource != draft.Resource || decision.InputHash != draft.InputHash ||
		decision.Decision.PolicyVersion != draft.Decision.PolicyVersion {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("%w: decision does not match approval resolution", ErrDecisionEventIntegrity)
	}
	var duplicate int
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_approval_resolutions
WHERE run_id = ? AND trial_id = ? AND correlation_id = ?`, draft.RunID, draft.TrialID, draft.CorrelationID).Scan(&duplicate); err == nil {
		return agent.ApprovalResolutionEvent{}, ErrDuplicateApprovalResolution
	} else if !errors.Is(err, sql.ErrNoRows) {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("check duplicate approval resolution: %w", err)
	}
	eventID := persistedEventID(draft.RunID, draft.TrialID, decision.Sequence, "approval:"+draft.ApprovalID)
	event, err := agent.BuildApprovalResolutionEvent(draft, eventID)
	if err != nil {
		return agent.ApprovalResolutionEvent{}, err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("encode approval resolution: %w", err)
	}
	canonical, err := agent.CanonicalJSON(encoded)
	if err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("canonicalize approval resolution: %w", err)
	}
	recordHash := decisionRecordHash(decisionHash, canonical)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_approval_resolutions(
    run_id, trial_id, correlation_id, approval_id, event_id, decision_event_id,
    event_json, decision_record_hash, record_hash
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.RunID, event.TrialID, event.CorrelationID, event.ApprovalID, event.EventID,
		event.DecisionEventID, canonical, decisionHash, recordHash); err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("insert approval resolution %q: %w", event.EventID, err)
	}
	if err := tx.Commit(); err != nil {
		return agent.ApprovalResolutionEvent{}, fmt.Errorf("commit approval resolution %q: %w", event.EventID, err)
	}
	return event, nil
}

func (s *Store) ApprovalResolutionEvents(ctx context.Context, runID, trialID string) ([]agent.ApprovalResolutionEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT a.event_json, a.decision_record_hash, a.record_hash, d.record_hash
FROM agent_approval_resolutions a
LEFT JOIN agent_decision_events d
  ON d.run_id = a.run_id AND d.trial_id = a.trial_id AND d.event_id = a.decision_event_id
WHERE a.run_id = ? AND a.trial_id = ? ORDER BY d.sequence`, runID, trialID)
	if err != nil {
		return nil, fmt.Errorf("query approval resolutions: %w", err)
	}
	defer rows.Close()
	events := make([]agent.ApprovalResolutionEvent, 0)
	for rows.Next() {
		var encoded []byte
		var storedDecisionHash, storedRecordHash string
		var currentDecisionHash sql.NullString
		if err := rows.Scan(&encoded, &storedDecisionHash, &storedRecordHash, &currentDecisionHash); err != nil {
			return nil, fmt.Errorf("scan approval resolution: %w", err)
		}
		event, err := agent.DecodeApprovalResolutionEvent(encoded)
		if err != nil {
			return nil, fmt.Errorf("%w: decode approval resolution: %v", ErrDecisionEventIntegrity, err)
		}
		canonical, err := agent.CanonicalJSON(encoded)
		if err != nil || !currentDecisionHash.Valid || storedDecisionHash != currentDecisionHash.String ||
			storedRecordHash != decisionRecordHash(currentDecisionHash.String, canonical) {
			return nil, fmt.Errorf("%w: inconsistent approval resolution %q", ErrDecisionEventIntegrity, event.EventID)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approval resolutions: %w", err)
	}
	return events, nil
}
