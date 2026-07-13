package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
)

const (
	appAgentHelperEnvironment = "CAILAB_APP_AGENT_HELPER"
	appToolHelperEnvironment  = "CAILAB_APP_TOOL_HELPER"
)

func TestRunAgentExecutesRegisteredToolAndPersistsTerminalEvidence(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	options := appAgentTestOptions(t, rangeRun.Compiled, "trial:1", "tool")
	result, err := service.RunAgent(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != "completed" || result.Session.Completion.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Decisions) != 1 || result.Decisions[0].Action != "test.read" || result.Decisions[0].Resource.ID != "resource:a" || result.Decisions[0].Decision.Effect != "allow" {
		t.Fatalf("decisions = %+v", result.Decisions)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Outcome.Status != "succeeded" {
		t.Fatalf("outcomes = %+v", result.Outcomes)
	}
	stored, err := store.AgentRun(ctx, rangeRun.ID, "trial:1")
	if err != nil || stored.Status != "completed" || stored.EndedAt == nil {
		t.Fatalf("stored run = %+v, error = %v", stored, err)
	}
}

func TestRunAgentRejectsRegistrationOutsideCanonicalScenarioBeforePersistence(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	options := appAgentTestOptions(t, rangeRun.Compiled, "trial:invalid", "reference")
	options.Tools[0].Manifest.Spec.Permissions[0].Resources[0] = "resource:missing"
	if _, err := service.RunAgent(ctx, options); err == nil || !strings.Contains(err.Error(), "not in the active scenario") {
		t.Fatalf("run error = %v", err)
	}
	if _, err := store.AgentRun(ctx, rangeRun.ID, "trial:invalid"); !errors.Is(err, state.ErrNoActiveAgentRun) {
		t.Fatalf("persisted invalid run error = %v", err)
	}
}

func TestRunAgentPersistsFailedTerminalRecordAfterProtocolFailure(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	options := appAgentTestOptions(t, rangeRun.Compiled, "trial:failed", "malformed")
	result, err := service.RunAgent(ctx, options)
	if err == nil || result.Run.Status != "failed" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	stored, readErr := store.AgentRun(ctx, rangeRun.ID, "trial:failed")
	if readErr != nil || stored.Status != "failed" || stored.EndedAt == nil {
		t.Fatalf("stored run = %+v, error = %v", stored, readErr)
	}
}

func TestRunAgentResolvesApprovedAndRejectedCalls(t *testing.T) {
	for _, test := range []struct {
		name         string
		approved     bool
		wantOutcomes int
	}{
		{name: "approved", approved: true, wantOutcomes: 1},
		{name: "rejected", approved: false, wantOutcomes: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, service, rangeRun := appAgentTestService(t, ctx)
			defer store.Close()
			options := appAgentTestOptions(t, rangeRun.Compiled, "trial:"+test.name, "approval")
			options.Policy.Rules[0].Effect = "require_approval"
			options.Approver = agent.ApproverFunc(func(_ context.Context, request agent.ApprovalRequest) (agent.ApprovalResolution, error) {
				if request.Resource.ID != "resource:a" || request.InputHash == "" {
					t.Fatalf("approval request = %+v", request)
				}
				return agent.ApprovalResolution{Approved: test.approved, ResolvedBy: "user:reviewer"}, nil
			})
			result, err := service.RunAgent(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			if result.Run.Status != "completed" || len(result.Approvals) != 1 || result.Approvals[0].Approved != test.approved || len(result.Outcomes) != test.wantOutcomes {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestReplayAgentTrialsAggregatesACompleteCompatibleSet(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	for index := 1; index <= 2; index++ {
		options := appAgentTestOptions(t, rangeRun.Compiled, fmt.Sprintf("trial:%d", index), "tool")
		options.TrialIndex = index
		options.TrialCount = 2
		if _, err := service.RunAgent(ctx, options); err != nil {
			t.Fatal(err)
		}
	}
	report, err := service.ReplayAgentTrials(ctx, "", []string{"trial:2", "trial:1"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Aggregate.Trials != 2 || report.Aggregate.CompletedTrials.Numerator != 2 ||
		report.Aggregate.AuthorizationRate.Numerator != 2 || report.Aggregate.AuthorizationRate.Denominator != 2 ||
		report.Aggregate.ExecutionSuccessRate.Numerator != 2 || report.Trials[0].TrialID != "trial:1" {
		t.Fatalf("report = %+v", report)
	}
	if _, err := service.ReplayAgentTrials(ctx, rangeRun.ID, []string{"trial:1"}); !errors.Is(err, agent.ErrIncompleteAgentTrialSet) {
		t.Fatalf("incomplete replay error = %v", err)
	}
	if _, err := store.StopActiveRun(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReplayAgentTrials(ctx, rangeRun.ID, []string{"trial:1", "trial:2"}); err != nil {
		t.Fatalf("replay stopped run: %v", err)
	}
}

func TestRunAgentRestoresCapturesAndScoresScenarioOutcome(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	remediated := rangeRun.Compiled
	remediated.Edges = nil
	replacement := []provider.Instance{{Provider: "aws", Engine: "floci", Name: "test-runtime", ContainerID: "replacement", Endpoint: "http://127.0.0.1:4566", Status: "ready"}}
	manager := &fakeProviderManager{snapshots: []scenario.Compiled{rangeRun.Compiled, remediated}, restoredRuntimes: replacement}
	service.provider = manager
	options := appAgentTestOptions(t, rangeRun.Compiled, "trial:state", "tool")
	options.CaptureState = true
	options.RestoreFixture = true
	result, err := service.RunAgent(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if !manager.restored || result.Run.State == nil || !result.Run.State.Restore || len(result.States) != 2 ||
		result.States[0].Verification.Passed || !result.States[1].Verification.Passed {
		t.Fatalf("result = %+v, manager = %+v", result, manager)
	}
	persisted, err := service.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Runtimes) != 1 || persisted.Runtimes[0].ContainerID != "replacement" {
		t.Fatalf("persisted runtimes = %+v", persisted.Runtimes)
	}
	report, err := service.ReplayAgentTrials(ctx, rangeRun.ID, []string{"trial:state"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != agent.ScenarioOutcomeProfile || report.Aggregate.TaskSuccessRate == nil ||
		report.Aggregate.TaskSuccessRate.Numerator != 1 || report.Aggregate.RemediationSuccessRate == nil ||
		report.Aggregate.RemediationSuccessRate.Numerator != 1 || report.Aggregate.InitialStateMatchRate == nil ||
		report.Aggregate.InitialStateMatchRate.Numerator != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunAgentRestoreFailureDoesNotLaunchAgent(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	manager := &fakeProviderManager{restoreErr: errors.New("replacement failed")}
	service.provider = manager
	options := appAgentTestOptions(t, rangeRun.Compiled, "trial:restore-failure", "tool")
	options.CaptureState = true
	options.RestoreFixture = true
	options.Command = []string{"/definitely-not-a-cailab-agent"}
	result, err := service.RunAgent(ctx, options)
	if err == nil || !strings.Contains(err.Error(), "replacement failed") {
		t.Fatalf("RunAgent() error = %v", err)
	}
	if !manager.restored || result.Run.Status != "failed" || len(result.States) != 0 {
		t.Fatalf("result = %+v, manager = %+v", result, manager)
	}
}

func TestReferenceAgentRunOptionsAreValidForActiveScenario(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	compiled := scenario.Compiled{Nodes: []scenario.Node{
		{ID: "tenant:a", Kind: "tenant", Type: "organization"},
		{ID: "resource:a", Kind: "resource", Tenant: "tenant:a", Type: "test", Classification: "internal"},
	}}
	options, err := ReferenceAgentRunOptions(compiled, executable, t.TempDir(), "trial:1")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAgentRunOptions(compiled, options); err != nil {
		t.Fatal(err)
	}
}

func TestUnsafeFixtureAgentRunOptionsBindScenarioGroundTruth(t *testing.T) {
	definition, err := scenario.Load(filepath.Join("..", "..", "scenarios", "acquisition-agent", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := scenario.Compile(definition, definition.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	options, err := UnsafeFixtureAgentRunOptions(compiled, "http://127.0.0.1:8000", executable, t.TempDir(), "trial:unsafe", "drive-runbook-export")
	if err != nil {
		t.Fatal(err)
	}
	if !options.CaptureState || !options.RestoreFixture || options.EvaluationFixtureID != "drive-runbook-export" || len(options.Tools) != 2 || len(options.Policy.Rules) != 2 {
		t.Fatalf("options = %+v", options)
	}
}

func TestAppAgentSubprocessHelper(t *testing.T) {
	agentMode := os.Getenv(appAgentHelperEnvironment)
	toolMode := os.Getenv(appToolHelperEnvironment)
	if agentMode == "" && toolMode == "" {
		return
	}
	if toolMode != "" {
		if err := agent.ServeReferenceTool(context.Background(), os.Stdin, os.Stdout, "test.read"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(10)
		}
		os.Exit(0)
	}
	if agentMode == "malformed" {
		fmt.Fprintln(os.Stdout, "not-json")
		return
	}
	decoder := agent.NewDecoder(os.Stdin)
	if _, err := decoder.Next(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(11)
	}
	encoder := agent.NewEncoder(os.Stdout)
	if err := encoder.Write(appAgentMessage(agent.MessageAgentReady, "message:ready", "", agent.AgentReadyPayload{AgentID: "agent:test", AgentVersion: "0.1.0"})); err != nil {
		os.Exit(12)
	}
	if agentMode == "tool" || agentMode == "approval" {
		call := appAgentMessage(agent.MessageToolCall, "call:1", "", agent.ToolCallPayload{
			Tool: "test.read", Action: "test.read", Resource: "resource:a", Arguments: json.RawMessage(`{"resource":"resource:a"}`),
		})
		if err := encoder.Write(call); err != nil {
			os.Exit(13)
		}
		response, err := decoder.Next()
		if agentMode == "tool" {
			if err != nil || response.Type != agent.MessageToolResult || response.CorrelationID != call.ID {
				os.Exit(14)
			}
		} else {
			if err != nil || response.Type != agent.MessageApprovalRequired || response.CorrelationID != call.ID {
				os.Exit(14)
			}
			resolved, err := decoder.Next()
			if err != nil || resolved.Type != agent.MessageApprovalResolved || resolved.CorrelationID != call.ID {
				os.Exit(16)
			}
			final, err := decoder.Next()
			if err != nil || final.Type != agent.MessageToolResult || final.CorrelationID != call.ID {
				os.Exit(17)
			}
		}
	}
	if err := encoder.Write(appAgentMessage(agent.MessageSessionComplete, "message:complete", "", agent.SessionCompletePayload{Status: "completed"})); err != nil {
		os.Exit(15)
	}
	os.Exit(0)
}

func appAgentTestService(t *testing.T, ctx context.Context) (*state.Store, *Service, state.Run) {
	t.Helper()
	store, err := state.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	compiled := scenario.Compiled{
		SchemaVersion: scenario.APIVersion, ScenarioName: "agent-test", ScenarioVersion: "0.1.0", Seed: 42,
		Digest: strings.Repeat("a", 64),
		Nodes: []scenario.Node{
			{ID: "tenant:a", Kind: "tenant", Type: "organization", DisplayName: "Tenant A"},
			{ID: "resource:a", Kind: "resource", Tenant: "tenant:a", Type: "test", DisplayName: "Resource A", Classification: "restricted"},
		},
		Edges: []scenario.Relationship{{ID: "edge:vulnerable", From: "tenant:a", To: "resource:a", Type: "can_access"}},
		Invariants: []scenario.Invariant{{
			ID: "path-closed", Type: "path_absent", From: "tenant:a", To: "resource:a", Severity: "high",
			Description: "The prohibited path is absent.",
		}},
	}
	digest, err := scenario.StateDigest(compiled)
	if err != nil {
		t.Fatal(err)
	}
	compiled.Digest = digest
	rangeRun, err := store.CreateRun(ctx, compiled)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.SetRuntimeBaseline(ctx, rangeRun.ID, nil, digest); err != nil {
		store.Close()
		t.Fatal(err)
	}
	rangeRun.BaselineDigest = digest
	service := New(store, nil)
	current := time.Date(2026, 7, 12, 23, 0, 0, 0, time.UTC)
	service.clock = func() time.Time {
		current = current.Add(time.Second)
		return current
	}
	return store, service, rangeRun
}

func appAgentTestOptions(t *testing.T, compiled scenario.Compiled, trialID, agentMode string) AgentRunOptions {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	manifest := agent.ToolManifest{
		APIVersion: agent.APIVersion, Kind: agent.ToolManifestKind,
		Metadata: agent.Metadata{Name: "test.read", Version: "0.1.0", Description: "Read the synthetic test resource."},
		Spec: agent.ToolManifestSpec{
			Transport:   agent.ToolTransport{Type: "subprocess", Command: []string{executable, "-test.run=^TestAppAgentSubprocessHelper$"}},
			InputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"resource":{"type":"string"}},"required":["resource"],"additionalProperties":false}`),
			Permissions: []agent.Permission{{Tenant: "tenant:a", Actions: []string{"test.read"}, Resources: []string{"resource:a"}}},
			Risk:        "low", TimeoutMillis: 2_000, Isolation: agent.Isolation{Network: "host", Filesystem: "host"},
		},
	}
	policy := agent.GovernancePolicy{
		APIVersion: agent.APIVersion, Kind: agent.GovernancePolicyKind, Version: "0.1.0", DefaultEffect: "deny",
		Rules: []agent.PolicyRule{{
			ID: "rule:allow", Effect: "allow", AgentID: "agent:test", Tool: "test.read", Action: "test.read",
			Resource: "resource:a", ResourceTenant: "tenant:a", ResourceClassification: "restricted",
		}},
	}
	return AgentRunOptions{
		Agent:       agent.AgentRef{ID: "agent:test", Version: "0.1.0", Adapter: "subprocess", Provider: "test", Model: "deterministic"},
		ActorTenant: "tenant:a",
		Command:     []string{executable, "-test.run=^TestAppAgentSubprocessHelper$"}, Directory: directory,
		Environment: []string{appAgentHelperEnvironment + "=" + agentMode},
		Policy:      policy,
		Tools:       []RegisteredTool{{Manifest: manifest, Directory: directory, Environment: []string{appToolHelperEnvironment + "=reference"}}},
		PromptHash:  strings.Repeat("b", 64), TrialID: trialID, TrialIndex: 1, TrialCount: 1, SessionTimeout: 5 * time.Second,
	}
}

func appAgentMessage(messageType, id, correlation string, payload any) agent.Message {
	data, _ := json.Marshal(payload)
	return agent.Message{ProtocolVersion: agent.ProtocolVersion, ID: id, Type: messageType, CorrelationID: correlation, Payload: data}
}
