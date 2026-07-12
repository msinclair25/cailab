package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

// Gateway evaluates and records tool calls but intentionally does not execute
// them. It implements ToolCallHandler for direct Session controller use.
type Gateway struct {
	Run      AgentRun
	Actor    ActorRef
	Policy   GovernancePolicy
	Resolver ToolCallResolver
	Events   DecisionEventAppender
	Clock    func() time.Time
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
	evaluation, err := EvaluatePolicy(g.Policy, AuthorizationRequest{
		Run: g.Run, Actor: g.Actor, Manifest: resolution.Manifest,
		Action: resolution.Action, Resource: resolution.Resource,
		CorrelationID: call.ID, Arguments: payload.Arguments,
	})
	if err != nil {
		return Message{}, fmt.Errorf("evaluate tool call %q: %w", call.ID, err)
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
		return gatewayMessage(MessageApprovalRequired, messageID, call.ID, ApprovalRequiredPayload{
			ApprovalID: evaluation.Decision.ApprovalID, ToolCallID: call.ID,
			Reason: "policy requires approval: " + evaluation.Decision.ReasonCode,
		}), nil
	}
	return gatewayMessage(MessageToolResult, messageID, call.ID, ToolResultPayload{
		Tool: payload.Tool, Status: "not_executed", Decision: evaluation.Decision,
	}), nil
}

func gatewayMessage(messageType, id, correlationID string, payload any) Message {
	data, _ := json.Marshal(payload)
	return Message{ProtocolVersion: ProtocolVersion, ID: id, Type: messageType, CorrelationID: correlationID, Payload: data}
}
