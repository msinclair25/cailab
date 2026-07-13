package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/app"
	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/scenario"
	"github.com/msinclair25/cailab/internal/state"
	"github.com/msinclair25/cailab/internal/verify"
)

const (
	ExitOK                 = 0
	ExitError              = 1
	ExitVerificationFailed = 3
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type CLI struct {
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	getenv    func(string) string
	lookupEnv func(string) (string, bool)
}

func New(stdout, stderr io.Writer) *CLI {
	return &CLI{stdin: os.Stdin, stdout: stdout, stderr: stderr, getenv: os.Getenv, lookupEnv: os.LookupEnv}
}

func (c *CLI) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		c.printUsage(c.stderr)
		return ExitError
	}

	var err error
	var code = ExitOK
	switch args[0] {
	case "help", "-h", "--help":
		c.printUsage(c.stdout)
	case "version":
		fmt.Fprintf(c.stdout, "cailab %s (commit %s, built %s)\n", Version, Commit, Date)
	case "doctor":
		err = c.runDoctor(ctx, args[1:])
	case "scenario":
		err = c.runScenario(args[1:])
	case "up":
		err = c.runUp(ctx, args[1:])
	case "status":
		err = c.runStatus(ctx, args[1:])
	case "mission":
		err = c.runMission(ctx, args[1:])
	case "agent":
		err = c.runAgent(ctx, args[1:])
	case "identity":
		err = c.runIdentity(ctx, args[1:])
	case "federation":
		err = c.runFederation(ctx, args[1:])
	case "graph":
		err = c.runGraph(ctx, args[1:])
	case "verify":
		code, err = c.runVerify(ctx, args[1:])
	case "reset":
		err = c.runReset(ctx, args[1:])
	case "down":
		err = c.runDown(ctx, args[1:])
	case "_runtime":
		err = c.runInternalRuntime(ctx, args[1:])
	case "_agent":
		err = c.runInternalAgent(ctx, args[1:])
	case "_tool":
		err = c.runInternalTool(ctx, args[1:])
	default:
		fmt.Fprintf(c.stderr, "unknown command %q\n\n", args[0])
		c.printUsage(c.stderr)
		return ExitError
	}

	if err != nil {
		fmt.Fprintf(c.stderr, "error: %v\n", err)
		if errors.Is(err, state.ErrNoActiveRun) {
			fmt.Fprintln(c.stderr, "hint: start a scenario with `cailab up <scenario>`")
		}
		return ExitError
	}
	return code
}

func (c *CLI) printUsage(w io.Writer) {
	fmt.Fprintln(w, `CloudAILab — local enterprise identity and AI-agent security range

Usage:
  cailab <command> [options]

Commands:
  doctor            Check local prerequisites
  scenario list     List available scenarios
  scenario show     Show a scenario briefing
  up                 Start and persist a scenario run
  status             Show the active run
  mission            Show the active mission
  agent              Validate registrations or run an agent trial
  identity           Validate tokens or rotate the active local issuer key
  federation         Exchange an authorized local token for temporary credentials
  graph path         Explain a directed trust path
  verify             Evaluate deterministic invariants
  reset              Restore the active scenario state
  down               Stop the active run
  version            Print build information

Run cailab <command> -h for command options.`)
}

