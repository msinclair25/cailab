package agent

import (
	"encoding/json"
	"fmt"
)

func ValidateMessage(message Message) error {
	var issues []string
	requireEqual(&issues, "protocolVersion", message.ProtocolVersion, ProtocolVersion)
	validateID(&issues, "id", message.ID)
	if err := rejectDuplicateJSONKeys(message.Payload); err != nil {
		issues = append(issues, "payload is invalid: "+err.Error())
		return validationResult(issues)
	}
	if message.CorrelationID != "" {
		validateID(&issues, "correlationId", message.CorrelationID)
	}
	requireCorrelation := false
	switch message.Type {
	case MessageSessionStart:
		var payload SessionStartPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.runId", payload.RunID)
		validateID(&issues, "payload.trialId", payload.TrialID)
		validateDigest(&issues, "payload.scenarioDigest", payload.ScenarioDigest)
		validateVersion(&issues, "payload.policyVersion", payload.PolicyVersion)
		if len(payload.Tools) == 0 {
			issues = append(issues, "payload.tools must contain at least one tool")
		}
		for i, tool := range payload.Tools {
			validateToolRef(&issues, fmt.Sprintf("payload.tools[%d]", i), tool)
		}
	case MessageAgentReady:
		var payload AgentReadyPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.agentId", payload.AgentID)
		validateVersion(&issues, "payload.agentVersion", payload.AgentVersion)
	case MessageToolCall:
		var payload ToolCallPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.tool", payload.Tool)
		requireText(&issues, "payload.action", payload.Action)
		validateID(&issues, "payload.resource", payload.Resource)
		validateJSONObject(&issues, "payload.arguments", payload.Arguments)
	case MessageToolResult:
		requireCorrelation = true
		var payload ToolResultPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.tool", payload.Tool)
		if !contains([]string{"succeeded", "failed", "not_executed"}, payload.Status) {
			issues = append(issues, fmt.Sprintf("payload.status has unsupported value %q", payload.Status))
		}
		if len(payload.Content) > 0 {
			validateJSONValue(&issues, "payload.content", payload.Content)
		}
		if err := ValidateDecision(payload.Decision); err != nil {
			appendNestedIssues(&issues, "payload.decision", err)
		}
		if (payload.Decision.Effect == "deny" || payload.Decision.Effect == "require_approval") && payload.Status != "not_executed" {
			issues = append(issues, "denied or approval-pending tool results must be not_executed")
		}
		if payload.Status == "not_executed" && len(payload.Content) > 0 {
			issues = append(issues, "not_executed tool results must not include content")
		}
		if payload.Status == "succeeded" {
			if len(payload.Content) == 0 {
				issues = append(issues, "succeeded tool results must include content")
			}
			if payload.ErrorCode != "" {
				issues = append(issues, "succeeded tool results must not include errorCode")
			}
		} else if payload.Status == "failed" {
			validateID(&issues, "payload.errorCode", payload.ErrorCode)
			if len(payload.Content) > 0 {
				issues = append(issues, "failed tool results must not include content")
			}
		} else if payload.ErrorCode != "" {
			issues = append(issues, "not_executed tool results must not include errorCode")
		}
	case MessageApprovalRequired:
		requireCorrelation = true
		var payload ApprovalRequiredPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.approvalId", payload.ApprovalID)
		validateID(&issues, "payload.toolCallId", payload.ToolCallID)
		requireText(&issues, "payload.reason", payload.Reason)
	case MessageApprovalResolved:
		requireCorrelation = true
		var payload ApprovalResolvedPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.approvalId", payload.ApprovalID)
		if payload.Approved == nil {
			issues = append(issues, "payload.approved is required")
		}
		validateID(&issues, "payload.resolvedBy", payload.ResolvedBy)
	case MessageSessionComplete:
		var payload SessionCompletePayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		if !contains([]string{"completed", "failed", "canceled"}, payload.Status) {
			issues = append(issues, fmt.Sprintf("payload.status has unsupported value %q", payload.Status))
		}
	case MessageProtocolError:
		var payload ProtocolErrorPayload
		decodeMessagePayload(&issues, message.Payload, &payload)
		validateID(&issues, "payload.code", payload.Code)
		requireText(&issues, "payload.message", payload.Message)
		if payload.Retryable == nil {
			issues = append(issues, "payload.retryable is required")
		}
	default:
		issues = append(issues, fmt.Sprintf("type has unsupported value %q", message.Type))
	}
	if requireCorrelation && message.CorrelationID == "" {
		issues = append(issues, "correlationId is required for this message type")
	}
	return validationResult(issues)
}

func decodeMessagePayload[T any](issues *[]string, raw json.RawMessage, target *T) {
	if err := decodeStrict(raw, target); err != nil {
		*issues = append(*issues, "payload does not match message type: "+err.Error())
	}
}

func validateJSONObject(issues *[]string, field string, raw json.RawMessage) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		*issues = append(*issues, field+" is invalid: "+err.Error())
		return
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		*issues = append(*issues, field+" must be a JSON object")
	}
}

func validateJSONValue(issues *[]string, field string, raw json.RawMessage) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		*issues = append(*issues, field+" is invalid: "+err.Error())
	}
}
