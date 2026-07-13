package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/scenario"
)

const agentRunPersistenceTimeout = 2 * time.Second

type RegisteredTool struct {
	Manifest    agent.ToolManifest
	Directory   string
	Environment []string
}

type AgentRunOptions struct {
	Agent          agent.AgentRef
	ActorTenant    string
	Command        []string
	Directory      string
	Environment    []string
	Container      *agent.ContainerRuntime
	Policy         agent.GovernancePolicy
	Tools          []RegisteredTool
	Approver       agent.Approver
	PromptHash     string
	TrialID        string
	TrialIndex     int
	TrialCount     int
	SessionTimeout time.Duration
}

type AgentRunResult struct {
	Run       agent.AgentRun
	Session   agent.SessionResult
	Decisions []agent.DecisionEvent
	Approvals []agent.ApprovalResolutionEvent
	Outcomes  []agent.ToolOutcomeEvent
}

func ValidateAgentRunOptions(compiled scenario.Compiled, options AgentRunOptions) error {
	_, _, err := validateAgentRegistrations(compiled, options)
	return err
}

func (s *Service) RunAgent(ctx context.Context, options AgentRunOptions) (AgentRunResult, error) {
	rangeRun, err := s.store.ActiveRun(ctx)
	if err != nil {
		return AgentRunResult{}, err
	}
	registrations, refs, err := validateAgentRegistrations(rangeRun.Compiled, options)
	if err != nil {
		return AgentRunResult{}, err
	}
	policyDigest, err := agent.DigestGovernancePolicy(options.Policy)
	if err != nil {
		return AgentRunResult{}, fmt.Errorf("validate governance policy: %w", err)
	}
	startedAt := s.clock().UTC()
	run := agent.AgentRun{
		APIVersion: agent.APIVersion,
		Kind:       agent.AgentRunKind,
		RunID:      rangeRun.ID,
		TrialID:    options.TrialID,
		Scenario: agent.ScenarioRef{
			Name: rangeRun.ScenarioName, Version: rangeRun.ScenarioVersion,
			Digest: rangeRun.Compiled.Digest, Seed: rangeRun.Seed,
		},
		Agent:      options.Agent,
		Policy:     agent.PolicyRef{Version: options.Policy.Version, Digest: policyDigest},
		PromptHash: options.PromptHash,
		Tools:      refs,
		Execution:  agent.ContainerExecutionRef(options.Container),
		Trial:      agent.TrialRef{Index: options.TrialIndex, Count: options.TrialCount},
		Status:     "running",
		StartedAt:  startedAt,
	}
	sessionConfig := agent.SessionConfig{
		Command: options.Command, Directory: options.Directory, Environment: options.Environment,
		Container: options.Container, Run: run, SessionTimeout: options.SessionTimeout,
	}
	if err := agent.ValidateSessionConfig(sessionConfig); err != nil {
		return AgentRunResult{}, fmt.Errorf("validate agent runtime: %w", err)
	}
	if err := s.store.BeginAgentRun(ctx, run); err != nil {
		return AgentRunResult{}, fmt.Errorf("begin agent trial %q: %w", run.TrialID, err)
	}

	resolver := newRegisteredToolResolver(rangeRun.Compiled, registrations)
	gateway := &agent.Gateway{
		Run:      run,
		Actor:    agent.ActorRef{ID: run.Agent.ID, Tenant: options.ActorTenant, Type: "agent"},
		Policy:   options.Policy,
		Resolver: resolver,
		Executor: registeredToolExecutor{tools: registrations},
		Approver: options.Approver,
		Events:   s.store,
		Clock:    s.clock,
	}
	session, sessionErr := agent.RunSession(ctx, sessionConfig, gateway)
	terminal := run
	endedAt := s.clock().UTC()
	terminal.EndedAt = &endedAt
	if sessionErr == nil {
		terminal.Status = session.Completion.Status
	} else if errors.Is(sessionErr, context.Canceled) {
		terminal.Status = "canceled"
	} else {
		terminal.Status = "failed"
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), agentRunPersistenceTimeout)
	defer cancel()
	if err := s.store.CompleteAgentRun(persistCtx, terminal); err != nil {
		return AgentRunResult{Run: terminal, Session: session}, fmt.Errorf("complete agent trial %q: %w", run.TrialID, err)
	}
	decisions, err := s.store.DecisionEvents(persistCtx, run.RunID, run.TrialID)
	if err != nil {
		return AgentRunResult{Run: terminal, Session: session}, fmt.Errorf("read agent trial decisions: %w", err)
	}
	approvals, err := s.store.ApprovalResolutionEvents(persistCtx, run.RunID, run.TrialID)
	if err != nil {
		return AgentRunResult{Run: terminal, Session: session, Decisions: decisions}, fmt.Errorf("read agent trial approvals: %w", err)
	}
	outcomes, err := s.store.ToolOutcomeEvents(persistCtx, run.RunID, run.TrialID)
	if err != nil {
		return AgentRunResult{Run: terminal, Session: session, Decisions: decisions, Approvals: approvals}, fmt.Errorf("read agent trial outcomes: %w", err)
	}
	result := AgentRunResult{Run: terminal, Session: session, Decisions: decisions, Approvals: approvals, Outcomes: outcomes}
	if sessionErr != nil {
		return result, sessionErr
	}
	return result, nil
}

