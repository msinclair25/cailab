package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestContainerImageReferencesRequireContentAddressing(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, image := range []string{"sha256:" + digest, "registry.example/team/agent@sha256:" + digest} {
		if err := ValidateContainerImageReference(image); err != nil {
			t.Fatalf("valid image %q: %v", image, err)
		}
	}
	for _, image := range []string{"", "agent:latest", "agent@sha256:short", "agent @sha256:" + digest, "sha256:" + strings.Repeat("A", 64)} {
		if err := ValidateContainerImageReference(image); err == nil {
			t.Fatalf("mutable or malformed image %q was accepted", image)
		}
	}
}

func TestContainerHostEndpointRequiresLocalUnixSocket(t *testing.T) {
	for _, endpoint := range []string{"unix:///var/run/docker.sock", "unix:///Users/test/.docker/run/docker.sock"} {
		if err := ValidateContainerHostEndpoint(endpoint); err != nil {
			t.Fatalf("valid endpoint %q: %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{"", "tcp://127.0.0.1:2375", "ssh://host", "npipe:////./pipe/docker_engine", "unix://relative", "unix:///var/run/docker.sock\n--host=tcp://remote"} {
		if err := ValidateContainerHostEndpoint(endpoint); err == nil {
			t.Fatalf("remote or malformed endpoint %q was accepted", endpoint)
		}
	}
}

func TestContainerImageVolumesAreRejected(t *testing.T) {
	for _, encoded := range []string{"null", "{}"} {
		if err := validateImageVolumes([]byte(encoded)); err != nil {
			t.Fatalf("empty volumes %q: %v", encoded, err)
		}
	}
	if err := validateImageVolumes([]byte(`{"/data":{}}`)); err == nil || !strings.Contains(err.Error(), "must not declare VOLUME") {
		t.Fatalf("declared volume error = %v", err)
	}
	if err := validateImageVolumes([]byte(`not-json`)); err == nil {
		t.Fatal("malformed image volume metadata was accepted")
	}
}

func TestDockerSecurityInfoRequiresNonRootlessCgroups(t *testing.T) {
	if err := validateDockerSecurityInfo([]byte(`["name=seccomp,profile=builtin","name=cgroupns"]` + "\nsystemd\n")); err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{
		`["name=seccomp,profile=builtin","name=rootless"]` + "\nsystemd\n",
		`["name=seccomp,profile=builtin"]` + "\nnone\n",
		"malformed\nsystemd\n",
	} {
		if err := validateDockerSecurityInfo([]byte(encoded)); err == nil {
			t.Fatalf("unsafe Docker security info %q was accepted", encoded)
		}
	}
}

func TestContainerSessionArgumentsEnforceNarrowBoundary(t *testing.T) {
	config := testContainerSessionConfig(t)
	normalized, err := normalizeSessionConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	args := containerCommandArgs(normalized, "cailab-agent-test")
	joined := strings.Join(args, " ")
	for _, required := range []string{
		"--host unix:///var/run/docker.sock run --rm --interactive --pull=never --init",
		"--log-driver none",
		"--network none", "--read-only",
		"--ipc none",
		"--tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m,mode=1777,uid=65532,gid=65532",
		"--user 65532:65532", "--cap-drop ALL",
		"--security-opt no-new-privileges=true", "--security-opt seccomp=builtin",
		"--memory 512m", "--memory-swap 512m", "--memory-swappiness 0",
		"--cpus 1", "--pids-limit 128", "--ulimit nofile=1024:1024",
		"--workdir /workspace", "--entrypoint /agent",
		config.Container.image + " --mode test",
	} {
		if !strings.Contains(joined, required) {
			t.Errorf("container arguments missing %q: %s", required, joined)
		}
	}
	for _, forbidden := range []string{"--env", "--volume", "--mount", "--publish", "--device", "--privileged", "--network host", "--pid host"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("container arguments contain forbidden %q: %s", forbidden, joined)
		}
	}
}

func TestContainerSessionValidationFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SessionConfig)
	}{
		{name: "mutable image", mutate: func(config *SessionConfig) { config.Container.image = "agent:latest" }},
		{name: "relative engine", mutate: func(config *SessionConfig) { config.Container.enginePath = "docker" }},
		{name: "remote engine", mutate: func(config *SessionConfig) { config.Container.host = "ssh://remote" }},
		{name: "host environment", mutate: func(config *SessionConfig) { config.Environment = []string{"TOKEN=secret"} }},
		{name: "host execution path", mutate: func(config *SessionConfig) { config.Command[0] = `C:\\agent.exe` }},
		{name: "missing metadata", mutate: func(config *SessionConfig) { config.Run.Execution = nil }},
		{name: "mismatched metadata", mutate: func(config *SessionConfig) { config.Run.Execution.Image = "sha256:" + strings.Repeat("b", 64) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testContainerSessionConfig(t)
			test.mutate(&config)
			if err := ValidateSessionConfig(config); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("error = %v, want invalid session", err)
			}
		})
	}
}

