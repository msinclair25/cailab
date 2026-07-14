package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureCreatesClosedScenarioBoundRegistrations(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "config")
	var output bytes.Buffer
	if err := configure(directory, &output); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"policy.json", "prompt.txt", "tool.json"} {
		if info, err := os.Stat(filepath.Join(directory, name)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("configured %s: info=%v err=%v", name, info, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(directory, "tool.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Spec struct {
			Transport struct {
				Command []string `json:"command"`
			} `json:"transport"`
			SensitiveFields []string `json:"sensitiveFields"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Spec.Transport.Command) != 2 || !filepath.IsAbs(manifest.Spec.Transport.Command[0]) || manifest.Spec.Transport.Command[1] != "tool" {
		t.Fatalf("tool command = %q", manifest.Spec.Transport.Command)
	}
	if len(manifest.Spec.SensitiveFields) != 1 || manifest.Spec.SensitiveFields[0] != "/content" {
		t.Fatalf("sensitive fields = %q", manifest.Spec.SensitiveFields)
	}
	for _, name := range []string{"policy.json", "prompt.txt"} {
		generated, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		committed, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if name == "policy.json" {
			generated = compactJSON(t, generated)
			committed = compactJSON(t, committed)
		}
		if !bytes.Equal(generated, committed) {
			t.Fatalf("generated %s differs from the release-packaged source copy", name)
		}
	}
	if err := configure(directory, io.Discard); err == nil {
		t.Fatal("configure overwrote an existing directory")
	}
}

func compactJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func TestAgentPerformsOneGovernedRead(t *testing.T) {
	start := encodeMessage(t, message{
		ProtocolVersion: protocolVersion, ID: "message:start", Type: "session.start",
		Payload: mustJSON(t, sessionStart{RunID: "run:1", TrialID: "trial:1", ScenarioDigest: strings.Repeat("a", 64), PolicyVersion: "0.1.0", Tools: []toolRef{{Name: toolName, Version: "0.1.0", Digest: strings.Repeat("b", 64)}}}),
	})
	result := encodeMessage(t, message{
		ProtocolVersion: protocolVersion, ID: "message:result", Type: "tool.result", CorrelationID: "call:read-runbook",
		Payload: mustJSON(t, toolResult{Tool: toolName, Status: "succeeded", Content: json.RawMessage(`{"content":"[REDACTED]"}`), Decision: decision{Effect: "allow", ReasonCode: "policy:allow", PolicyVersion: "0.1.0"}}),
	})
	var output bytes.Buffer
	if err := serveAgent(context.Background(), strings.NewReader(start+result), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("agent emitted %d frames: %s", len(lines), output.String())
	}
	var call message
	if err := json.Unmarshal([]byte(lines[1]), &call); err != nil {
		t.Fatal(err)
	}
	if call.Type != "tool.call" || call.ID != "call:read-runbook" || bytes.Contains(call.Payload, []byte("REDACTED")) {
		t.Fatalf("tool call = %+v", call)
	}
	var complete message
	if err := json.Unmarshal([]byte(lines[2]), &complete); err != nil {
		t.Fatal(err)
	}
	if complete.Type != "session.complete" {
		t.Fatalf("completion = %+v", complete)
	}
}

func TestToolUsesOnlyFixedLoopbackProviderTarget(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != "/drive/v3/files/drive_file_agent_runbook?alt=media" || request.Header.Get("Authorization") != "Bearer synthetic-token" {
			t.Errorf("provider request = %s auth=%q", request.URL.RequestURI(), request.Header.Get("Authorization"))
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("synthetic content"))
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	request := toolRequest{
		ProtocolVersion: protocolVersion, CallID: "call:1", Tool: toolName, Action: toolAction,
		Resource: resourceRef{ID: toolResource, Tenant: "tenant:northstar", Classification: "internal"}, Arguments: json.RawMessage(`{}`),
	}
	var input, output bytes.Buffer
	if err := json.NewEncoder(&input).Encode(request); err != nil {
		t.Fatal(err)
	}
	if err := serveTool(context.Background(), &input, &output, server.URL, "synthetic-token"); err != nil {
		t.Fatal(err)
	}
	var response toolResponse
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "succeeded" || !bytes.Contains(response.Content, []byte("synthetic content")) {
		t.Fatalf("tool response = %+v", response)
	}
	if err := serveTool(context.Background(), bytes.NewReader(input.Bytes()), io.Discard, "https://example.com", "synthetic-token"); err == nil {
		t.Fatal("tool accepted a non-loopback endpoint")
	}
}

func TestToolFailsClosedForMismatchedTarget(t *testing.T) {
	request := toolRequest{
		ProtocolVersion: protocolVersion, CallID: "call:1", Tool: toolName, Action: toolAction,
		Resource: resourceRef{ID: "google:other", Tenant: "tenant:northstar", Classification: "internal"}, Arguments: json.RawMessage(`{}`),
	}
	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(request); err != nil {
		t.Fatal(err)
	}
	if err := serveTool(context.Background(), &input, io.Discard, "http://127.0.0.1:1234", "synthetic-token"); err == nil {
		t.Fatal("tool accepted a mismatched canonical target")
	}
}

func encodeMessage(t *testing.T, value message) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
