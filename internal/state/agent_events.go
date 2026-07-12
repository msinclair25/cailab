package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/msinclair25/cailab/internal/agent"
)

var (
	ErrDuplicateDecisionEvent = errors.New("decision event correlation already exists")
	ErrDecisionEventIntegrity = errors.New("decision event integrity check failed")
)

func (s *Store) AppendDecisionEvent(ctx context.Context, draft agent.DecisionEventDraft) (agent.DecisionEvent, error) {
	if err := agent.ValidateDecisionEventDraft(draft); err != nil {
		return agent.DecisionEvent{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("begin decision event append: %w", err)
	}
	defer tx.Rollback()

	var active int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM runs WHERE id = ? AND status = 'active'`, draft.RunID).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		return agent.DecisionEvent{}, ErrNoActiveRun
	} else if err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("verify decision event run %q: %w", draft.RunID, err)
	}
	var duplicate int
	if err := tx.QueryRowContext(ctx, `
SELECT 1 FROM agent_decision_events WHERE run_id = ? AND trial_id = ? AND correlation_id = ?`,
		draft.RunID, draft.TrialID, draft.CorrelationID).Scan(&duplicate); err == nil {
		return agent.DecisionEvent{}, ErrDuplicateDecisionEvent
	} else if !errors.Is(err, sql.ErrNoRows) {
		return agent.DecisionEvent{}, fmt.Errorf("check duplicate decision event: %w", err)
	}

	var nextSequence int64
	var previousHash string
	newHead := false
	err = tx.QueryRowContext(ctx, `
SELECT next_sequence, last_hash FROM agent_event_heads WHERE run_id = ? AND trial_id = ?`,
		draft.RunID, draft.TrialID).Scan(&nextSequence, &previousHash)
	if errors.Is(err, sql.ErrNoRows) {
		nextSequence = 1
		previousHash = ""
		newHead = true
	} else if err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("read decision event head: %w", err)
	}
	if nextSequence < 1 {
		return agent.DecisionEvent{}, fmt.Errorf("%w: invalid next sequence %d", ErrDecisionEventIntegrity, nextSequence)
	}
	if err := verifyDecisionEventChain(ctx, tx, draft.RunID, draft.TrialID, nextSequence, previousHash); err != nil {
		return agent.DecisionEvent{}, err
	}

	eventID := persistedEventID(draft.RunID, draft.TrialID, uint64(nextSequence), draft.CorrelationID)
	event, err := agent.BuildDecisionEvent(draft, eventID, uint64(nextSequence))
	if err != nil {
		return agent.DecisionEvent{}, err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("encode decision event %q: %w", event.EventID, err)
	}
	canonical, err := agent.CanonicalJSON(encoded)
	if err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("canonicalize decision event %q: %w", event.EventID, err)
	}
	recordHash := decisionRecordHash(previousHash, canonical)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_decision_events(
    run_id, trial_id, sequence, event_id, correlation_id, event_json, previous_hash, record_hash
) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		event.RunID, event.TrialID, event.Sequence, event.EventID, event.CorrelationID, canonical, previousHash, recordHash); err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("insert decision event %q: %w", event.EventID, err)
	}
	if newHead {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_event_heads(run_id, trial_id, next_sequence, last_hash) VALUES(?, ?, ?, ?)`,
			event.RunID, event.TrialID, nextSequence+1, recordHash); err != nil {
			return agent.DecisionEvent{}, fmt.Errorf("create decision event head: %w", err)
		}
	} else {
		result, err := tx.ExecContext(ctx, `
UPDATE agent_event_heads SET next_sequence = ?, last_hash = ?
WHERE run_id = ? AND trial_id = ? AND next_sequence = ? AND last_hash = ?`,
			nextSequence+1, recordHash, event.RunID, event.TrialID, nextSequence, previousHash)
		if err != nil {
			return agent.DecisionEvent{}, fmt.Errorf("advance decision event head: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return agent.DecisionEvent{}, fmt.Errorf("confirm decision event head: %w", err)
		}
		if rows != 1 {
			return agent.DecisionEvent{}, fmt.Errorf("%w: decision event head changed during append", ErrDecisionEventIntegrity)
		}
	}
	if err := tx.Commit(); err != nil {
		return agent.DecisionEvent{}, fmt.Errorf("commit decision event %q: %w", event.EventID, err)
	}
	return event, nil
}

