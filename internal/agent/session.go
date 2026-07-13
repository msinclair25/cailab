package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultHandshakeTimeout = 5 * time.Second
	defaultSessionTimeout   = 60 * time.Second
	defaultCleanupTimeout   = 2 * time.Second
	defaultMaxStderrBytes   = 64 << 10
	defaultMaxMessages      = 256
	defaultMaxTranscript    = 8 << 20
)

var (
	ErrInvalidSession    = errors.New("invalid agent session")
	ErrHandshakeTimeout  = errors.New("agent handshake timed out")
	ErrSessionTimeout    = errors.New("agent session timed out")
	ErrProtocolViolation = errors.New("agent protocol violation")
	ErrAgentExit         = errors.New("agent process exited unexpectedly")
)

// SessionConfig describes one explicitly selected agent. Command is an argv
// vector and is never interpreted by a shell. In host mode, Environment is the
// complete child environment. Container mode requires it to be empty and treats
// Command and Directory as paths inside the image.
type SessionConfig struct {
	Command            []string
	Directory          string
	Environment        []string
	Container          *ContainerRuntime
	Run                AgentRun
	HandshakeTimeout   time.Duration
	SessionTimeout     time.Duration
	CleanupTimeout     time.Duration
	MaxStderrBytes     int
	MaxMessages        int
	MaxTranscriptBytes int
}

// ToolCallHandler handles one structurally valid call after lifecycle and tool
// membership checks. The governed gateway owns authorization and execution.
type ToolCallHandler interface {
	HandleToolCall(context.Context, Message, ToolCallPayload) (Message, error)
}

type ApprovalContinuationHandler interface {
	ToolCallHandler
	ResolveApproval(context.Context, Message, ToolCallPayload, Message) (Message, error)
	ContinueApprovedToolCall(context.Context, Message, ToolCallPayload, Message) (Message, error)
}

// ToolCallHandlerFunc adapts a function to ToolCallHandler.
type ToolCallHandlerFunc func(context.Context, Message, ToolCallPayload) (Message, error)

func (f ToolCallHandlerFunc) HandleToolCall(ctx context.Context, message Message, payload ToolCallPayload) (Message, error) {
	return f(ctx, message, payload)
}

// SessionResult contains the agent-originated frames and bounded diagnostics.
// Callers must treat both as untrusted and potentially sensitive.
type SessionResult struct {
	Completion      SessionCompletePayload
	Messages        []Message
	Stderr          string
	StderrTruncated bool
}

// SessionError preserves the stage and bounded child diagnostics while keeping
// sentinel and context errors available through errors.Is.
type SessionError struct {
	Stage           string
	Err             error
	Stderr          string
	StderrTruncated bool
}

func (e *SessionError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("agent session %s: %v", e.Stage, e.Err)
	}
	suffix := ""
	if e.StderrTruncated {
		suffix = " (truncated)"
	}
	return fmt.Sprintf("agent session %s: %v; stderr captured: %d bytes%s", e.Stage, e.Err, len(e.Stderr), suffix)
}

func (e *SessionError) Unwrap() error { return e.Err }

type normalizedSessionConfig struct {
	SessionConfig
	allowedTools map[string]ToolRef
}

type decodedMessage struct {
	message Message
	err     error
}

// ValidateSessionConfig checks launch, lifecycle, and run constraints without
// starting the configured agent process.
func ValidateSessionConfig(config SessionConfig) error {
	_, err := normalizeSessionConfig(config)
	return err
}