func validateAgentRegistrations(compiled scenario.Compiled, options AgentRunOptions) (map[string]RegisteredTool, []agent.ToolRef, error) {
	if options.ActorTenant == "" {
		return nil, nil, errors.New("agent actor tenant is required")
	}
	if !containsTenant(compiled, options.ActorTenant) {
		return nil, nil, fmt.Errorf("agent actor tenant %q is not in the active scenario", options.ActorTenant)
	}
	if len(options.Tools) == 0 {
		return nil, nil, errors.New("at least one registered tool is required")
	}
	registrations := make(map[string]RegisteredTool, len(options.Tools))
	refs := make([]agent.ToolRef, 0, len(options.Tools))
	for _, registration := range options.Tools {
		name := registration.Manifest.Metadata.Name
		if _, exists := registrations[name]; exists {
			return nil, nil, fmt.Errorf("tool %q is registered more than once", name)
		}
		if err := agent.ValidateToolRuntime(registration.Manifest, registration.Directory, registration.Environment); err != nil {
			return nil, nil, fmt.Errorf("validate tool %q registration: %w", name, err)
		}
		if err := validateManifestResources(compiled, registration.Manifest); err != nil {
			return nil, nil, err
		}
		digest, err := agent.DigestToolManifest(registration.Manifest)
		if err != nil {
			return nil, nil, fmt.Errorf("digest tool %q: %w", name, err)
		}
		registrations[name] = registration
		refs = append(refs, agent.ToolRef{Name: name, Version: registration.Manifest.Metadata.Version, Digest: digest})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	if err := validatePolicyResources(compiled, options.Agent.ID, options.Policy, registrations); err != nil {
		return nil, nil, err
	}
	return registrations, refs, nil
}

func validateManifestResources(compiled scenario.Compiled, manifest agent.ToolManifest) error {
	resources := scenarioResources(compiled)
	for _, permission := range manifest.Spec.Permissions {
		for _, resourceID := range permission.Resources {
			resource, exists := resources[resourceID]
			if !exists {
				return fmt.Errorf("tool %q permission resource %q is not in the active scenario", manifest.Metadata.Name, resourceID)
			}
			if resource.Tenant != permission.Tenant {
				return fmt.Errorf("tool %q permission resource %q belongs to tenant %q, not %q", manifest.Metadata.Name, resourceID, resource.Tenant, permission.Tenant)
			}
		}
	}
	return nil
}

func validatePolicyResources(compiled scenario.Compiled, agentID string, policy agent.GovernancePolicy, tools map[string]RegisteredTool) error {
	if err := agent.ValidateGovernancePolicy(policy); err != nil {
		return fmt.Errorf("validate governance policy: %w", err)
	}
	resources := scenarioResources(compiled)
	for _, rule := range policy.Rules {
		if rule.AgentID != agentID {
			continue
		}
		registration, exists := tools[rule.Tool]
		if !exists {
			return fmt.Errorf("policy rule %q references unregistered tool %q", rule.ID, rule.Tool)
		}
		resource, exists := resources[rule.Resource]
		if !exists || resource.Tenant != rule.ResourceTenant || resource.Classification != rule.ResourceClassification {
			return fmt.Errorf("policy rule %q resource metadata does not match the active scenario", rule.ID)
		}
		if !manifestAllows(registration.Manifest, rule.Action, rule.Resource, rule.ResourceTenant) {
			return fmt.Errorf("policy rule %q exceeds tool %q permission ceiling", rule.ID, rule.Tool)
		}
	}
	return nil
}

func manifestAllows(manifest agent.ToolManifest, action, resource, tenant string) bool {
	for _, permission := range manifest.Spec.Permissions {
		if permission.Tenant == tenant && containsString(permission.Actions, action) && containsString(permission.Resources, resource) {
			return true
		}
	}
	return false
}

func containsTenant(compiled scenario.Compiled, tenant string) bool {
	for _, node := range compiled.Nodes {
		if node.Kind == "tenant" && node.ID == tenant {
			return true
		}
	}
	return false
}

func scenarioResources(compiled scenario.Compiled) map[string]agent.ResourceRef {
	resources := make(map[string]agent.ResourceRef)
	for _, node := range compiled.Nodes {
		if node.Kind == "resource" {
			resources[node.ID] = agent.ResourceRef{ID: node.ID, Tenant: node.Tenant, Classification: node.Classification}
		}
	}
	return resources
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

type registeredToolResolver struct {
	resources map[string]agent.ResourceRef
	tools     map[string]RegisteredTool
}

func newRegisteredToolResolver(compiled scenario.Compiled, tools map[string]RegisteredTool) registeredToolResolver {
	return registeredToolResolver{resources: scenarioResources(compiled), tools: tools}
}

func (r registeredToolResolver) ResolveToolCall(_ context.Context, _ agent.Message, payload agent.ToolCallPayload) (agent.ToolCallResolution, error) {
	registration, exists := r.tools[payload.Tool]
	if !exists {
		return agent.ToolCallResolution{}, fmt.Errorf("tool %q is not registered", payload.Tool)
	}
	resource, exists := r.resources[payload.Resource]
	if !exists {
		return agent.ToolCallResolution{}, fmt.Errorf("resource %q is not in the active canonical scenario", payload.Resource)
	}
	return agent.ToolCallResolution{Manifest: registration.Manifest, Action: payload.Action, Resource: resource}, nil
}

type registeredToolExecutor struct {
	tools map[string]RegisteredTool
}

func (e registeredToolExecutor) Execute(ctx context.Context, manifest agent.ToolManifest, request agent.ToolExecutionRequest) (agent.ToolExecutionResult, error) {
	registration, exists := e.tools[manifest.Metadata.Name]
	if !exists {
		return agent.ToolExecutionResult{}, fmt.Errorf("tool %q is not registered", manifest.Metadata.Name)
	}
	executor := agent.SubprocessToolExecutor{Directory: registration.Directory, Environment: registration.Environment}
	return executor.Execute(ctx, manifest, request)
}

func ReferenceAgentRunOptions(compiled scenario.Compiled, executable, directory, trialID string) (AgentRunOptions, error) {
	resources := scenarioResources(compiled)
	resourceIDs := make([]string, 0, len(resources))
	for id := range resources {
		resourceIDs = append(resourceIDs, id)
	}
	sort.Strings(resourceIDs)
	if len(resourceIDs) == 0 {
		return AgentRunOptions{}, errors.New("reference baseline requires one canonical resource")
	}
	resource := resources[resourceIDs[0]]
	manifest := agent.ToolManifest{
		APIVersion: agent.APIVersion,
		Kind:       agent.ToolManifestKind,
		Metadata:   agent.Metadata{Name: "cloudailab.reference", Version: "0.1.0", Description: "Deterministic inert tool registered by the reference baseline."},
		Spec: agent.ToolManifestSpec{
			Transport:   agent.ToolTransport{Type: "subprocess", Command: []string{executable, "_tool", "reference", "--tool", "cloudailab.reference"}},
			InputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`),
			Permissions: []agent.Permission{{Tenant: resource.Tenant, Actions: []string{"cloudailab.reference"}, Resources: []string{resource.ID}}},
			Risk:        "low", TimeoutMillis: 5_000,
			Isolation: agent.Isolation{Network: "host", Filesystem: "host"},
		},
	}
	promptHash, err := agent.DigestJSON([]byte(`{"baseline":"cloudailab-reference-v1"}`))
	if err != nil {
		return AgentRunOptions{}, err
	}
	return AgentRunOptions{
		Agent:       agent.AgentRef{ID: "agent:reference", Version: "0.1.0", Adapter: "subprocess", Provider: "cloudailab", Model: "deterministic-reference"},
		ActorTenant: resource.Tenant,
		Command:     []string{executable, "_agent", "reference", "--id", "agent:reference", "--version", "0.1.0"},
		Directory:   directory,
		Policy:      agent.GovernancePolicy{APIVersion: agent.APIVersion, Kind: agent.GovernancePolicyKind, Version: "0.1.0", DefaultEffect: "deny", Rules: []agent.PolicyRule{}},
		Tools:       []RegisteredTool{{Manifest: manifest, Directory: directory}},
		PromptHash:  promptHash,
		TrialID:     trialID, TrialIndex: 1, TrialCount: 1,
		SessionTimeout: 30 * time.Second,
	}, nil
}
