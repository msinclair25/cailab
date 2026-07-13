package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/state"
)

func TestRunAgentCampaignRestoresEveryTrialAndReturnsAggregate(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	manager := &fakeProviderManager{}
	service.provider = manager
	options := appAgentTestOptions(t, rangeRun.Compiled, "ignored", "tool")
	options.CaptureState = true
	options.RestoreFixture = true

	result, err := service.RunAgentCampaign(ctx, AgentCampaignOptions{
		Run: options, TrialPrefix: "campaign:reference", Trials: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.restoreCount != 3 || len(result.Runs) != 3 || len(result.TrialIDs) != 3 {
		t.Fatalf("restore count = %d, runs = %d, trial IDs = %v", manager.restoreCount, len(result.Runs), result.TrialIDs)
	}
	if result.TrialIDs[0] != "campaign:reference:1" || result.Runs[2].Run.Trial.Index != 3 || result.Runs[2].Run.Trial.Count != 3 {
		t.Fatalf("campaign result = %+v", result)
	}
	if result.Report.Profile != agent.ScenarioOutcomeProfile || result.Report.Aggregate.Trials != 3 ||
		result.Report.Aggregate.CompletedTrials.Numerator != 3 || result.Report.Aggregate.InitialStateMatchRate == nil ||
		result.Report.Aggregate.InitialStateMatchRate.Numerator != 3 {
		t.Fatalf("report = %+v", result.Report)
	}
}

func TestRunAgentCampaignMeasuresTerminalAgentFailures(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	service.provider = &fakeProviderManager{}
	options := appAgentTestOptions(t, rangeRun.Compiled, "ignored", "malformed")
	options.CaptureState = true
	options.RestoreFixture = true

	result, err := service.RunAgentCampaign(ctx, AgentCampaignOptions{
		Run: options, TrialPrefix: "campaign:failed", Trials: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Report.Aggregate.Trials != 2 || result.Report.Aggregate.CompletedTrials.Numerator != 0 ||
		result.Report.Trials[0].Status != "failed" || result.Report.Trials[1].Status != "failed" {
		t.Fatalf("report = %+v", result.Report)
	}
}

func TestRunAgentCampaignPreflightsAllTrialIDs(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	existing := appAgentTestOptions(t, rangeRun.Compiled, "campaign:duplicate:2", "reference")
	if _, err := service.RunAgent(ctx, existing); err != nil {
		t.Fatal(err)
	}
	service.provider = &fakeProviderManager{}
	options := appAgentTestOptions(t, rangeRun.Compiled, "ignored", "reference")
	options.CaptureState = true
	options.RestoreFixture = true

	_, err := service.RunAgentCampaign(ctx, AgentCampaignOptions{
		Run: options, TrialPrefix: "campaign:duplicate", Trials: 3,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("campaign error = %v", err)
	}
	if _, err := store.AgentRun(ctx, rangeRun.ID, "campaign:duplicate:1"); !errors.Is(err, state.ErrNoActiveAgentRun) {
		t.Fatalf("campaign persisted before preflight completed: %v", err)
	}
}

func TestRunAgentCampaignStopsWhenRestorationCannotProduceEvidence(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	service.provider = &fakeProviderManager{restoreErr: errors.New("restore unavailable")}
	options := appAgentTestOptions(t, rangeRun.Compiled, "ignored", "reference")
	options.CaptureState = true
	options.RestoreFixture = true

	result, err := service.RunAgentCampaign(ctx, AgentCampaignOptions{
		Run: options, TrialPrefix: "campaign:restore", Trials: 3,
	})
	var campaignErr *AgentCampaignError
	if !errors.As(err, &campaignErr) || campaignErr.TrialID != "campaign:restore:1" || campaignErr.Recorded != 1 {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if !strings.Contains(err.Error(), "incomplete set cannot be replayed") || len(result.Runs) != 1 || result.Runs[0].Run.Status != "failed" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if _, err := store.AgentRun(ctx, rangeRun.ID, "campaign:restore:2"); !errors.Is(err, state.ErrNoActiveAgentRun) {
		t.Fatalf("unexpected second trial: %v", err)
	}
}

func TestRunAgentCampaignValidatesBoundedRestoredConfiguration(t *testing.T) {
	ctx := context.Background()
	store, service, rangeRun := appAgentTestService(t, ctx)
	defer store.Close()
	options := appAgentTestOptions(t, rangeRun.Compiled, "ignored", "reference")

	for _, test := range []struct {
		name       string
		campaign   AgentCampaignOptions
		wantPhrase string
	}{
		{name: "single", campaign: AgentCampaignOptions{Run: options, TrialPrefix: "campaign:one", Trials: 1}, wantPhrase: "between 2 and"},
		{name: "no restore", campaign: AgentCampaignOptions{Run: options, TrialPrefix: "campaign:no-restore", Trials: 2}, wantPhrase: "require fixture restoration"},
		{name: "invalid prefix", campaign: func() AgentCampaignOptions {
			value := options
			value.CaptureState = true
			value.RestoreFixture = true
			return AgentCampaignOptions{Run: value, TrialPrefix: "Campaign Bad", Trials: 2}
		}(), wantPhrase: "validate campaign trial prefix"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.RunAgentCampaign(ctx, test.campaign); err == nil || !strings.Contains(err.Error(), test.wantPhrase) {
				t.Fatalf("campaign error = %v", err)
			}
		})
	}
}