// RunSession launches and owns one direct subprocess or enforced container until
// it exits. It enforces protocol direction, ordering, identity, correlation,
// duplicate-ID, timeout, and message-count constraints. Host subprocess mode
// does not isolate filesystem or network access.
func RunSession(ctx context.Context, config SessionConfig, handler ToolCallHandler) (SessionResult, error) {
	normalized, err := normalizeSessionConfig(config)
	if err != nil {
		return SessionResult{}, &SessionError{Stage: "validate", Err: err}
	}

	sessionCtx, cancel := context.WithTimeout(ctx, normalized.SessionTimeout)
	defer cancel()

	command, cleanupRuntime := prepareSessionCommand(sessionCtx, normalized)
	command.WaitDelay = normalized.CleanupTimeout

	stdin, err := command.StdinPipe()
	if err != nil {
		return SessionResult{}, &SessionError{Stage: "prepare stdin", Err: err}
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return SessionResult{}, &SessionError{Stage: "prepare stdout", Err: err}
	}
	diagnostics := &boundedCapture{limit: normalized.MaxStderrBytes}
	command.Stderr = diagnostics

	if err := command.Start(); err != nil {
		return SessionResult{}, &SessionError{Stage: "start", Err: errors.Join(err, cleanupRuntime())}
	}

	decoderResults := make(chan decodedMessage, 1)
	go decodeMessages(sessionCtx, NewDecoder(stdout), decoderResults)

	result := SessionResult{Messages: make([]Message, 0, 8)}
	fail := func(stage string, cause error) (SessionResult, error) {
		cancel()
		_ = stdin.Close()
		waitErr := command.Wait()
		if waitErr != nil && errors.Is(cause, ErrAgentExit) {
			cause = fmt.Errorf("%w: %v", cause, waitErr)
		}
		if cleanupErr := cleanupRuntime(); cleanupErr != nil {
			cause = errors.Join(cause, fmt.Errorf("cleanup runtime: %w", cleanupErr))
		}
		result.Stderr = diagnostics.String()
		result.StderrTruncated = diagnostics.Truncated()
		return result, &SessionError{
			Stage: stage, Err: cause, Stderr: result.Stderr, StderrTruncated: result.StderrTruncated,
		}
	}

	start := sessionStartMessage(normalized.Run)
	if err := writeMessage(sessionCtx, NewEncoder(stdin), start); err != nil {
		return fail("send session.start", err)
	}

	handshakeTimer := time.NewTimer(normalized.HandshakeTimeout)
	defer handshakeTimer.Stop()
	first, err := nextSessionMessage(sessionCtx, handshakeTimer.C, decoderResults)
	if err != nil {
		if errors.Is(err, errHandshakeDeadline) {
			return fail("handshake", ErrHandshakeTimeout)
		}
		return fail("handshake", classifySessionContext(sessionCtx, err))
	}
	result.Messages = append(result.Messages, first)
	transcriptBytes := encodedMessageSize(first)
	if transcriptBytes > normalized.MaxTranscriptBytes {
		return fail("handshake", fmt.Errorf("%w: transcript exceeds %d bytes", ErrProtocolViolation, normalized.MaxTranscriptBytes))
	}
	seenIDs := map[string]struct{}{start.ID: {}, first.ID: {}}
	sendControllerResponse := func(response, call Message) error {
		if err := validateControllerResponse(response, call); err != nil {
			return err
		}
		if _, exists := seenIDs[response.ID]; exists {
			return fmt.Errorf("%w: duplicate message id %q", ErrProtocolViolation, response.ID)
		}
		seenIDs[response.ID] = struct{}{}
		return writeMessage(sessionCtx, NewEncoder(stdin), response)
	}
	if first.Type == MessageProtocolError {
		return fail("handshake", remoteProtocolError(first))
	}
	if first.Type != MessageAgentReady {
		return fail("handshake", fmt.Errorf("%w: first agent message must be %s, got %s", ErrProtocolViolation, MessageAgentReady, first.Type))
	}
	var ready AgentReadyPayload
	if err := json.Unmarshal(first.Payload, &ready); err != nil {
		return fail("handshake", fmt.Errorf("%w: decode ready payload: %v", ErrProtocolViolation, err))
	}
	if ready.AgentID != normalized.Run.Agent.ID || ready.AgentVersion != normalized.Run.Agent.Version {
		return fail("handshake", fmt.Errorf("%w: agent identity %s@%s does not match expected %s@%s", ErrProtocolViolation, ready.AgentID, ready.AgentVersion, normalized.Run.Agent.ID, normalized.Run.Agent.Version))
	}

	for {
		message, err := nextSessionMessage(sessionCtx, nil, decoderResults)
		if err != nil {
			return fail("receive", classifySessionContext(sessionCtx, err))
		}
		if len(result.Messages) >= normalized.MaxMessages {
			return fail("receive", fmt.Errorf("%w: message count exceeds %d", ErrProtocolViolation, normalized.MaxMessages))
		}
		messageBytes := encodedMessageSize(message)
		if transcriptBytes+messageBytes > normalized.MaxTranscriptBytes {
			return fail("receive", fmt.Errorf("%w: transcript exceeds %d bytes", ErrProtocolViolation, normalized.MaxTranscriptBytes))
		}
		if _, exists := seenIDs[message.ID]; exists {
			return fail("receive", fmt.Errorf("%w: duplicate message id %q", ErrProtocolViolation, message.ID))
		}
		seenIDs[message.ID] = struct{}{}
		result.Messages = append(result.Messages, message)
		transcriptBytes += messageBytes

		switch message.Type {
		case MessageToolCall:
			var call ToolCallPayload
			if err := json.Unmarshal(message.Payload, &call); err != nil {
				return fail("tool call", fmt.Errorf("%w: decode tool call: %v", ErrProtocolViolation, err))
			}
			if _, allowed := normalized.allowedTools[call.Tool]; !allowed {
				return fail("tool call", fmt.Errorf("%w: tool %q is not declared for this run", ErrProtocolViolation, call.Tool))
			}
			if handler == nil {
				return fail("tool call", fmt.Errorf("%w: no tool-call handler is configured", ErrProtocolViolation))
			}
			response, err := handleToolCall(sessionCtx, handler, message, call)
			if err != nil {
				if sessionCtx.Err() != nil {
					return fail("tool call", classifySessionContext(sessionCtx, err))
				}
				return fail("tool call", fmt.Errorf("handle %s: %w", call.Tool, err))
			}
			if err := sendControllerResponse(response, message); err != nil {
				return fail("tool response", classifySessionContext(sessionCtx, err))
			}
			if response.Type == MessageApprovalRequired {
				continuation, ok := handler.(ApprovalContinuationHandler)
				if !ok {
					continue
				}
				resolved, err := resolveApproval(sessionCtx, continuation, message, call, response)
				if err != nil {
					return fail("approval resolution", classifySessionContext(sessionCtx, err))
				}
				if err := validateApprovalResolutionResponse(resolved, message, response); err != nil {
					return fail("approval resolution", err)
				}
				if _, exists := seenIDs[resolved.ID]; exists {
					return fail("approval resolution", fmt.Errorf("%w: duplicate message id %q", ErrProtocolViolation, resolved.ID))
				}
				seenIDs[resolved.ID] = struct{}{}
				if err := writeMessage(sessionCtx, NewEncoder(stdin), resolved); err != nil {
					return fail("approval resolution", classifySessionContext(sessionCtx, err))
				}
				final, err := continueApprovedToolCall(sessionCtx, continuation, message, call, resolved)
				if err != nil {
					return fail("approved tool call", classifySessionContext(sessionCtx, err))
				}
				if final.Type != MessageToolResult {
					return fail("approved tool call", fmt.Errorf("%w: approval continuation returned %s, want %s", ErrProtocolViolation, final.Type, MessageToolResult))
				}
				if err := sendControllerResponse(final, message); err != nil {
					return fail("approved tool call", classifySessionContext(sessionCtx, err))
				}
			}
		case MessageSessionComplete:
			if err := json.Unmarshal(message.Payload, &result.Completion); err != nil {
				return fail("complete", fmt.Errorf("%w: decode completion: %v", ErrProtocolViolation, err))
			}
			_ = stdin.Close()
			if err := command.Wait(); err != nil {
				cleanupErr := cleanupRuntime()
				result.Stderr = diagnostics.String()
				result.StderrTruncated = diagnostics.Truncated()
				cause := error(fmt.Errorf("%w: %w", ErrAgentExit, err))
				if sessionCtx.Err() != nil {
					cause = classifySessionContext(sessionCtx, sessionCtx.Err())
				}
				return result, &SessionError{Stage: "wait", Err: errors.Join(cause, cleanupErr), Stderr: result.Stderr, StderrTruncated: result.StderrTruncated}
			}
			if err := cleanupRuntime(); err != nil {
				result.Stderr = diagnostics.String()
				result.StderrTruncated = diagnostics.Truncated()
				return result, &SessionError{Stage: "cleanup", Err: err, Stderr: result.Stderr, StderrTruncated: result.StderrTruncated}
			}
			result.Stderr = diagnostics.String()
			result.StderrTruncated = diagnostics.Truncated()
			return result, nil
		case MessageProtocolError:
			return fail("agent error", remoteProtocolError(message))
		default:
			return fail("receive", fmt.Errorf("%w: agent cannot send %s", ErrProtocolViolation, message.Type))
		}
	}
}

