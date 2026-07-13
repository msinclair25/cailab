package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

var ErrInvalidGateway = errors.New("invalid governed tool gateway")

type ToolCallResolution struct {
	Manifest ToolManifest
	Action   string
	Resource ResourceRef
}

type ToolCallResolver interface {
	ResolveToolCall(context.Context, Message, ToolCallPayload) (ToolCallResolution, error)
}

type ToolCallResolverFunc func(context.Context, Message, ToolCallPayload) (ToolCallResolution, error)

func (f ToolCallResolverFunc) ResolveToolCall(ctx context.Context, message Message, payload ToolCallPayload) (ToolCallResolution, error) {
	return f(ctx, message, payload)
}

type DecisionEventAppender interface {
	AppendDecisionEvent(context.Context, DecisionEventDraft) (DecisionEvent, error)
}

type ToolEvidenceAppender interface {
	DecisionEventAppender
	AppendApprovalResolutionEvent(context.Context, ApprovalResolutionEventDraft) (ApprovalResolutionEvent, error)
	AppendToolOutcomeEvent(context.Context, ToolOutcomeEventDraft) (ToolOutcomeEvent, error)
}

// Gateway evaluates, records, and conditionally executes tool calls. It
// implements ToolCallHandler for direct Session controller use.
type Gateway struct {
	Run       AgentRun
	Actor     ActorRef
	Policy    GovernancePolicy
	Resolver  ToolCallResolver
	Executor  ToolExecutor
	Approver  Approver
	Events    ToolEvidenceAppender
	Clock     func() time.Time
	pendingMu sync.Mutex
	pending   map[string]*pendingApproval
}

type pendingApproval struct {
	request       AuthorizationRequest
	call          Message
	payload       ToolCallPayload
	decisionEvent DecisionEvent
	resolution    ApprovalResolution
	evaluation    Evaluation
	approvalEvent ApprovalResolutionEvent
}

func (g *Gateway) HandleToolCall(ctx context.Context, call Message, payload ToolCallPayload) (Message, error) {
	if g == nil || g.Resolver == nil || g.Events == nil || g.Clock == nil {
		return Message{}, fmt.Errorf("%w: run, resolver, event appender, and clock are required", ErrInvalidGateway)
	}
	resolution, err := g.Resolver.ResolveToolCall(ctx, call, payload)
	if err != nil {
		return Message{}, fmt.Errorf("resolve tool call %q: %w", call.ID, err)
	}
	if resolution.Manifest.Metadata.Name != payload.Tool {
		return Message{}, fmt.Errorf("%w: resolver manifest %q does not match requested tool %q", ErrInvalidGateway, resolution.Manifest.Metadata.Name, payload.Tool)
	}
	authorizationRequest := AuthorizationRequest{
		Run: g.Run, Actor: g.Actor, Manifest: resolution.Manifest,
		Action: resolution.Action, Resource: resolution.Resource,
		CorrelationID: call.ID, Arguments: payload.Arguments,
	}
	evaluation, err := EvaluatePolicy(g.Policy, authorizationRequest)
	if err != nil {
		return Message{}, fmt.Errorf("evaluate tool call %q: %w", call.ID, err)
	}
	if (evaluation.Decision.Effect == "allow" || evaluation.Decision.Effect == "redact") && g.Executor == nil {
		return Message{}, fmt.Errorf("%w: executor is required for %s decision", ErrInvalidGateway, evaluation.Decision.Effect)
	}
	draft := DecisionEventDraft{
		OccurredAt: g.Clock().UTC(), RunID: g.Run.RunID, TrialID: g.Run.TrialID,
		CorrelationID: call.ID, Actor: g.Actor, Tool: evaluation.Tool,
		Action: resolution.Action, Resource: resolution.Resource,
		Decision: evaluation.Decision, Outcome: Outcome{Status: "not_executed"},
		InputHash: evaluation.InputHash,
	}
	if err := ValidateDecisionEventDraft(draft); err != nil {
		return Message{}, fmt.Errorf("prepare decision evidence: %w", err)
	}
	event, err := g.Events.AppendDecisionEvent(ctx, draft)
	if err != nil {
		return Message{}, fmt.Errorf("persist decision evidence for %q: %w", call.ID, err)
	}
	expectedEvent, buildErr := BuildDecisionEvent(draft, event.EventID, event.Sequence)
	if buildErr != nil || !reflect.DeepEqual(event, expectedEvent) {
		return Message{}, fmt.Errorf("%w: event appender returned mismatched evidence", ErrInvalidGateway)
	}

	messageID := deterministicIdentifier("message", call.ID, evaluation.Decision.Effect)
	if evaluation.Decision.Effect == "require_approval" {
		g.storePendingApproval(call.ID, &pendingApproval{
			request: authorizationRequest, call: call, payload: payload, decisionEvent: event,
		})
		return gatewayMessage(MessageApprovalRequired, messageID, call.ID, ApprovalRequiredPayload{
			ApprovalID: evaluation.Decision.ApprovalID, ToolCallID: call.ID,
			Reason: "policy requires approval: " + evaluation.Decision.ReasonCode,
		}), nil
	}
	if evaluation.Decision.Effect == "deny" {
		return gatewayMessage(MessageToolResult, messageID, call.ID, ToolResultPayload{
			Tool: payload.Tool, Status: "not_executed", Decision: evaluation.Decision,
		}), nil
	}
	return g.executeToolCall(ctx, call, payload, resolution, evaluation, event.EventID, "")
}

