package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const toolHelperMode = "CAILAB_TEST_TOOL_MODE"

func TestSubprocessToolExecutorRunsReferenceTool(t *testing.T) {
	t.Setenv("CAILAB_PARENT_TOOL_SECRET", "must-not-reach-child")
	manifest, executor := toolExecutionFixture(t, "success")
	result, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"fileId":"google:file"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" || string(result.Content) != `{"ok":true,"tool":"google.drive.read"}` {
		t.Fatalf("result = %+v", result)
	}
}

func TestSubprocessToolExecutorReturnsDeclaredFailure(t *testing.T) {
	manifest, executor := toolExecutionFixture(t, "failed")
	result, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"fileId":"google:file"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || result.ErrorCode != "tool:not_found" || len(result.Content) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSubprocessToolExecutorRejectsInvalidInputBeforeLaunch(t *testing.T) {
	manifest, executor := toolExecutionFixture(t, "success")
	manifest.Spec.Transport.Command[0] = "/path/that/does/not/exist"
	_, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"unexpected":true}`)))
	var execution *ToolExecutionError
	if !errors.As(err, &execution) || execution.Code != "executor:invalid_input" || !errors.Is(err, ErrToolInput) {
		t.Fatalf("error = %v", err)
	}
}

func TestSubprocessToolExecutorPreservesPreLaunchFailureCodes(t *testing.T) {
	t.Run("manifest", func(t *testing.T) {
		manifest, executor := toolExecutionFixture(t, "success")
		manifest.APIVersion = "invalid"
		_, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"fileId":"google:file"}`)))
		var execution *ToolExecutionError
		if !errors.As(err, &execution) || execution.Code != "executor:invalid_manifest" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("runtime", func(t *testing.T) {
		manifest, executor := toolExecutionFixture(t, "success")
		manifest.Spec.Transport.Command[0] = "relative-tool"
		_, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"fileId":"google:file"}`)))
		var execution *ToolExecutionError
		if !errors.As(err, &execution) || execution.Code != "executor:invalid_runtime" {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestSubprocessToolExecutorBoundsDiagnostics(t *testing.T) {
	manifest, executor := toolExecutionFixture(t, "stderr")
	executor.MaxStderrBytes = 32
	result, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"fileId":"google:file"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Stderr) != 32 || !result.StderrTruncated || result.Stderr != strings.Repeat("x", 32) {
		t.Fatalf("stderr len=%d truncated=%v value=%q", len(result.Stderr), result.StderrTruncated, result.Stderr)
	}
}

func TestSubprocessToolExecutorRejectsProtocolAndLifecycleFailures(t *testing.T) {
	tests := map[string]string{
		"malformed response":   "invalid_response",
		"invalid utf8":         "invalid_response",
		"extra response":       "invalid_response",
		"oversized output":     "output_limit",
		"wrong correlation":    "correlation_mismatch",
		"nonzero process exit": "process_exit",
		"timeout":              "timeout",
	}
	for mode, codeSuffix := range tests {
		t.Run(mode, func(t *testing.T) {
			manifest, executor := toolExecutionFixture(t, mode)
			if mode == "timeout" {
				manifest.Spec.TimeoutMillis = 100
			}
			_, err := executor.Execute(context.Background(), manifest, toolExecutionRequest(json.RawMessage(`{"fileId":"google:file"}`)))
			var execution *ToolExecutionError
			if !errors.As(err, &execution) || execution.Code != "executor:"+codeSuffix {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestToolInputSchemaValidationIsDraft2020AndOffline(t *testing.T) {
	schema := json.RawMessage(`{
      "$schema":"https://json-schema.org/draft/2020-12/schema",
      "type":"object",
      "$defs":{"identifier":{"type":"string","pattern":"^google:"}},
      "properties":{"fileId":{"$ref":"#/$defs/identifier"}},
      "required":["fileId"],
      "additionalProperties":false
    }`)
	if err := ValidateToolInput(schema, json.RawMessage(`{"fileId":"google:file"}`)); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"missing required":    `{}`,
		"wrong pattern":       `{"fileId":"aws:file"}`,
		"additional property": `{"fileId":"google:file","extra":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateToolInput(schema, json.RawMessage(input)); !errors.Is(err, ErrToolInput) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	remote := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"x":{"$ref":"https://example.com/schema"}},"additionalProperties":false}`)
	if err := ValidateToolInput(remote, json.RawMessage(`{}`)); !errors.Is(err, ErrToolInputSchema) || !strings.Contains(err.Error(), "fragment-local") {
		t.Fatalf("remote reference error = %v", err)
	}
}

func TestToolExecutionHelperProcess(t *testing.T) {
	mode := os.Getenv(toolHelperMode)
	if mode == "" {
		return
	}
	if mode == "timeout" {
		time.Sleep(10 * time.Second)
		return
	}
	if mode == "nonzero process exit" {
		fmt.Fprintln(os.Stderr, "intentional tool exit")
		os.Exit(17)
	}
	if mode == "malformed response" {
		fmt.Fprintln(os.Stdout, "not-json")
		os.Exit(0)
	}
	if mode == "invalid utf8" {
		_, _ = os.Stdout.Write([]byte{0xff, '\n'})
		os.Exit(0)
	}
	if mode == "oversized output" {
		fmt.Fprintln(os.Stdout, strings.Repeat("x", MaxFrameBytes+128))
		os.Exit(0)
	}
	if os.Getenv("CAILAB_PARENT_TOOL_SECRET") != "" {
		fmt.Fprintln(os.Stderr, "inherited parent environment")
		os.Exit(22)
	}
	if expected := os.Getenv("CAILAB_EXPECTED_TOOL_DIRECTORY"); expected != "" {
		workingDirectory, err := os.Getwd()
		workingInfo, workingErr := os.Stat(workingDirectory)
		expectedInfo, expectedErr := os.Stat(expected)
		if err != nil || workingErr != nil || expectedErr != nil || !os.SameFile(workingInfo, expectedInfo) {
			fmt.Fprintln(os.Stderr, "unexpected tool working directory")
			os.Exit(23)
		}
	}
	request := readToolHelperRequest()
	if mode == "stderr" {
		fmt.Fprint(os.Stderr, strings.Repeat("x", 256))
	}
	if mode == "success" || mode == "stderr" {
		writeToolHelperResponse(ToolExecutionResponse{ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "succeeded", Content: json.RawMessage(`{"ok":true,"tool":"google.drive.read"}`)})
		os.Exit(0)
	}
	if mode == "failed" {
		writeToolHelperResponse(ToolExecutionResponse{ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "failed", ErrorCode: "tool:not_found"})
		os.Exit(0)
	}
	if mode == "wrong correlation" {
		writeToolHelperResponse(ToolExecutionResponse{ProtocolVersion: ProtocolVersion, CallID: "call:other", Status: "succeeded", Content: json.RawMessage(`{"ok":true}`)})
		os.Exit(0)
	}
	if mode == "extra response" {
		response := ToolExecutionResponse{ProtocolVersion: ProtocolVersion, CallID: request.CallID, Status: "succeeded", Content: json.RawMessage(`{"ok":true}`)}
		writeToolHelperResponse(response)
		writeToolHelperResponse(response)
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, "unknown tool helper mode")
	os.Exit(18)
}

func readToolHelperRequest() ToolExecutionRequest {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		fmt.Fprintln(os.Stderr, "missing tool request")
		os.Exit(19)
	}
	var request ToolExecutionRequest
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(20)
	}
	return request
}

func writeToolHelperResponse(response ToolExecutionResponse) {
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(21)
	}
	fmt.Fprintln(os.Stdout, string(data))
}

func toolExecutionFixture(t *testing.T, mode string) (ToolManifest, SubprocessToolExecutor) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	manifest := validToolManifest()
	manifest.Metadata.Name = "google.drive.read"
	manifest.Metadata.Version = "0.1.0"
	manifest.Spec.Transport.Command = []string{executable, "-test.run=^TestToolExecutionHelperProcess$"}
	manifest.Spec.InputSchema = json.RawMessage(`{
      "$schema":"https://json-schema.org/draft/2020-12/schema",
      "type":"object",
      "properties":{"fileId":{"type":"string"}},
      "required":["fileId"],
      "additionalProperties":false
    }`)
	manifest.Spec.SensitiveFields = nil
	directory := t.TempDir()
	return manifest, SubprocessToolExecutor{
		Directory: directory, Environment: []string{toolHelperMode + "=" + mode, "CAILAB_EXPECTED_TOOL_DIRECTORY=" + directory},
		CleanupTimeout: 250 * time.Millisecond, MaxStderrBytes: 1 << 10,
	}
}

func toolExecutionRequest(arguments json.RawMessage) ToolExecutionRequest {
	return ToolExecutionRequest{
		ProtocolVersion: ProtocolVersion, CallID: "call:1", Tool: "google.drive.read", Action: "drive.files.get",
		Resource: ResourceRef{ID: "google:file", Tenant: "tenant:northstar", Classification: "restricted"}, Arguments: arguments,
	}
}
