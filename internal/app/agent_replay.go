package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/msinclair25/cailab/internal/agent"
)

func (s *Service) ReplayAgentTrials(ctx context.Context, runID string, trialIDs []string) (agent.AgentEvaluationReport, error) {
	if len(trialIDs) == 0 {
		return agent.AgentEvaluationReport{}, errors.New("at least one trial ID is required")
	}
	if runID == "" {
		run, err := s.store.ActiveRun(ctx)
		if err != nil {
			return agent.AgentEvaluationReport{}, err
		}
		runID = run.ID
	}
	traces := make([]agent.AgentTrace, 0, len(trialIDs))
	seen := make(map[string]struct{}, len(trialIDs))
	for _, trialID := range trialIDs {
		if _, exists := seen[trialID]; exists {
			return agent.AgentEvaluationReport{}, fmt.Errorf("trial ID %q was selected more than once", trialID)
		}
		seen[trialID] = struct{}{}
		trace, err := s.store.AgentTrace(ctx, runID, trialID)
		if err != nil {
			return agent.AgentEvaluationReport{}, fmt.Errorf("read agent trace %q: %w", trialID, err)
		}
		traces = append(traces, trace)
	}
	report, err := agent.ReplayAgentTraces(traces)
	if err != nil {
		return agent.AgentEvaluationReport{}, fmt.Errorf("replay agent traces: %w", err)
	}
	return report, nil
}
