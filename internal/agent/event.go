package agent

import "fmt"

func ValidateDecisionEventDraft(draft DecisionEventDraft) error {
	_, err := BuildDecisionEvent(draft, "event:pending", 1)
	return err
}

func ValidateToolOutcomeEvent(event ToolOutcomeEvent) error {
	var issues []string
	requireEqual(&issues, "apiVersion", event.APIVersion, APIVersion)
	requireEqual(&issues, "kind", event.Kind, ToolOutcomeEventKind)
	validateID(&issues, "eventId", event.EventID)
	validateTimestamp(&issues, "occurredAt", event.OccurredAt)
	validateID(&issues, "runId", event.RunID)
	validateID(&issues, "trialId", event.TrialID)
	validateID(&issues, "correlationId", event.CorrelationID)
	validateID(&issues, "decisionEventId", event.DecisionEventID)
	validateToolRef(&issues, "tool", event.Tool)
	if event.Outcome.Status != "succeeded" && event.Outcome.Status != "failed" {
		issues = append(issues, fmt.Sprintf("outcome.status has unsupported value %q", event.Outcome.Status))
	}
	if event.Outcome.Status == "failed" {
		validateID(&issues, "outcome.errorCode", event.Outcome.ErrorCode)
		if event.OutputHash != "" {
			issues = append(issues, "outputHash must be absent for failed outcome")
		}
	} else {
		if event.Outcome.ErrorCode != "" {
			issues = append(issues, "outcome.errorCode must be absent for succeeded outcome")
		}
		validateDigest(&issues, "outputHash", event.OutputHash)
	}
	return validationResult(issues)
}

func BuildToolOutcomeEvent(draft ToolOutcomeEventDraft, eventID string) (ToolOutcomeEvent, error) {
	event := ToolOutcomeEvent{
		APIVersion: APIVersion, Kind: ToolOutcomeEventKind, EventID: eventID,
		OccurredAt: draft.OccurredAt, RunID: draft.RunID, TrialID: draft.TrialID,
		CorrelationID: draft.CorrelationID, DecisionEventID: draft.DecisionEventID,
		Tool: draft.Tool, Outcome: draft.Outcome, OutputHash: draft.OutputHash,
	}
	if err := ValidateToolOutcomeEvent(event); err != nil {
		return ToolOutcomeEvent{}, fmt.Errorf("build tool outcome event: %w", err)
	}
	return event, nil
}

// BuildDecisionEvent completes and validates a store-assigned event identity.
func BuildDecisionEvent(draft DecisionEventDraft, eventID string, sequence uint64) (DecisionEvent, error) {
	event := DecisionEvent{
		APIVersion: APIVersion, Kind: DecisionEventKind,
		EventID: eventID, Sequence: sequence, OccurredAt: draft.OccurredAt,
		RunID: draft.RunID, TrialID: draft.TrialID, CorrelationID: draft.CorrelationID,
		Actor: draft.Actor, Tool: draft.Tool, Action: draft.Action, Resource: draft.Resource,
		Decision: draft.Decision, Outcome: draft.Outcome,
		InputHash: draft.InputHash, OutputHash: draft.OutputHash,
	}
	if err := ValidateDecisionEvent(event); err != nil {
		return DecisionEvent{}, fmt.Errorf("build decision event: %w", err)
	}
	return event, nil
}
