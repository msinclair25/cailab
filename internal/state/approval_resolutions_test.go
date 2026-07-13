package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/agent"
)

func TestApprovalResolutionLinksToRequirementAndPersists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, runID := eventTestStore(t, ctx, path)
	decision := appendApprovalDecision(t, ctx, store, runID, "call:approval")
	draft := approvalResolutionDraft(decision, true, agent.Decision{Effect: "allow", ReasonCode: "rule:approval", PolicyVersion: "0.1.0"})
	event, err := store.AppendApprovalResolutionEvent(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if !event.Approved || event.DecisionEventID != decision.EventID {
		t.Fatalf("approval = %+v", event)
	}
	if _, err := store.AppendApprovalResolutionEvent(ctx, draft); !errors.Is(err, ErrDuplicateApprovalResolution) {
		t.Fatalf("duplicate error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	events, err := store.ApprovalResolutionEvents(ctx, runID, "trial:1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("events = %+v", events)
	}
}

func TestRejectedApprovalPersistsDenyAndCannotAuthorizeOutcome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	decision := appendApprovalDecision(t, ctx, store, runID, "call:rejected")
	approval, err := store.AppendApprovalResolutionEvent(ctx, approvalResolutionDraft(decision, false, agent.Decision{
		Effect: "deny", ReasonCode: "approval:rejected", PolicyVersion: "0.1.0",
	}))
	if err != nil {
		t.Fatal(err)
	}
	draft := toolOutcomeDraft(decision, agent.Outcome{Status: "succeeded"}, strings.Repeat("f", 64))
	draft.ApprovalEventID = approval.EventID
	if _, err := store.AppendToolOutcomeEvent(ctx, draft); !errors.Is(err, ErrDecisionEventIntegrity) {
		t.Fatalf("outcome error = %v", err)
	}
}

func TestApprovedResolutionAuthorizesLinkedToolOutcome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	decision := appendApprovalDecision(t, ctx, store, runID, "call:approved")
	approval, err := store.AppendApprovalResolutionEvent(ctx, approvalResolutionDraft(decision, true, agent.Decision{
		Effect: "allow", ReasonCode: "rule:approval", PolicyVersion: "0.1.0",
	}))
	if err != nil {
		t.Fatal(err)
	}
	draft := toolOutcomeDraft(decision, agent.Outcome{Status: "succeeded"}, strings.Repeat("f", 64))
	draft.ApprovalEventID = approval.EventID
	outcome, err := store.AppendToolOutcomeEvent(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ApprovalEventID != approval.EventID {
		t.Fatalf("outcome = %+v", outcome)
	}
	events, err := store.ToolOutcomeEvents(ctx, runID, "trial:1")
	if err != nil || len(events) != 1 || events[0].ApprovalEventID != approval.EventID {
		t.Fatalf("events = %+v, error = %v", events, err)
	}
}

func TestApprovalIntegrityFailurePreventsOutcome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	decision := appendApprovalDecision(t, ctx, store, runID, "call:tampered")
	approval, err := store.AppendApprovalResolutionEvent(ctx, approvalResolutionDraft(decision, true, agent.Decision{
		Effect: "allow", ReasonCode: "rule:approval", PolicyVersion: "0.1.0",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE agent_approval_resolutions SET record_hash = ? WHERE event_id = ?`, strings.Repeat("0", 64), approval.EventID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApprovalResolutionEvents(ctx, runID, "trial:1"); !errors.Is(err, ErrDecisionEventIntegrity) {
		t.Fatalf("read integrity error = %v", err)
	}
	draft := toolOutcomeDraft(decision, agent.Outcome{Status: "succeeded"}, strings.Repeat("f", 64))
	draft.ApprovalEventID = approval.EventID
	if _, err := store.AppendToolOutcomeEvent(ctx, draft); !errors.Is(err, ErrDecisionEventIntegrity) {
		t.Fatalf("append integrity error = %v", err)
	}
}

func appendApprovalDecision(t *testing.T, ctx context.Context, store *Store, runID, correlationID string) agent.DecisionEvent {
	t.Helper()
	draft := decisionDraft(runID, correlationID)
	draft.Decision = agent.Decision{
		Effect: "require_approval", ReasonCode: "rule:approval", PolicyVersion: "0.1.0", ApprovalID: "approval:" + strings.TrimPrefix(correlationID, "call:"),
	}
	decision, err := store.AppendDecisionEvent(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	return decision
}

func approvalResolutionDraft(decision agent.DecisionEvent, approved bool, resulting agent.Decision) agent.ApprovalResolutionEventDraft {
	return agent.ApprovalResolutionEventDraft{
		OccurredAt: decision.OccurredAt.Add(1), RunID: decision.RunID, TrialID: decision.TrialID,
		CorrelationID: decision.CorrelationID, ApprovalID: decision.Decision.ApprovalID,
		DecisionEventID: decision.EventID, ResolvedBy: "user:reviewer", Approved: approved,
		Tool: decision.Tool, Action: decision.Action, Resource: decision.Resource,
		Decision: resulting, InputHash: decision.InputHash,
	}
}
