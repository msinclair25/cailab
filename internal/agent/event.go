package agent

import "fmt"

func ValidateDecisionEventDraft(draft DecisionEventDraft) error {
	_, err := BuildDecisionEvent(draft, "event:pending", 1)
	return err
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
