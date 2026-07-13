package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	agentManagedLabel         = "dev.cloudailab.agent"
	agentRunLabel             = "dev.cloudailab.run"
	agentTrialLabel           = "dev.cloudailab.trial"
	containerCleanupTimeout   = 10 * time.Second
	containerMemoryLimit      = "512m"
	containerCPULimit         = "1"
	containerPIDLimit         = "128"
	containerTemporaryStorage = "64m"
	containerProfile          = "docker-strict-v1"
)

var containerImagePattern = regexp.MustCompile(`^(?:sha256:[a-f0-9]{64}|[^\s@]+@sha256:[a-f0-9]{64})$`)

// ContainerRuntime selects the enforced Docker boundary for an agent session.
// The command and directory in SessionConfig refer to paths inside the image.
type ContainerRuntime struct {
	enginePath string
	host       string
	image      string
	runner     containerCommandRunner
}

type containerCommandRunner interface {
	Run(context.Context, string, ...string) (string, error)
}

type execContainerCommandRunner struct{}

func (execContainerCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("run %s: %w", name, err)
	}
	return string(output), nil
}

func ValidateContainerImageReference(image string) error {
	if !containerImagePattern.MatchString(image) {
		return errors.New("container image must be sha256:<64 hex> or repository@sha256:<64 hex>")
	}
	return nil
}

func ValidateContainerHostEndpoint(host string) error {
	endpoint, err := url.Parse(host)
	if err != nil || endpoint.Scheme != "unix" || endpoint.Host != "" || !path.IsAbs(endpoint.Path) ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" || strings.ContainsAny(host, "\r\n\x00") {
		return errors.New("container engine endpoint must be an absolute local unix:// socket")
	}
	return nil
}

// NewContainerRuntime verifies a local Linux Docker engine and an already
// present volume-free image before returning an executable isolation contract.
func NewContainerRuntime(ctx context.Context, enginePath, image string) (*ContainerRuntime, error) {
	if !filepath.IsAbs(enginePath) {
		return nil, errors.New("container engine must be an absolute host path")
	}
	if err := ValidateContainerImageReference(image); err != nil {
		return nil, err
	}
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	endpointOutput, err := exec.CommandContext(inspectCtx, enginePath, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect active Docker context: %w: %s", err, strings.TrimSpace(string(endpointOutput)))
	}
	host := strings.TrimSpace(string(endpointOutput))
	if err := ValidateContainerHostEndpoint(host); err != nil {
		return nil, fmt.Errorf("validate active Docker context: %w", err)
	}
	serverOutput, err := exec.CommandContext(inspectCtx, enginePath, "--host", host, "version", "--format", "{{.Server.Os}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect local Docker engine: %w: %s", err, strings.TrimSpace(string(serverOutput)))
	}
	if strings.TrimSpace(string(serverOutput)) != "linux" {
		return nil, fmt.Errorf("Docker isolation requires a local Linux container engine, got %q", strings.TrimSpace(string(serverOutput)))
	}
	securityOutput, err := exec.CommandContext(inspectCtx, enginePath, "--host", host, "info", "--format", "{{json .SecurityOptions}}\n{{.CgroupDriver}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect Docker security configuration: %w: %s", err, strings.TrimSpace(string(securityOutput)))
	}
	if err := validateDockerSecurityInfo(securityOutput); err != nil {
		return nil, err
	}
	volumeOutput, err := exec.CommandContext(inspectCtx, enginePath, "--host", host, "image", "inspect", "--format", "{{json .Config.Volumes}}", image).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect Docker agent image; pull or build it first: %w: %s", err, strings.TrimSpace(string(volumeOutput)))
	}
	if err := validateImageVolumes(volumeOutput); err != nil {
		return nil, err
	}
	return &ContainerRuntime{enginePath: enginePath, host: host, image: image}, nil
}

func validateDockerSecurityInfo(encoded []byte) error {
	lines := strings.Split(strings.TrimSpace(string(encoded)), "\n")
	if len(lines) != 2 {
		return errors.New("Docker security inspection returned an unexpected response")
	}
	var options []string
	if err := json.Unmarshal([]byte(lines[0]), &options); err != nil {
		return fmt.Errorf("decode Docker security options: %w", err)
	}
	for _, option := range options {
		if option == "name=rootless" || strings.HasPrefix(option, "name=rootless,") {
			return errors.New("Docker rootless mode is not supported for isolated agent runs because cgroup limits may be ignored")
		}
	}
	if lines[1] == "" || lines[1] == "none" {
		return errors.New("Docker isolated agent runs require an active cgroup driver")
	}
	return nil
}

