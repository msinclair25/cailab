package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/agent"
	"github.com/msinclair25/cailab/internal/app"
	"github.com/msinclair25/cailab/internal/provider"
	"github.com/msinclair25/cailab/internal/state"
)

const (
	cliAgentHelperEnvironment = "CAILAB_CLI_AGENT_HELPER"
	cliToolHelperEnvironment  = "CAILAB_CLI_TOOL_HELPER"
)

func TestWriteOwnerOnlyJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	value := map[string]string{"accessKeyId": "synthetic-access", "secretAccessKey": "synthetic-secret"}
	if err := writeOwnerOnlyJSON(path, value); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["secretAccessKey"] != "synthetic-secret" {
		t.Fatalf("credential JSON = %+v", decoded)
	}
}

func TestWalkingSkeletonLifecycle(t *testing.T) {
	ctx := context.Background()
	stateDir := t.TempDir()
	scenarioPath := writeScenario(t)
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)

	assertRun := func(wantCode int, args ...string) string {
		t.Helper()
		stdout.Reset()
		stderr.Reset()
		if code := c.Run(ctx, args); code != wantCode {
			t.Fatalf("Run(%v) code = %d, want %d; stderr=%s", args, code, wantCode, stderr.String())
		}
		return stdout.String()
	}

	output := assertRun(ExitOK, "up", "--state-dir", stateDir, scenarioPath)
	if !strings.Contains(output, "is active") {
		t.Fatalf("up output = %q", output)
	}
	output = assertRun(ExitOK, "status", "--state-dir", stateDir)
	if !strings.Contains(output, "walking-skeleton@0.1.0") {
		t.Fatalf("status output = %q", output)
	}
	output = assertRun(ExitOK, "status", "--state-dir", stateDir, "--format", "json")
	var status statusDocument
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatalf("decode status JSON: %v; output=%s", err, output)
	}
	if status.APIVersion != statusAPIVersion || status.Kind != "RangeStatus" || status.Run.Scenario != "walking-skeleton" {
		t.Fatalf("status JSON = %+v", status)
	}
	output = assertRun(ExitOK, "mission", "--state-dir", stateDir)
	if !strings.Contains(output, "Inspect the path") {
		t.Fatalf("mission output = %q", output)
	}
	output = assertRun(ExitOK, "graph", "path", "--state-dir", stateDir, "principal:a", "resource:a")
	if !strings.Contains(output, "can_access") {
		t.Fatalf("graph output = %q", output)
	}
	output = assertRun(ExitOK, "verify", "--state-dir", stateDir)
	if !strings.Contains(output, "1 passed, 0 failed") {
		t.Fatalf("verify output = %q", output)
	}
	assertRun(ExitOK, "reset", "--state-dir", stateDir)
	assertRun(ExitOK, "down", "--state-dir", stateDir)
	output = assertRun(ExitOK, "down", "--state-dir", stateDir)
	if !strings.Contains(output, "no active run") {
		t.Fatalf("second down output = %q", output)
	}
	if code := c.Run(ctx, []string{"status", "--state-dir", stateDir}); code != ExitError {
		t.Fatalf("status after down code = %d, want %d", code, ExitError)
	}
}

func TestDataOnlyScenarioStarterPublicLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	stateDir := t.TempDir()
	scenarioPath := filepath.Join("..", "..", "examples", "scenario-starter", "scenario.yaml")
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)

	assertRun := func(wantCode int, args ...string) string {
		t.Helper()
		stdout.Reset()
		stderr.Reset()
		if code := c.Run(ctx, args); code != wantCode {
			t.Fatalf("Run(%v) code = %d, want %d; stderr=%s", args, code, wantCode, stderr.String())
		}
		return stdout.String()
	}

	validated := assertRun(ExitOK, "scenario", "validate", scenarioPath)
	if !strings.Contains(validated, "community-trust-path@0.1.0 is valid") || !strings.Contains(validated, "invariants: 2") {
		t.Fatalf("validate output = %q", validated)
	}
	assertRun(ExitOK, "up", "--state-dir", stateDir, scenarioPath)
	junitPath := filepath.Join(t.TempDir(), "verification.xml")
	assertRun(ExitOK, "verify", "--state-dir", stateDir, "--format", "junit", "--output", junitPath)
	junitData, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(junitData), `<testsuite name="CloudAILab verification: community-trust-path" tests="2" failures="0">`) {
		t.Fatalf("JUnit report = %s", junitData)
	}
	verified := assertRun(ExitOK, "verify", "--state-dir", stateDir)
	if !strings.Contains(verified, "2 passed, 0 failed") {
		t.Fatalf("verify output = %q", verified)
	}
	assertRun(ExitOK, "down", "--state-dir", stateDir)
	if code := c.Run(ctx, []string{"status", "--state-dir", stateDir}); code != ExitError {
		t.Fatalf("status after cleanup code = %d, want %d", code, ExitError)
	}
}

