package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if code := c.Run(ctx, []string{"verify", "--state-dir", stateDir}); code != ExitVerificationFailed {
		t.Fatalf("verify code = %d, want %d; stderr=%s", code, ExitVerificationFailed, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 failed") {
		t.Fatalf("verify output = %q", stdout.String())
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
