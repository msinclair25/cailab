package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/scenario"
)

type recordedCall struct {
	name string
	args []string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type fakeRunner struct {
	calls      []recordedCall
	port       string
	runID      string
	inspect    string
	discovered string
	managed    string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, recordedCall{name: name, args: append([]string(nil), args...)})
	if len(args) == 0 {
		return "", fmt.Errorf("missing command")
	}
	switch args[0] {
	case "run":
		return "container-id\n", nil
	case "port":
		return f.port + "\n", nil
	case "inspect":
		if f.inspect != "" {
			return f.inspect, nil
		}
		if strings.Contains(strings.Join(args, " "), managedLabel) {
			if f.managed == "" {
				return "true\n", nil
			}
			return f.managed + "\n", nil
		}
		return f.runID + "\n", nil
	case "ps":
		return f.discovered, nil
	case "rm", "logs":
		return "", nil
	default:
		return "", fmt.Errorf("unexpected docker command %q", args[0])
	}
}

func TestDockerManagerDiscoversUnrecordedRuntimeForCleanup(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{runID: "interrupted-run", discovered: "cailab-interrupted-floci\n"}
	manager := &DockerManager{runner: runner, httpClient: http.DefaultClient, now: time.Now}
	if err := manager.Stop(context.Background(), "interrupted-run", nil); err != nil {
		t.Fatal(err)
	}
	last := runner.calls[len(runner.calls)-1]
	if got := strings.Join(last.args, " "); got != "rm --force cailab-interrupted-floci" {
		t.Fatalf("last call = %q", got)
	}
}

func TestDockerManagerLifecycle(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_floci/init" {
			t.Fatalf("health path = %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"completed":{"ready":true}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	runID := "aws-slice-1234"
	runner := &fakeRunner{port: "127.0.0.1:4566", runID: runID}
	manager := &DockerManager{runner: runner, httpClient: client, now: time.Now}
	compiled := scenario.Compiled{Runtimes: scenario.Runtimes{AWS: &scenario.AWSRuntime{
		Engine: "floci", Image: scenario.FlociImage, IAMEnforcement: true,
	}}}

	instances, err := manager.Start(context.Background(), runID, compiled)
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 || instances[0].Endpoint != "http://127.0.0.1:4566" || instances[0].Status != "ready" {
		t.Fatalf("instances = %+v", instances)
	}
	runArgs := strings.Join(runner.calls[0].args, " ")
	for _, required := range []string{
		"--publish 127.0.0.1::4566",
		"--user 1001:0",
		"--cap-drop ALL",
		"--security-opt no-new-privileges",
		"FLOCI_SERVICES_IAM_ENFORCEMENT_ENABLED=true",
		"FLOCI_STORAGE_MODE=memory",
	} {
		if !strings.Contains(runArgs, required) {
			t.Errorf("docker run args %q do not contain %q", runArgs, required)
		}
	}
	if err := manager.Stop(context.Background(), runID, instances); err != nil {
		t.Fatal(err)
	}
	last := runner.calls[len(runner.calls)-1]
	if got := strings.Join(last.args, " "); got != "rm --force "+instances[0].Name {
		t.Fatalf("last call = %q", got)
	}
}

func TestDockerManagerReplacesOwnedFlociAtSamePort(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"completed":{"ready":true}}`)),
			Header:     make(http.Header),
		}, nil
	})}
	runID := "aws-restore"
	runner := &fakeRunner{runID: runID, port: "127.0.0.1:4566"}
	manager := &DockerManager{runner: runner, httpClient: client, now: time.Now}
	compiled := scenario.Compiled{
		Runtimes:  scenario.Runtimes{AWS: &scenario.AWSRuntime{Engine: "floci", Image: scenario.FlociImage, IAMEnforcement: true}},
		Providers: scenario.Providers{AWS: &scenario.AWSProvider{Region: "us-east-1"}},
	}
	instance := Instance{
		Provider: "aws", Engine: "floci", Name: containerName(runID, "floci"),
		Endpoint: "http://127.0.0.1:4566", Status: "ready",
	}
	restored, err := manager.Restore(context.Background(), runID, []Instance{instance}, compiled)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0].Endpoint != instance.Endpoint || restored[0].ContainerID != "container-id" {
		t.Fatalf("restored = %+v", restored)
	}
	var removed, replaced bool
	for _, call := range runner.calls {
		args := strings.Join(call.args, " ")
		removed = removed || args == "rm --force "+instance.Name
		replaced = replaced || strings.Contains(args, "run --detach") && strings.Contains(args, "--publish 127.0.0.1:4566:4566")
	}
	if !removed || !replaced {
		t.Fatalf("Docker calls = %+v", runner.calls)
	}
}

func TestDockerManagerRestoreRejectsOwnershipMismatch(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{runID: "another-run"}
	manager := &DockerManager{runner: runner, httpClient: http.DefaultClient, now: time.Now}
	runID := "expected-run"
	compiled := scenario.Compiled{
		Runtimes:  scenario.Runtimes{AWS: &scenario.AWSRuntime{Engine: "floci", Image: scenario.FlociImage}},
		Providers: scenario.Providers{AWS: &scenario.AWSProvider{Region: "us-east-1"}},
	}
	_, err := manager.Restore(context.Background(), runID, []Instance{{
		Provider: "aws", Engine: "floci", Name: containerName(runID, "floci"), Endpoint: "http://127.0.0.1:4566",
	}}, compiled)
	if err == nil || !strings.Contains(err.Error(), "run label does not match") {
		t.Fatalf("Restore() error = %v", err)
	}
}

func TestDockerManagerRefusesMismatchedRunLabel(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{inspect: "another-run\n"}
	manager := &DockerManager{runner: runner, httpClient: http.DefaultClient, now: time.Now}
	err := manager.Stop(context.Background(), "expected-run", []Instance{{Engine: "floci", Name: "cailab-owned"}})
	if err == nil || !strings.Contains(err.Error(), "run label does not match") {
		t.Fatalf("Stop() error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %v, want inspect only", runner.calls)
	}
}

func TestDockerManagerRejectsRuntimeOutsideAllowlist(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{}
	manager := &DockerManager{runner: runner, httpClient: http.DefaultClient, now: time.Now}
	compiled := scenario.Compiled{Runtimes: scenario.Runtimes{AWS: &scenario.AWSRuntime{
		Engine: "floci", Image: "example.invalid/floci@sha256:" + strings.Repeat("a", 64), IAMEnforcement: true,
	}}}
	if _, err := manager.Start(context.Background(), "tampered-run", compiled); err == nil {
		t.Fatal("Start() accepted a runtime outside the allowlist")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runtime rejection invoked Docker: %v", runner.calls)
	}
}

func TestLoopbackEndpointRejectsWildcardBinding(t *testing.T) {
	t.Parallel()
	if _, err := loopbackEndpoint("0.0.0.0:4566"); err == nil {
		t.Fatal("loopbackEndpoint() accepted wildcard binding")
	}
}

func TestLastNonemptyLineHandlesDockerPullOutput(t *testing.T) {
	t.Parallel()
	output := "Pulling image\nDigest: sha256:example\ncontainer-id\n"
	if got := lastNonemptyLine(output); got != "container-id" {
		t.Fatalf("lastNonemptyLine() = %q", got)
	}
}
