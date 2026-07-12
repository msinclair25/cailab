package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/agent"
)

func TestToolOutcomeLinksToExecutableDecisionAndPersists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, runID := eventTestStore(t, ctx, path)
	decision, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1"))
	if err != nil {
		t.Fatal(err)
	}
	draft := toolOutcomeDraft(decision, agent.Outcome{Status: "succeeded"}, strings.Repeat("d", 64))
	outcome, err := store.AppendToolOutcomeEvent(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.DecisionEventID != decision.EventID || outcome.OutputHash == "" {
		t.Fatalf("outcome = %+v", outcome)
	}
	if _, err := store.AppendToolOutcomeEvent(ctx, draft); !errors.Is(err, ErrDuplicateToolOutcome) {
		t.Fatalf("duplicate outcome error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	outcomes, err := store.ToolOutcomeEvents(ctx, runID, "trial:1")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Outcome.Status != "succeeded" {
		t.Fatalf("persisted outcomes = %+v", outcomes)
	}
}

func TestToolOutcomeRejectsNonExecutableDecision(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	draft := decisionDraft(runID, "call:deny")
	draft.Decision = agent.Decision{Effect: "deny", ReasonCode: "rule:deny", PolicyVersion: "0.1.0"}
	decision, err := store.AppendDecisionEvent(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendToolOutcomeEvent(ctx, toolOutcomeDraft(decision, agent.Outcome{Status: "failed", ErrorCode: "executor:blocked"}, "")); !errors.Is(err, ErrDecisionEventIntegrity) {
		t.Fatalf("outcome error = %v", err)
	}
}

func TestToolOutcomeIntegrityDetectsMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	decision, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendToolOutcomeEvent(ctx, toolOutcomeDraft(decision, agent.Outcome{Status: "succeeded"}, strings.Repeat("d", 64))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE agent_tool_outcomes SET event_json = '{"bad":true}' WHERE run_id = ?`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ToolOutcomeEvents(ctx, runID, "trial:1"); !errors.Is(err, ErrDecisionEventIntegrity) {
		t.Fatalf("integrity error = %v", err)
	}
}

func toolOutcomeDraft(decision agent.DecisionEvent, outcome agent.Outcome, outputHash string) agent.ToolOutcomeEventDraft {
	return agent.ToolOutcomeEventDraft{
		OccurredAt: decision.OccurredAt.Add(1), RunID: decision.RunID, TrialID: decision.TrialID,
		CorrelationID: decision.CorrelationID, DecisionEventID: decision.EventID,
		Tool: decision.Tool, Outcome: outcome, OutputHash: outputHash,
	}
}
