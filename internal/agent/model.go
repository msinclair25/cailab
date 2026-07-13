package agent

import (
	"context"
	"encoding/json"
	"time"
)

const (
	APIVersion      = "cloudailab.dev/agent/v1alpha1"
	ProtocolVersion = "1.1"
	MaxFrameBytes   = 1 << 20

	ToolManifestKind            = "ToolManifest"
	AgentRunKind                = "AgentRun"
	DecisionEventKind           = "DecisionEvent"
	GovernancePolicyKind        = "GovernancePolicy"
	ToolOutcomeEventKind        = "ToolOutcomeEvent"
	ApprovalResolutionEventKind = "ApprovalResolutionEvent"
)

const (
	MessageSessionStart     = "session.start"
	MessageAgentReady       = "agent.ready"
	MessageToolCall         = "tool.call"
	MessageToolResult       = "tool.result"
	MessageApprovalRequired = "approval.required"
	MessageApprovalResolved = "approval.resolved"
	MessageSessionComplete  = "session.complete"
	MessageProtocolError    = "protocol.error"
)

type Metadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type ToolManifest struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   Metadata         `json:"metadata"`
	Spec       ToolManifestSpec `json:"spec"`
}

type ToolManifestSpec struct {
	Transport       ToolTransport   `json:"transport"`
	InputSchema     json.RawMessage `json:"inputSchema"`
	Permissions     []Permission    `json:"permissions"`
	Risk            string          `json:"risk"`
	TimeoutMillis   int             `json:"timeoutMillis"`
	Isolation       Isolation       `json:"isolation"`
	SensitiveFields []string        `json:"sensitiveFields"`
}

type ToolTransport struct {
	Type    string   `json:"type"`
	Command []string `json:"command"`
}

type Permission struct {
	Tenant    string   `json:"tenant"`
	Actions   []string `json:"actions"`
	Resources []string `json:"resources"`
}

type Isolation struct {
	Network    string `json:"network"`
	Filesystem string `json:"filesystem"`
}

type GovernancePolicy struct {
	APIVersion    string       `json:"apiVersion"`
	Kind          string       `json:"kind"`
	Version       string       `json:"version"`
	DefaultEffect string       `json:"defaultEffect"`
	Rules         []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	ID                     string   `json:"id"`
	Effect                 string   `json:"effect"`
	AgentID                string   `json:"agentId"`
	Tool                   string   `json:"tool"`
	Action                 string   `json:"action"`
	Resource               string   `json:"resource"`
	ResourceTenant         string   `json:"resourceTenant"`
	ResourceClassification string   `json:"resourceClassification"`
	Redactions             []string `json:"redactions,omitempty"`
}

type AgentRun struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	RunID      string             `json:"runId"`
	TrialID    string             `json:"trialId"`
	Scenario   ScenarioRef        `json:"scenario"`
	Agent      AgentRef           `json:"agent"`
	Policy     PolicyRef          `json:"policy"`
	PromptHash string             `json:"promptHash"`
	Tools      []ToolRef          `json:"tools"`
	Execution  *AgentExecutionRef `json:"execution,omitempty"`
	Trial      TrialRef           `json:"trial"`
	Status     string             `json:"status"`
	StartedAt  time.Time          `json:"startedAt"`
	EndedAt    *time.Time         `json:"endedAt,omitempty"`
}

type ScenarioRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
	Seed    int64  `json:"seed"`
}