func normalizeSessionConfig(config SessionConfig) (normalizedSessionConfig, error) {
	if len(config.Command) == 0 {
		return normalizedSessionConfig{}, fmt.Errorf("%w: command must contain an executable", ErrInvalidSession)
	}
	for i, value := range config.Command {
		if value == "" || strings.ContainsRune(value, 0) || strings.ContainsAny(value, "\r\n") {
			return normalizedSessionConfig{}, fmt.Errorf("%w: command[%d] must be nonempty and contain no NUL or line breaks", ErrInvalidSession, i)
		}
	}
	if config.Container == nil {
		if !filepath.IsAbs(config.Command[0]) {
			return normalizedSessionConfig{}, fmt.Errorf("%w: executable must be an absolute path", ErrInvalidSession)
		}
		if !filepath.IsAbs(config.Directory) {
			return normalizedSessionConfig{}, fmt.Errorf("%w: directory must be an absolute path", ErrInvalidSession)
		}
		if config.Run.Execution != nil {
			return normalizedSessionConfig{}, fmt.Errorf("%w: host subprocess must not claim enforced execution metadata", ErrInvalidSession)
		}
	} else {
		if !path.IsAbs(config.Command[0]) || !path.IsAbs(config.Directory) {
			return normalizedSessionConfig{}, fmt.Errorf("%w: container executable and directory must be absolute POSIX paths", ErrInvalidSession)
		}
		if !filepath.IsAbs(config.Container.enginePath) {
			return normalizedSessionConfig{}, fmt.Errorf("%w: container engine must be an absolute host path", ErrInvalidSession)
		}
		if err := ValidateContainerImageReference(config.Container.image); err != nil {
			return normalizedSessionConfig{}, fmt.Errorf("%w: %v", ErrInvalidSession, err)
		}
		if err := ValidateContainerHostEndpoint(config.Container.host); err != nil {
			return normalizedSessionConfig{}, fmt.Errorf("%w: %v", ErrInvalidSession, err)
		}
		if len(config.Environment) != 0 {
			return normalizedSessionConfig{}, fmt.Errorf("%w: container mode does not forward host environment variables", ErrInvalidSession)
		}
		if !sameExecutionRef(config.Run.Execution, ContainerExecutionRef(config.Container)) {
			return normalizedSessionConfig{}, fmt.Errorf("%w: run execution metadata does not match container runtime", ErrInvalidSession)
		}
	}
	seenEnvironment := make(map[string]struct{}, len(config.Environment))
	for i, entry := range config.Environment {
		name, _, ok := strings.Cut(entry, "=")
		key := strings.ToUpper(name)
		if !ok || !validEnvironmentName(name) || strings.ContainsRune(entry, 0) {
			return normalizedSessionConfig{}, fmt.Errorf("%w: environment[%d] must use a portable NAME=VALUE form without NUL", ErrInvalidSession, i)
		}
		if _, duplicate := seenEnvironment[key]; duplicate {
			return normalizedSessionConfig{}, fmt.Errorf("%w: environment contains duplicate variable %q", ErrInvalidSession, name)
		}
		seenEnvironment[key] = struct{}{}
	}
	if err := ValidateAgentRun(config.Run); err != nil {
		return normalizedSessionConfig{}, fmt.Errorf("%w: %w", ErrInvalidSession, err)
	}
	if config.Run.Status != "planned" && config.Run.Status != "running" {
		return normalizedSessionConfig{}, fmt.Errorf("%w: run status must be planned or running", ErrInvalidSession)
	}
	if config.HandshakeTimeout == 0 {
		config.HandshakeTimeout = defaultHandshakeTimeout
	}
	if config.SessionTimeout == 0 {
		config.SessionTimeout = defaultSessionTimeout
	}
	if config.CleanupTimeout == 0 {
		config.CleanupTimeout = defaultCleanupTimeout
	}
	if config.MaxStderrBytes == 0 {
		config.MaxStderrBytes = defaultMaxStderrBytes
	}
	if config.MaxMessages == 0 {
		config.MaxMessages = defaultMaxMessages
	}
	if config.MaxTranscriptBytes == 0 {
		config.MaxTranscriptBytes = defaultMaxTranscript
	}
	if config.HandshakeTimeout < time.Millisecond || config.HandshakeTimeout > 30*time.Second {
		return normalizedSessionConfig{}, fmt.Errorf("%w: handshake timeout must be between 1ms and 30s", ErrInvalidSession)
	}
	if config.SessionTimeout < config.HandshakeTimeout || config.SessionTimeout > 30*time.Minute {
		return normalizedSessionConfig{}, fmt.Errorf("%w: session timeout must be at least the handshake timeout and no more than 30m", ErrInvalidSession)
	}
	if config.CleanupTimeout < time.Millisecond || config.CleanupTimeout > 30*time.Second {
		return normalizedSessionConfig{}, fmt.Errorf("%w: cleanup timeout must be between 1ms and 30s", ErrInvalidSession)
	}
	if config.MaxStderrBytes < 1 || config.MaxStderrBytes > MaxFrameBytes {
		return normalizedSessionConfig{}, fmt.Errorf("%w: stderr limit must be between 1 and %d bytes", ErrInvalidSession, MaxFrameBytes)
	}
	if config.MaxMessages < 2 || config.MaxMessages > 10_000 {
		return normalizedSessionConfig{}, fmt.Errorf("%w: message limit must be between 2 and 10000", ErrInvalidSession)
	}
	if config.MaxTranscriptBytes < MaxFrameBytes || config.MaxTranscriptBytes > 64<<20 {
		return normalizedSessionConfig{}, fmt.Errorf("%w: transcript limit must be between %d and %d bytes", ErrInvalidSession, MaxFrameBytes, 64<<20)
	}
	allowedTools := make(map[string]ToolRef, len(config.Run.Tools))
	for _, tool := range config.Run.Tools {
		allowedTools[tool.Name] = tool
	}
	return normalizedSessionConfig{SessionConfig: config, allowedTools: allowedTools}, nil
}

