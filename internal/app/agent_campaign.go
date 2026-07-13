package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/state"
)

const MaxAgentCampaignTrials = 100

// AgentCampaignOptions describes one bounded repeated set. Run contains the
// immutable configuration shared by every trial; trial identity and position
// are assigned by the campaign runner.
type AgentCampaignOptions struct {
	Run         AgentRunOptions
	TrialPrefix string
	Trials      int
}

type AgentCampaignResult struct {
	Runs     []AgentRunResult
	TrialIDs []string
	Report   agent.AgentEvaluationReport
}

// AgentCampaignError reports a fail-closed partial campaign. Recorded trials
// remain immutable evidence, but the incomplete declared set cannot be replayed.
type AgentCampaignError struct {
	TrialID  string
	Recorded int
	Total    int
	Err      error
}

func (e *AgentCampaignError) Error() string {
	return fmt.Sprintf("agent campaign stopped at %q after recording %d of %d trials; the incomplete set cannot be replayed: %v", e.TrialID, e.Recorded, e.Total, e.Err)
}

func (e *AgentCampaignError) Unwrap() error { return e.Err }

// RunAgentCampaign restores and evaluates a complete repeated trial set. It
// continues across agent-level terminal failures only when the required before
// and after state evidence was captured; control-plane or evidence failures stop
// the set immediately.
func (s *Service) RunAgentCampaign(ctx context.Context, options AgentCampaignOptions) (AgentCampaignResult, error) {
	result := AgentCampaignResult{}
	if options.Trials < 2 || options.Trials > MaxAgentCampaignTrials {
		return result, fmt.Errorf("campaign trial count must be between 2 and %d", MaxAgentCampaignTrials)
	}
	if !options.Run.RestoreFixture || !options.Run.CaptureState {
		return result, errors.New("agent campaigns require fixture restoration and state capture")
	}
	if err := agent.ValidateIdentifier(options.TrialPrefix); err != nil {
		return result, fmt.Errorf("validate campaign trial prefix: %w", err)
	}
	rangeRun, err := s.store.ActiveRun(ctx)
	if err != nil {
		return result, err
	}
	if err := ValidateAgentRunOptions(rangeRun.Compiled, options.Run); err != nil {
		return result, err
	}

	result.TrialIDs = make([]string, options.Trials)
	for index := 1; index <= options.Trials; index++ {
		trialID := options.TrialPrefix + ":" + strconv.Itoa(index)
		if err := agent.ValidateIdentifier(trialID); err != nil {
			return AgentCampaignResult{}, fmt.Errorf("validate derived campaign trial ID %q: %w", trialID, err)
		}
		result.TrialIDs[index-1] = trialID
		if _, err := s.store.AgentRun(ctx, rangeRun.ID, trialID); err == nil {
			return AgentCampaignResult{}, fmt.Errorf("campaign trial ID %q already exists; choose a new --trial-prefix", trialID)
		} else if !errors.Is(err, state.ErrNoActiveAgentRun) {
			return AgentCampaignResult{}, fmt.Errorf("preflight campaign trial ID %q: %w", trialID, err)
		}
	}

	result.Runs = make([]AgentRunResult, 0, options.Trials)
	for index, trialID := range result.TrialIDs {
		if err := ctx.Err(); err != nil {
			return result, &AgentCampaignError{TrialID: trialID, Recorded: len(result.Runs), Total: options.Trials, Err: err}
		}
		trial := options.Run
		trial.TrialID = trialID
		trial.TrialIndex = index + 1
		trial.TrialCount = options.Trials
		trialResult, runErr := s.RunAgent(ctx, trial)
		if trialResult.Run.TrialID != "" {
			result.Runs = append(result.Runs, trialResult)
		}
		if runErr != nil {
			completeStateEvidence := trialResult.Run.EndedAt != nil && len(trialResult.States) == 2
			if ctx.Err() != nil || !completeStateEvidence {
				return result, &AgentCampaignError{TrialID: trialID, Recorded: len(result.Runs), Total: options.Trials, Err: runErr}
			}
		}
	}

	result.Report, err = s.ReplayAgentTrials(ctx, rangeRun.ID, result.TrialIDs)
	if err != nil {
		return result, fmt.Errorf("evaluate completed agent campaign: %w", err)
	}
	return result, nil
}
