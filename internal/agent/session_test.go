package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const helperModeEnvironment = "CAILAB_TEST_AGENT_MODE"

func TestRunSessionWithDeterministicReferenceAgent(t *testing.T) {
	t.Setenv("CAILAB_PARENT_SECRET", "must-not-reach-child")
	config := testSessionConfig(t, "reference")
	config.Environment = append(config.Environment, "CAILAB_EXPECTED_DIRECTORY="+config.Directory)
	result, err := RunSession(context.Background(), config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completion.Status != "completed" || len(result.Messages) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Messages[0].Type != MessageAgentReady || result.Messages[1].Type != MessageSessionComplete {
		t.Fatalf("message types = %s, %s", result.Messages[0].Type, result.Messages[1].Type)
	}
}

func TestReferenceAgentOutputIsByteStable(t *testing.T) {
	start := sessionStartMessage(testSessionConfig(t, "reference").Run)
	var framed bytes.Buffer
	if err := NewEncoder(&framed).Write(start); err != nil {
		t.Fatal(err)
	}
	run := func() string {
		t.Helper()
		var output bytes.Buffer
		if err := ServeReferenceAgent(context.Background(), bytes.NewReader(framed.Bytes()), &output, ReferenceAgentConfig{
			ID: "agent:reference", Version: "0.1.0",
		}); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	first, second := run(), run()
	if first != second {
		t.Fatalf("reference output changed:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestRunSessionDispatchesDeclaredToolAndCorrelatesResult(t *testing.T) {
	config := testSessionConfig(t, "tool")
	called := false
	handler := ToolCallHandlerFunc(func(_ context.Context, call Message, payload ToolCallPayload) (Message, error) {
		called = true
		if payload.Tool != "google.drive.read" || string(payload.Arguments) != `{"fileId":"google:file"}` {
			t.Fatalf("tool payload = %+v", payload)
		}
		return testMessage(MessageToolResult, "message:result", call.ID, ToolResultPayload{
			Tool: "google.drive.read", Status: "succeeded", Content: json.RawMessage(`{"ok":true}`),
			Decision: Decision{Effect: "allow", ReasonCode: "policy:allow", PolicyVersion: "0.1.0"},
		}), nil
	})
	result, err := RunSession(context.Background(), config, handler)
	if err != nil {
		t.Fatal(err)
	}
	if !called || result.Completion.Status != "completed" || len(result.Messages) != 3 {
		t.Fatalf("called=%v result=%+v", called, result)
	}
}

func TestRunSessionDispatchesApprovalRequirement(t *testing.T) {
	config := testSessionConfig(t, "approval")
	handler := ToolCallHandlerFunc(func(_ context.Context, call Message, _ ToolCallPayload) (Message, error) {
		return testMessage(MessageApprovalRequired, "message:approval", call.ID, ApprovalRequiredPayload{
			ApprovalID: "approval:1", ToolCallID: call.ID, Reason: "restricted data requires explicit approval",
		}), nil
	})
	result, err := RunSession(context.Background(), config, handler)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completion.Status != "completed" {
		t.Fatalf("completion = %+v", result.Completion)
	}
}

func TestRunSessionRejectsLifecycleAndProtocolViolations(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "message before ready", mode: "wrong-order"},
		{name: "malformed stdout", mode: "malformed"},
		{name: "duplicate id", mode: "duplicate"},
		{name: "wrong identity", mode: "wrong-identity"},
		{name: "controller-only direction", mode: "wrong-direction"},
		{name: "undeclared tool", mode: "undeclared-tool"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := RunSession(context.Background(), testSessionConfig(t, test.mode), nil)
			if !errors.Is(err, ErrProtocolViolation) {
				t.Fatalf("error = %v, want protocol violation", err)
			}
		})
	}
}

func TestRunSessionEnforcesHandshakeSessionAndParentCancellation(t *testing.T) {
	t.Run("handshake", func(t *testing.T) {
		config := testSessionConfig(t, "silent")
		config.HandshakeTimeout = 30 * time.Millisecond
		_, err := RunSession(context.Background(), config, nil)
		if !errors.Is(err, ErrHandshakeTimeout) {
			t.Fatalf("error = %v, want handshake timeout", err)
		}
	})

	t.Run("session", func(t *testing.T) {
		config := testSessionConfig(t, "ready-hang")
		config.HandshakeTimeout = 100 * time.Millisecond
		config.SessionTimeout = 50 * time.Millisecond
		_, err := RunSession(context.Background(), config, nil)
		if !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("error = %v, want invalid timeout ordering", err)
		}

		config.SessionTimeout = 120 * time.Millisecond
		_, err = RunSession(context.Background(), config, nil)
		if !errors.Is(err, ErrSessionTimeout) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want session deadline", err)
		}
	})

	t.Run("wait after completion", func(t *testing.T) {
		config := testSessionConfig(t, "complete-hang")
		config.HandshakeTimeout = 50 * time.Millisecond
		config.SessionTimeout = 120 * time.Millisecond
		_, err := RunSession(context.Background(), config, nil)
		if !errors.Is(err, ErrSessionTimeout) {
			t.Fatalf("error = %v, want session timeout", err)
		}
	})

	t.Run("handler ignores cancellation", func(t *testing.T) {
		config := testSessionConfig(t, "tool")
		config.HandshakeTimeout = 50 * time.Millisecond
		config.SessionTimeout = 120 * time.Millisecond
		handler := ToolCallHandlerFunc(func(context.Context, Message, ToolCallPayload) (Message, error) {
			time.Sleep(500 * time.Millisecond)
			return Message{}, nil
		})
		_, err := RunSession(context.Background(), config, handler)
		if !errors.Is(err, ErrSessionTimeout) {
			t.Fatalf("error = %v, want session timeout", err)
		}
	})

	t.Run("parent cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		config := testSessionConfig(t, "silent")
		config.HandshakeTimeout = time.Second
		time.AfterFunc(30*time.Millisecond, cancel)
		_, err := RunSession(ctx, config, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
	})
}

