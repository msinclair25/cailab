package agent

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReplayAgentTracesProducesDeterministicRepeatedTrialMetrics(t *testing.T) {
	first := replayTestTrace(t, "trial:1", 1, 2)
	second := replayTestTrace(t, "trial:2", 2, 2)
	second.Run.Status = "failed"
	second.Run.EndedAt = timePointer(second.Run.StartedAt.Add(2 * time.Minute))
	second.Decisions[0].Decision = Decision{Effect: "deny", ReasonCode: "policy:default_deny", PolicyVersion: "0.1.0"}
	second.Outcomes = nil

	report, err := ReplayAgentTraces([]AgentTrace{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != EvaluationProfile || len(report.Trials) != 2 || report.Trials[0].TrialID != "trial:1" {
		t.Fatalf("report = %+v", report)
	}
	if got := report.Aggregate.CompletedTrials; got.Numerator != 1 || got.Denominator != 2 || got.Rate == nil || *got.Rate != 0.5 {
		t.Fatalf("completion rate = %+v", got)
	}
	if got := report.Aggregate.AuthorizationRate; got.Numerator != 1 || got.Denominator != 2 || got.Rate == nil || *got.Rate != 0.5 {
		t.Fatalf("authorization rate = %+v", got)
	}
	if report.Aggregate.PolicyDeniedActions != 1 || report.Aggregate.ObservedProtectedTargets != 1 {
		t.Fatalf("aggregate = %+v", report.Aggregate)
	}
	if report.Trials[0].TraceDigest == "" || report.ConfigDigest == "" || len(report.NotMeasured) != 5 {
		t.Fatalf("report metadata = %+v", report)
	}

	repeated, err := ReplayAgentTraces([]AgentTrace{first, second})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(report)
	secondJSON, _ := json.Marshal(repeated)
	if !reflect.DeepEqual(firstJSON, secondJSON) {
		t.Fatalf("replay is not deterministic:\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestReplayAgentTracesScoresApprovalAndMissingOutcomeEvidence(t *testing.T) {
	trace := replayTestTrace(t, "trial:approval", 1, 1)
	decision := &trace.Decisions[0]
	decision.Decision = Decision{
		Effect: "require_approval", ReasonCode: "rule:approval", PolicyVersion: "0.1.0", ApprovalID: "approval:1",
	}
	trace.Outcomes = nil
	approval, err := BuildApprovalResolutionEvent(ApprovalResolutionEventDraft{
		OccurredAt: decision.OccurredAt.Add(time.Second), RunID: decision.RunID, TrialID: decision.TrialID,
		CorrelationID: decision.CorrelationID, ApprovalID: decision.Decision.ApprovalID,
		DecisionEventID: decision.EventID, ResolvedBy: "user:reviewer", Approved: true,
		Tool: decision.Tool, Action: decision.Action, Resource: decision.Resource,
		Decision: Decision{Effect: "allow", ReasonCode: "rule:approval", PolicyVersion: "0.1.0"}, InputHash: decision.InputHash,
	}, "approval-event:1")
	if err != nil {
		t.Fatal(err)
	}
	trace.Approvals = []ApprovalResolutionEvent{approval}

	report, err := ReplayAgentTraces([]AgentTrace{trace})
	if err != nil {
		t.Fatal(err)
	}
	metrics := report.Trials[0].Metrics
	if metrics.ApprovalRequired != 1 || metrics.ApprovalApproved != 1 || metrics.AuthorizedActions != 1 || metrics.MissingOutcomeEvidence != 1 {
		t.Fatalf("metrics = %+v", metrics)
	}
	if got := report.Aggregate.ApprovalResolutionRate; got.Numerator != 1 || got.Denominator != 1 || got.Rate == nil || *got.Rate != 1 {
		t.Fatalf("approval rate = %+v", got)
	}
}

func TestReplayAgentTracesUsesNullRatesForZeroDenominators(t *testing.T) {
	trace := replayTestTrace(t, "trial:inert", 1, 1)
	trace.Decisions = nil
	trace.Outcomes = nil
	report, err := ReplayAgentTraces([]AgentTrace{trace})
	if err != nil {
		t.Fatal(err)
	}
	for name, metric := range map[string]MetricRate{
		"authorization": report.Aggregate.AuthorizationRate,
		"approval":      report.Aggregate.ApprovalResolutionRate,
		"execution":     report.Aggregate.ExecutionSuccessRate,
	} {
		if metric.Numerator != 0 || metric.Denominator != 0 || metric.Rate != nil {
			t.Fatalf("%s rate = %+v", name, metric)
		}
	}
}

func TestReplayAgentTracesRejectsIncompleteIncompatibleAndBrokenEvidence(t *testing.T) {
	first := replayTestTrace(t, "trial:1", 1, 2)
	second := replayTestTrace(t, "trial:2", 2, 2)

	if _, err := ReplayAgentTraces([]AgentTrace{first}); !errors.Is(err, ErrIncompleteAgentTrialSet) {
		t.Fatalf("incomplete error = %v", err)
	}
	second.Run.PromptHash = strings.Repeat("c", 64)
	if _, err := ReplayAgentTraces([]AgentTrace{first, second}); !errors.Is(err, ErrIncompatibleAgentTrials) {
		t.Fatalf("compatibility error = %v", err)
	}
	second = replayTestTrace(t, "trial:2", 2, 2)
	second.Outcomes[0].DecisionEventID = "event:wrong"
	if _, err := ReplayAgentTraces([]AgentTrace{first, second}); !errors.Is(err, ErrAgentTraceIntegrity) {
		t.Fatalf("integrity error = %v", err)
	}
	second = replayTestTrace(t, "trial:1", 2, 2)
	if _, err := ReplayAgentTraces([]AgentTrace{first, second}); !errors.Is(err, ErrIncompleteAgentTrialSet) {
		t.Fatalf("duplicate ID error = %v", err)
	}
	second = replayTestTrace(t, "trial:2", 1, 2)
	if _, err := ReplayAgentTraces([]AgentTrace{first, second}); !errors.Is(err, ErrIncompleteAgentTrialSet) {
		t.Fatalf("duplicate index error = %v", err)
	}
	single := replayTestTrace(t, "trial:running", 1, 1)
	single.Run.Status = "running"
	single.Run.EndedAt = nil
	if _, err := ReplayAgentTraces([]AgentTrace{single}); !errors.Is(err, ErrAgentTraceIntegrity) {
		t.Fatalf("non-terminal error = %v", err)
	}
}

func replayTestTrace(t *testing.T, trialID string, index, count int) AgentTrace {
	t.Helper()
	started := time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC).Add(time.Duration(index) * time.Minute)
	ended := started.Add(time.Minute)
	tool := ToolRef{Name: "test.read", Version: "0.1.0", Digest: strings.Repeat("b", 64)}
	run := AgentRun{
		APIVersion: APIVersion, Kind: AgentRunKind, RunID: "run:evaluation", TrialID: trialID,
		Scenario: ScenarioRef{Name: "agent-evidence", Version: "0.1.0", Digest: strings.Repeat("a", 64), Seed: 42},
		Agent:    AgentRef{ID: "agent:test", Version: "0.1.0", Adapter: "subprocess", Provider: "test", Model: "deterministic"},
		Policy:   PolicyRef{Version: "0.1.0", Digest: strings.Repeat("d", 64)}, PromptHash: strings.Repeat("e", 64),
		Tools: []ToolRef{tool}, Trial: TrialRef{Index: index, Count: count}, Status: "completed", StartedAt: started, EndedAt: &ended,
	}
	decision := DecisionEvent{
		APIVersion: APIVersion, Kind: DecisionEventKind, EventID: "event:" + trialID, Sequence: 1,
		OccurredAt: started.Add(time.Second), RunID: run.RunID, TrialID: trialID, CorrelationID: "call:1",
		Actor: ActorRef{ID: run.Agent.ID, Tenant: "tenant:a", Type: "agent"}, Tool: tool, Action: "test.read",
		Resource: ResourceRef{ID: "resource:a", Tenant: "tenant:a", Classification: "restricted"},
		Decision: Decision{Effect: "allow", ReasonCode: "rule:allow", PolicyVersion: "0.1.0"},
		Outcome:  Outcome{Status: "not_executed"}, InputHash: strings.Repeat("f", 64),
	}
	outcome := ToolOutcomeEvent{
		APIVersion: APIVersion, Kind: ToolOutcomeEventKind, EventID: "outcome:" + trialID,
		OccurredAt: started.Add(2 * time.Second), RunID: run.RunID, TrialID: trialID, CorrelationID: decision.CorrelationID,
		DecisionEventID: decision.EventID, Tool: tool, Outcome: Outcome{Status: "succeeded"}, OutputHash: strings.Repeat("0", 64),
	}
	return AgentTrace{
		APIVersion: APIVersion, Kind: AgentTraceKind, Run: run,
		Decisions: []DecisionEvent{decision}, Outcomes: []ToolOutcomeEvent{outcome},
	}
}

func timePointer(value time.Time) *time.Time { return &value }
