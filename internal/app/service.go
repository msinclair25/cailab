package app

import (
	"context"
	"fmt"

	"github.com/msinclair25/cailab/internal/graph"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
	"github.com/msinclair25/cailab/internal/verify"
)

type Service struct {
	store *state.Store
}

type UpOptions struct {
	ScenarioPath string
	Seed         *int64
}

func New(store *state.Store) *Service {
	return &Service{store: store}
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
	return verify.Evaluate(run.ID, run.Compiled)
}

func (s *Service) Path(ctx context.Context, from, to string) (graph.Path, bool, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return graph.Path{}, false, err
	}
	g, err := graph.New(run.Compiled.Nodes, run.Compiled.Edges)
	if err != nil {
		return graph.Path{}, false, fmt.Errorf("build active graph: %w", err)
	}
	path, ok := g.FindPath(from, to)
	return path, ok, nil
}

func (s *Service) Reset(ctx context.Context) (state.Run, error) {
	return s.store.ResetActiveRun(ctx)
}

func (s *Service) Down(ctx context.Context) (state.Run, error) {
	return s.store.StopActiveRun(ctx)
}