func (c *CLI) runDoctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", c.stderr)
	jsonOutput := fs.Bool("json", false, "emit JSON")
	scenarioRoot := fs.String("scenario-root", "scenarios", "scenario catalog directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: cailab doctor [--scenario-root DIR] [scenario]")
	}
	requireDocker := false
	var scenarioName string
	if fs.NArg() == 1 {
		path, err := scenario.Resolve(*scenarioRoot, fs.Arg(0))
		if err != nil {
			return err
		}
		definition, err := scenario.Load(path)
		if err != nil {
			return err
		}
		scenarioName = definition.Metadata.Name
		requireDocker = definition.Spec.Runtimes.AWS != nil
	}

	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	checks := []check{{Name: "platform", Status: "pass", Detail: runtime.GOOS + "/" + runtime.GOARCH}}
	if scenarioName != "" {
		checks = append(checks, check{Name: "scenario", Status: "pass", Detail: scenarioName})
	}
	dockerFailureStatus := "warn"
	if requireDocker {
		dockerFailureStatus = "fail"
	}
	if scenarioName != "" && !requireDocker {
		checks = append(checks, check{Name: "docker", Status: "pass", Detail: "not required by this scenario"})
	} else {
		path, err := exec.LookPath("docker")
		if err != nil {
			checks = append(checks, check{Name: "docker-cli", Status: dockerFailureStatus, Detail: "Docker CLI not found; required only for AWS/Floci scenarios"})
		} else {
			checks = append(checks, check{Name: "docker-cli", Status: "pass", Detail: path})
			dockerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			output, dockerErr := exec.CommandContext(dockerCtx, path, "info", "--format", "{{.ServerVersion}}").CombinedOutput()
			if dockerErr != nil {
				detail := strings.TrimSpace(string(output))
				if detail == "" {
					detail = dockerErr.Error()
				}
				checks = append(checks, check{Name: "docker-engine", Status: dockerFailureStatus, Detail: detail})
			} else {
				version := strings.TrimSpace(string(output))
				if dockerVersionSupported(version) {
					checks = append(checks, check{Name: "docker-engine", Status: "pass", Detail: "server " + version})
				} else {
					checks = append(checks, check{Name: "docker-engine", Status: dockerFailureStatus, Detail: "server " + version + "; AWS/Floci scenarios require Docker 20.10+"})
				}
			}
		}
	}

	failed := false
	for _, item := range checks {
		if item.Status == "fail" {
			failed = true
		}
	}
	if *jsonOutput {
		encoder := json.NewEncoder(c.stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(map[string]any{"checks": checks}); err != nil {
			return err
		}
		if failed {
			return errors.New("one or more prerequisite checks failed")
		}
		return nil
	}
	for _, item := range checks {
		mark := "✓"
		if item.Status == "warn" {
			mark = "!"
		} else if item.Status != "pass" {
			mark = "✗"
		}
		fmt.Fprintf(c.stdout, "%s %-16s %s\n", mark, item.Name, item.Detail)
	}
	if failed {
		return errors.New("one or more prerequisite checks failed")
	}
	return nil
}

func dockerVersionSupported(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil {
		return false
	}
	return major > 20 || major == 20 && minor >= 10
}

func (c *CLI) runScenario(args []string) error {
	if len(args) == 0 {
		return errors.New("scenario requires `list` or `show`")
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("scenario list", c.stderr)
		root := fs.String("root", "scenarios", "scenario catalog directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("scenario list accepts no positional arguments")
		}
		summaries, err := scenario.List(*root)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(c.stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "SCENARIO\tVERSION\tOBJECTIVES\tTITLE")
		for _, summary := range summaries {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", summary.Name, summary.Version, summary.Objectives, summary.Title)
		}
		return w.Flush()
	case "show":
		fs := newFlagSet("scenario show", c.stderr)
		root := fs.String("root", "scenarios", "scenario catalog directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: cailab scenario show [--root DIR] <scenario>")
		}
		path, err := scenario.Resolve(*root, fs.Arg(0))
		if err != nil {
			return err
		}
		definition, err := scenario.Load(path)
		if err != nil {
			return err
		}
		printScenario(c.stdout, definition)
		return nil
	default:
		return fmt.Errorf("unknown scenario command %q", args[0])
	}
}

func (c *CLI) runUp(ctx context.Context, args []string) error {
	fs := newFlagSet("up", c.stderr)
	root := fs.String("scenario-root", "scenarios", "scenario catalog directory")
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	seed := fs.Int64("seed", 0, "override scenario seed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: cailab up [options] <scenario>")
	}
	path, err := scenario.Resolve(*root, fs.Arg(0))
	if err != nil {
		return err
	}
	var seedOverride *int64
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedOverride = seed
		}
	})
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	run, err := service.Up(ctx, app.UpOptions{ScenarioPath: path, Seed: seedOverride})
	if err != nil {
		if errors.Is(err, state.ErrActiveRun) {
			return errors.New("an active run already exists; use `cailab down` before starting another")
		}
		return err
	}
	fmt.Fprintf(c.stdout, "✓ scenario validated and compiled\n")
	fmt.Fprintf(c.stdout, "✓ run %s is active\n", run.ID)
	fmt.Fprintf(c.stdout, "  nodes: %d, relationships: %d, invariants: %d\n", len(run.Compiled.Nodes), len(run.Compiled.Edges), len(run.Compiled.Invariants))
	if len(run.Runtimes) == 0 {
		fmt.Fprintln(c.stdout, "  M0 mode: provider runtimes are not started")
	}
	for _, runtime := range run.Runtimes {
		fmt.Fprintf(c.stdout, "  %s endpoint: %s\n", strings.ToUpper(runtime.Provider), runtime.Endpoint)
		if runtime.Provider == "microsoft" {
			fmt.Fprintf(c.stdout, "  Microsoft authorization: Bearer %s\n", provider.LocalGraphToken)
		}
	}
	fmt.Fprintln(c.stdout, "\nRun `cailab mission` to inspect the lab.")
	return nil
}