type AgentRef struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Adapter  string `json:"adapter"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type PolicyRef struct {
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type ToolRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type TrialRef struct {
	Index int `json:"index"`
	Count int `json:"count"`
}

// AgentExecutionRef records an enforced execution boundary. Its absence means
// the documented host subprocess mode.
type AgentExecutionRef struct {
	Mode       string `json:"mode"`
	Engine     string `json:"engine"`
	Profile    string `json:"profile"`
	Image      string `json:"image"`
	Network    string `json:"network"`
	Filesystem string `json:"filesystem"`
}

type Decision struct {
	Effect        string   `json:"effect"`
	ReasonCode    string   `json:"reasonCode"`
	PolicyVersion string   `json:"policyVersion"`
	Redactions    []string `json:"redactions,omitempty"`
	ApprovalID    string   `json:"approvalId,omitempty"`
}

type DecisionEvent struct {
	APIVersion    string      `json:"apiVersion"`
	Kind          string      `json:"kind"`
	EventID       string      `json:"eventId"`
	Sequence      uint64      `json:"sequence"`
	OccurredAt    time.Time   `json:"occurredAt"`
	RunID         string      `json:"runId"`
	TrialID       string      `json:"trialId"`
	CorrelationID string      `json:"correlationId"`
	Actor         ActorRef    `json:"actor"`
	Tool          ToolRef     `json:"tool"`
	Action        string      `json:"action"`
	Resource      ResourceRef `json:"resource"`
	Decision      Decision    `json:"decision"`
	Outcome       Outcome     `json:"outcome"`
	InputHash     string      `json:"inputHash"`
	OutputHash    string      `json:"outputHash,omitempty"`
}

// DecisionEventDraft contains a complete decision record except for the
// append-only event identifier and sequence assigned by the evidence store.
type DecisionEventDraft struct {
	OccurredAt    time.Time
	RunID         string
	TrialID       string
	CorrelationID string
	Actor         ActorRef
	Tool          ToolRef
	Action        string
	Resource      ResourceRef
	Decision      Decision
	Outcome       Outcome
	InputHash     string
	OutputHash    string
}

type ToolOutcomeEvent struct {
	APIVersion      string    `json:"apiVersion"`
	Kind            string    `json:"kind"`
	EventID         string    `json:"eventId"`
	OccurredAt      time.Time `json:"occurredAt"`
	RunID           string    `json:"runId"`
	TrialID         string    `json:"trialId"`
	CorrelationID   string    `json:"correlationId"`
	DecisionEventID string    `json:"decisionEventId"`
	ApprovalEventID string    `json:"approvalEventId,omitempty"`
	Tool            ToolRef   `json:"tool"`
	Outcome         Outcome   `json:"outcome"`
	OutputHash      string    `json:"outputHash,omitempty"`
}

type ToolOutcomeEventDraft struct {
	OccurredAt      time.Time
	RunID           string
	TrialID         string
	CorrelationID   string
	DecisionEventID string
	ApprovalEventID string
	Tool            ToolRef
	Outcome         Outcome
	OutputHash      string
}

type ApprovalRequest struct {
	ApprovalID      string
	DecisionEventID string
	RunID           string
	TrialID         string
	CorrelationID   string
	Actor           ActorRef
	Tool            ToolRef
	Action          string
	Resource        ResourceRef
	ReasonCode      string
	InputHash       string
}

type ApprovalResolution struct {
	Approved   bool
	ResolvedBy string
}

type Approver interface {
	ResolveApproval(context.Context, ApprovalRequest) (ApprovalResolution, error)
}

type ApproverFunc func(context.Context, ApprovalRequest) (ApprovalResolution, error)

func (f ApproverFunc) ResolveApproval(ctx context.Context, request ApprovalRequest) (ApprovalResolution, error) {
	return f(ctx, request)
}

type ApprovalResolutionEvent struct {
	APIVersion      string      `json:"apiVersion"`
	Kind            string      `json:"kind"`
	EventID         string      `json:"eventId"`
	OccurredAt      time.Time   `json:"occurredAt"`
	RunID           string      `json:"runId"`
	TrialID         string      `json:"trialId"`
	CorrelationID   string      `json:"correlationId"`
	ApprovalID      string      `json:"approvalId"`
	DecisionEventID string      `json:"decisionEventId"`
	ResolvedBy      string      `json:"resolvedBy"`
	Approved        bool        `json:"approved"`
	Tool            ToolRef     `json:"tool"`
	Action          string      `json:"action"`
	Resource        ResourceRef `json:"resource"`
	Decision        Decision    `json:"decision"`
	InputHash       string      `json:"inputHash"`
}

type ApprovalResolutionEventDraft struct {
	OccurredAt      time.Time
	RunID           string
	TrialID         string
	CorrelationID   string
	ApprovalID      string
	DecisionEventID string
	ResolvedBy      string
	Approved        bool
	Tool            ToolRef
	Action          string
	Resource        ResourceRef
	Decision        Decision
	InputHash       string
}

type ToolExecutionRequest struct {
	ProtocolVersion string          `json:"protocolVersion"`
	CallID          string          `json:"callId"`
	Tool            string          `json:"tool"`
	Action          string          `json:"action"`
	Resource        ResourceRef     `json:"resource"`
	Arguments       json.RawMessage `json:"arguments"`
}

type ToolExecutionResponse struct {
	ProtocolVersion string          `json:"protocolVersion"`
	CallID          string          `json:"callId"`
	Status          string          `json:"status"`
	Content         json.RawMessage `json:"content,omitempty"`
	ErrorCode       string          `json:"errorCode,omitempty"`
}

type ActorRef struct {
	ID     string `json:"id"`
	Tenant string `json:"tenant"`
	Type   string `json:"type"`
}

type ResourceRef struct {
	ID             string `json:"id"`
	Tenant         string `json:"tenant"`
	Classification string `json:"classification"`
}

type Outcome struct {
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
}

type Message struct {
	ProtocolVersion string          `json:"protocolVersion"`
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	Payload         json.RawMessage `json:"payload"`
}

type SessionStartPayload struct {
	RunID          string    `json:"runId"`
	TrialID        string    `json:"trialId"`
	ScenarioDigest string    `json:"scenarioDigest"`
	PolicyVersion  string    `json:"policyVersion"`
	Tools          []ToolRef `json:"tools"`
}

type AgentReadyPayload struct {
	AgentID      string `json:"agentId"`
	AgentVersion string `json:"agentVersion"`
}

type ToolCallPayload struct {
	Tool      string          `json:"tool"`
	Action    string          `json:"action"`
	Resource  string          `json:"resource"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResultPayload struct {
	Tool      string          `json:"tool"`
	Status    string          `json:"status"`
	Content   json.RawMessage `json:"content,omitempty"`
	ErrorCode string          `json:"errorCode,omitempty"`
	Decision  Decision        `json:"decision"`
}

type ApprovalRequiredPayload struct {
	ApprovalID string `json:"approvalId"`
	ToolCallID string `json:"toolCallId"`
	Reason     string `json:"reason"`
}

type ApprovalResolvedPayload struct {
	ApprovalID string `json:"approvalId"`
	Approved   *bool  `json:"approved"`
	ResolvedBy string `json:"resolvedBy"`
}

type SessionCompletePayload struct {
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
}

type ProtocolErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable *bool  `json:"retryable"`
}
