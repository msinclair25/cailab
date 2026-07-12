package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestToolManifestValidationAndStableDigest(t *testing.T) {
	manifest := validToolManifest()
	if err := ValidateToolManifest(manifest); err != nil {
		t.Fatal(err)
	}
	first, err := DigestToolManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Spec.InputSchema = json.RawMessage(`{
      "additionalProperties": false,
      "required": ["query"],
      "properties": {"query": {"type": "string"}},
      "type": "object",
      "$schema": "https://json-schema.org/draft/2020-12/schema"
    }`)
	second, err := DigestToolManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("tool digests first=%q second=%q", first, second)
	}
}

func TestToolManifestRejectsUnsafeOrAmbiguousContracts(t *testing.T) {
	tests := map[string]func(*ToolManifest){
		"missing permissions": func(manifest *ToolManifest) { manifest.Spec.Permissions = nil },
		"host line break":     func(manifest *ToolManifest) { manifest.Spec.Transport.Command[0] = "tool\nnext" },
		"open schema": func(manifest *ToolManifest) {
			manifest.Spec.InputSchema = json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}`)
		},
		"invalid isolation": func(manifest *ToolManifest) { manifest.Spec.Isolation.Network = "internet" },
		"duplicate action": func(manifest *ToolManifest) {
			manifest.Spec.Permissions[0].Actions = []string{"drive.files.get", "drive.files.get"}
		},
		"invalid pointer": func(manifest *ToolManifest) { manifest.Spec.SensitiveFields = []string{"password"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			manifest := validToolManifest()
			mutate(&manifest)
			if err := ValidateToolManifest(manifest); err == nil {
				t.Fatal("validation succeeded")
			}
		})
	}
}

func TestStrictDecodingRejectsUnknownAndDuplicateFields(t *testing.T) {
	manifest := validToolManifest()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	unknown := bytes.Replace(data, []byte(`"kind":"ToolManifest"`), []byte(`"kind":"ToolManifest","unexpected":true`), 1)
	if _, err := DecodeToolManifest(unknown); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
	duplicate := []byte(`{"apiVersion":"cloudailab.dev/agent/v1alpha1","apiVersion":"other","kind":"ToolManifest"}`)
	if _, err := DecodeToolManifest(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate field error = %v", err)
	}
}

func TestAgentRunAndDecisionEventContracts(t *testing.T) {
	started := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	ended := started.Add(time.Minute)
	run := AgentRun{
		APIVersion: APIVersion, Kind: AgentRunKind, RunID: "run:flagship", TrialID: "trial:1",
		Scenario: ScenarioRef{Name: "acquisition-agent", Version: "0.1.0", Digest: testDigest, Seed: 42},
		Agent:    AgentRef{ID: "agent:reference", Version: "0.1.0", Adapter: "subprocess", Provider: "cloudailab", Model: "deterministic-reference"},
		Policy:   PolicyRef{Version: "0.1.0", Digest: testDigest}, PromptHash: testDigest,
		Tools: []ToolRef{{Name: "google.drive.read", Version: "0.1.0", Digest: testDigest}},
		Trial: TrialRef{Index: 1, Count: 3}, Status: "completed", StartedAt: started, EndedAt: &ended,
	}
	if err := ValidateAgentRun(run); err != nil {
		t.Fatal(err)
	}
	runData, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAgentRun(runData); err != nil {
		t.Fatal(err)
	}
	event := DecisionEvent{
		APIVersion: APIVersion, Kind: DecisionEventKind, EventID: "event:1", Sequence: 1,
		OccurredAt: started, RunID: run.RunID, TrialID: run.TrialID, CorrelationID: "call:1",
		Actor: ActorRef{ID: run.Agent.ID, Tenant: "tenant:northstar", Type: "agent"},
		Tool:  run.Tools[0], Action: "drive.files.get",
		Resource: ResourceRef{ID: "google:agent-runbook", Tenant: "tenant:northstar", Classification: "internal"},
		Decision: Decision{Effect: "allow", ReasonCode: "policy:allowed", PolicyVersion: "0.1.0"},
		Outcome:  Outcome{Status: "succeeded"}, InputHash: testDigest, OutputHash: testDigest,
	}
	if err := ValidateDecisionEvent(event); err != nil {
		t.Fatal(err)
	}
	eventData, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDecisionEvent(eventData); err != nil {
		t.Fatal(err)
	}
	event.Decision.Effect = "deny"
	if err := ValidateDecisionEvent(event); err == nil || !strings.Contains(err.Error(), "not_executed") {
		t.Fatalf("deny outcome error = %v", err)
	}
}

func TestDecisionEffectsAreStructurallyDistinct(t *testing.T) {
	tests := []Decision{
		{Effect: "allow", ReasonCode: "policy:allow", PolicyVersion: "0.1.0"},
		{Effect: "deny", ReasonCode: "policy:deny", PolicyVersion: "0.1.0"},
		{Effect: "redact", ReasonCode: "policy:redact", PolicyVersion: "0.1.0", Redactions: []string{"/token"}},
		{Effect: "require_approval", ReasonCode: "policy:approval", PolicyVersion: "0.1.0", ApprovalID: "approval:1"},
	}
	for _, decision := range tests {
		if err := ValidateDecision(decision); err != nil {
			t.Fatalf("%s: %v", decision.Effect, err)
		}
	}
	invalid := Decision{Effect: "allow", ReasonCode: "policy:allow", PolicyVersion: "0.1.0", ApprovalID: "approval:1"}
	if err := ValidateDecision(invalid); err == nil {
		t.Fatal("allow decision accepted approvalId")
	}
}

func TestJSONLinesCodecAndCorrelation(t *testing.T) {
	start := message(t, MessageSessionStart, "message:start", "", SessionStartPayload{
		RunID: "run:flagship", TrialID: "trial:1", ScenarioDigest: testDigest, PolicyVersion: "0.1.0",
		Tools: []ToolRef{{Name: "google.drive.read", Version: "0.1.0", Digest: testDigest}},
	})
	call := message(t, MessageToolCall, "call:1", "", ToolCallPayload{Tool: "google.drive.read", Arguments: json.RawMessage(`{"fileId":"drive_file_agent_runbook"}`)})
	var stream bytes.Buffer
	encoder := NewEncoder(&stream)
	if err := encoder.Write(start); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Write(call); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(stream.String(), "\n"); lines != 2 {
		t.Fatalf("JSON Lines frame count = %d", lines)
	}
	decoder := NewDecoder(&stream)
	for _, want := range []string{MessageSessionStart, MessageToolCall} {
		got, err := decoder.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got.Type != want {
			t.Fatalf("message type = %q, want %q", got.Type, want)
		}
	}
	if _, err := decoder.Next(); err != io.EOF {
		t.Fatalf("end of stream error = %v", err)
	}
	result := messageForFuzz(MessageToolResult, "result:1", "", ToolResultPayload{
		Tool: "google.drive.read", Status: "succeeded", Content: json.RawMessage(`{"ok":true}`),
		Decision: Decision{Effect: "allow", ReasonCode: "policy:allow", PolicyVersion: "0.1.0"},
	})
	if err := ValidateMessage(result); err == nil || !strings.Contains(err.Error(), "correlationId") {
		t.Fatalf("missing correlation error = %v", err)
	}
}

func TestProtocolRejectsOversizedAndInvalidUTF8Frames(t *testing.T) {
	oversized := bytes.Repeat([]byte{'x'}, MaxFrameBytes+1)
	if _, err := NewDecoder(bytes.NewReader(append(oversized, '\n'))).Next(); err == nil {
		t.Fatal("oversized frame accepted")
	}
	if _, err := NewDecoder(bytes.NewReader([]byte{0xff, '\n'})).Next(); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
}

func TestCommittedAgentSchemasAreValidJSON(t *testing.T) {
	for _, name := range []string{"tool-manifest.json", "agent-run.json", "protocol-message.json", "decision-event.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "agent", "v1alpha1", name))
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("%s has unexpected $schema", name)
		}
	}
}

func TestRedactJSONUsesRFC6901PointersWithoutMutatingInput(t *testing.T) {
	input := []byte(`{"token":"secret","nested":{"a/b":"private","keep":true},"items":[{"password":"p"}]}`)
	redacted, err := RedactJSON(input, []string{"/token", "/nested/a~1b", "/items/0/password"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"items":[{"password":"[REDACTED]"}],"nested":{"a/b":"[REDACTED]","keep":true},"token":"[REDACTED]"}`
	if string(redacted) != want {
		t.Fatalf("redacted JSON = %s", redacted)
	}
	if !bytes.Contains(input, []byte(`"secret"`)) {
		t.Fatal("input was mutated")
	}
	if _, err := RedactJSON(input, []string{"/missing"}); err == nil {
		t.Fatal("missing pointer accepted")
	}
}

func FuzzProtocolDecoder(f *testing.F) {
	retryable := false
	valid := messageForFuzz(MessageProtocolError, "message:error", "", ProtocolErrorPayload{Code: "bad_request", Message: "invalid", Retryable: &retryable})
	data, _ := json.Marshal(valid)
	f.Add(append(data, '\n'))
	f.Add([]byte("not-json\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxFrameBytes+1 {
			t.Skip()
		}
		_, _ = NewDecoder(bytes.NewReader(input)).Next()
	})
}