func (c *CLI) runStatus(ctx context.Context, args []string) error {
	fs, stateDir, err := c.parseStateFlags("status", args)
	if err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("status accepts no positional arguments")
	}
	service, closeStore, err := c.openService(ctx, stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	run, err := service.Status(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "Run:       %s\n", run.ID)
	fmt.Fprintf(c.stdout, "Scenario:  %s@%s\n", run.ScenarioName, run.ScenarioVersion)
	fmt.Fprintf(c.stdout, "Status:    %s\n", run.Status)
	fmt.Fprintf(c.stdout, "Seed:      %d\n", run.Seed)
	fmt.Fprintf(c.stdout, "Digest:    %s\n", run.Compiled.Digest)
	for _, runtime := range run.Runtimes {
		fmt.Fprintf(c.stdout, "Runtime:   %s/%s %s (%s)\n", runtime.Provider, runtime.Engine, runtime.Endpoint, runtime.Status)
	}
	return nil
}

func (c *CLI) runMission(ctx context.Context, args []string) error {
	fs, stateDir, err := c.parseStateFlags("mission", args)
	if err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("mission accepts no positional arguments")
	}
	service, closeStore, err := c.openService(ctx, stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	compiled, err := service.Mission(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "%s\n\n%s\n\nObjectives:\n", compiled.Title, compiled.Briefing)
	for _, objective := range compiled.Objectives {
		fmt.Fprintf(c.stdout, "[ ] %s — %s\n", objective.ID, objective.Description)
	}
	return nil
}

func (c *CLI) runAgent(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cailab agent <validate|run> [options]")
	}
	switch args[0] {
	case "validate":
		return c.runAgentValidate(ctx, args[1:])
	case "run":
		if len(args) < 2 || (args[1] != "reference" && args[1] != "subprocess") {
			return errors.New("usage: cailab agent run <reference|subprocess> [options]")
		}
		if args[1] == "reference" {
			return c.runReferenceAgent(ctx, args[2:])
		}
		return c.runSubprocessAgent(ctx, args[2:])
	default:
		return errors.New("usage: cailab agent <validate|run> [options]")
	}
}

func (c *CLI) runAgentValidate(ctx context.Context, args []string) error {
	fs := newFlagSet("agent validate", c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	policyPath := fs.String("policy", "", "governance policy JSON file")
	agentID := fs.String("agent-id", "", "agent identifier used to validate policy bindings")
	actorTenant := fs.String("actor-tenant", "", "agent tenant in the active scenario")
	var toolPaths, toolEnvironmentNames stringListFlag
	fs.Var(&toolPaths, "tool", "tool manifest JSON file; repeat for multiple tools")
	fs.Var(&toolEnvironmentNames, "tool-env", "environment variable explicitly forwarded to every registered tool")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *policyPath == "" || *agentID == "" || *actorTenant == "" || len(toolPaths) == 0 {
		return errors.New("usage: cailab agent validate --policy FILE --tool FILE --agent-id ID --actor-tenant TENANT [options]")
	}
	policy, err := loadGovernancePolicy(*policyPath)
	if err != nil {
		return err
	}
	toolEnvironment, err := c.explicitEnvironment(toolEnvironmentNames)
	if err != nil {
		return err
	}
	registrations, err := loadRegisteredTools(toolPaths, toolEnvironment)
	if err != nil {
		return err
	}
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	rangeRun, err := service.Status(ctx)
	if err != nil {
		return err
	}
	options := app.AgentRunOptions{
		Agent: agent.AgentRef{ID: *agentID}, ActorTenant: *actorTenant,
		Policy: policy, Tools: registrations,
	}
	if err := app.ValidateAgentRunOptions(rangeRun.Compiled, options); err != nil {
		return err
	}
	policyDigest, err := agent.DigestGovernancePolicy(policy)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "✓ policy %s validated (%s)\n", policy.Version, policyDigest)
	for _, registration := range registrations {
		digest, err := agent.DigestToolManifest(registration.Manifest)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.stdout, "✓ tool %s@%s registered (%s)\n", registration.Manifest.Metadata.Name, registration.Manifest.Metadata.Version, digest)
	}
	fmt.Fprintln(c.stdout, "warning: validation does not isolate the declared subprocesses")
	return nil
}