func validateImageVolumes(encoded []byte) error {
	if strings.TrimSpace(string(encoded)) == "null" {
		return nil
	}
	var volumes map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &volumes); err != nil {
		return fmt.Errorf("decode Docker image volumes: %w", err)
	}
	if len(volumes) != 0 {
		return errors.New("Docker agent image must not declare VOLUME paths")
	}
	return nil
}

// EnginePath returns the verified absolute Docker CLI path.
func (r *ContainerRuntime) EnginePath() string { return r.enginePath }

// HostEndpoint returns the verified local Unix socket endpoint.
func (r *ContainerRuntime) HostEndpoint() string { return r.host }

// ImageReference returns the verified content-addressed image reference.
func (r *ContainerRuntime) ImageReference() string { return r.image }

// ContainerExecutionRef returns the immutable run metadata for a runtime.
func ContainerExecutionRef(runtime *ContainerRuntime) *AgentExecutionRef {
	if runtime == nil {
		return nil
	}
	return &AgentExecutionRef{
		Mode: "container", Engine: "docker", Profile: containerProfile, Image: runtime.image,
		Network: "none", Filesystem: "read_only",
	}
}

func sameExecutionRef(left, right *AgentExecutionRef) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func prepareSessionCommand(ctx context.Context, config normalizedSessionConfig) (*exec.Cmd, func() error) {
	if config.Container == nil {
		command := exec.CommandContext(ctx, config.Command[0], config.Command[1:]...)
		command.Dir = config.Directory
		command.Env = append([]string{}, config.Environment...)
		return command, func() error { return nil }
	}

	container := config.Container
	name := agentContainerName(config.Run.RunID, config.Run.TrialID)
	command := exec.CommandContext(ctx, container.enginePath, containerCommandArgs(config, name)...)
	runner := container.runner
	if runner == nil {
		runner = execContainerCommandRunner{}
	}
	cleanup := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), containerCleanupTimeout)
		defer cancel()
		format := fmt.Sprintf(`{{index .Config.Labels %q}}|{{index .Config.Labels %q}}`, agentRunLabel, agentTrialLabel)
		output, err := runner.Run(cleanupCtx, container.enginePath, "--host", container.host, "inspect", "--format", format, name)
		if err != nil {
			if dockerObjectMissing(output, err) {
				return nil
			}
			return fmt.Errorf("inspect agent container %q: %w: %s", name, err, strings.TrimSpace(output))
		}
		want := config.Run.RunID + "|" + config.Run.TrialID
		if strings.TrimSpace(output) != want {
			return fmt.Errorf("refuse to remove agent container %q: run or trial label does not match", name)
		}
		output, err = runner.Run(cleanupCtx, container.enginePath, "--host", container.host, "rm", "--force", name)
		if err != nil && !dockerObjectMissing(output, err) {
			return fmt.Errorf("remove agent container %q: %w: %s", name, err, strings.TrimSpace(output))
		}
		return nil
	}
	return command, cleanup
}

func containerCommandArgs(config normalizedSessionConfig, name string) []string {
	return append([]string{"--host", config.Container.host}, containerSessionArgs(config, name)...)
}

func containerSessionArgs(config normalizedSessionConfig, name string) []string {
	args := []string{
		"run", "--rm", "--interactive", "--pull=never", "--init",
		"--log-driver", "none",
		"--name", name,
		"--label", agentManagedLabel + "=true",
		"--label", agentRunLabel + "=" + config.Run.RunID,
		"--label", agentTrialLabel + "=" + config.Run.TrialID,
		"--network", "none",
		"--ipc", "none",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=" + containerTemporaryStorage + ",mode=1777,uid=65532,gid=65532",
		"--user", "65532:65532",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges=true",
		"--security-opt", "seccomp=builtin",
		"--memory", containerMemoryLimit,
		"--memory-swap", containerMemoryLimit,
		"--memory-swappiness", "0",
		"--cpus", containerCPULimit,
		"--pids-limit", containerPIDLimit,
		"--ulimit", "nofile=1024:1024",
		"--workdir", config.Directory,
		"--entrypoint", config.Command[0],
		config.Container.image,
	}
	return append(args, config.Command[1:]...)
}

func agentContainerName(runID, trialID string) string {
	sum := sha256.Sum256([]byte(runID + "\x00" + trialID))
	return "cailab-agent-" + hex.EncodeToString(sum[:8])
}

func dockerObjectMissing(output string, err error) bool {
	text := strings.ToLower(output + " " + err.Error())
	return strings.Contains(text, "no such container") || strings.Contains(text, "no such object")
}