func validEnvironmentName(name string) bool {
	if name == "" || !((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z') || name[0] == '_') {
		return false
	}
	for _, character := range name[1:] {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
}

func decodeMessages(ctx context.Context, decoder *Decoder, output chan<- decodedMessage) {
	for {
		message, err := decoder.Next()
		select {
		case output <- decodedMessage{message: message, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

var errHandshakeDeadline = errors.New("handshake deadline reached")

func nextSessionMessage(ctx context.Context, deadline <-chan time.Time, input <-chan decodedMessage) (Message, error) {
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-deadline:
		return Message{}, errHandshakeDeadline
	case result := <-input:
		return result.message, result.err
	}
}

func writeMessage(ctx context.Context, encoder *Encoder, message Message) error {
	result := make(chan error, 1)
	go func() { result <- encoder.Write(message) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type toolCallResult struct {
	message Message
	err     error
}

func handleToolCall(ctx context.Context, handler ToolCallHandler, call Message, payload ToolCallPayload) (Message, error) {
	result := make(chan toolCallResult, 1)
	go func() {
		message, err := handler.HandleToolCall(ctx, call, payload)
		result <- toolCallResult{message: message, err: err}
	}()
	select {
	case handled := <-result:
		return handled.message, handled.err
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func resolveApproval(ctx context.Context, handler ApprovalContinuationHandler, call Message, payload ToolCallPayload, required Message) (Message, error) {
	result := make(chan toolCallResult, 1)
	go func() {
		message, err := handler.ResolveApproval(ctx, call, payload, required)
		result <- toolCallResult{message: message, err: err}
	}()
	select {
	case handled := <-result:
		return handled.message, handled.err
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func continueApprovedToolCall(ctx context.Context, handler ApprovalContinuationHandler, call Message, payload ToolCallPayload, resolved Message) (Message, error) {
	result := make(chan toolCallResult, 1)
	go func() {
		message, err := handler.ContinueApprovedToolCall(ctx, call, payload, resolved)
		result <- toolCallResult{message: message, err: err}
	}()
	select {
	case handled := <-result:
		return handled.message, handled.err
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func sessionStartMessage(run AgentRun) Message {
	payload, _ := json.Marshal(SessionStartPayload{
		RunID: run.RunID, TrialID: run.TrialID, ScenarioDigest: run.Scenario.Digest,
		PolicyVersion: run.Policy.Version, Tools: append([]ToolRef(nil), run.Tools...),
	})
	return Message{ProtocolVersion: ProtocolVersion, ID: "message:start", Type: MessageSessionStart, Payload: payload}
}

func validateControllerResponse(response, call Message) error {
	if err := ValidateMessage(response); err != nil {
		return fmt.Errorf("%w: invalid controller response: %v", ErrProtocolViolation, err)
	}
	if response.Type != MessageToolResult && response.Type != MessageApprovalRequired {
		return fmt.Errorf("%w: tool handler returned unsupported %s", ErrProtocolViolation, response.Type)
	}
	if response.CorrelationID != call.ID {
		return fmt.Errorf("%w: response correlation %q does not match call %q", ErrProtocolViolation, response.CorrelationID, call.ID)
	}
	if response.Type == MessageToolResult {
		var callPayload ToolCallPayload
		var resultPayload ToolResultPayload
		if err := json.Unmarshal(call.Payload, &callPayload); err != nil {
			return fmt.Errorf("%w: decode correlated tool call: %v", ErrProtocolViolation, err)
		}
		if err := json.Unmarshal(response.Payload, &resultPayload); err != nil || resultPayload.Tool != callPayload.Tool {
			return fmt.Errorf("%w: tool result must name correlated tool %q", ErrProtocolViolation, callPayload.Tool)
		}
	}
	if response.Type == MessageApprovalRequired {
		var payload ApprovalRequiredPayload
		if err := json.Unmarshal(response.Payload, &payload); err != nil || payload.ToolCallID != call.ID {
			return fmt.Errorf("%w: approval toolCallId must match call %q", ErrProtocolViolation, call.ID)
		}
	}
	return nil
}

func validateApprovalResolutionResponse(resolved, call, required Message) error {
	if err := ValidateMessage(resolved); err != nil {
		return fmt.Errorf("%w: invalid approval resolution: %v", ErrProtocolViolation, err)
	}
	if resolved.Type != MessageApprovalResolved || resolved.CorrelationID != call.ID {
		return fmt.Errorf("%w: approval resolution must correlate to call %q", ErrProtocolViolation, call.ID)
	}
	var requiredPayload ApprovalRequiredPayload
	var resolvedPayload ApprovalResolvedPayload
	if err := json.Unmarshal(required.Payload, &requiredPayload); err != nil {
		return fmt.Errorf("%w: decode approval requirement: %v", ErrProtocolViolation, err)
	}
	if err := json.Unmarshal(resolved.Payload, &resolvedPayload); err != nil || resolvedPayload.ApprovalID != requiredPayload.ApprovalID {
		return fmt.Errorf("%w: approval resolution id must match requirement %q", ErrProtocolViolation, requiredPayload.ApprovalID)
	}
	return nil
}

func encodedMessageSize(message Message) int {
	data, _ := json.Marshal(message)
	return len(data) + 1
}

func remoteProtocolError(message Message) error {
	var payload ProtocolErrorPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return fmt.Errorf("%w: decode protocol error: %v", ErrProtocolViolation, err)
	}
	return fmt.Errorf("%w: agent reported %s: %s", ErrProtocolViolation, payload.Code, payload.Message)
}

func classifySessionContext(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrSessionTimeout, context.DeadlineExceeded)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: stdout closed before session.complete", ErrAgentExit)
	}
	return fmt.Errorf("%w: %v", ErrProtocolViolation, err)
}

type boundedCapture struct {
	buffer bytes.Buffer
	limit  int
	total  int
}

func (c *boundedCapture) Write(data []byte) (int, error) {
	c.total += len(data)
	remaining := c.limit - c.buffer.Len()
	if remaining > 0 {
		if len(data) < remaining {
			remaining = len(data)
		}
		_, _ = c.buffer.Write(data[:remaining])
	}
	return len(data), nil
}

func (c *boundedCapture) String() string  { return c.buffer.String() }
func (c *boundedCapture) Truncated() bool { return c.total > c.limit }
