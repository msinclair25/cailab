package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/msinclair25/cailab/internal/agent"
)

var ErrDuplicateToolOutcome = errors.New("tool outcome correlation already exists")

func (s *Store) AppendToolOutcomeEvent(ctx context.Context, draft agent.ToolOutcomeEventDraft) (agent.ToolOutcomeEvent, error) {
	if _, err := agent.BuildToolOutcomeEvent(draft, "outcome:pending"); err != nil {
		return agent.ToolOutcomeEvent{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("begin tool outcome append: %w", err)
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM runs WHERE id = ? AND status = 'active'`, draft.RunID).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		return agent.ToolOutcomeEvent{}, ErrNoActiveRun
	} else if err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("verify tool outcome run: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_runs
WHERE run_id = ? AND trial_id = ? AND terminal_json IS NULL`, draft.RunID, draft.TrialID).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		return agent.ToolOutcomeEvent{}, ErrNoActiveAgentRun
	} else if err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("verify active agent trial for tool outcome: %w", err)
	}
	var nextSequence int64
	var headHash string
	if err := tx.QueryRowContext(ctx, `
SELECT next_sequence, last_hash FROM agent_event_heads WHERE run_id = ? AND trial_id = ?`,
		draft.RunID, draft.TrialID).Scan(&nextSequence, &headHash); err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("%w: read decision event head: %v", ErrDecisionEventIntegrity, err)
	}
	if err := verifyDecisionEventChain(ctx, tx, draft.RunID, draft.TrialID, nextSequence, headHash); err != nil {
		return agent.ToolOutcomeEvent{}, err
	}
	var decisionJSON []byte
	var decisionHash string
	err = tx.QueryRowContext(ctx, `
SELECT event_json, record_hash FROM agent_decision_events
WHERE run_id = ? AND trial_id = ? AND correlation_id = ? AND event_id = ?`,
		draft.RunID, draft.TrialID, draft.CorrelationID, draft.DecisionEventID).Scan(&decisionJSON, &decisionHash)
	if errors.Is(err, sql.ErrNoRows) {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("%w: matching decision event is missing", ErrDecisionEventIntegrity)
	}
	if err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("read tool outcome decision: %w", err)
	}
	decision, err := agent.DecodeDecisionEvent(decisionJSON)
	if err != nil || decision.Tool != draft.Tool || decision.Outcome.Status != "not_executed" ||
		(decision.Decision.Effect != "allow" && decision.Decision.Effect != "redact") {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("%w: decision does not authorize an outcome", ErrDecisionEventIntegrity)
	}
	var duplicate int
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_tool_outcomes WHERE run_id = ? AND trial_id = ? AND correlation_id = ?`,
		draft.RunID, draft.TrialID, draft.CorrelationID).Scan(&duplicate); err == nil {
		return agent.ToolOutcomeEvent{}, ErrDuplicateToolOutcome
	} else if !errors.Is(err, sql.ErrNoRows) {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("check duplicate tool outcome: %w", err)
	}
	eventID := persistedEventID(draft.RunID, draft.TrialID, decision.Sequence, "outcome:"+draft.CorrelationID)
	event, err := agent.BuildToolOutcomeEvent(draft, eventID)
	if err != nil {
		return agent.ToolOutcomeEvent{}, err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("encode tool outcome: %w", err)
	}
	canonical, err := agent.CanonicalJSON(encoded)
	if err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("canonicalize tool outcome: %w", err)
	}
	recordHash := decisionRecordHash(decisionHash, canonical)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_tool_outcomes(
    run_id, trial_id, correlation_id, event_id, decision_event_id, event_json, decision_record_hash, record_hash
) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		event.RunID, event.TrialID, event.CorrelationID, event.EventID, event.DecisionEventID,
		canonical, decisionHash, recordHash); err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("insert tool outcome %q: %w", event.EventID, err)
	}
	if err := tx.Commit(); err != nil {
		return agent.ToolOutcomeEvent{}, fmt.Errorf("commit tool outcome %q: %w", event.EventID, err)
	}
	return event, nil
}

func (s *Store) ToolOutcomeEvents(ctx context.Context, runID, trialID string) ([]agent.ToolOutcomeEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT o.event_json, o.decision_record_hash, o.record_hash, d.record_hash
FROM agent_tool_outcomes o
LEFT JOIN agent_decision_events d
  ON d.run_id = o.run_id AND d.trial_id = o.trial_id AND d.correlation_id = o.correlation_id
WHERE o.run_id = ? AND o.trial_id = ? ORDER BY d.sequence`, runID, trialID)
	if err != nil {
		return nil, fmt.Errorf("query tool outcomes: %w", err)
	}
	defer rows.Close()
	events := make([]agent.ToolOutcomeEvent, 0)
	for rows.Next() {
		var encoded []byte
		var storedDecisionHash, storedRecordHash string
		var currentDecisionHash sql.NullString
		if err := rows.Scan(&encoded, &storedDecisionHash, &storedRecordHash, &currentDecisionHash); err != nil {
			return nil, fmt.Errorf("scan tool outcome: %w", err)
		}
		event, err := agent.DecodeToolOutcomeEvent(encoded)
		if err != nil {
			return nil, fmt.Errorf("%w: decode tool outcome: %v", ErrDecisionEventIntegrity, err)
		}
		canonical, err := agent.CanonicalJSON(encoded)
		if err != nil || !currentDecisionHash.Valid || storedDecisionHash != currentDecisionHash.String || storedRecordHash != decisionRecordHash(currentDecisionHash.String, canonical) {
			return nil, fmt.Errorf("%w: inconsistent tool outcome %q", ErrDecisionEventIntegrity, event.EventID)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tool outcomes: %w", err)
	}
	return events, nil
}