func (c *CLI) runReferenceAgent(ctx context.Context, args []string) error {
	fs := newFlagSet("agent run reference", c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	trialID := fs.String("trial-id", "trial:1", "unique trial identifier within the active range run")
	jsonOutput := fs.Bool("json", false, "emit evidence-safe JSON summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: cailab agent run reference [--state-dir DIR] [--trial-id ID] [--json]")
	}
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	rangeRun, err := service.Status(ctx)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve cailab executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("resolve absolute cailab executable: %w", err)
	}
	directory, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolve reference working directory: %w", err)
	}
	options, err := app.ReferenceAgentRunOptions(rangeRun.Compiled, executable, directory, *trialID)
	if err != nil {
		return err
	}
	result, runErr := service.RunAgent(ctx, options)
	if result.Run.TrialID != "" {
		if err := c.renderAgentRunResult(result, *jsonOutput); err != nil {
			return err
		}
	}
	if runErr != nil {
		return runErr
	}
	if result.Run.Status != "completed" {
		return fmt.Errorf("agent trial %q ended with status %s", result.Run.TrialID, result.Run.Status)
	}
	return nil
}

func (c *CLI) runSubprocessAgent(ctx context.Context, args []string) error {
	fs := newFlagSet("agent run subprocess", c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	policyPath := fs.String("policy", "", "governance policy JSON file")
	promptPath := fs.String("prompt-file", "", "prompt file hashed into run metadata; content is not sent by CloudAILab")
	agentID := fs.String("agent-id", "", "agent identifier")
	agentVersion := fs.String("agent-version", "", "agent semantic version")
	providerName := fs.String("provider", "", "agent or model provider label")
	modelName := fs.String("model", "", "agent model or implementation label")
	actorTenant := fs.String("actor-tenant", "", "agent tenant in the active scenario")
	command := fs.String("command", "", "absolute agent executable path")
	directory := fs.String("directory", "", "absolute agent working directory; defaults to the current directory")
	trialID := fs.String("trial-id", "trial:1", "unique trial identifier within the active range run")
	trialIndex := fs.Int("trial-index", 1, "one-based trial index")
	trialCount := fs.Int("trial-count", 1, "declared trial count")
	sessionTimeout := fs.Duration("timeout", 60*time.Second, "whole agent session timeout")
	jsonOutput := fs.Bool("json", false, "emit evidence-safe JSON summary")
	var toolPaths, commandArguments, agentEnvironmentNames, toolEnvironmentNames stringListFlag
	fs.Var(&toolPaths, "tool", "tool manifest JSON file; repeat for multiple tools")
	fs.Var(&commandArguments, "arg", "agent argv value; repeat to preserve argument boundaries")
	fs.Var(&agentEnvironmentNames, "agent-env", "environment variable explicitly forwarded to the agent")
	fs.Var(&toolEnvironmentNames, "tool-env", "environment variable explicitly forwarded to every registered tool")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *policyPath == "" || *promptPath == "" || *agentID == "" || *agentVersion == "" ||
		*providerName == "" || *modelName == "" || *actorTenant == "" || *command == "" || len(toolPaths) == 0 {
		return errors.New("usage: cailab agent run subprocess --policy FILE --tool FILE --prompt-file FILE --agent-id ID --agent-version VERSION --provider NAME --model NAME --actor-tenant TENANT --command ABSOLUTE_PATH [options]")
	}
	policy, err := loadGovernancePolicy(*policyPath)
	if err != nil {
		return err
	}
	toolEnvironment, err := c.explicitEnvironment(toolEnvironmentNames)
	if err != nil {
		return err
	}
	registrations, err := loadRegisteredTools(toolPaths, toolEnvironment)
	if err != nil {
		return err
	}
	agentEnvironment, err := c.explicitEnvironment(agentEnvironmentNames)
	if err != nil {
		return err
	}
	prompt, err := readBoundedFile(*promptPath, agent.MaxFrameBytes)
	if err != nil {
		return fmt.Errorf("read prompt file: %w", err)
	}
	promptDigest := sha256.Sum256(prompt)
	workingDirectory := *directory
	if workingDirectory == "" {
		workingDirectory = "."
	}
	workingDirectory, err = filepath.Abs(workingDirectory)
	if err != nil {
		return fmt.Errorf("resolve agent working directory: %w", err)
	}
	executable, err := filepath.Abs(*command)
	if err != nil {
		return fmt.Errorf("resolve agent executable: %w", err)
	}
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	options := app.AgentRunOptions{
		Agent:       agent.AgentRef{ID: *agentID, Version: *agentVersion, Adapter: "subprocess", Provider: *providerName, Model: *modelName},
		ActorTenant: *actorTenant,
		Command:     append([]string{executable}, commandArguments...), Directory: workingDirectory, Environment: agentEnvironment,
		Policy: policy, Tools: registrations, PromptHash: hex.EncodeToString(promptDigest[:]),
		TrialID: *trialID, TrialIndex: *trialIndex, TrialCount: *trialCount, SessionTimeout: *sessionTimeout,
	}
	result, runErr := service.RunAgent(ctx, options)
	if result.Run.TrialID != "" {
		if err := c.renderAgentRunResult(result, *jsonOutput); err != nil {
			return err
		}
	}
	if runErr != nil {
		return runErr
	}
	if result.Run.Status != "completed" {
		return fmt.Errorf("agent trial %q ended with status %s", result.Run.TrialID, result.Run.Status)
	}
	return nil
}

func (c *CLI) explicitEnvironment(names []string) ([]string, error) {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		key := strings.ToUpper(name)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("environment variable %q was selected more than once", name)
		}
		value, exists := c.lookupEnv(name)
		if !exists {
			return nil, fmt.Errorf("selected environment variable %q is not set", name)
		}
		seen[key] = struct{}{}
		result = append(result, name+"="+value)
	}
	return result, nil
}

