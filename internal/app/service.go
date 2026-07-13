package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/msinclair25/cailab/internal/graph"
	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
	"github.com/msinclair25/cailab/internal/verify"
)

type Service struct {
	store    *state.Store
	provider provider.Manager
	clock    func() time.Time
}

type UpOptions struct {
	ScenarioReference string
	ScenarioRoot      string
	Seed              *int64
}

func New(store *state.Store, providerManager provider.Manager) *Service {
	return &Service{store: store, provider: providerManager, clock: time.Now}
}

func (s *Service) Up(ctx context.Context, options UpOptions) (state.Run, error) {
	definition, err := scenario.LoadReference(options.ScenarioRoot, options.ScenarioReference)
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
	baseline, err := s.provider.Snapshot(ctx, instances, compiled)
	if err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, compiled)
		_, _ = s.store.StopActiveRun(context.Background())
		return state.Run{}, fmt.Errorf("capture normalized provider baseline: %w", err)
	}
	baselineDigest, err := scenario.StateDigest(baseline)
	if err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, compiled)
		_, _ = s.store.StopActiveRun(context.Background())
		return state.Run{}, fmt.Errorf("digest normalized provider baseline: %w", err)
	}
	if err := s.store.SetRuntimeBaseline(ctx, run.ID, instances, baselineDigest); err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, compiled)
		_, _ = s.store.StopActiveRun(context.Background())
		return state.Run{}, err
	}
	run.Runtimes = instances
	run.BaselineDigest = baselineDigest
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

func (s *Service) RotateIdentity(ctx context.Context) (provider.OIDCJWKSet, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return provider.OIDCJWKSet{}, err
	}
	return s.provider.RotateIdentity(ctx, run.ID, run.Runtimes)
}

func (s *Service) ValidateIdentity(ctx context.Context, token, tokenType, audience string) (provider.OIDCClaims, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return provider.OIDCClaims{}, err
	}
	for _, runtime := range run.Runtimes {
		if runtime.Provider == "oidc" && runtime.Engine == "native" {
			return provider.ValidateOIDCRuntimeToken(ctx, runtime.Endpoint, token, tokenType, audience)
		}
	}
	return provider.OIDCClaims{}, errors.New("active scenario has no OIDC runtime")
}

func (s *Service) AssumeAWSWebIdentity(ctx context.Context, token, roleNode string) (provider.FederatedCredentials, error) {
	run, err := s.store.ActiveRun(ctx)
	if err != nil {
		return provider.FederatedCredentials{}, err
	}
	if run.Compiled.Providers.AWS == nil {
		return provider.FederatedCredentials{}, errors.New("active scenario has no AWS provider")
	}
	var oidcEndpoint, awsEndpoint string
	for _, runtime := range run.Runtimes {
		switch {
		case runtime.Provider == "oidc" && runtime.Engine == "native":
			oidcEndpoint = runtime.Endpoint
		case runtime.Provider == "aws" && runtime.Engine == "floci":
			awsEndpoint = runtime.Endpoint
		}
	}
	if oidcEndpoint == "" || awsEndpoint == "" {
		return provider.FederatedCredentials{}, errors.New("active scenario does not have recorded OIDC and AWS runtimes")
	}
	role, ok := findAWSRole(run.Compiled.Providers.AWS.Roles, roleNode)
	if !ok || role.WebIdentity == nil {
		return provider.FederatedCredentials{}, fmt.Errorf("AWS role %q has no declared web-identity trust", roleNode)
	}
	claims, err := provider.ValidateOIDCRuntimeToken(ctx, oidcEndpoint, token, "access", role.WebIdentity.Audience)
	if err != nil {
		return provider.FederatedCredentials{}, fmt.Errorf("validate federation token: %w", err)
	}
	snapshot, err := s.provider.Snapshot(ctx, run.Runtimes, run.Compiled)
	if err != nil {
		return provider.FederatedCredentials{}, fmt.Errorf("snapshot provider state: %w", err)
	}
	authorizedRole, err := provider.AuthorizeAWSWebIdentity(snapshot, claims, roleNode)
	if err != nil {
		return provider.FederatedCredentials{}, fmt.Errorf("authorize federation: %w", err)
	}
	return provider.AssumeAWSWebIdentity(ctx, awsEndpoint, run.Compiled.Providers.AWS.Region, authorizedRole, token, "cailab-federation")
}

func findAWSRole(roles []scenario.AWSRole, node string) (scenario.AWSRole, bool) {
	for _, role := range roles {
		if role.Node == node {
			return role, true
		}
	}
	return scenario.AWSRole{}, false
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
	baseline, err := s.provider.Snapshot(ctx, instances, run.Compiled)
	if err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, run.Compiled)
		return state.Run{}, fmt.Errorf("capture normalized provider baseline: %w", err)
	}
	baselineDigest, err := scenario.StateDigest(baseline)
	if err != nil {
		_ = s.provider.Stop(context.Background(), run.ID, instances, run.Compiled)
		return state.Run{}, fmt.Errorf("digest normalized provider baseline: %w", err)
	}
	if err := s.store.SetRuntimeBaseline(ctx, run.ID, instances, baselineDigest); err != nil {
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
	run.BaselineDigest = baselineDigest
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
	return len(run.Runtimes) > 0 || run.Compiled.Runtimes.AWS != nil || run.Compiled.Runtimes.Microsoft != nil || run.Compiled.Runtimes.Google != nil || run.Compiled.Runtimes.OIDC != nil
}