func (g *Gateway) executeToolCall(ctx context.Context, call Message, payload ToolCallPayload, resolution ToolCallResolution, evaluation Evaluation, decisionEventID, approvalEventID string) (Message, error) {
	result, executionErr := g.Executor.Execute(ctx, resolution.Manifest, ToolExecutionRequest{
		ProtocolVersion: ProtocolVersion,
		CallID:          call.ID,
		Tool:            payload.Tool,
		Action:          resolution.Action,
		Resource:        resolution.Resource,
		Arguments:       evaluation.Arguments,
	})
	outcome := Outcome{Status: "failed"}
	var content json.RawMessage
	var outputHash string
	if executionErr != nil {
		var typed *ToolExecutionError
		if errors.As(executionErr, &typed) {
			outcome.ErrorCode = typed.Code
		} else {
			outcome.ErrorCode = "executor:failed"
		}
	} else if result.Status == "failed" {
		outcome.ErrorCode = result.ErrorCode
	} else if result.Status == "succeeded" {
		content, executionErr = protectToolOutput(result.Content, resolution.Manifest.Spec.SensitiveFields)
		if executionErr != nil {
			outcome.ErrorCode = "executor:output_redaction_failed"
		} else {
			outputHash, executionErr = DigestJSON(content)
			if executionErr != nil {
				outcome.ErrorCode = "executor:output_hash_failed"
			} else {
				outcome = Outcome{Status: "succeeded"}
			}
		}
	} else {
		outcome.ErrorCode = "executor:invalid_result"
	}
	outcomeDraft := ToolOutcomeEventDraft{
		OccurredAt: g.Clock().UTC(), RunID: g.Run.RunID, TrialID: g.Run.TrialID,
		CorrelationID: call.ID, DecisionEventID: decisionEventID, ApprovalEventID: approvalEventID, Tool: evaluation.Tool,
		Outcome: outcome, OutputHash: outputHash,
	}
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer persistCancel()
	outcomeEvent, err := g.Events.AppendToolOutcomeEvent(persistCtx, outcomeDraft)
	if err != nil {
		return Message{}, fmt.Errorf("persist tool outcome for %q: %w", call.ID, err)
	}
	expectedOutcome, buildErr := BuildToolOutcomeEvent(outcomeDraft, outcomeEvent.EventID)
	if buildErr != nil || !reflect.DeepEqual(outcomeEvent, expectedOutcome) {
		return Message{}, fmt.Errorf("%w: event appender returned mismatched tool outcome", ErrInvalidGateway)
	}
	status := "failed"
	if outcome.Status == "succeeded" {
		status = "succeeded"
	}
	return gatewayMessage(MessageToolResult, deterministicIdentifier("message", call.ID, "tool_result", evaluation.Decision.Effect), call.ID, ToolResultPayload{
		Tool: payload.Tool, Status: status, Content: content, ErrorCode: outcome.ErrorCode, Decision: evaluation.Decision,
	}), nil
}

