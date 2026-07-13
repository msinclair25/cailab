package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGovernancePolicyValidationDecodeAndDigest(t *testing.T) {
	policy, _ := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	first, err := DigestGovernancePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeGovernancePolicy(data)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DigestGovernancePolicy(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("policy digests = %q and %q", first, second)
	}
	unknown := bytes.Replace(data, []byte(`"kind":"GovernancePolicy"`), []byte(`"kind":"GovernancePolicy","unexpected":true`), 1)
	if _, err := DecodeGovernancePolicy(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestGovernancePolicyRejectsUnsafeContracts(t *testing.T) {
	tests := map[string]func(*GovernancePolicy){
		"default allow":       func(policy *GovernancePolicy) { policy.DefaultEffect = "allow" },
		"duplicate rule":      func(policy *GovernancePolicy) { policy.Rules = append(policy.Rules, policy.Rules[0]) },
		"redact without path": func(policy *GovernancePolicy) { policy.Rules[0].Effect = "redact" },
		"allow with path": func(policy *GovernancePolicy) {
			policy.Rules[0].Redactions = []string{"/token"}
		},
		"invalid classification": func(policy *GovernancePolicy) { policy.Rules[0].ResourceClassification = "secret" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			policy, _ := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
			mutate(&policy)
			if err := ValidateGovernancePolicy(policy); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("error = %v, want invalid policy", err)
			}
		})
	}
}

func TestPolicyDecisionPrecedenceIsOrderIndependent(t *testing.T) {
	rules := []PolicyRule{
		policyRule("rule:allow", "allow"),
		policyRule("rule:redact", "redact", "/token"),
		policyRule("rule:approval", "require_approval"),
		policyRule("rule:deny", "deny"),
	}
	policy, request := policyRequest(t, rules)
	first, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Decision.Effect != "deny" || first.Decision.ReasonCode != "rule:deny" {
		t.Fatalf("decision = %+v", first.Decision)
	}
	for left, right := 0, len(rules)-1; left < right; left, right = left+1, right-1 {
		rules[left], rules[right] = rules[right], rules[left]
	}
	policy, request = policyRequest(t, rules)
	second, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Decision, second.Decision) {
		t.Fatalf("order changed decision: first=%+v second=%+v", first.Decision, second.Decision)
	}
}

func TestPolicyApprovalIsStableAndPrecedesRedaction(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{
		policyRule("rule:redact", "redact", "/token"),
		policyRule("rule:approval", "require_approval"),
	})
	first, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Decision.Effect != "require_approval" || first.Decision.ApprovalID == "" || first.Decision.ApprovalID != second.Decision.ApprovalID {
		t.Fatalf("approval decisions = %+v and %+v", first.Decision, second.Decision)
	}
	request.CorrelationID = "call:other"
	third, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if third.Decision.ApprovalID == first.Decision.ApprovalID {
		t.Fatal("approval identifier was reused across correlations")
	}
}

func TestPolicyRedactsCanonicalArgumentsWithoutMutatingInput(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{
		policyRule("rule:z", "redact", "/nested/password"),
		policyRule("rule:a", "redact", "/token", "/nested/password"),
	})
	original := append([]byte(nil), request.Arguments...)
	evaluation, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision.Effect != "redact" || !reflect.DeepEqual(evaluation.Decision.Redactions, []string{"/nested/password", "/token"}) {
		t.Fatalf("decision = %+v", evaluation.Decision)
	}
	want := `{"fileId":"google:agent-runbook","nested":{"password":"[REDACTED]"},"token":"[REDACTED]"}`
	if string(evaluation.Arguments) != want {
		t.Fatalf("redacted arguments = %s", evaluation.Arguments)
	}
	if !bytes.Equal(request.Arguments, original) || !bytes.Contains(request.Arguments, []byte("synthetic-secret")) {
		t.Fatal("input arguments were mutated")
	}
	wantHash, err := DigestJSON(original)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.InputHash != wantHash {
		t.Fatalf("input hash = %q, want %q", evaluation.InputHash, wantHash)
	}
}

