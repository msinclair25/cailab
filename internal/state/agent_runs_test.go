package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
)

func TestAgentRunLifecyclePersistsImmutableStartAndTerminalRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, runID := eventTestStoreWithoutAgent(t, ctx, path)
	run := stateTestAgentRun(runID)
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginAgentRun(ctx, run); !errors.Is(err, ErrDuplicateAgentRun) {
		t.Fatalf("duplicate begin error = %v", err)
	}
	stored, err := store.AgentRun(ctx, runID, run.TrialID)
	if err != nil || stored.Status != "running" {
		t.Fatalf("stored run = %+v, error = %v", stored, err)
	}
	endedAt := run.StartedAt.Add(time.Minute)
	terminal := run
	terminal.Status = "completed"
	terminal.EndedAt = &endedAt
	if err := store.CompleteAgentRun(ctx, terminal); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteAgentRun(ctx, terminal); !errors.Is(err, ErrNoActiveAgentRun) {
		t.Fatalf("duplicate completion error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stored, err = store.AgentRun(ctx, runID, run.TrialID)
	if err != nil || stored.Status != "completed" || stored.EndedAt == nil || !stored.EndedAt.Equal(endedAt) {
		t.Fatalf("terminal run = %+v, error = %v", stored, err)
	}
	if _, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:after-complete")); !errors.Is(err, ErrNoActiveAgentRun) {
		t.Fatalf("post-completion decision error = %v", err)
	}
}

func TestAgentRunCompletionRejectsImmutableMetadataChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStoreWithoutAgent(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	run := stateTestAgentRun(runID)
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	endedAt := run.StartedAt.Add(time.Second)
	terminal := run
	terminal.Agent.Model = "changed"
	terminal.Status = "failed"
	terminal.EndedAt = &endedAt
	if err := store.CompleteAgentRun(ctx, terminal); !errors.Is(err, ErrAgentRunIntegrity) {
		t.Fatalf("completion error = %v", err)
	}
}

func TestAgentRunCompletionRejectsIsolationMetadataChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStoreWithoutAgent(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	run := stateTestAgentRun(runID)
	run.Execution = &agent.AgentExecutionRef{
		Mode: "container", Engine: "docker", Profile: "docker-strict-v1", Image: "sha256:" + strings.Repeat("a", 64),
		Network: "none", Filesystem: "read_only",
	}
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	endedAt := run.StartedAt.Add(time.Second)
	terminal := run
	terminal.Execution = &agent.AgentExecutionRef{
		Mode: "container", Engine: "docker", Profile: "docker-strict-v1", Image: "sha256:" + strings.Repeat("b", 64),
		Network: "none", Filesystem: "read_only",
	}
	terminal.Status = "failed"
	terminal.EndedAt = &endedAt
	if err := store.CompleteAgentRun(ctx, terminal); !errors.Is(err, ErrAgentRunIntegrity) {
		t.Fatalf("completion error = %v", err)
	}
}

func TestAgentRunIntegrityDetectsStoredMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStoreWithoutAgent(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	run := stateTestAgentRun(runID)
	if err := store.BeginAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE agent_runs SET start_json = '{"bad":true}' WHERE run_id = ? AND trial_id = ?`, runID, run.TrialID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AgentRun(ctx, runID, run.TrialID); !errors.Is(err, ErrAgentRunIntegrity) {
		t.Fatalf("integrity error = %v", err)
	}
}

func TestToolOutcomeCannotAppendAfterAgentRunCompletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, runID := eventTestStore(t, ctx, filepath.Join(t.TempDir(), "state.db"))
	defer store.Close()
	decision, err := store.AppendDecisionEvent(ctx, decisionDraft(runID, "call:1"))
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.AgentRun(ctx, runID, "trial:1")
	if err != nil {
		t.Fatal(err)
	}
	endedAt := run.StartedAt.Add(time.Minute)
	run.Status = "completed"
	run.EndedAt = &endedAt
	if err := store.CompleteAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendToolOutcomeEvent(ctx, toolOutcomeDraft(decision, agent.Outcome{Status: "succeeded"}, strings.Repeat("f", 64))); !errors.Is(err, ErrNoActiveAgentRun) {
		t.Fatalf("post-completion outcome error = %v", err)
	}
}

func stateTestAgentRun(runID string) agent.AgentRun {
	return agent.AgentRun{
		APIVersion: agent.APIVersion, Kind: agent.AgentRunKind, RunID: runID, TrialID: "trial:1",
		Scenario: agent.ScenarioRef{Name: "agent-evidence", Version: "0.1.0", Digest: strings.Repeat("a", 64), Seed: 42},
		Agent:    agent.AgentRef{ID: "agent:reference", Version: "0.1.0", Adapter: "subprocess", Provider: "cloudailab", Model: "deterministic-reference"},
		Policy:   agent.PolicyRef{Version: "0.1.0", Digest: strings.Repeat("d", 64)}, PromptHash: strings.Repeat("e", 64),
		Tools: []agent.ToolRef{{Name: "google.drive.read", Version: "0.1.0", Digest: strings.Repeat("b", 64)}},
		Trial: agent.TrialRef{Index: 1, Count: 1}, Status: "running", StartedAt: time.Date(2026, 7, 12, 21, 0, 0, 0, time.UTC),
	}
}
