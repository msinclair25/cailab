package state

import (
	"context"
	"fmt"

	"github.com/msinclair25/cailab/internal/agent"
)

// AgentTrace reads one terminal trial through every integrity-checking evidence
// path. It does not expose SQLite chain bookkeeping or raw protocol content.
func (s *Store) AgentTrace(ctx context.Context, runID, trialID string) (agent.AgentTrace, error) {
	run, err := s.AgentRun(ctx, runID, trialID)
	if err != nil {
		return agent.AgentTrace{}, err
	}
	decisions, err := s.DecisionEvents(ctx, runID, trialID)
	if err != nil {
		return agent.AgentTrace{}, fmt.Errorf("read trace decisions: %w", err)
	}
	approvals, err := s.ApprovalResolutionEvents(ctx, runID, trialID)
	if err != nil {
		return agent.AgentTrace{}, fmt.Errorf("read trace approvals: %w", err)
	}
	outcomes, err := s.ToolOutcomeEvents(ctx, runID, trialID)
	if err != nil {
		return agent.AgentTrace{}, fmt.Errorf("read trace outcomes: %w", err)
	}
	states, err := s.TrialStateEvidence(ctx, runID, trialID)
	if err != nil {
		return agent.AgentTrace{}, fmt.Errorf("read trace states: %w", err)
	}
	return agent.AgentTrace{
		APIVersion: agent.APIVersion, Kind: agent.AgentTraceKind, Run: run,
		Decisions: decisions, Approvals: approvals, Outcomes: outcomes, States: states,
	}, nil
}