func loadGovernancePolicy(path string) (agent.GovernancePolicy, error) {
	data, err := readBoundedFile(path, agent.MaxFrameBytes)
	if err != nil {
		return agent.GovernancePolicy{}, fmt.Errorf("read governance policy %q: %w", path, err)
	}
	policy, err := agent.DecodeGovernancePolicy(data)
	if err != nil {
		return agent.GovernancePolicy{}, fmt.Errorf("load governance policy %q: %w", path, err)
	}
	return policy, nil
}

func loadRegisteredTools(paths []string, environment []string) ([]app.RegisteredTool, error) {
	registrations := make([]app.RegisteredTool, 0, len(paths))
	for _, path := range paths {
		data, err := readBoundedFile(path, agent.MaxFrameBytes)
		if err != nil {
			return nil, fmt.Errorf("read tool manifest %q: %w", path, err)
		}
		manifest, err := agent.DecodeToolManifest(data)
		if err != nil {
			return nil, fmt.Errorf("load tool manifest %q: %w", path, err)
		}
		absolutePath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve tool manifest %q: %w", path, err)
		}
		registration := app.RegisteredTool{
			Manifest: manifest, Directory: filepath.Dir(absolutePath), Environment: append([]string(nil), environment...),
		}
		if err := agent.ValidateToolRuntime(registration.Manifest, registration.Directory, registration.Environment); err != nil {
			return nil, fmt.Errorf("validate tool manifest %q runtime: %w", path, err)
		}
		registrations = append(registrations, registration)
	}
	return registrations, nil
}

func readBoundedFile(path string, limit int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, nil
}