func (g *Gateway) ResolveApproval(ctx context.Context, call Message, payload ToolCallPayload, required Message) (Message, error) {
	pending, err := g.pendingApproval(call, payload)
	if err != nil {
		return Message{}, err
	}
	if g.Approver == nil {
		return Message{}, fmt.Errorf("%w: no approver is configured", ErrInvalidGateway)
	}
	var requiredPayload ApprovalRequiredPayload
	if err := json.Unmarshal(required.Payload, &requiredPayload); err != nil || required.Type != MessageApprovalRequired ||
		required.CorrelationID != call.ID || requiredPayload.ApprovalID != pending.decisionEvent.Decision.ApprovalID {
		return Message{}, fmt.Errorf("%w: approval requirement does not match pending call", ErrInvalidGateway)
	}
	resolution, err := g.Approver.ResolveApproval(ctx, ApprovalRequest{
		ApprovalID: pending.decisionEvent.Decision.ApprovalID, DecisionEventID: pending.decisionEvent.EventID,
		RunID: g.Run.RunID, TrialID: g.Run.TrialID, CorrelationID: call.ID,
		Actor: g.Actor, Tool: pending.decisionEvent.Tool, Action: pending.decisionEvent.Action,
		Resource: pending.decisionEvent.Resource, ReasonCode: pending.decisionEvent.Decision.ReasonCode,
		InputHash: pending.decisionEvent.InputHash,
	})
	if err != nil {
		return Message{}, fmt.Errorf("resolve approval %q: %w", requiredPayload.ApprovalID, err)
	}
	evaluation, err := ReevaluateApprovedPolicy(g.Policy, pending.request, requiredPayload.ApprovalID, resolution.Approved)
	if err != nil {
		return Message{}, fmt.Errorf("re-evaluate approval %q: %w", requiredPayload.ApprovalID, err)
	}
	draft := ApprovalResolutionEventDraft{
		OccurredAt: g.Clock().UTC(), RunID: g.Run.RunID, TrialID: g.Run.TrialID,
		CorrelationID: call.ID, ApprovalID: requiredPayload.ApprovalID,
		DecisionEventID: pending.decisionEvent.EventID, ResolvedBy: resolution.ResolvedBy,
		Approved: resolution.Approved, Tool: evaluation.Tool, Action: pending.request.Action,
		Resource: pending.request.Resource, Decision: evaluation.Decision, InputHash: evaluation.InputHash,
	}
	if _, err := BuildApprovalResolutionEvent(draft, "approval-event:pending"); err != nil {
		return Message{}, fmt.Errorf("prepare approval evidence: %w", err)
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	event, err := g.Events.AppendApprovalResolutionEvent(persistCtx, draft)
	if err != nil {
		return Message{}, fmt.Errorf("persist approval evidence for %q: %w", call.ID, err)
	}
	expected, buildErr := BuildApprovalResolutionEvent(draft, event.EventID)
	if buildErr != nil || !reflect.DeepEqual(event, expected) {
		return Message{}, fmt.Errorf("%w: event appender returned mismatched approval evidence", ErrInvalidGateway)
	}
	g.pendingMu.Lock()
	current := g.pending[call.ID]
	if current != pending || current.approvalEvent.EventID != "" {
		g.pendingMu.Unlock()
		return Message{}, fmt.Errorf("%w: pending approval changed during resolution", ErrInvalidGateway)
	}
	current.resolution = resolution
	current.evaluation = evaluation
	current.approvalEvent = event
	g.pendingMu.Unlock()
	approved := resolution.Approved
	return gatewayMessage(MessageApprovalResolved, deterministicIdentifier("message", call.ID, "approval_resolved"), call.ID, ApprovalResolvedPayload{
		ApprovalID: requiredPayload.ApprovalID, Approved: &approved, ResolvedBy: resolution.ResolvedBy,
	}), nil
}

func (g *Gateway) ContinueApprovedToolCall(ctx context.Context, call Message, payload ToolCallPayload, resolved Message) (Message, error) {
	pending, err := g.takeResolvedApproval(call, payload, resolved)
	if err != nil {
		return Message{}, err
	}
	decision := pending.evaluation.Decision
	messageID := deterministicIdentifier("message", call.ID, "approval_result", decision.Effect)
	if decision.Effect == "deny" {
		return gatewayMessage(MessageToolResult, messageID, call.ID, ToolResultPayload{
			Tool: payload.Tool, Status: "not_executed", Decision: decision,
		}), nil
	}
	if decision.Effect != "allow" && decision.Effect != "redact" {
		return Message{}, fmt.Errorf("%w: resolved approval produced unsupported %s decision", ErrInvalidGateway, decision.Effect)
	}
	if g.Executor == nil {
		return Message{}, fmt.Errorf("%w: executor is required for approved %s decision", ErrInvalidGateway, decision.Effect)
	}
	resolution := ToolCallResolution{Manifest: pending.request.Manifest, Action: pending.request.Action, Resource: pending.request.Resource}
	return g.executeToolCall(ctx, call, payload, resolution, pending.evaluation, pending.decisionEvent.EventID, pending.approvalEvent.EventID)
}

func (g *Gateway) storePendingApproval(correlationID string, pending *pendingApproval) {
	g.pendingMu.Lock()
	defer g.pendingMu.Unlock()
	if g.pending == nil {
		g.pending = make(map[string]*pendingApproval)
	}
	g.pending[correlationID] = pending
}

func (g *Gateway) pendingApproval(call Message, payload ToolCallPayload) (*pendingApproval, error) {
	g.pendingMu.Lock()
	defer g.pendingMu.Unlock()
	pending := g.pending[call.ID]
	if pending == nil || !reflect.DeepEqual(pending.call, call) || !reflect.DeepEqual(pending.payload, payload) || pending.approvalEvent.EventID != "" {
		return nil, fmt.Errorf("%w: no unresolved approval for call %q", ErrInvalidGateway, call.ID)
	}
	return pending, nil
}

func (g *Gateway) takeResolvedApproval(call Message, payload ToolCallPayload, resolved Message) (*pendingApproval, error) {
	g.pendingMu.Lock()
	defer g.pendingMu.Unlock()
	pending := g.pending[call.ID]
	if pending == nil || !reflect.DeepEqual(pending.call, call) || !reflect.DeepEqual(pending.payload, payload) || pending.approvalEvent.EventID == "" {
		return nil, fmt.Errorf("%w: no resolved approval for call %q", ErrInvalidGateway, call.ID)
	}
	var resolution ApprovalResolvedPayload
	if err := json.Unmarshal(resolved.Payload, &resolution); err != nil || resolved.Type != MessageApprovalResolved ||
		resolved.CorrelationID != call.ID || resolution.ApprovalID != pending.approvalEvent.ApprovalID ||
		resolution.Approved == nil || *resolution.Approved != pending.resolution.Approved || resolution.ResolvedBy != pending.resolution.ResolvedBy {
		return nil, fmt.Errorf("%w: approval resolution message does not match persisted evidence", ErrInvalidGateway)
	}
	delete(g.pending, call.ID)
	return pending, nil
}

func protectToolOutput(content json.RawMessage, pointers []string) (json.RawMessage, error) {
	canonical, err := CanonicalJSON(content)
	if err != nil {
		return nil, err
	}
	if len(pointers) == 0 {
		return canonical, nil
	}
	return RedactJSON(canonical, pointers)
}

func gatewayMessage(messageType, id, correlationID string, payload any) Message {
	data, _ := json.Marshal(payload)
	return Message{ProtocolVersion: ProtocolVersion, ID: id, Type: messageType, CorrelationID: correlationID, Payload: data}
}