func TestContainerCleanupRequiresMatchingOwnershipLabels(t *testing.T) {
	t.Run("matching", func(t *testing.T) {
		runner := &fakeContainerCommandRunner{inspect: "run:reference|trial:1\n"}
		config := testContainerSessionConfig(t)
		config.Container.runner = runner
		normalized, err := normalizeSessionConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		_, cleanup := prepareSessionCommand(context.Background(), normalized)
		if err := cleanup(); err != nil {
			t.Fatal(err)
		}
		if len(runner.calls) != 2 || runner.calls[0] != "inspect" || runner.calls[1] != "rm" {
			t.Fatalf("cleanup calls = %v", runner.calls)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		runner := &fakeContainerCommandRunner{inspect: "run:other|trial:1\n"}
		config := testContainerSessionConfig(t)
		config.Container.runner = runner
		normalized, err := normalizeSessionConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		_, cleanup := prepareSessionCommand(context.Background(), normalized)
		if err := cleanup(); err == nil || !strings.Contains(err.Error(), "label does not match") {
			t.Fatalf("cleanup error = %v", err)
		}
		if len(runner.calls) != 1 || runner.calls[0] != "inspect" {
			t.Fatalf("cleanup calls = %v", runner.calls)
		}
	})

	t.Run("already removed", func(t *testing.T) {
		runner := &fakeContainerCommandRunner{err: errors.New("No such container")}
		config := testContainerSessionConfig(t)
		config.Container.runner = runner
		normalized, err := normalizeSessionConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		_, cleanup := prepareSessionCommand(context.Background(), normalized)
		if err := cleanup(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAgentRunValidatesExecutionMetadata(t *testing.T) {
	config := testContainerSessionConfig(t)
	if err := ValidateAgentRun(config.Run); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(config.Run)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeAgentRun(data)
	if err != nil || !sameExecutionRef(decoded.Execution, config.Run.Execution) {
		t.Fatalf("decoded run = %+v, error = %v", decoded, err)
	}
	config.Run.Execution.Network = "host"
	if err := ValidateAgentRun(config.Run); err == nil {
		t.Fatal("agent run accepted a host network isolation claim")
	}
}

func testContainerSessionConfig(t *testing.T) SessionConfig {
	t.Helper()
	config := testSessionConfig(t, "reference")
	image := "sha256:" + strings.Repeat("a", 64)
	config.Command = []string{"/agent", "--mode", "test"}
	config.Directory = "/workspace"
	config.Environment = nil
	config.Container = &ContainerRuntime{enginePath: "/usr/bin/docker", host: "unix:///var/run/docker.sock", image: image}
	config.Run.Execution = &AgentExecutionRef{
		Mode: "container", Engine: "docker", Profile: containerProfile, Image: image, Network: "none", Filesystem: "read_only",
	}
	return config
}

type fakeContainerCommandRunner struct {
	inspect string
	err     error
	calls   []string
}

func (f *fakeContainerCommandRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	if len(args) < 3 || args[0] != "--host" {
		return "", errors.New("missing Docker command")
	}
	command := args[2]
	f.calls = append(f.calls, command)
	if command == "inspect" {
		return f.inspect, f.err
	}
	return "", f.err
}