type agentRunSummary struct {
	Run        agent.AgentRun                `json:"run"`
	Completion *agent.SessionCompletePayload `json:"completion,omitempty"`
	Decisions  []agent.DecisionEvent         `json:"decisions"`
	Outcomes   []agent.ToolOutcomeEvent      `json:"outcomes"`
}

func (c *CLI) renderAgentRunResult(result app.AgentRunResult, jsonOutput bool) error {
	if jsonOutput {
		var completion *agent.SessionCompletePayload
		if result.Session.Completion.Status != "" {
			value := result.Session.Completion
			completion = &value
		}
		encoded, err := json.MarshalIndent(agentRunSummary{
			Run: result.Run, Completion: completion,
			Decisions: result.Decisions, Outcomes: result.Outcomes,
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("encode agent run summary: %w", err)
		}
		fmt.Fprintln(c.stdout, string(encoded))
		return nil
	}
	fmt.Fprintf(c.stdout, "✓ agent trial %s ended with status %s\n", result.Run.TrialID, result.Run.Status)
	fmt.Fprintf(c.stdout, "Agent: %s@%s (%s / %s)\n", result.Run.Agent.ID, result.Run.Agent.Version, result.Run.Agent.Provider, result.Run.Agent.Model)
	fmt.Fprintf(c.stdout, "Evidence: %d decision(s), %d tool outcome(s)\n", len(result.Decisions), len(result.Outcomes))
	fmt.Fprintln(c.stdout, "Warning: subprocess ownership is not filesystem, network, syscall, or descendant isolation.")
	return nil
}

type stringListFlag []string

func (f *stringListFlag) String() string { return strings.Join(*f, ",") }

func (f *stringListFlag) Set(value string) error {
	if value == "" {
		return errors.New("value must not be empty")
	}
	*f = append(*f, value)
	return nil
}

func (c *CLI) runIdentity(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cailab identity <rotate|validate> [options]")
	}
	if args[0] == "validate" {
		return c.runIdentityValidate(ctx, args[1:])
	}
	if args[0] != "rotate" {
		return fmt.Errorf("unknown identity command %q", args[0])
	}
	fs, stateDir, err := c.parseStateFlags("identity rotate", args[1:])
	if err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("identity rotate accepts no positional arguments")
	}
	service, closeStore, err := c.openService(ctx, stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	keys, err := service.RotateIdentity(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "✓ local issuer signing key rotated; %d verification key(s) remain published\n", len(keys.Keys))
	return nil
}

func (c *CLI) runIdentityValidate(ctx context.Context, args []string) error {
	fs := newFlagSet("identity validate", c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	tokenFile := fs.String("token-file", "", "file containing one raw JWT")
	tokenType := fs.String("type", "", "token type: id or access")
	audience := fs.String("audience", "", "required token audience")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *tokenFile == "" || (*tokenType != "id" && *tokenType != "access") || *audience == "" {
		return errors.New("usage: cailab identity validate --token-file FILE --type <id|access> --audience VALUE [--state-dir DIR]")
	}
	info, err := os.Stat(*tokenFile)
	if err != nil {
		return fmt.Errorf("inspect token file: %w", err)
	}
	if info.Size() > 64<<10 {
		return errors.New("token file exceeds 64 KiB")
	}
	data, err := os.ReadFile(*tokenFile)
	if err != nil {
		return fmt.Errorf("read token file: %w", err)
	}
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	claims, err := service.ValidateIdentity(ctx, strings.TrimSpace(string(data)), *tokenType, *audience)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(claims, "", "  ")
	if err != nil {
		return fmt.Errorf("encode validated claims: %w", err)
	}
	fmt.Fprintln(c.stdout, string(encoded))
	return nil
}

func (c *CLI) runFederation(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "assume-aws" {
		return errors.New("usage: cailab federation assume-aws --token-file FILE --role-node ID --output FILE [--state-dir DIR]")
	}
	fs := newFlagSet("federation assume-aws", c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	tokenFile := fs.String("token-file", "", "file containing one raw JWT")
	roleNode := fs.String("role-node", "", "canonical AWS role node")
	output := fs.String("output", "", "owner-only JSON credential file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *tokenFile == "" || *roleNode == "" || *output == "" {
		return errors.New("usage: cailab federation assume-aws --token-file FILE --role-node ID --output FILE [--state-dir DIR]")
	}
	info, err := os.Stat(*tokenFile)
	if err != nil {
		return fmt.Errorf("inspect token file: %w", err)
	}
	if info.Size() > 64<<10 {
		return errors.New("token file exceeds 64 KiB")
	}
	data, err := os.ReadFile(*tokenFile)
	if err != nil {
		return fmt.Errorf("read token file: %w", err)
	}
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	credentials, err := service.AssumeAWSWebIdentity(ctx, strings.TrimSpace(string(data)), *roleNode)
	if err != nil {
		return err
	}
	if err := writeOwnerOnlyJSON(*output, credentials); err != nil {
		return err
	}
	absolute, err := filepath.Abs(*output)
	if err != nil {
		absolute = *output
	}
	fmt.Fprintf(c.stdout, "✓ temporary AWS credentials written to %s (expires %s)\n", absolute, credentials.Expiration.UTC().Format(time.RFC3339))
	return nil
}

func writeOwnerOnlyJSON(path string, value any) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".cailab-credentials-*")
	if err != nil {
		return fmt.Errorf("create credential file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure credential file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode credential file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync credential file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close credential file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish credential file: %w", err)
	}
	committed = true
	return nil
}