func TestPolicyFailsClosedForMissingRedactionAndManifestCeiling(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:redact", "redact", "/missing")})
	evaluation, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision.Effect != "deny" || evaluation.Decision.ReasonCode != "policy:redaction_failed" {
		t.Fatalf("redaction failure decision = %+v", evaluation.Decision)
	}

	policy, request = policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	request.Action = "drive.files.delete"
	evaluation, err = EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision.Effect != "deny" || evaluation.Decision.ReasonCode != "manifest:permission_denied" {
		t.Fatalf("manifest ceiling decision = %+v", evaluation.Decision)
	}

	policy, request = policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	request.Resource.Tenant = "tenant:other"
	evaluation, err = EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision.Effect != "deny" || evaluation.Decision.ReasonCode != "manifest:permission_denied" {
		t.Fatalf("cross-tenant ceiling decision = %+v", evaluation.Decision)
	}
}

func TestPolicyDefaultsToDenyAndRejectsReferenceDrift(t *testing.T) {
	policy, request := policyRequest(t, nil)
	evaluation, err := EvaluatePolicy(policy, request)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision.Effect != "deny" || evaluation.Decision.ReasonCode != "policy:default_deny" {
		t.Fatalf("default decision = %+v", evaluation.Decision)
	}
	request.Run.Policy.Digest = testDigest
	if _, err := EvaluatePolicy(policy, request); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("policy drift error = %v", err)
	}
}