func (s *Store) DecisionEvents(ctx context.Context, runID, trialID string) ([]agent.DecisionEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT sequence, correlation_id, event_json, previous_hash, record_hash
FROM agent_decision_events WHERE run_id = ? AND trial_id = ? ORDER BY sequence`, runID, trialID)
	if err != nil {
		return nil, fmt.Errorf("query decision events: %w", err)
	}
	events := make([]agent.DecisionEvent, 0)
	expectedSequence := uint64(1)
	previousHash := ""
	for rows.Next() {
		var sequence int64
		var correlationID, storedPreviousHash, storedRecordHash string
		var encoded []byte
		if err := rows.Scan(&sequence, &correlationID, &encoded, &storedPreviousHash, &storedRecordHash); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan decision event: %w", err)
		}
		event, err := agent.DecodeDecisionEvent(encoded)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("%w: decode sequence %d: %v", ErrDecisionEventIntegrity, sequence, err)
		}
		canonical, err := agent.CanonicalJSON(encoded)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("%w: canonicalize sequence %d: %v", ErrDecisionEventIntegrity, sequence, err)
		}
		if sequence < 1 || event.Sequence != uint64(sequence) || event.Sequence != expectedSequence ||
			event.RunID != runID || event.TrialID != trialID || event.CorrelationID != correlationID ||
			storedPreviousHash != previousHash || storedRecordHash != decisionRecordHash(previousHash, canonical) {
			rows.Close()
			return nil, fmt.Errorf("%w: inconsistent sequence %d", ErrDecisionEventIntegrity, sequence)
		}
		events = append(events, event)
		expectedSequence++
		previousHash = storedRecordHash
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate decision events: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close decision event rows: %w", err)
	}

	var headSequence int64
	var headHash string
	err = s.db.QueryRowContext(ctx, `
SELECT next_sequence, last_hash FROM agent_event_heads WHERE run_id = ? AND trial_id = ?`, runID, trialID).Scan(&headSequence, &headHash)
	if errors.Is(err, sql.ErrNoRows) {
		if len(events) == 0 {
			return events, nil
		}
		return nil, fmt.Errorf("%w: missing event head", ErrDecisionEventIntegrity)
	}
	if err != nil {
		return nil, fmt.Errorf("read decision event head: %w", err)
	}
	if headSequence != int64(expectedSequence) || headHash != previousHash {
		return nil, fmt.Errorf("%w: event head does not match records", ErrDecisionEventIntegrity)
	}
	return events, nil
}

func persistedEventID(runID, trialID string, sequence uint64, correlationID string) string {
	value := fmt.Sprintf("%s\x00%s\x00%d\x00%s", runID, trialID, sequence, correlationID)
	digest := sha256.Sum256([]byte(value))
	return "event:" + hex.EncodeToString(digest[:])[:24]
}

func decisionRecordHash(previousHash string, event []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(previousHash))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(event)
	return hex.EncodeToString(hash.Sum(nil))
}

type decisionEventQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func verifyDecisionEventChain(ctx context.Context, queryer decisionEventQueryer, runID, trialID string, nextSequence int64, headHash string) error {
	rows, err := queryer.QueryContext(ctx, `
SELECT sequence, correlation_id, event_json, previous_hash, record_hash
FROM agent_decision_events WHERE run_id = ? AND trial_id = ? ORDER BY sequence`, runID, trialID)
	if err != nil {
		return fmt.Errorf("inspect decision event chain: %w", err)
	}
	defer rows.Close()
	expectedSequence := int64(1)
	previousHash := ""
	for rows.Next() {
		var sequence int64
		var correlationID, storedPreviousHash, storedRecordHash string
		var encoded []byte
		if err := rows.Scan(&sequence, &correlationID, &encoded, &storedPreviousHash, &storedRecordHash); err != nil {
			return fmt.Errorf("inspect decision event record: %w", err)
		}
		event, err := agent.DecodeDecisionEvent(encoded)
		if err != nil {
			return fmt.Errorf("%w: decode sequence %d: %v", ErrDecisionEventIntegrity, sequence, err)
		}
		canonical, err := agent.CanonicalJSON(encoded)
		if err != nil {
			return fmt.Errorf("%w: canonicalize sequence %d: %v", ErrDecisionEventIntegrity, sequence, err)
		}
		if sequence != expectedSequence || event.Sequence != uint64(sequence) || event.RunID != runID ||
			event.TrialID != trialID || event.CorrelationID != correlationID || storedPreviousHash != previousHash ||
			storedRecordHash != decisionRecordHash(previousHash, canonical) {
			return fmt.Errorf("%w: inconsistent sequence %d", ErrDecisionEventIntegrity, sequence)
		}
		expectedSequence++
		previousHash = storedRecordHash
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect decision event chain: %w", err)
	}
	if expectedSequence != nextSequence || previousHash != headHash {
		return fmt.Errorf("%w: event head does not match records", ErrDecisionEventIntegrity)
	}
	return nil
}