func TestPublicHelpIsSuccessfulAndUsesStandardOutput(t *testing.T) {
	t.Parallel()
	tests := [][]string{
		{"--help"},
		{"version", "--help"},
		{"doctor", "--help"},
		{"scenario", "--help"},
		{"scenario", "list", "--help"},
		{"scenario", "show", "--help"},
		{"scenario", "validate", "--help"},
		{"quickstart", "--help"},
		{"up", "--help"},
		{"status", "--help"},
		{"mission", "--help"},
		{"agent", "--help"},
		{"agent", "validate", "--help"},
		{"agent", "run", "--help"},
		{"agent", "run", "reference", "--help"},
		{"agent", "run", "subprocess", "--help"},
		{"agent", "run", "safe", "--help"},
		{"agent", "run", "unsafe", "--help"},
		{"agent", "campaign", "--help"},
		{"agent", "campaign", "reference", "--help"},
		{"agent", "campaign", "safe", "--help"},
		{"agent", "campaign", "unsafe", "--help"},
		{"agent", "replay", "--help"},
		{"identity", "--help"},
		{"identity", "rotate", "--help"},
		{"identity", "validate", "--help"},
		{"federation", "--help"},
		{"federation", "assume-aws", "--help"},
		{"graph", "--help"},
		{"graph", "path", "--help"},
		{"verify", "--help"},
		{"reset", "--help"},
		{"down", "--help"},
	}

	for _, args := range tests {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := New(&stdout, &stderr).Run(context.Background(), args)
			if code != ExitOK {
				t.Fatalf("Run(%v) code = %d, want %d; stderr=%q", args, code, ExitOK, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%v) stderr = %q, want empty", args, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("Run(%v) stdout = %q, want usage", args, stdout.String())
			}
		})
	}
}