func TestGatewayPersistsBeforeReturningNonExecutingResult(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	store := &memoryEventAppender{}
	gateway := testGateway(policy, request, store)
	call := testMessage(MessageToolCall, "call:gateway", "", gatewayToolCall(request, request.Arguments))
	var payload ToolCallPayload
	if err := json.Unmarshal(call.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	response, err := gateway.HandleToolCall(context.Background(), call, payload)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != MessageToolResult || len(store.events) != 1 {
		t.Fatalf("response=%+v events=%+v", response, store.events)
	}
	var result ToolResultPayload
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" || result.Decision.Effect != "allow" || len(store.outcomes) != 1 {
		t.Fatalf("tool result = %+v", result)
	}
	eventData, err := json.Marshal(store.events[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(eventData, []byte("synthetic-secret")) || store.events[0].InputHash == "" {
		t.Fatalf("event leaked raw input or omitted hash: %s", eventData)
	}
}

func TestGatewayReturnsApprovalOnlyAfterEvidenceCommit(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:approval", "require_approval")})
	store := &memoryEventAppender{}
	gateway := testGateway(policy, request, store)
	call := testMessage(MessageToolCall, "call:approval", "", gatewayToolCall(request, request.Arguments))
	var payload ToolCallPayload
	_ = json.Unmarshal(call.Payload, &payload)
	response, err := gateway.HandleToolCall(context.Background(), call, payload)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != MessageApprovalRequired || len(store.events) != 1 || store.events[0].Decision.Effect != "require_approval" {
		t.Fatalf("response=%+v events=%+v", response, store.events)
	}
}

func TestGatewayRedactsDeclaredSensitiveOutputBeforeResponseAndHash(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	request.Manifest.Spec.SensitiveFields = []string{"/token"}
	manifestDigest, err := DigestToolManifest(request.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	request.Run.Tools[0].Digest = manifestDigest
	store := &memoryEventAppender{}
	gateway := testGateway(policy, request, store)
	gateway.Executor = fakeToolExecutor{result: ToolExecutionResult{
		Status: "succeeded", Content: json.RawMessage(`{"ok":true,"token":"raw-secret"}`),
	}}
	call := testMessage(MessageToolCall, "call:output", "", gatewayToolCall(request, request.Arguments))
	var payload ToolCallPayload
	_ = json.Unmarshal(call.Payload, &payload)
	response, err := gateway.HandleToolCall(context.Background(), call, payload)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(response.Payload, []byte("raw-secret")) || !bytes.Contains(response.Payload, []byte("[REDACTED]")) {
		t.Fatalf("response did not protect output: %s", response.Payload)
	}
	wantHash, err := DigestJSON([]byte(`{"ok":true,"token":"[REDACTED]"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.outcomes) != 1 || store.outcomes[0].OutputHash != wantHash {
		t.Fatalf("outcomes = %+v, want hash %q", store.outcomes, wantHash)
	}
}

func TestGatewayFailsClosedWhenEvidenceCannotCommit(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	store := &memoryEventAppender{err: errors.New("disk unavailable")}
	gateway := testGateway(policy, request, store)
	call := testMessage(MessageToolCall, "call:failure", "", gatewayToolCall(request, request.Arguments))
	var payload ToolCallPayload
	_ = json.Unmarshal(call.Payload, &payload)
	if _, err := gateway.HandleToolCall(context.Background(), call, payload); err == nil || !strings.Contains(err.Error(), "persist decision evidence") {
		t.Fatalf("error = %v", err)
	}
}

func TestGatewayDoesNotExecuteInvalidInput(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	store := &memoryEventAppender{}
	gateway := testGateway(policy, request, store)
	gateway.Executor = panicToolExecutor{}
	invalid := json.RawMessage(`{"fileId":"google:agent-runbook","unexpected":true}`)
	call := testMessage(MessageToolCall, "call:invalid", "", gatewayToolCall(request, invalid))
	response, err := gateway.HandleToolCall(context.Background(), call, gatewayToolCall(request, invalid))
	if err != nil {
		t.Fatal(err)
	}
	var result ToolResultPayload
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "not_executed" || result.Decision.ReasonCode != "schema:invalid_input" || len(store.outcomes) != 0 {
		t.Fatalf("result=%+v outcomes=%+v", result, store.outcomes)
	}
}

func TestGatewayRecordsExecutorFailureAsOutcome(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	store := &memoryEventAppender{}
	gateway := testGateway(policy, request, store)
	gateway.Executor = fakeToolExecutor{err: &ToolExecutionError{Code: "executor:timeout", Err: context.DeadlineExceeded}}
	call := testMessage(MessageToolCall, "call:timeout", "", gatewayToolCall(request, request.Arguments))
	response, err := gateway.HandleToolCall(context.Background(), call, gatewayToolCall(request, request.Arguments))
	if err != nil {
		t.Fatal(err)
	}
	var result ToolResultPayload
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || result.ErrorCode != "executor:timeout" || len(store.outcomes) != 1 || store.outcomes[0].Outcome.ErrorCode != "executor:timeout" {
		t.Fatalf("result=%+v outcomes=%+v", result, store.outcomes)
	}
}

func TestGatewayWithholdsResultWhenOutcomeCannotCommit(t *testing.T) {
	policy, request := policyRequest(t, []PolicyRule{policyRule("rule:allow", "allow")})
	store := &memoryEventAppender{outcomeErr: errors.New("disk full")}
	gateway := testGateway(policy, request, store)
	call := testMessage(MessageToolCall, "call:outcome-failure", "", gatewayToolCall(request, request.Arguments))
	if _, err := gateway.HandleToolCall(context.Background(), call, gatewayToolCall(request, request.Arguments)); err == nil || !strings.Contains(err.Error(), "persist tool outcome") {
		t.Fatalf("error = %v", err)
	}
	if len(store.events) != 1 || len(store.outcomes) != 0 {
		t.Fatalf("events=%+v outcomes=%+v", store.events, store.outcomes)
	}
}

func TestSessionUsesGovernedGatewayHandler(t *testing.T) {
	config := testSessionConfig(t, "tool")
	config.HandshakeTimeout = 2 * time.Second
	config.SessionTimeout = 15 * time.Second
	manifest := validToolManifest()
	manifest.Metadata.Name = "google.drive.read"
	manifest.Metadata.Version = "0.1.0"
	manifest.Spec.InputSchema = policyInputSchema()
	manifest.Spec.SensitiveFields = nil
	manifest.Spec.Permissions = []Permission{{Tenant: "tenant:northstar", Actions: []string{"drive.files.get"}, Resources: []string{"google:agent-runbook"}}}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Spec.Transport.Command = []string{executable, "-test.run=^TestToolExecutionHelperProcess$"}
	manifestDigest, err := DigestToolManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	policy := GovernancePolicy{APIVersion: APIVersion, Kind: GovernancePolicyKind, Version: "0.1.0", DefaultEffect: "deny", Rules: []PolicyRule{policyRule("rule:allow", "allow")}}
	policyDigest, err := DigestGovernancePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	config.Run.Tools[0] = ToolRef{Name: manifest.Metadata.Name, Version: manifest.Metadata.Version, Digest: manifestDigest}
	config.Run.Policy = PolicyRef{Version: policy.Version, Digest: policyDigest}
	store := &memoryEventAppender{}
	gateway := &Gateway{
		Run: config.Run, Actor: ActorRef{ID: config.Run.Agent.ID, Tenant: "tenant:northstar", Type: "agent"}, Policy: policy,
		Resolver: ToolCallResolverFunc(func(context.Context, Message, ToolCallPayload) (ToolCallResolution, error) {
			return ToolCallResolution{Manifest: manifest, Action: "drive.files.get", Resource: ResourceRef{ID: "google:agent-runbook", Tenant: "tenant:northstar", Classification: "restricted"}}, nil
		}),
		Executor: SubprocessToolExecutor{
			Directory: t.TempDir(), Environment: []string{toolHelperMode + "=success"}, CleanupTimeout: 250 * time.Millisecond,
		},
		Events: store, Clock: func() time.Time { return time.Date(2026, 7, 12, 22, 0, 0, 0, time.UTC) },
	}
	result, err := RunSession(context.Background(), config, gateway)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completion.Status != "completed" || len(store.events) != 1 || len(store.outcomes) != 1 || store.outcomes[0].Outcome.Status != "succeeded" {
		t.Fatalf("result=%+v events=%+v", result, store.events)
	}
}

type memoryEventAppender struct {
	events     []DecisionEvent
	outcomes   []ToolOutcomeEvent
	err        error
	outcomeErr error
}

func (s *memoryEventAppender) AppendDecisionEvent(_ context.Context, draft DecisionEventDraft) (DecisionEvent, error) {
	if s.err != nil {
		return DecisionEvent{}, s.err
	}
	event, err := BuildDecisionEvent(draft, deterministicIdentifier("event", draft.CorrelationID), uint64(len(s.events)+1))
	if err != nil {
		return DecisionEvent{}, err
	}
	s.events = append(s.events, event)
	return event, nil
}

func (s *memoryEventAppender) AppendToolOutcomeEvent(_ context.Context, draft ToolOutcomeEventDraft) (ToolOutcomeEvent, error) {
	if s.outcomeErr != nil {
		return ToolOutcomeEvent{}, s.outcomeErr
	}
	if s.err != nil {
		return ToolOutcomeEvent{}, s.err
	}
	event, err := BuildToolOutcomeEvent(draft, deterministicIdentifier("outcome", draft.CorrelationID))
	if err != nil {
		return ToolOutcomeEvent{}, err
	}
	s.outcomes = append(s.outcomes, event)
	return event, nil
}

type panicToolExecutor struct{}

func (panicToolExecutor) Execute(context.Context, ToolManifest, ToolExecutionRequest) (ToolExecutionResult, error) {
	panic("executor must not be called")
}

type fakeToolExecutor struct {
	result ToolExecutionResult
	err    error
}

func (e fakeToolExecutor) Execute(context.Context, ToolManifest, ToolExecutionRequest) (ToolExecutionResult, error) {
	if e.err != nil {
		return ToolExecutionResult{}, e.err
	}
	if e.result.Status != "" {
		return e.result, nil
	}
	return ToolExecutionResult{Status: "succeeded", Content: json.RawMessage(`{"ok":true}`)}, nil
}

func testGateway(policy GovernancePolicy, request AuthorizationRequest, store ToolEvidenceAppender) *Gateway {
	return &Gateway{
		Run: request.Run, Actor: request.Actor, Policy: policy,
		Resolver: ToolCallResolverFunc(func(context.Context, Message, ToolCallPayload) (ToolCallResolution, error) {
			return ToolCallResolution{Manifest: request.Manifest, Action: request.Action, Resource: request.Resource}, nil
		}),
		Executor: fakeToolExecutor{}, Events: store, Clock: func() time.Time { return time.Date(2026, 7, 12, 22, 0, 0, 0, time.UTC) },
	}
}

func gatewayToolCall(request AuthorizationRequest, arguments json.RawMessage) ToolCallPayload {
	return ToolCallPayload{Tool: request.Manifest.Metadata.Name, Action: request.Action, Resource: request.Resource.ID, Arguments: arguments}
}

func policyRequest(t *testing.T, rules []PolicyRule) (GovernancePolicy, AuthorizationRequest) {
	t.Helper()
	manifest := validToolManifest()
	manifest.Metadata.Name = "google.drive.read"
	manifest.Metadata.Version = "0.1.0"
	manifest.Spec.InputSchema = policyInputSchema()
	manifest.Spec.Permissions = []Permission{{Tenant: "tenant:northstar", Actions: []string{"drive.files.get"}, Resources: []string{"google:agent-runbook"}}}
	manifest.Spec.SensitiveFields = nil
	manifestDigest, err := DigestToolManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	policy := GovernancePolicy{APIVersion: APIVersion, Kind: GovernancePolicyKind, Version: "0.1.0", DefaultEffect: "deny", Rules: append([]PolicyRule{}, rules...)}
	policyDigest, err := DigestGovernancePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	run := AgentRun{
		APIVersion: APIVersion, Kind: AgentRunKind, RunID: "run:gateway", TrialID: "trial:1",
		Scenario: ScenarioRef{Name: "acquisition-agent", Version: "0.1.0", Digest: testDigest, Seed: 42},
		Agent:    AgentRef{ID: "agent:reference", Version: "0.1.0", Adapter: "subprocess", Provider: "cloudailab", Model: "deterministic-reference"},
		Policy:   PolicyRef{Version: policy.Version, Digest: policyDigest}, PromptHash: testDigest,
		Tools: []ToolRef{{Name: manifest.Metadata.Name, Version: manifest.Metadata.Version, Digest: manifestDigest}},
		Trial: TrialRef{Index: 1, Count: 1}, Status: "running", StartedAt: time.Date(2026, 7, 12, 21, 0, 0, 0, time.UTC),
	}
	request := AuthorizationRequest{
		Run: run, Actor: ActorRef{ID: run.Agent.ID, Tenant: "tenant:northstar", Type: "agent"}, Manifest: manifest,
		Action: "drive.files.get", Resource: ResourceRef{ID: "google:agent-runbook", Tenant: "tenant:northstar", Classification: "restricted"},
		CorrelationID: "call:1", Arguments: json.RawMessage(`{"token":"synthetic-secret","nested":{"password":"private"},"fileId":"google:agent-runbook"}`),
	}
	return policy, request
}

func policyRule(id, effect string, redactions ...string) PolicyRule {
	return PolicyRule{
		ID: id, Effect: effect, AgentID: "agent:reference", Tool: "google.drive.read", Action: "drive.files.get",
		Resource: "google:agent-runbook", ResourceTenant: "tenant:northstar", ResourceClassification: "restricted",
		Redactions: redactions,
	}
}

func policyInputSchema() json.RawMessage {
	return json.RawMessage(`{
      "$schema":"https://json-schema.org/draft/2020-12/schema",
      "type":"object",
      "properties":{
        "fileId":{"type":"string"},
        "token":{"type":"string"},
        "nested":{"type":"object","properties":{"password":{"type":"string"}},"additionalProperties":false}
      },
      "required":["fileId"],
      "additionalProperties":false
    }`)
}
