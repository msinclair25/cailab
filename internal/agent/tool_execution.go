package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrToolExecution = errors.New("tool execution failed")

type ToolExecutionResult struct {
	Status          string
	Content         json.RawMessage
	ErrorCode       string
	Stderr          string
	StderrTruncated bool
}

type ToolExecutionError struct {
	Code            string
	Err             error
	Stderr          string
	StderrTruncated bool
}

func (e *ToolExecutionError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	suffix := ""
	if e.StderrTruncated {
		suffix = " (truncated)"
	}
	return fmt.Sprintf("%s: %v; stderr captured: %d bytes%s", e.Code, e.Err, len(e.Stderr), suffix)
}

func (e *ToolExecutionError) Unwrap() error { return e.Err }

type ToolExecutor interface {
	Execute(context.Context, ToolManifest, string, json.RawMessage) (ToolExecutionResult, error)
}

type SubprocessToolExecutor struct {
	Directory      string
	Environment    []string
	CleanupTimeout time.Duration
	MaxStderrBytes int
}

func (e SubprocessToolExecutor) Execute(ctx context.Context, manifest ToolManifest, callID string, arguments json.RawMessage) (ToolExecutionResult, error) {
	if err := ValidateToolManifest(manifest); err != nil {
		return ToolExecutionResult{}, executionError("executor:invalid_manifest", err, nil)
	}
	if err := ValidateToolInput(manifest.Spec.InputSchema, arguments); err != nil {
		return ToolExecutionResult{}, executionError("executor:invalid_input", err, nil)
	}
	if !filepath.IsAbs(manifest.Spec.Transport.Command[0]) {
		return ToolExecutionResult{}, executionError("executor:invalid_command", errors.New("tool executable must be an absolute path"), nil)
	}
	if !filepath.IsAbs(e.Directory) {
		return ToolExecutionResult{}, executionError("executor:invalid_directory", errors.New("tool working directory must be an absolute path"), nil)
	}
	for i, value := range manifest.Spec.Transport.Command {
		if value == "" || strings.ContainsRune(value, 0) || strings.ContainsAny(value, "\r\n") {
			return ToolExecutionResult{}, executionError("executor:invalid_command", fmt.Errorf("command[%d] is unsafe", i), nil)
		}
	}
	if err := validateExplicitEnvironment(e.Environment); err != nil {
		return ToolExecutionResult{}, executionError("executor:invalid_environment", err, nil)
	}
	cleanupTimeout := e.CleanupTimeout
	if cleanupTimeout == 0 {
		cleanupTimeout = defaultCleanupTimeout
	}
	if cleanupTimeout < time.Millisecond || cleanupTimeout > 30*time.Second {
		return ToolExecutionResult{}, executionError("executor:invalid_cleanup", errors.New("cleanup timeout must be between 1ms and 30s"), nil)
	}
	stderrLimit := e.MaxStderrBytes
	if stderrLimit == 0 {
		stderrLimit = defaultMaxStderrBytes
	}
	if stderrLimit < 1 || stderrLimit > MaxFrameBytes {
		return ToolExecutionResult{}, executionError("executor:invalid_stderr_limit", fmt.Errorf("stderr limit must be between 1 and %d", MaxFrameBytes), nil)
	}

	request := ToolExecutionRequest{ProtocolVersion: ProtocolVersion, CallID: callID, Tool: manifest.Metadata.Name, Arguments: arguments}
	if err := ValidateToolExecutionRequest(request); err != nil {
		return ToolExecutionResult{}, executionError("executor:invalid_request", err, nil)
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		return ToolExecutionResult{}, executionError("executor:encode_request", err, nil)
	}
	if len(requestData) > MaxFrameBytes {
		return ToolExecutionResult{}, executionError("executor:request_limit", fmt.Errorf("request exceeds %d bytes", MaxFrameBytes), nil)
	}
	requestData = append(requestData, '\n')

	executionCtx, cancel := context.WithTimeout(ctx, time.Duration(manifest.Spec.TimeoutMillis)*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(executionCtx, manifest.Spec.Transport.Command[0], manifest.Spec.Transport.Command[1:]...)
	command.Dir = e.Directory
	command.Env = append([]string{}, e.Environment...)
	command.WaitDelay = cleanupTimeout
	command.Stdin = bytes.NewReader(requestData)
	stdout := &boundedCapture{limit: MaxFrameBytes + 1}
	diagnostics := &boundedCapture{limit: stderrLimit}
	command.Stdout = stdout
	command.Stderr = diagnostics
	if err := command.Run(); err != nil {
		code := "executor:process_exit"
		if errors.Is(executionCtx.Err(), context.DeadlineExceeded) {
			code = "executor:timeout"
		} else if executionCtx.Err() != nil {
			code = "executor:canceled"
		}
		return ToolExecutionResult{}, executionError(code, fmt.Errorf("%w: %v", ErrToolExecution, err), diagnostics)
	}
	if stdout.Truncated() {
		return ToolExecutionResult{}, executionError("executor:output_limit", fmt.Errorf("%w: stdout exceeds %d bytes", ErrToolExecution, MaxFrameBytes), diagnostics)
	}
	response, err := decodeToolExecutionResponse([]byte(stdout.String()))
	if err != nil {
		return ToolExecutionResult{}, executionError("executor:invalid_response", fmt.Errorf("%w: %v", ErrToolExecution, err), diagnostics)
	}
	if response.CallID != callID {
		return ToolExecutionResult{}, executionError("executor:correlation_mismatch", fmt.Errorf("%w: response callId %q does not match %q", ErrToolExecution, response.CallID, callID), diagnostics)
	}
	return ToolExecutionResult{
		Status: response.Status, Content: response.Content, ErrorCode: response.ErrorCode,
		Stderr: diagnostics.String(), StderrTruncated: diagnostics.Truncated(),
	}, nil
}

func ValidateToolExecutionRequest(request ToolExecutionRequest) error {
	var issues []string
	requireEqual(&issues, "protocolVersion", request.ProtocolVersion, ProtocolVersion)
	validateID(&issues, "callId", request.CallID)
	validateID(&issues, "tool", request.Tool)
	validateJSONObject(&issues, "arguments", request.Arguments)
	return validationResult(issues)
}

func ValidateToolExecutionResponse(response ToolExecutionResponse) error {
	var issues []string
	requireEqual(&issues, "protocolVersion", response.ProtocolVersion, ProtocolVersion)
	validateID(&issues, "callId", response.CallID)
	if response.Status != "succeeded" && response.Status != "failed" {
		issues = append(issues, fmt.Sprintf("status has unsupported value %q", response.Status))
	}
	switch response.Status {
	case "succeeded":
		if len(response.Content) == 0 {
			issues = append(issues, "content is required for succeeded status")
		} else {
			validateJSONValue(&issues, "content", response.Content)
		}
		if response.ErrorCode != "" {
			issues = append(issues, "errorCode must be absent for succeeded status")
		}
	case "failed":
		validateID(&issues, "errorCode", response.ErrorCode)
		if len(response.Content) > 0 {
			issues = append(issues, "content must be absent for failed status")
		}
	}
	return validationResult(issues)
}

func decodeToolExecutionResponse(data []byte) (ToolExecutionResponse, error) {
	if len(data) == 0 || !bytes.HasSuffix(data, []byte{'\n'}) {
		return ToolExecutionResponse{}, errors.New("tool response must end with a newline")
	}
	if !utf8.Valid(data) {
		return ToolExecutionResponse{}, errors.New("tool response must be UTF-8")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), MaxFrameBytes+1)
	if !scanner.Scan() {
		return ToolExecutionResponse{}, errors.New("tool response frame is missing")
	}
	frame := append([]byte(nil), scanner.Bytes()...)
	if len(frame) == 0 || len(frame) > MaxFrameBytes {
		return ToolExecutionResponse{}, errors.New("tool response frame is empty or oversized")
	}
	if scanner.Scan() {
		return ToolExecutionResponse{}, errors.New("tool response must contain exactly one frame")
	}
	if err := scanner.Err(); err != nil {
		return ToolExecutionResponse{}, fmt.Errorf("read tool response: %w", err)
	}
	var response ToolExecutionResponse
	if err := decodeStrict(frame, &response); err != nil {
		return ToolExecutionResponse{}, fmt.Errorf("decode tool response: %w", err)
	}
	if err := ValidateToolExecutionResponse(response); err != nil {
		return ToolExecutionResponse{}, err
	}
	return response, nil
}

func validateExplicitEnvironment(environment []string) error {
	seen := make(map[string]struct{}, len(environment))
	for i, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		key := strings.ToUpper(name)
		if !ok || !validEnvironmentName(name) || strings.ContainsRune(entry, 0) {
			return fmt.Errorf("environment[%d] must use portable NAME=VALUE form", i)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("environment contains duplicate variable %q", name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func executionError(code string, err error, diagnostics *boundedCapture) error {
	execution := &ToolExecutionError{Code: code, Err: err}
	if diagnostics != nil {
		execution.Stderr = diagnostics.String()
		execution.StderrTruncated = diagnostics.Truncated()
	}
	return execution
}
