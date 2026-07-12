package agent

import (
	"encoding/json"
	"time"
)

const (
	APIVersion      = "cloudailab.dev/agent/v1alpha1"
	ProtocolVersion = "1.0"
	MaxFrameBytes   = 1 << 20

	ToolManifestKind  = "ToolManifest"
	AgentRunKind      = "AgentRun"
	DecisionEventKind = "DecisionEvent"
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

type AgentRun struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	RunID      string      `json:"runId"`
	TrialID    string      `json:"trialId"`
	Scenario   ScenarioRef `json:"scenario"`
	Agent      AgentRef    `json:"agent"`
	Policy     PolicyRef   `json:"policy"`
	PromptHash string      `json:"promptHash"`
	Tools      []ToolRef   `json:"tools"`
	Trial      TrialRef    `json:"trial"`
	Status     string      `json:"status"`
	StartedAt  time.Time   `json:"startedAt"`
	EndedAt    *time.Time  `json:"endedAt,omitempty"`
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
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResultPayload struct {
	Tool     string          `json:"tool"`
	Status   string          `json:"status"`
	Content  json.RawMessage `json:"content,omitempty"`
	Decision Decision        `json:"decision"`
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
