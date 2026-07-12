package app

import (
	"context"
	"fmt"

	"github.com/msinclair25/cailab/internal/graph"
	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
	"github.com/msinclair25/cailab/internal/verify"
)

type Service struct {
	store    *state.Store
	provider provider.Manager
}

type UpOptions struct {
	ScenarioPath string
	Seed         *int64
}

func New(store *state.Store, providerManager provider.Manager) *Service {
	return &Service{store: store, provider: providerManager}
}

func (s *Service) Up(ctx context.Context, options UpOptions) (state.Run, error) {
	definition, err := scenario.Load(options.ScenarioPath)
	if err != nil {
		return state.Run{}, err
	}
	seed := definition.Spec.Seed
	if options.Seed != nil {
		seed = *options.Seed
	}
	compiled, err := scenario.Compile(definition, seed)
	if err != nil {
		return state.Run{}, fmt.Errorf("compile scenario: %w", err)
	}
	run, err := s.store.CreateRun(ctx, compiled)
	if err != nil {
		return state.Run{}, err
	}
	instances, err := s.provider.Start(ctx, run.ID, compiled)
	if err != nil {
		_, _ = s.store.StopActiveRun(context.Background())
		return state.Run{}, err
	}
	if err := s.store.SetRuntimes(ctx, run.ID, instances); err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, compiled)
		_, _ = s.store.StopActiveRun(context.Background())
		return state.Run{}, err
	}
	run.Runtimes = instances
	return run, nil
}

func (s *Service) Status(ctx context.Context) (state.Run, error) {
	return s.store.ActiveRun(ctx)
}

func (s *Service) Mission(ctx context.Context) (scenario.Compiled, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return scenario.Compiled{}, err
	}
	return run.Compiled, nil
}

func (s *Service) Verify(ctx context.Context) (verify.Report, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return verify.Report{}, err
	}
	compiled, err := s.provider.Snapshot(ctx, run.Runtimes, run.Compiled)
	if err != nil {
		return verify.Report{}, fmt.Errorf("snapshot provider state: %w", err)
	}
	return verify.Evaluate(run.ID, compiled)
}

func (s *Service) Path(ctx context.Context, from, to string) (graph.Path, bool, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return graph.Path{}, false, err
	}
	compiled, err := s.provider.Snapshot(ctx, run.Runtimes, run.Compiled)
	if err != nil {
		return graph.Path{}, false, fmt.Errorf("snapshot provider state: %w", err)
	}
	g, err := graph.New(compiled.Nodes, compiled.Edges)
	if err != nil {
		return graph.Path{}, false, fmt.Errorf("build active graph: %w", err)
	}
	path, ok := g.FindPath(from, to)
	return path, ok, nil
}

func (s *Service) Reset(ctx context.Context) (state.Run, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return state.Run{}, err
	}
	if hasProviderRuntime(run) {
		if err := s.provider.Stop(ctx, run.ID, run.Runtimes, run.Compiled); err != nil {
			return state.Run{}, err
		}
	}
	if err := s.store.SetRuntimes(ctx, run.ID, nil); err != nil {
		return state.Run{}, err
	}
	instances, err := s.provider.Start(ctx, run.ID, run.Compiled)
	if err != nil {
		return state.Run{}, err
	}
	if err := s.store.SetRuntimes(ctx, run.ID, instances); err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, run.Compiled)
		return state.Run{}, err
	}
	resetRun, err := s.store.ResetActiveRun(ctx)
	if err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, run.Compiled)
		_ = s.store.SetRuntimes(context.Background(), run.ID, nil)
		return state.Run{}, err
	}
	run = resetRun
	run.Runtimes = instances
	return run, nil
}

func (s *Service) Down(ctx context.Context) (state.Run, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return state.Run{}, err
	}
	if hasProviderRuntime(run) {
		if err := s.provider.Stop(ctx, run.ID, run.Runtimes, run.Compiled); err != nil {
			return state.Run{}, err
		}
	}
	return s.store.StopActiveRun(ctx)
}

func hasProviderRuntime(run state.Run) bool {
	return len(run.Runtimes) > 0 || run.Compiled.Runtimes.AWS != nil || run.Compiled.Runtimes.Microsoft != nil
}