func (c *CLI) runGraph(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "path" {
		return errors.New("usage: cailab graph path [--state-dir DIR] <from> <to>")
	}
	fs, stateDir, err := c.parseStateFlags("graph path", args[1:])
	if err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("usage: cailab graph path [--state-dir DIR] <from> <to>")
	}
	service, closeStore, err := c.openService(ctx, stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	path, ok, err := service.Path(ctx, fs.Arg(0), fs.Arg(1))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(c.stdout, "No directed path found.")
		return nil
	}
	if len(path.Edges) == 0 {
		fmt.Fprintln(c.stdout, path.Nodes[0])
		return nil
	}
	fmt.Fprintln(c.stdout, path.Nodes[0])
	for i, edge := range path.Edges {
		actions := ""
		if len(edge.Actions) > 0 {
			actions = " [" + strings.Join(edge.Actions, ", ") + "]"
		}
		fmt.Fprintf(c.stdout, "  └─ %s%s → %s\n", edge.Type, actions, path.Nodes[i+1])
	}
	return nil
}

func (c *CLI) runVerify(ctx context.Context, args []string) (int, error) {
	fs := newFlagSet("verify", c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	format := fs.String("format", "text", "output format: text, json, or markdown")
	output := fs.String("output", "", "write report to a file")
	if err := fs.Parse(args); err != nil {
		return ExitError, err
	}
	if fs.NArg() != 0 {
		return ExitError, errors.New("verify accepts no positional arguments")
	}
	service, closeStore, err := c.openService(ctx, *stateDir)
	if err != nil {
		return ExitError, err
	}
	defer closeStore()
	report, err := service.Verify(ctx)
	if err != nil {
		return ExitError, err
	}
	data, err := renderReport(report, *format)
	if err != nil {
		return ExitError, err
	}
	if *output != "" {
		if err := os.WriteFile(*output, data, 0o600); err != nil {
			return ExitError, fmt.Errorf("write report %q: %w", *output, err)
		}
		fmt.Fprintf(c.stdout, "report written to %s\n", *output)
	} else {
		_, _ = c.stdout.Write(data)
	}
	if !report.Passed {
		return ExitVerificationFailed, nil
	}
	return ExitOK, nil
}

func (c *CLI) runReset(ctx context.Context, args []string) error {
	fs, stateDir, err := c.parseStateFlags("reset", args)
	if err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("reset accepts no positional arguments")
	}
	service, closeStore, err := c.openService(ctx, stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	run, err := service.Reset(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "✓ run %s restored to its compiled state\n", run.ID)
	return nil
}

func (c *CLI) runDown(ctx context.Context, args []string) error {
	fs, stateDir, err := c.parseStateFlags("down", args)
	if err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("down accepts no positional arguments")
	}
	service, closeStore, err := c.openService(ctx, stateDir)
	if err != nil {
		return err
	}
	defer closeStore()
	run, err := service.Down(ctx)
	if err != nil {
		if errors.Is(err, state.ErrNoActiveRun) {
			fmt.Fprintln(c.stdout, "✓ no active run")
			return nil
		}
		return err
	}
	fmt.Fprintf(c.stdout, "✓ run %s stopped\n", run.ID)
	return nil
}