func TestRunSessionBoundsStderrAndReportsUnexpectedExit(t *testing.T) {
	t.Run("bounded diagnostics", func(t *testing.T) {
		config := testSessionConfig(t, "stderr")
		config.MaxStderrBytes = 32
		result, err := RunSession(context.Background(), config, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Stderr) != 32 || !result.StderrTruncated || result.Stderr != strings.Repeat("x", 32) {
			t.Fatalf("stderr len=%d truncated=%v value=%q", len(result.Stderr), result.StderrTruncated, result.Stderr)
		}
	})

	t.Run("exit status", func(t *testing.T) {
		_, err := RunSession(context.Background(), testSessionConfig(t, "exit"), nil)
		if !errors.Is(err, ErrAgentExit) {
			t.Fatalf("error = %v, want agent exit", err)
		}
		if strings.Contains(err.Error(), "intentional helper exit") {
			t.Fatalf("formatted error leaked stderr: %v", err)
		}
		var sessionError *SessionError
		if !errors.As(err, &sessionError) || !strings.Contains(sessionError.Stderr, "intentional helper exit") {
			t.Fatalf("bounded diagnostic field missing: %#v", sessionError)
		}
	})
}

func TestRunSessionValidatesLaunchBoundaryAndHandlerResponse(t *testing.T) {
	config := testSessionConfig(t, "reference")
	config.Command[0] = "relative-agent"
	if _, err := RunSession(context.Background(), config, nil); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("relative command error = %v", err)
	}

	config = testSessionConfig(t, "reference")
	config.Environment = []string{"TOKEN=one", "token=two"}
	if _, err := RunSession(context.Background(), config, nil); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("duplicate environment error = %v", err)
	}

	config = testSessionConfig(t, "tool")
	handler := ToolCallHandlerFunc(func(_ context.Context, _ Message, _ ToolCallPayload) (Message, error) {
		return testMessage(MessageToolResult, "message:result", "call:other", ToolResultPayload{
			Tool: "google.drive.read", Status: "not_executed",
			Decision: Decision{Effect: "deny", ReasonCode: "policy:deny", PolicyVersion: "0.1.0"},
		}), nil
	})
	if _, err := RunSession(context.Background(), config, handler); !errors.Is(err, ErrProtocolViolation) {
		t.Fatalf("correlation error = %v", err)
	}

	config = testSessionConfig(t, "tool")
	handler = ToolCallHandlerFunc(func(_ context.Context, call Message, _ ToolCallPayload) (Message, error) {
		return testMessage(MessageToolResult, "message:result", call.ID, ToolResultPayload{
			Tool: "microsoft.graph.write", Status: "not_executed",
			Decision: Decision{Effect: "deny", ReasonCode: "policy:deny", PolicyVersion: "0.1.0"},
		}), nil
	})
	if _, err := RunSession(context.Background(), config, handler); !errors.Is(err, ErrProtocolViolation) || !strings.Contains(err.Error(), "correlated tool") {
		t.Fatalf("tool-name error = %v", err)
	}
}

