package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDockerAgentIsolationIntegration(t *testing.T) {
	if os.Getenv("CAILAB_AGENT_ISOLATION_INTEGRATION") != "1" {
		t.Skip("set CAILAB_AGENT_ISOLATION_INTEGRATION=1 to run Docker agent isolation integration")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal("Docker CLI is required for the isolation integration")
	}
	docker, err = filepath.Abs(docker)
	if err != nil {
		t.Fatal(err)
	}
	host := localDockerHost(t, docker)
	image := buildIsolationProbeImage(t, docker, host)
	containerRuntime, err := NewContainerRuntime(context.Background(), docker, image)
	if err != nil {
		t.Fatal(err)
	}

	run := func(trialID string, command []string, ctx context.Context) (SessionResult, error) {
		t.Helper()
		config := testSessionConfig(t, "reference")
		config.Command = command
		config.Directory = "/workspace"
		config.Environment = nil
		config.Container = containerRuntime
		config.Run.TrialID = trialID
		config.Run.Execution = &AgentExecutionRef{
			Mode: "container", Engine: "docker", Profile: containerProfile, Image: image, Network: "none", Filesystem: "read_only",
		}
		config.HandshakeTimeout = 2 * time.Second
		config.SessionTimeout = 10 * time.Second
		return RunSession(ctx, config, nil)
	}

	result, err := run("trial:isolation", []string{"/probe"}, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Completion.Status != "completed" || result.Completion.Summary != "host_absent=true network_blocked=true root_read_only=true device_blocked=true tmp_writable=true non_root=true" {
		t.Fatalf("isolation probe completion = %+v", result.Completion)
	}
	assertNoAgentContainer(t, docker, host, "run:reference", "trial:isolation")

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)
	_, err = run("trial:canceled", []string{"/probe", "hang"}, ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled isolation session error = %v", err)
	}
	assertNoAgentContainer(t, docker, host, "run:reference", "trial:canceled")
}

func localDockerHost(t *testing.T, docker string) string {
	t.Helper()
	output, err := exec.Command(docker, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect Docker context: %v: %s", err, output)
	}
	host := strings.TrimSpace(string(output))
	if err := ValidateContainerHostEndpoint(host); err != nil {
		t.Fatalf("Docker context endpoint %q: %v", host, err)
	}
	output, err = exec.Command(docker, "--host", host, "version", "--format", "{{.Server.Os}}").CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "linux" {
		t.Fatalf("Docker server OS: %v: %s", err, output)
	}
	return host
}

func buildIsolationProbeImage(t *testing.T, docker, host string) string {
	t.Helper()
	directory := t.TempDir()
	source := `package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "time"
)

type message struct {
    ProtocolVersion string ` + "`json:\"protocolVersion\"`" + `
    ID string ` + "`json:\"id\"`" + `
    Type string ` + "`json:\"type\"`" + `
    Payload any ` + "`json:\"payload\"`" + `
}

func main() {
    scanner := bufio.NewScanner(os.Stdin)
    if !scanner.Scan() { os.Exit(2) }
    encoder := json.NewEncoder(os.Stdout)
    _ = encoder.Encode(message{ProtocolVersion:"1.1", ID:"message:ready", Type:"agent.ready", Payload:map[string]any{"agentId":"agent:reference", "agentVersion":"0.1.0"}})
    if len(os.Args) > 1 && os.Args[1] == "hang" { time.Sleep(time.Hour) }
    _, hostErr := os.ReadFile("/workspace/host-sentinel")
    connection, networkErr := net.DialTimeout("tcp", "1.1.1.1:80", 250*time.Millisecond)
    if connection != nil { _ = connection.Close() }
    rootErr := os.WriteFile("/root-write", []byte("x"), 0600)
    deviceErr := os.WriteFile("/dev/probe", []byte("x"), 0600)
    tmpErr := os.WriteFile("/tmp/probe", []byte("x"), 0600)
    summary := fmt.Sprintf("host_absent=%t network_blocked=%t root_read_only=%t device_blocked=%t tmp_writable=%t non_root=%t", hostErr != nil, networkErr != nil, rootErr != nil, deviceErr != nil, tmpErr == nil, os.Geteuid() != 0)
    _ = encoder.Encode(message{ProtocolVersion:"1.1", ID:"message:complete", Type:"session.complete", Payload:map[string]any{"status":"completed", "summary":summary}})
}
`
	if err := os.WriteFile(filepath.Join(directory, "probe.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "host-sentinel"), []byte("must not reach the agent"), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(directory, "probe"), filepath.Join(directory, "probe.go"))
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build isolation probe: %v: %s", err, output)
	}
	dockerfile := "FROM scratch\nCOPY --chown=65532:65532 probe /probe\nWORKDIR /workspace\nUSER 65532:65532\n"
	if err := os.WriteFile(filepath.Join(directory, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	tag := "cailab-agent-isolation-test:" + strings.TrimPrefix(agentContainerName(directory, t.Name()), "cailab-agent-")
	command := exec.Command(docker, "--host", host, "build", "--quiet", "--tag", tag, directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build isolation image: %v: %s", err, output)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = exec.CommandContext(ctx, docker, "--host", host, "image", "rm", "--force", tag).CombinedOutput()
	})
	output, err := exec.Command(docker, "--host", host, "image", "inspect", "--format", "{{.Id}}", tag).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect isolation image: %v: %s", err, output)
	}
	image := strings.TrimSpace(string(output))
	if err := ValidateContainerImageReference(image); err != nil {
		t.Fatalf("built image ID %q: %v", image, err)
	}
	return image
}

func assertNoAgentContainer(t *testing.T, docker, host, runID, trialID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, docker, "--host", host, "ps", "-a",
		"--filter", "label="+agentManagedLabel+"=true",
		"--filter", "label="+agentRunLabel+"="+runID,
		"--filter", "label="+agentTrialLabel+"="+trialID,
		"--format", "{{.Names}}").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect agent container cleanup: %v: %s", err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf("agent container leaked after cleanup: %s", output)
	}
}