func (c *CLI) parseStateFlags(name string, args []string) (*flag.FlagSet, string, error) {
	fs := newFlagSet(name, c.stderr)
	stateDir := fs.String("state-dir", c.defaultStateDir(), "state directory")
	if err := fs.Parse(args); err != nil {
		return nil, "", err
	}
	return fs, *stateDir, nil
}

func (c *CLI) defaultStateDir() string {
	if home := c.getenv("CAILAB_HOME"); home != "" {
		return home
	}
	return ".cloudailab"
}

func (c *CLI) openService(ctx context.Context, stateDir string) (*app.Service, func(), error) {
	store, err := state.Open(ctx, filepath.Join(stateDir, "cailab.db"))
	if err != nil {
		return nil, nil, err
	}
	return app.New(store, provider.NewManager(stateDir)), func() { _ = store.Close() }, nil
}

func (c *CLI) runInternalRuntime(ctx context.Context, args []string) error {
	if len(args) == 0 || (args[0] != "microsoft" && args[0] != "google" && args[0] != "oidc") {
		return errors.New("invalid private runtime command")
	}
	providerName := args[0]
	fs := newFlagSet("_runtime "+providerName, c.stderr)
	config := fs.String("config", "", "private runtime configuration")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *config == "" {
		return fmt.Errorf("invalid private %s runtime configuration", providerName)
	}
	if providerName == "microsoft" {
		return provider.ServeMicrosoftRuntime(ctx, *config)
	}
	if providerName == "google" {
		return provider.ServeGoogleRuntime(ctx, *config)
	}
	return provider.ServeOIDCRuntime(ctx, *config)
}

func (c *CLI) runInternalAgent(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "reference" {
		return errors.New("invalid private agent command")
	}
	fs := newFlagSet("_agent reference", c.stderr)
	id := fs.String("id", "agent:reference", "agent identifier")
	version := fs.String("version", "0.1.0", "agent version")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("invalid private reference-agent configuration")
	}
	return agent.ServeReferenceAgent(ctx, c.stdin, c.stdout, agent.ReferenceAgentConfig{ID: *id, Version: *version})
}

func (c *CLI) runInternalTool(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "reference" {
		return errors.New("invalid private tool command")
	}
	fs := newFlagSet("_tool reference", c.stderr)
	tool := fs.String("tool", "cloudailab.reference", "expected tool name")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("invalid private reference-tool configuration")
	}
	return agent.ServeReferenceTool(ctx, c.stdin, c.stdout, *tool)
}

func newFlagSet(name string, output io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	return fs
}

func printScenario(w io.Writer, definition scenario.Scenario) {
	fmt.Fprintf(w, "%s\n\n", definition.Metadata.Title)
	fmt.Fprintf(w, "Name: %s\nVersion: %s\nSeed: %d\n\n", definition.Metadata.Name, definition.Metadata.Version, definition.Spec.Seed)
	fmt.Fprintln(w, definition.Spec.Briefing)
	fmt.Fprintln(w, "\nObjectives:")
	for _, objective := range definition.Spec.Objectives {
		fmt.Fprintf(w, "- %s: %s\n", objective.ID, objective.Description)
	}
}

func renderReport(report verify.Report, format string) ([]byte, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("render JSON report: %w", err)
		}
		return append(data, '\n'), nil
	case "markdown":
		return []byte(verify.Markdown(report)), nil
	case "text":
		var b strings.Builder
		status := "PASS"
		if !report.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "CloudAILab verification: %s — %s\n", report.Scenario, status)
		for _, result := range report.Results {
			mark := "PASS"
			if !result.Passed {
				mark = "FAIL"
			}
			fmt.Fprintf(&b, "%s  %-20s %s\n", mark, result.InvariantID, result.Message)
			for _, evidence := range result.Evidence {
				fmt.Fprintf(&b, "      %s\n", evidence)
			}
		}
		fmt.Fprintf(&b, "Results: %d passed, %d failed\n", report.PassedCount, report.FailedCount)
		return []byte(b.String()), nil
	default:
		return nil, fmt.Errorf("unsupported report format %q", format)
	}
}
