package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/verify"
)

func TestTrialStateEvidencePersistsOrderedPhasesAndClosesTrace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStoreWithoutAgent(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	run := stateTestAgentRun(runID)
	run.State = &agent.TrialStateRef{Profile: "scenario-state-v1", BaselineDigest: run.Scenario.Digest, Restore: true}
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	before := stateEvidenceForTest(run, "before", true, run.StartedAt.Add(time.Second), false)
	if err := store.AppendTrialStateEvidence(ctx, before); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTrialStateEvidence(ctx, before); !errors.Is(err, ErrDuplicateTrialStateEvidence) {
		t.Fatalf("duplicate before error = %v", err)
	}
	if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1")); err != nil {
		t.Fatal(err)
	}
	after := stateEvidenceForTest(run, "after", false, run.StartedAt.Add(2*time.Second), true)
	after.SnapshotDigest = strings.Repeat("f", 64)
	if err := store.AppendTrialStateEvidence(ctx, after); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:closed")); !errors.Is(err, ErrTrialStateEvidenceIntegrity) {
		t.Fatalf("post-state decision error = %v", err)
	}
	endedAt := run.StartedAt.Add(time.Minute)
	terminal := run
	terminal.Status = "completed"
	terminal.EndedAt = &endedAt
	if err := store.CompleteAgentRun(ctx, terminal); err != nil {
		t.Fatal(err)
	}
	states, err := store.TrialStateEvidence(ctx, runID, run.TrialID)
	if err != nil || len(states) != 2 || states[0].Phase != "before" || states[1].Phase != "after" || !states[1].Verification.Passed {
		t.Fatalf("states = %+v, error = %v", states, err)
	}
}

func TestTrialStateEvidenceRejectsAfterWithoutBeforeAndDetectsMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStoreWithoutAgent(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	run := stateTestAgentRun(runID)
	run.State = &agent.TrialStateRef{Profile: "scenario-state-v1", BaselineDigest: run.Scenario.Digest}
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	after := stateEvidenceForTest(run, "after", false, run.StartedAt.Add(time.Second), true)
	if err := store.AppendTrialStateEvidence(ctx, after); !errors.Is(err, ErrTrialStateEvidenceIntegrity) {
		t.Fatalf("orphan after error = %v", err)
	}
	before := stateEvidenceForTest(run, "before", false, run.StartedAt.Add(time.Second), false)
	if err := store.AppendTrialStateEvidence(ctx, before); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE agent_trial_state_evidence SET evidence_json = '{"bad":true}' WHERE run_id = ? AND trial_id = ?`, runID, run.TrialID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TrialStateEvidence(ctx, runID, run.TrialID); !errors.Is(err, ErrTrialStateEvidenceIntegrity) {
		t.Fatalf("mutation error = %v", err)
	}
}

func TestTrialStateEvidenceRejectsOversizedRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStoreWithoutAgent(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	run := stateTestAgentRun(runID)
	run.State = &agent.TrialStateRef{Profile: agent.TrialStateProfile, BaselineDigest: run.Scenario.Digest}
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	evidence := stateEvidenceForTest(run, "before", false, run.StartedAt.Add(time.Second), false)
	evidence.Verification.Results[0].Evidence = []string{strings.Repeat("x", agent.MaxFrameBytes)}
	if err := store.AppendTrialStateEvidence(ctx, evidence); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized evidence error = %v", err)
	}
}

func stateEvidenceForTest(run agent.AgentRun, phase string, restored bool, captured time.Time, passed bool) agent.TrialStateEvidence {
	failedCount, passedCount := 1, 0
	message := "prohibited path exists"
	if passed {
		failedCount, passedCount = 0, 1
		message = "prohibited path is absent"
	}
	return agent.TrialStateEvidence{
		APIVersion: agent.APIVersion, Kind: agent.TrialStateEvidenceKind,
		RunID: run.RunID, TrialID: run.TrialID, Phase: phase, CapturedAt: captured,
		SnapshotDigest: run.Scenario.Digest, FixtureRestored: restored,
		Verification: verify.Report{
			RunID: run.RunID, Scenario: run.Scenario.Name, ScenarioVersion: run.Scenario.Version, Digest: run.Scenario.Digest,
			Passed: passed, PassedCount: passedCount, FailedCount: failedCount,
			Results: []verify.Result{{
				InvariantID: "path-closed", Severity: "high", Description: "The prohibited path is absent.",
				Passed: passed, Message: message,
			}},
		},
	}
}