func TestQuickstartCompletesVerificationAndCleanup(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)
	if code := c.Run(context.Background(), []string{"quickstart", "--state-dir", stateDir}); code != ExitOK {
		t.Fatalf("quickstart code = %d; stderr=%s; stdout=%s", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	for _, want := range []string{"guided no-Docker quick start", "no provider runtimes", "CloudAILab verification: walking-skeleton — PASS", "Quick start complete"} {
		if !strings.Contains(output, want) {
			t.Fatalf("quickstart output missing %q: %s", want, output)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := c.Run(context.Background(), []string{"status", "--state-dir", stateDir}); code != ExitError {
		t.Fatalf("status after quickstart code = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), "no active run") {
		t.Fatalf("status stderr = %q, want no active run", stderr.String())
	}
}

func TestHelpTokenRemainsAFlagValueWhenDeclared(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)
	fs := newFlagSet("test", &stderr)
	var values stringListFlag
	fs.Var(&values, "arg", "argument")
	if err := c.parseFlags(fs, []string{"--arg", "--help"}); err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if len(values) != 1 || values[0] != "--help" {
		t.Fatalf("values = %v, want [--help]", values)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no help", stdout.String())
	}
}

func TestStatusProjectionIsStableAndEvidenceSafe(t *testing.T) {
	t.Parallel()
	run := state.Run{
		ID: "run:1", ScenarioName: "scenario", ScenarioVersion: "0.1.0", Status: "active", Seed: 42,
		BaselineDigest: "baseline",
		Runtimes: []provider.Instance{
			{Provider: "google", Engine: "native", Endpoint: "http://127.0.0.1:2", Status: "ready", Name: "private-name", ProcessID: 99, ControlPath: "control-secret"},
			{Provider: "aws", Engine: "floci", Endpoint: "http://127.0.0.1:1", Status: "ready", ContainerID: "container-secret", Image: "image-secret"},
		},
	}
	run.Compiled.Digest = "plan"

	projected := projectStatus(run)
	if projected.APIVersion != statusAPIVersion || projected.Run.PlanDigest != "plan" || projected.Run.BaselineDigest != "baseline" {
		t.Fatalf("projection = %+v", projected)
	}
	if len(projected.Runtimes) != 2 || projected.Runtimes[0].Provider != "aws" || projected.Runtimes[1].Provider != "google" {
		t.Fatalf("runtime order = %+v", projected.Runtimes)
	}
	data, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	for _, prohibited := range []string{"control-secret", "container-secret", "image-secret", "private-name", "processId", "controlPath"} {
		if bytes.Contains(data, []byte(prohibited)) {
			t.Fatalf("status JSON exposes %q: %s", prohibited, data)
		}
	}
}

func TestBuiltInScenarioCommandsNeedNoRepositoryCatalog(t *testing.T) {
	ctx := context.Background()
	stateDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)

	if code := c.Run(ctx, []string{"scenario", "list"}); code != ExitOK {
		t.Fatalf("scenario list code = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "walking-skeleton") {
		t.Fatalf("scenario list output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{"scenario", "show", "walking-skeleton"}); code != ExitOK {
		t.Fatalf("scenario show code = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cross-Provider Path") {
		t.Fatalf("scenario show output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{"up", "--state-dir", stateDir, "walking-skeleton"}); code != ExitOK {
		t.Fatalf("up built-in scenario code = %d; stderr=%s", code, stderr.String())
	}
	if code := c.Run(ctx, []string{"down", "--state-dir", stateDir}); code != ExitOK {
		t.Fatalf("down built-in scenario code = %d; stderr=%s", code, stderr.String())
	}
}

func TestVerificationFailureUsesDedicatedExitCode(t *testing.T) {
	ctx := context.Background()
	stateDir := t.TempDir()
	scenarioPath := writeScenario(t)
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "type: path_exists", "type: path_absent", 1))
	if err := os.WriteFile(scenarioPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)
	if code := c.Run(ctx, []string{"up", "--state-dir", stateDir, scenarioPath}); code != ExitOK {
		t.Fatalf("up code = %d; stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	reportPath := filepath.Join(t.TempDir(), "failed-verification.xml")
	if code := c.Run(ctx, []string{"verify", "--state-dir", stateDir, "--format", "junit", "--output", reportPath}); code != ExitVerificationFailed {
		t.Fatalf("verify code = %d, want %d; stderr=%s", code, ExitVerificationFailed, stderr.String())
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), `failures="1"`) || !strings.Contains(string(report), `<failure`) {
		t.Fatalf("failed JUnit report = %s", report)
	}
}

func TestDockerVersionSupported(t *testing.T) {
	t.Parallel()
	tests := map[string]bool{
		"20.9.9":  false,
		"20.10.0": true,
		"29.5.3":  true,
		"invalid": false,
	}
	for version, want := range tests {
		if got := dockerVersionSupported(version); got != want {
			t.Errorf("dockerVersionSupported(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestInternalReferenceAgentUsesProtocolStreams(t *testing.T) {
	startPayload, err := json.Marshal(agent.SessionStartPayload{
		RunID: "run:reference", TrialID: "trial:1",
		ScenarioDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PolicyVersion:  "0.1.0",
		Tools:          []agent.ToolRef{{Name: "google.drive.read", Version: "0.1.0", Digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	if err := agent.NewEncoder(&input).Write(agent.Message{
		ProtocolVersion: agent.ProtocolVersion, ID: "message:start", Type: agent.MessageSessionStart, Payload: startPayload,
	}); err != nil {
		t.Fatal(err)
	}
	var output, diagnostics bytes.Buffer
	c := New(&output, &diagnostics)
	c.stdin = &input
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if code := c.Run(ctx, []string{"_agent", "reference"}); code != ExitOK {
		t.Fatalf("code = %d; stderr=%s", code, diagnostics.String())
	}
	decoder := agent.NewDecoder(&output)
	for _, expected := range []string{agent.MessageAgentReady, agent.MessageSessionComplete} {
		message, err := decoder.Next()
		if err != nil {
			t.Fatal(err)
		}
		if message.Type != expected {
			t.Fatalf("message type = %q, want %q", message.Type, expected)
		}
	}
}

func TestInternalReferenceToolUsesProtocolStreams(t *testing.T) {
	request, err := json.Marshal(agent.ToolExecutionRequest{
		ProtocolVersion: agent.ProtocolVersion, CallID: "call:1", Tool: "google.drive.read",
		Action: "drive.files.get", Resource: agent.ResourceRef{ID: "google:file", Tenant: "tenant:northstar", Classification: "restricted"},
		Arguments: json.RawMessage(`{"fileId":"google:file"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.NewBuffer(append(request, '\n'))
	var output, diagnostics bytes.Buffer
	c := New(&output, &diagnostics)
	c.stdin = input
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if code := c.Run(ctx, []string{"_tool", "reference", "--tool", "google.drive.read"}); code != ExitOK {
		t.Fatalf("code = %d; stderr=%s", code, diagnostics.String())
	}
	var response agent.ToolExecutionResponse
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "succeeded" || response.CallID != "call:1" {
		t.Fatalf("response = %+v", response)
	}
}

func TestPublicAgentValidateAndSubprocessRun(t *testing.T) {
	ctx := context.Background()
	stateDir := t.TempDir()
	scenarioPath := writeScenario(t)
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)
	if code := c.Run(ctx, []string{"up", "--state-dir", stateDir, scenarioPath}); code != ExitOK {
		t.Fatalf("up code = %d; stderr=%s", code, stderr.String())
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	manifest := agent.ToolManifest{
		APIVersion: agent.APIVersion, Kind: agent.ToolManifestKind,
		Metadata: agent.Metadata{Name: "test.read", Version: "0.1.0", Description: "Read one synthetic test resource."},
		Spec: agent.ToolManifestSpec{
			Transport:   agent.ToolTransport{Type: "subprocess", Command: []string{executable, "-test.run=^TestCLIAgentSubprocessHelper$"}},
			InputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"resource":{"type":"string"},"token":{"type":"string"}},"required":["resource","token"],"additionalProperties":false}`),
			Permissions: []agent.Permission{{Tenant: "tenant-a", Actions: []string{"test.read"}, Resources: []string{"resource:a"}}},
			Risk:        "low", TimeoutMillis: 2_000, Isolation: agent.Isolation{Network: "host", Filesystem: "host"},
		},
	}
	policy := agent.GovernancePolicy{
		APIVersion: agent.APIVersion, Kind: agent.GovernancePolicyKind, Version: "0.1.0", DefaultEffect: "deny",
		Rules: []agent.PolicyRule{{
			ID: "rule:allow", Effect: "allow", AgentID: "agent:cli-test", Tool: "test.read", Action: "test.read",
			Resource: "resource:a", ResourceTenant: "tenant-a", ResourceClassification: "internal",
		}},
	}
	manifestPath := writeAgentJSON(t, "tool.json", manifest)
	policyPath := writeAgentJSON(t, "policy.json", policy)
	promptPath := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("inspect the synthetic resource"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cliAgentHelperEnvironment, "tool")
	t.Setenv(cliToolHelperEnvironment, "reference")
	t.Setenv("CAILAB_CLI_PARENT_SECRET", "must-not-reach-child")
	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{
		"agent", "validate", "--state-dir", stateDir, "--policy", policyPath, "--tool", manifestPath,
		"--agent-id", "agent:cli-test", "--actor-tenant", "tenant-a", "--tool-env", cliToolHelperEnvironment,
	}); code != ExitOK {
		t.Fatalf("validate code = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "tool test.read@0.1.0 registered") {
		t.Fatalf("validate output = %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{
		"agent", "run", "subprocess", "--state-dir", stateDir, "--policy", policyPath, "--tool", manifestPath,
		"--prompt-file", promptPath, "--agent-id", "agent:cli-test", "--agent-version", "0.1.0",
		"--provider", "test", "--model", "deterministic", "--actor-tenant", "tenant-a",
		"--command", executable, "--arg", "-test.run=^TestCLIAgentSubprocessHelper$", "--directory", stateDir,
		"--agent-env", cliAgentHelperEnvironment, "--tool-env", cliToolHelperEnvironment, "--restore-fixture", "--json",
	}); code != ExitOK {
		t.Fatalf("run code = %d; stderr=%s; stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"status": "completed"`) || !strings.Contains(stdout.String(), `"effect": "allow"`) {
		t.Fatalf("run output = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"outcome": {`) || !strings.Contains(stdout.String(), `"status": "succeeded"`) {
		t.Fatalf("run output did not contain a successful tool outcome: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "raw-cli-secret") {
		t.Fatalf("evidence-safe output leaked raw arguments: %s", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{
		"agent", "replay", "--state-dir", stateDir, "--trial-id", "trial:1", "--format", "json",
	}); code != ExitOK {
		t.Fatalf("replay code = %d; stderr=%s; stdout=%s", code, stderr.String(), stdout.String())
	}
	var evaluation agent.AgentEvaluationReport
	if err := json.Unmarshal(stdout.Bytes(), &evaluation); err != nil {
		t.Fatal(err)
	}
	if evaluation.Profile != agent.ScenarioOutcomeProfile || evaluation.Aggregate.CompletedTrials.Numerator != 1 ||
		evaluation.Aggregate.ExecutionSuccessRate.Numerator != 1 || evaluation.Aggregate.TaskSuccessRate == nil ||
		evaluation.Aggregate.TaskSuccessRate.Numerator != 1 || len(evaluation.NotMeasured) == 0 {
		t.Fatalf("evaluation = %+v", evaluation)
	}

	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{
		"agent", "campaign", "reference", "--state-dir", stateDir, "--trials", "2",
		"--trial-prefix", "campaign:cli-reference", "--format", "json",
	}); code != ExitOK {
		t.Fatalf("campaign code = %d; stderr=%s; stdout=%s", code, stderr.String(), stdout.String())
	}
	var campaign agent.AgentEvaluationReport
	if err := json.Unmarshal(stdout.Bytes(), &campaign); err != nil {
		t.Fatal(err)
	}
	if campaign.Profile != agent.ScenarioOutcomeProfile || campaign.Aggregate.Trials != 2 ||
		campaign.Aggregate.CompletedTrials.Denominator != 2 || campaign.Trials[0].TrialID != "campaign:cli-reference:1" {
		t.Fatalf("campaign = %+v", campaign)
	}

	policy.Rules[0].Effect = "require_approval"
	approvalPolicyPath := writeAgentJSON(t, "approval-policy.json", policy)
	t.Setenv(cliAgentHelperEnvironment, "approval")
	stdout.Reset()
	stderr.Reset()
	if code := c.Run(ctx, []string{
		"agent", "run", "subprocess", "--state-dir", stateDir, "--policy", approvalPolicyPath, "--tool", manifestPath,
		"--prompt-file", promptPath, "--agent-id", "agent:cli-test", "--agent-version", "0.1.0",
		"--provider", "test", "--model", "deterministic", "--actor-tenant", "tenant-a",
		"--command", executable, "--arg", "-test.run=^TestCLIAgentSubprocessHelper$", "--directory", stateDir,
		"--agent-env", cliAgentHelperEnvironment, "--tool-env", cliToolHelperEnvironment,
		"--trial-id", "trial:approval-rejected", "--json",
	}); code != ExitOK {
		t.Fatalf("default rejection code = %d; stderr=%s; stdout=%s", code, stderr.String(), stdout.String())
	}
	var rejected agentRunSummary
	if err := json.Unmarshal(stdout.Bytes(), &rejected); err != nil {
		t.Fatal(err)
	}
	if len(rejected.Approvals) != 1 || rejected.Approvals[0].Approved || rejected.Approvals[0].Decision.ReasonCode != "approval:rejected" || len(rejected.Outcomes) != 0 {
		t.Fatalf("default rejection summary = %+v", rejected)
	}
}

func TestPromptApproverRequiresExactCorrelatedConfirmationWithoutRawArguments(t *testing.T) {
	request := agent.ApprovalRequest{
		ApprovalID: "approval:1", DecisionEventID: "event:1", RunID: "run:1", TrialID: "trial:1", CorrelationID: "call:1",
		Actor:  agent.ActorRef{ID: "agent:test", Tenant: "tenant:a", Type: "agent"},
		Tool:   agent.ToolRef{Name: "test.read", Version: "0.1.0", Digest: strings.Repeat("a", 64)},
		Action: "test.read", Resource: agent.ResourceRef{ID: "resource:a", Tenant: "tenant:a", Classification: "restricted"},
		ReasonCode: "rule:approval", InputHash: strings.Repeat("b", 64),
	}
	for _, test := range []struct {
		name        string
		input       string
		wantApprove bool
	}{
		{name: "exact", input: "approve approval:1\n", wantApprove: true},
		{name: "mismatch", input: "approve approval:other\n", wantApprove: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			approver := promptApprover{input: bufio.NewReader(strings.NewReader(test.input)), output: &output, resolvedBy: "user:reviewer"}
			resolution, err := approver.ResolveApproval(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.Approved != test.wantApprove || resolution.ResolvedBy != "user:reviewer" {
				t.Fatalf("resolution = %+v", resolution)
			}
			if strings.Contains(output.String(), "raw-cli-secret") || strings.Contains(output.String(), request.InputHash) {
				t.Fatalf("approval prompt leaked protected input data: %q", output.String())
			}
		})
	}
}

func TestAgentContainerRuntimeRejectsUnsafePublicConfiguration(t *testing.T) {
	c := New(io.Discard, io.Discard)
	digest := "sha256:" + strings.Repeat("a", 64)
	if _, err := c.agentContainerRuntime(context.Background(), "host", digest, nil); err == nil || !strings.Contains(err.Error(), "--image") {
		t.Fatalf("host image error = %v", err)
	}
	if _, err := c.agentContainerRuntime(context.Background(), "docker", "agent:latest", nil); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("mutable image error = %v", err)
	}
	if _, err := c.agentContainerRuntime(context.Background(), "docker", digest, []string{"TOKEN"}); err == nil || !strings.Contains(err.Error(), "--agent-env") {
		t.Fatalf("Docker environment error = %v", err)
	}
	if _, err := c.agentContainerRuntime(context.Background(), "unknown", "", nil); err == nil || !strings.Contains(err.Error(), "host or docker") {
		t.Fatalf("unknown isolation error = %v", err)
	}
}

func TestDockerAgentIsolationCLIConfigurationIntegration(t *testing.T) {
	if os.Getenv("CAILAB_AGENT_ISOLATION_INTEGRATION") != "1" {
		t.Skip("set CAILAB_AGENT_ISOLATION_INTEGRATION=1 to inspect the local Docker isolation context")
	}
	c := New(io.Discard, io.Discard)
	runtime, err := c.agentContainerRuntime(context.Background(), "docker", "sha256:"+strings.Repeat("a", 64), nil)
	if runtime != nil || err == nil || !strings.Contains(err.Error(), "pull or build it first") {
		t.Fatalf("container runtime = %+v, error = %v", runtime, err)
	}
}

func TestAgentRunSummaryDescribesOnlyEnforcedIsolation(t *testing.T) {
	result := app.AgentRunResult{Run: agent.AgentRun{
		TrialID: "trial:isolated", Status: "completed",
		Agent: agent.AgentRef{ID: "agent:test", Version: "0.1.0", Provider: "local", Model: "fixture"},
		Execution: &agent.AgentExecutionRef{
			Mode: "container", Engine: "docker", Profile: "docker-strict-v1", Image: "sha256:" + strings.Repeat("a", 64),
			Network: "none", Filesystem: "read_only",
		},
	}}
	var output, diagnostics bytes.Buffer
	c := New(&output, &diagnostics)
	if err := c.renderAgentRunResult(result, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Docker network none") || !strings.Contains(output.String(), result.Run.Execution.Image) || !strings.Contains(output.String(), "tool subprocesses remain trusted and unisolated") {
		t.Fatalf("isolated summary = %q", output.String())
	}
	if strings.Contains(output.String(), "subprocess ownership is not filesystem") {
		t.Fatalf("isolated summary used host warning: %q", output.String())
	}
}

func TestAgentEvaluationRenderingIsDeterministicAndLabelsUnmeasuredMetrics(t *testing.T) {
	half := 0.5
	report := agent.AgentEvaluationReport{
		APIVersion: agent.APIVersion, Kind: agent.AgentEvaluationReportKind, Profile: agent.EvaluationProfile,
		RunID: "run:evaluation", ConfigDigest: strings.Repeat("a", 64),
		Trials: []agent.TrialEvaluation{{
			TrialID: "trial:1", TrialIndex: 1, Status: "completed", TraceDigest: strings.Repeat("b", 64),
		}},
		Aggregate: agent.AggregateMetrics{
			Trials: 1, CompletedTrials: agent.MetricRate{Numerator: 1, Denominator: 1, Rate: floatPointer(1)},
			AuthorizationRate:      agent.MetricRate{Numerator: 1, Denominator: 2, Rate: &half},
			ApprovalResolutionRate: agent.MetricRate{}, ExecutionSuccessRate: agent.MetricRate{},
		},
		NotMeasured: []agent.MeasurementLimitation{{Metric: "task_success", Reason: "terminal completion is not scenario verification"}},
	}
	markdown, err := renderAgentEvaluationReport(report, "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "1/2 (50.0%)") || !strings.Contains(string(markdown), "`task_success`") {
		t.Fatalf("markdown = %s", markdown)
	}
	jsonReport, err := renderAgentEvaluationReport(report, "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(jsonReport), "generatedAt") || !strings.Contains(string(jsonReport), `"rate": 0.5`) {
		t.Fatalf("JSON = %s", jsonReport)
	}
	if _, err := renderAgentEvaluationReport(report, "xml"); err == nil {
		t.Fatal("unsupported evaluation format was accepted")
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestCLIAgentSubprocessHelper(t *testing.T) {
	if os.Getenv("CAILAB_CLI_PARENT_SECRET") != "" {
		os.Exit(19)
	}
	agentMode := os.Getenv(cliAgentHelperEnvironment)
	toolMode := os.Getenv(cliToolHelperEnvironment)
	if agentMode == "" && toolMode == "" {
		return
	}
	if toolMode != "" {
		if err := agent.ServeReferenceTool(context.Background(), os.Stdin, os.Stdout, "test.read"); err != nil {
			os.Exit(20)
		}
		os.Exit(0)
	}
	decoder := agent.NewDecoder(os.Stdin)
	if _, err := decoder.Next(); err != nil {
		os.Exit(21)
	}
	encoder := agent.NewEncoder(os.Stdout)
	if err := encoder.Write(cliAgentMessage(agent.MessageAgentReady, "message:ready", "", agent.AgentReadyPayload{AgentID: "agent:cli-test", AgentVersion: "0.1.0"})); err != nil {
		os.Exit(22)
	}
	call := cliAgentMessage(agent.MessageToolCall, "call:1", "", agent.ToolCallPayload{
		Tool: "test.read", Action: "test.read", Resource: "resource:a",
		Arguments: json.RawMessage(`{"resource":"resource:a","token":"raw-cli-secret"}`),
	})
	if err := encoder.Write(call); err != nil {
		os.Exit(23)
	}
	response, err := decoder.Next()
	if agentMode == "approval" {
		if err != nil || response.Type != agent.MessageApprovalRequired || response.CorrelationID != call.ID {
			os.Exit(24)
		}
		resolved, err := decoder.Next()
		if err != nil || resolved.Type != agent.MessageApprovalResolved || resolved.CorrelationID != call.ID {
			os.Exit(26)
		}
		response, err = decoder.Next()
	}
	if err != nil || response.Type != agent.MessageToolResult || response.CorrelationID != call.ID {
		os.Exit(27)
	}
	if err := encoder.Write(cliAgentMessage(agent.MessageSessionComplete, "message:complete", "", agent.SessionCompletePayload{Status: "completed"})); err != nil {
		os.Exit(25)
	}
	os.Exit(0)
}

func cliAgentMessage(messageType, id, correlation string, payload any) agent.Message {
	data, _ := json.Marshal(payload)
	return agent.Message{ProtocolVersion: agent.ProtocolVersion, ID: id, Type: messageType, CorrelationID: correlation, Payload: data}
}

func writeAgentJSON(t *testing.T, name string, value any) string {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDoctorUsesScenarioSpecificPrerequisites(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	c := New(&stdout, &stderr)
	code := c.Run(context.Background(), []string{
		"doctor", "--scenario-root", filepath.Join("..", "..", "scenarios"), "microsoft-consent",
	})
	if code != ExitOK {
		t.Fatalf("doctor code = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "docker") || !strings.Contains(stdout.String(), "not required by this scenario") {
		t.Fatalf("doctor output = %q", stdout.String())
	}
}

func writeScenario(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	data := `apiVersion: cloudailab.dev/v1alpha1
kind: Scenario
metadata:
  name: walking-skeleton
  version: 0.1.0
  title: Walking Skeleton
spec:
  seed: 42
  briefing: Test the public CLI lifecycle.
  objectives:
    - id: inspect
      description: Inspect the path.
  tenants:
    - id: tenant-a
      name: Tenant A
      providers: [local]
  principals:
    - id: principal:a
      tenant: tenant-a
      type: human
      displayName: Principal A
  resources:
    - id: resource:a
      tenant: tenant-a
      type: test_resource
      displayName: Resource A
      classification: internal
  relationships:
    - id: edge:a
      from: principal:a
      to: resource:a
      type: can_access
  verification:
    invariants:
      - id: path-visible
        type: path_exists
        from: principal:a
        to: resource:a
        severity: low
        description: The path must exist.
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
