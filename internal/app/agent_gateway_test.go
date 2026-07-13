package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
)

func TestGovernedToolCallPersistsEvidenceInActiveRunStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := state.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rangeRun, err := store.CreateRun(ctx, scenario.Compiled{
		SchemaVersion: scenario.APIVersion, ScenarioName: "gateway-integration", ScenarioVersion: "0.1.0",
		Seed: 42, Digest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := integrationToolManifest()
	manifestDigest, err := agent.DigestToolManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	policy := agent.GovernancePolicy{
		APIVersion: agent.APIVersion, Kind: agent.GovernancePolicyKind, Version: "0.1.0", DefaultEffect: "deny",
		Rules: []agent.PolicyRule{{
			ID: "rule:allow", Effect: "allow", AgentID: "agent:reference", Tool: manifest.Metadata.Name,
			Action: "drive.files.get", Resource: "google:agent-runbook", ResourceTenant: "tenant:northstar", ResourceClassification: "restricted",
		}},
	}
	policyDigest, err := agent.DigestGovernancePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	agentRun := agent.AgentRun{
		APIVersion: agent.APIVersion, Kind: agent.AgentRunKind, RunID: rangeRun.ID, TrialID: "trial:1",
		Scenario: agent.ScenarioRef{Name: rangeRun.ScenarioName, Version: rangeRun.ScenarioVersion, Digest: rangeRun.Compiled.Digest, Seed: rangeRun.Seed},
		Agent:    agent.AgentRef{ID: "agent:reference", Version: "0.1.0", Adapter: "subprocess", Provider: "cloudailab", Model: "deterministic-reference"},
		Policy:   agent.PolicyRef{Version: policy.Version, Digest: policyDigest}, PromptHash: strings.Repeat("b", 64),
		Tools: []agent.ToolRef{{Name: manifest.Metadata.Name, Version: manifest.Metadata.Version, Digest: manifestDigest}},
		Trial: agent.TrialRef{Index: 1, Count: 1}, Status: "running", StartedAt: time.Date(2026, 7, 12, 22, 0, 0, 0, time.UTC),
	}
	if err := store.BeginAgentRun(ctx, agentRun); err != nil {
		t.Fatal(err)
	}
	gateway := &agent.Gateway{
		Run: agentRun, Actor: agent.ActorRef{ID: agentRun.Agent.ID, Tenant: "tenant:northstar", Type: "agent"}, Policy: policy,
		Resolver: agent.ToolCallResolverFunc(func(context.Context, agent.Message, agent.ToolCallPayload) (agent.ToolCallResolution, error) {
			return agent.ToolCallResolution{
				Manifest: manifest, Action: "drive.files.get",
				Resource: agent.ResourceRef{ID: "google:agent-runbook", Tenant: "tenant:northstar", Classification: "restricted"},
			}, nil
		}),
		Executor: integrationExecutor{}, Events: store, Clock: func() time.Time { return time.Date(2026, 7, 12, 22, 1, 0, 0, time.UTC) },
	}
	arguments := json.RawMessage(`{"fileId":"google:agent-runbook","token":"synthetic-secret"}`)
	call := integrationMessage(t, "call:1", manifest.Metadata.Name, arguments)
	response, err := gateway.HandleToolCall(ctx, call, agent.ToolCallPayload{Tool: manifest.Metadata.Name, Action: "drive.files.get", Resource: "google:agent-runbook", Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != agent.MessageToolResult {
		t.Fatalf("response type = %q", response.Type)
	}
	events, err := store.DecisionEvents(ctx, rangeRun.ID, agentRun.TrialID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Sequence != 1 || events[0].Decision.Effect != "allow" || events[0].Outcome.Status != "not_executed" {
		t.Fatalf("events = %+v", events)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "synthetic-secret") {
		t.Fatalf("persisted evidence leaked raw arguments: %s", encoded)
	}
	outcomes, err := store.ToolOutcomeEvents(ctx, rangeRun.ID, agentRun.TrialID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Outcome.Status != "succeeded" || outcomes[0].OutputHash == "" {
		t.Fatalf("outcomes = %+v", outcomes)
	}
}

type integrationExecutor struct{}

func (integrationExecutor) Execute(context.Context, agent.ToolManifest, agent.ToolExecutionRequest) (agent.ToolExecutionResult, error) {
	return agent.ToolExecutionResult{Status: "succeeded", Content: json.RawMessage(`{"ok":true}`)}, nil
}

func integrationToolManifest() agent.ToolManifest {
	return agent.ToolManifest{
		APIVersion: agent.APIVersion, Kind: agent.ToolManifestKind,
		Metadata: agent.Metadata{Name: "google.drive.read", Version: "0.1.0", Description: "Read one declared synthetic Drive file."},
		Spec: agent.ToolManifestSpec{
			Transport:   agent.ToolTransport{Type: "subprocess", Command: []string{"/usr/bin/cailab-google-tool"}},
			InputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"fileId":{"type":"string"},"token":{"type":"string"}},"required":["fileId"],"additionalProperties":false}`),
			Permissions: []agent.Permission{{Tenant: "tenant:northstar", Actions: []string{"drive.files.get"}, Resources: []string{"google:agent-runbook"}}},
			Risk:        "medium", TimeoutMillis: 5000,
			Isolation: agent.Isolation{Network: "loopback", Filesystem: "none"},
		},
	}
}

func integrationMessage(t *testing.T, id, tool string, arguments json.RawMessage) agent.Message {
	t.Helper()
	payload, err := json.Marshal(agent.ToolCallPayload{Tool: tool, Action: "drive.files.get", Resource: "google:agent-runbook", Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	message := agent.Message{ProtocolVersion: agent.ProtocolVersion, ID: id, Type: agent.MessageToolCall, Payload: payload}
	if err := agent.ValidateMessage(message); err != nil {
		t.Fatal(err)
	}
	return message
}
