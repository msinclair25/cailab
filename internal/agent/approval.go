package agent

import "fmt"

func BuildApprovalResolutionEvent(draft ApprovalResolutionEventDraft, eventID string) (ApprovalResolutionEvent, error) {
	event := ApprovalResolutionEvent{
		APIVersion: APIVersion, Kind: ApprovalResolutionEventKind, EventID: eventID,
		OccurredAt: draft.OccurredAt, RunID: draft.RunID, TrialID: draft.TrialID,
		CorrelationID: draft.CorrelationID, ApprovalID: draft.ApprovalID, DecisionEventID: draft.DecisionEventID,
		ResolvedBy: draft.ResolvedBy, Approved: draft.Approved, Tool: draft.Tool,
		Action: draft.Action, Resource: draft.Resource, Decision: draft.Decision, InputHash: draft.InputHash,
	}
	if err := ValidateApprovalResolutionEvent(event); err != nil {
		return ApprovalResolutionEvent{}, err
	}
	return event, nil
}

func ValidateApprovalResolutionEvent(event ApprovalResolutionEvent) error {
	var issues []string
	requireEqual(&issues, "apiVersion", event.APIVersion, APIVersion)
	requireEqual(&issues, "kind", event.Kind, ApprovalResolutionEventKind)
	validateID(&issues, "eventId", event.EventID)
	validateTimestamp(&issues, "occurredAt", event.OccurredAt)
	validateID(&issues, "runId", event.RunID)
	validateID(&issues, "trialId", event.TrialID)
	validateID(&issues, "correlationId", event.CorrelationID)
	validateID(&issues, "approvalId", event.ApprovalID)
	validateID(&issues, "decisionEventId", event.DecisionEventID)
	validateID(&issues, "resolvedBy", event.ResolvedBy)
	validateToolRef(&issues, "tool", event.Tool)
	requireSafeText(&issues, "action", event.Action)
	validateID(&issues, "resource.id", event.Resource.ID)
	validateID(&issues, "resource.tenant", event.Resource.Tenant)
	if !contains([]string{"public", "internal", "confidential", "restricted"}, event.Resource.Classification) {
		issues = append(issues, fmt.Sprintf("resource.classification has unsupported value %q", event.Resource.Classification))
	}
	if err := ValidateDecision(event.Decision); err != nil {
		appendNestedIssues(&issues, "decision", err)
	}
	if event.Decision.Effect == "require_approval" {
		issues = append(issues, "resolved decision must not require approval")
	}
	if event.Approved && event.Decision.Effect != "allow" && event.Decision.Effect != "redact" {
		issues = append(issues, "approved resolution must produce allow or redact")
	}
	if !event.Approved && (event.Decision.Effect != "deny" || event.Decision.ReasonCode != "approval:rejected") {
		issues = append(issues, "rejected approval must produce approval:rejected deny")
	}
	validateDigest(&issues, "inputHash", event.InputHash)
	return validationResult(issues)
}
