package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/msinclair25/cailab/internal/scenario"
)

const (
	managedLabel = "dev.cloudailab.managed"
	runLabel     = "dev.cloudailab.run"
)

type commandRunner interface {
	Run(context.Context, string, ...string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("run %s: %w", name, err)
	}
	return string(output), nil
}

// DockerManager starts pinned provider images through the local Docker CLI.
type DockerManager struct {
	runner     commandRunner
	httpClient *http.Client
	now        func() time.Time
}

func NewDockerManager() *DockerManager {
	return &DockerManager{
		runner: execRunner{},
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		now: time.Now,
	}
}

func (m *DockerManager) Start(ctx context.Context, runID string, compiled scenario.Compiled) ([]Instance, error) {
	if compiled.Runtimes.AWS == nil {
		return nil, nil
	}
	config := compiled.Runtimes.AWS
	if config.Engine != "floci" || config.Image != scenario.FlociImage {
		return nil, errors.New("AWS runtime is not allowlisted by this CloudAILab build")
	}
	name := containerName(runID, "floci")
	args := []string{
		"run", "--detach",
		"--name", name,
		"--pull=missing",
		"--publish", "127.0.0.1::4566",
		"--label", managedLabel + "=true",
		"--label", runLabel + "=" + runID,
		"--env", fmt.Sprintf("FLOCI_SERVICES_IAM_ENFORCEMENT_ENABLED=%t", config.IAMEnforcement),
		"--user", "1001:0",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--memory", "512m",
		"--cpus", "1",
		config.Image,
	}
	output, err := m.runner.Run(ctx, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("start AWS runtime: %w: %s", err, strings.TrimSpace(output))
	}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = m.runner.Run(cleanupCtx, "docker", "rm", "--force", name)
	}
	containerID := lastNonemptyLine(output)
	if containerID == "" {
		cleanup()
		return nil, errors.New("start AWS runtime: Docker returned no container ID")
	}

	portOutput, err := m.runner.Run(ctx, "docker", "port", name, "4566/tcp")
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("inspect AWS runtime port: %w: %s", err, strings.TrimSpace(portOutput))
	}
	endpoint, err := loopbackEndpoint(portOutput)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := m.waitReady(ctx, endpoint); err != nil {
		logs, _ := m.runner.Run(context.Background(), "docker", "logs", name)
		cleanup()
		return nil, fmt.Errorf("AWS runtime readiness: %w; logs: %s", err, strings.TrimSpace(logs))
	}
	if err := hydrateAWS(ctx, endpoint, compiled.Providers.AWS); err != nil {
		logs, _ := m.runner.Run(context.Background(), "docker", "logs", name)
		cleanup()
		return nil, fmt.Errorf("hydrate AWS runtime: %w; logs: %s", err, strings.TrimSpace(logs))
	}

	return []Instance{{
		Provider: "aws", Engine: config.Engine, ContainerID: containerID,
		Name: name, Endpoint: endpoint, Image: config.Image, Status: "ready",
	}}, nil
}

func (m *DockerManager) Snapshot(ctx context.Context, instances []Instance, compiled scenario.Compiled) (scenario.Compiled, error) {
	if compiled.Providers.AWS == nil {
		return compiled, nil
	}
	for _, instance := range instances {
		if instance.Provider == "aws" && instance.Engine == "floci" {
			return snapshotAWS(ctx, instance.Endpoint, compiled)
		}
	}
	return scenario.Compiled{}, errors.New("active scenario requires an AWS runtime, but none is recorded")
}

func (m *DockerManager) Stop(ctx context.Context, runID string, instances []Instance) error {
	if len(instances) == 0 {
		output, err := m.runner.Run(ctx, "docker", "ps", "-a",
			"--filter", "label="+managedLabel+"=true",
			"--filter", "label="+runLabel+"="+runID,
			"--format", "{{.Names}}")
		if err != nil {
			return fmt.Errorf("discover runtimes for run %q: %w: %s", runID, err, strings.TrimSpace(output))
		}
		for _, name := range strings.Fields(output) {
			instances = append(instances, Instance{Engine: "floci", Name: name})
		}
	}
	var stopErrors []error
	for _, instance := range instances {
		if instance.Engine != "floci" || instance.Name == "" {
			continue
		}
		label, err := m.runner.Run(ctx, "docker", "inspect", "--format", "{{index .Config.Labels \""+runLabel+"\"}}", instance.Name)
		if err != nil {
			if isMissingContainer(label) {
				continue
			}
			stopErrors = append(stopErrors, fmt.Errorf("inspect runtime %q: %w", instance.Name, err))
			continue
		}
		if strings.TrimSpace(label) != runID {
			stopErrors = append(stopErrors, fmt.Errorf("refuse to remove runtime %q: run label does not match", instance.Name))
			continue
		}
		output, err := m.runner.Run(ctx, "docker", "rm", "--force", instance.Name)
		if err != nil && !isMissingContainer(output) {
			stopErrors = append(stopErrors, fmt.Errorf("remove runtime %q: %w: %s", instance.Name, err, strings.TrimSpace(output)))
		}
	}
	return errors.Join(stopErrors...)
}

func (m *DockerManager) waitReady(ctx context.Context, endpoint string) error {
	deadline := m.now().Add(15 * time.Second)
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/_floci/init", nil)
		if err != nil {
			return err
		}
		response, err := m.httpClient.Do(request)
		if err == nil {
			var status struct {
				Completed struct {
					Ready bool `json:"ready"`
				} `json:"completed"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status)
			response.Body.Close()
			if response.StatusCode == http.StatusOK && decodeErr == nil && status.Completed.Ready {
				return nil
			}
		}
		if !m.now().Before(deadline) {
			return errors.New("timed out waiting for /_floci/init")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func lastNonemptyLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

func loopbackEndpoint(portOutput string) (string, error) {
	for _, line := range strings.Split(strings.TrimSpace(portOutput), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "127.0.0.1:") {
			return "http://" + line, nil
		}
	}
	return "", fmt.Errorf("Docker did not publish Floci on IPv4 loopback: %q", strings.TrimSpace(portOutput))
}

func containerName(runID, suffix string) string {
	sum := sha256.Sum256([]byte(runID))
	return "cailab-" + hex.EncodeToString(sum[:8]) + "-" + suffix
}

func isMissingContainer(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "no such container") || strings.Contains(lower, "no such object")
}