func validToolManifest() ToolManifest {
	return ToolManifest{
		APIVersion: APIVersion, Kind: ToolManifestKind,
		Metadata: Metadata{Name: "google.drive.read", Version: "0.1.0", Description: "Read one declared synthetic Drive file."},
		Spec: ToolManifestSpec{
			Transport: ToolTransport{Type: "subprocess", Command: []string{"/usr/bin/cailab-google-tool"}},
			InputSchema: json.RawMessage(`{
        "$schema":"https://json-schema.org/draft/2020-12/schema",
        "type":"object",
        "properties":{"query":{"type":"string"}},
        "required":["query"],
        "additionalProperties":false
      }`),
			Permissions: []Permission{{Tenant: "tenant:northstar", Actions: []string{"drive.files.get"}, Resources: []string{"google:agent-runbook"}}},
			Risk:        "medium", TimeoutMillis: 5000,
			Isolation: Isolation{Network: "loopback", Filesystem: "none"}, SensitiveFields: []string{"/token"},
		},
	}
}

func message(t *testing.T, messageType, id, correlation string, payload any) Message {
	t.Helper()
	message := messageForFuzz(messageType, id, correlation, payload)
	if err := ValidateMessage(message); err != nil {
		t.Fatal(err)
	}
	return message
}

func messageForFuzz(messageType, id, correlation string, payload any) Message {
	data, _ := json.Marshal(payload)
	return Message{ProtocolVersion: ProtocolVersion, ID: id, Type: messageType, CorrelationID: correlation, Payload: data}
}