func TestRunSessionEnforcesMessageLimit(t *testing.T) {
	config := testSessionConfig(t, "tool")
	config.MaxMessages = 2
	handler := ToolCallHandlerFunc(func(_ context.Context, call Message, _ ToolCallPayload) (Message, error) {
		return testMessage(MessageToolResult, "message:result", call.ID, ToolResultPayload{
			Tool: "google.drive.read", Status: "not_executed",
			Decision: Decision{Effect: "deny", ReasonCode: "policy:deny", PolicyVersion: "0.1.0"},
		}), nil
	})
	if _, err := RunSession(context.Background(), config, handler); !errors.Is(err, ErrProtocolViolation) || !strings.Contains(err.Error(), "message count") {
		t.Fatalf("error = %v, want message-count violation", err)
	}
}

func TestRunSessionEnforcesTranscriptByteLimit(t *testing.T) {
	config := testSessionConfig(t, "large-complete")
	config.MaxTranscriptBytes = MaxFrameBytes
	if _, err := RunSession(context.Background(), config, nil); !errors.Is(err, ErrProtocolViolation) || !strings.Contains(err.Error(), "transcript") {
		t.Fatalf("error = %v, want transcript-size violation", err)
	}
}

func TestSessionHelperProcess(t *testing.T) {
	mode := os.Getenv(helperModeEnvironment)
	if mode == "" {
		return
	}
	if os.Getenv("CAILAB_PARENT_SECRET") != "" {
		fmt.Fprintln(os.Stderr, "inherited parent environment")
		os.Exit(7)
	}
	if expected := os.Getenv("CAILAB_EXPECTED_DIRECTORY"); expected != "" {
		workingDirectory, err := os.Getwd()
		workingInfo, workingErr := os.Stat(workingDirectory)
		expectedInfo, expectedErr := os.Stat(expected)
		if err != nil || workingErr != nil || expectedErr != nil || !os.SameFile(workingInfo, expectedInfo) {
			fmt.Fprintln(os.Stderr, "unexpected working directory")
			os.Exit(8)
		}
	}
	if mode == "reference" || mode == "stderr" {
		if mode == "stderr" {
			fmt.Fprint(os.Stderr, strings.Repeat("x", 256))
		}
		if err := ServeReferenceAgent(context.Background(), os.Stdin, os.Stdout, ReferenceAgentConfig{ID: "agent:reference", Version: "0.1.0"}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		return
	}
	input := NewDecoder(os.Stdin)
	start, err := input.Next()
	if err != nil || start.Type != MessageSessionStart {
		fmt.Fprintln(os.Stderr, "invalid test session start")
		os.Exit(2)
	}
	if mode == "silent" {
		time.Sleep(10 * time.Second)
		return
	}
	if mode == "wrong-order" {
		mustWriteHelper(testMessage(MessageSessionComplete, "message:complete", "", SessionCompletePayload{Status: "completed"}))
		return
	}
	if mode == "malformed" {
		fmt.Fprintln(os.Stdout, "not-json")
		return
	}
	if mode == "exit" {
		fmt.Fprintln(os.Stderr, "intentional helper exit")
		os.Exit(17)
	}
	ready := testMessage(MessageAgentReady, "message:ready", "", AgentReadyPayload{AgentID: "agent:reference", AgentVersion: "0.1.0"})
	if mode == "wrong-identity" {
		ready = testMessage(MessageAgentReady, "message:ready", "", AgentReadyPayload{AgentID: "agent:other", AgentVersion: "0.1.0"})
	}
	mustWriteHelper(ready)
	switch mode {
	case "ready-hang":
		time.Sleep(10 * time.Second)
	case "complete-hang":
		mustWriteHelper(testMessage(MessageSessionComplete, "message:complete", "", SessionCompletePayload{Status: "completed"}))
		time.Sleep(10 * time.Second)
	case "large-complete":
		empty := testMessage(MessageSessionComplete, "message:complete", "", SessionCompletePayload{Status: "completed"})
		summaryBytes := MaxFrameBytes - encodedMessageSize(empty) - 16
		mustWriteHelper(testMessage(MessageSessionComplete, "message:complete", "", SessionCompletePayload{
			Status: "completed", Summary: strings.Repeat("z", summaryBytes),
		}))
	case "duplicate":
		mustWriteHelper(ready)
	case "wrong-direction":
		mustWriteHelper(testMessage(MessageToolResult, "message:result", "call:1", ToolResultPayload{
			Tool: "google.drive.read", Status: "not_executed",
			Decision: Decision{Effect: "deny", ReasonCode: "policy:deny", PolicyVersion: "0.1.0"},
		}))
	case "undeclared-tool":
		mustWriteHelper(testMessage(MessageToolCall, "call:1", "", ToolCallPayload{Tool: "microsoft.graph.write", Action: "graph.write", Resource: "microsoft:directory", Arguments: json.RawMessage(`{}`)}))
	case "tool", "approval":
		call := testMessage(MessageToolCall, "call:1", "", ToolCallPayload{Tool: "google.drive.read", Action: "drive.files.get", Resource: "google:file", Arguments: json.RawMessage(`{"fileId":"google:file"}`)})
		mustWriteHelper(call)
		response, err := input.Next()
		expectedType := MessageToolResult
		if mode == "approval" {
			expectedType = MessageApprovalRequired
		}
		if err != nil || response.Type != expectedType || response.CorrelationID != call.ID {
			fmt.Fprintln(os.Stderr, "invalid test tool response")
			os.Exit(4)
		}
		mustWriteHelper(testMessage(MessageSessionComplete, "message:complete", "", SessionCompletePayload{Status: "completed"}))
	default:
		fmt.Fprintln(os.Stderr, "unsupported test helper mode")
		os.Exit(5)
	}
}

func mustWriteHelper(message Message) {
	if err := NewEncoder(os.Stdout).Write(message); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
}

func testSessionConfig(t *testing.T, mode string) SessionConfig {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return SessionConfig{
		Command:     []string{executable, "-test.run=^TestSessionHelperProcess$"},
		Directory:   t.TempDir(),
		Environment: []string{helperModeEnvironment + "=" + mode},
		Run: AgentRun{
			APIVersion: APIVersion, Kind: AgentRunKind, RunID: "run:reference", TrialID: "trial:1",
			Scenario: ScenarioRef{Name: "acquisition-agent", Version: "0.1.0", Digest: testDigest, Seed: 42},
			Agent:    AgentRef{ID: "agent:reference", Version: "0.1.0", Adapter: "subprocess", Provider: "cloudailab", Model: "deterministic-reference"},
			Policy:   PolicyRef{Version: "0.1.0", Digest: testDigest}, PromptHash: testDigest,
			Tools: []ToolRef{{Name: "google.drive.read", Version: "0.1.0", Digest: testDigest}},
			Trial: TrialRef{Index: 1, Count: 1}, Status: "planned", StartedAt: time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC),
		},
		HandshakeTimeout:   500 * time.Millisecond,
		SessionTimeout:     2 * time.Second,
		CleanupTimeout:     250 * time.Millisecond,
		MaxStderrBytes:     1 << 10,
		MaxMessages:        100,
		MaxTranscriptBytes: 2 << 20,
	}
}

func testMessage(messageType, id, correlation string, payload any) Message {
	data, _ := json.Marshal(payload)
	return Message{ProtocolVersion: ProtocolVersion, ID: id, Type: messageType, CorrelationID: correlation, Payload: data}
}
