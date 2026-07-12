package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/scenario"
)

func TestDecisionEventsAppendWithMonotonicSequenceAndPersist(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, runID := eventTestStore(t, ctx, path)
	first, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:2"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Sequence != 1 || second.Sequence != 2 || first.EventID == second.EventID {
		t.Fatalf("events = %+v, %+v", first, second)
	}
	if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:2")); !errors.Is(err, ErrDuplicateDecisionEvent) {
		t.Fatalf("duplicate append error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	events, err := store.DecisionEvents(ctx, runID, "trial:1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].CorrelationID != "call:1" || events[1].CorrelationID != "call:2" {
		t.Fatalf("persisted events = %+v", events)
	}
}

func TestDecisionEventAppendRequiresActiveRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	if _, err := store.StopActiveRun(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1")); !errors.Is(err, ErrNoActiveRun) {
		t.Fatalf("append error = %v, want no active run", err)
	}
}

func TestDecisionEventIntegrityDetectsStoredMutationAndDeletion(t *testing.T) {
	tests := map[string]func(*testing.T, *Store, string){
		"event JSON mutation": func(t *testing.T, store *Store, runID string) {
			if _, err := store.db.Exec(`UPDATE agent_decision_events SET event_json = '{"bad":true}' WHERE run_id = ? AND sequence = 1`, runID); err != nil {
				t.Fatal(err)
			}
		},
		"record hash mutation": func(t *testing.T, store *Store, runID string) {
			if _, err := store.db.Exec(`UPDATE agent_decision_events SET record_hash = ? WHERE run_id = ? AND sequence = 1`, strings.Repeat("0", 64), runID); err != nil {
				t.Fatal(err)
			}
		},
		"record deletion": func(t *testing.T, store *Store, runID string) {
			if _, err := store.db.Exec(`DELETE FROM agent_decision_events WHERE run_id = ? AND sequence = 1`, runID); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
			defer store.Close()
			if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1")); err != nil {
				t.Fatal(err)
			}
			if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:2")); err != nil {
				t.Fatal(err)
			}
			mutate(t, store, runID)
			if _, err := store.DecisionEvents(ctx, runID, "trial:1"); !errors.Is(err, ErrDecisionEventIntegrity) {
				t.Fatalf("integrity error = %v", err)
			}
			if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:3")); !errors.Is(err, ErrDecisionEventIntegrity) {
				t.Fatalf("append integrity error = %v", err)
			}
		})
	}
}

func TestDecisionEventStoreRejectsInvalidDraft(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	draft := decisionDraft(runID, "call:1")
	draft.Decision.Effect = "allow"
	draft.Decision.ApprovalID = "approval:invalid"
	if _, err := store.AppendDecisionEvent(ctx, draft); err == nil {
		t.Fatal("invalid draft was persisted")
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM agent_decision_events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("event count = %d, want 0", count)
	}
}

func eventTestStore(t *testing.T, ctx context.Context, path string) (*Store, string) {
	t.Helper()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateRun(ctx, scenario.Compiled{
		SchemaVersion: scenario.APIVersion, ScenarioName: "agent-evidence", ScenarioVersion: "0.1.0",
		Seed: 42, Digest: strings.Repeat("a", 64),
	})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	return store, run.ID
}

func decisionDraft(runID, correlationID string) agent.DecisionEventDraft {
	return agent.DecisionEventDraft{
		OccurredAt: time.Date(2026, 7, 12, 22, 0, 0, 0, time.UTC),
		RunID:      runID, TrialID: "trial:1", CorrelationID: correlationID,
		Actor:    agent.ActorRef{ID: "agent:reference", Tenant: "tenant:northstar", Type: "agent"},
		Tool:     agent.ToolRef{Name: "google.drive.read", Version: "0.1.0", Digest: strings.Repeat("b", 64)},
		Action:   "drive.files.get",
		Resource: agent.ResourceRef{ID: "google:agent-runbook", Tenant: "tenant:northstar", Classification: "restricted"},
		Decision: agent.Decision{Effect: "allow", ReasonCode: "policy:rule:allow", PolicyVersion: "0.1.0"},
		Outcome:  agent.Outcome{Status: "not_executed"}, InputHash: strings.Repeat("c", 64),
	}
}
