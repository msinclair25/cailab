package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	helperModeEnvironment     = "CAILAB_PORTFOLIO_DEMO_HELPER"
	helperEndpointEnvironment = "CAILAB_PORTFOLIO_DEMO_ENDPOINT"
	helperLogEnvironment      = "CAILAB_PORTFOLIO_DEMO_LOG"
	helperMarkerEnvironment   = "CAILAB_PORTFOLIO_DEMO_MARKER"
)

func TestMain(m *testing.M) {
	if os.Getenv(helperModeEnvironment) == "1" {
		os.Exit(runHelper(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func TestRunExecutesRecordingWorkflowAndCleanup(t *testing.T) {
	temporary := t.TempDir()
	stateDir := filepath.Join(temporary, "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(temporary, "commands.log")
	markerPath := filepath.Join(temporary, "remediated")

	const endpoint = "http://127.0.0.1:61234"
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer "+graphToken {
			return nil, fmt.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		response := &http.Response{Header: make(http.Header), Request: request}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == assignmentPath:
			response.StatusCode = http.StatusOK
			response.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"value":[{"id":%q,"principalId":"group:contractors"},{"id":%q,"principalId":"group:admins"}]}`, riskyAssignmentID, approvedAssignmentID)))
		case request.Method == http.MethodDelete && request.URL.Path == assignmentPath+"/"+riskyAssignmentID:
			if err := os.WriteFile(markerPath, []byte("complete\n"), 0o600); err != nil {
				return nil, err
			}
			response.StatusCode = http.StatusNoContent
			response.Body = io.NopCloser(strings.NewReader(""))
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL)
		}
		return response, nil
	})}

	t.Setenv(helperModeEnvironment, "1")
	t.Setenv(helperEndpointEnvironment, endpoint)
	t.Setenv(helperLogEnvironment, logPath)
	t.Setenv(helperMarkerEnvironment, markerPath)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	checkedEndpoints := make(map[string]bool)
	err = runWithDependencies(context.Background(), []string{
		"--cailab", binary,
		"--state-dir", stateDir,
		"--expected-version", "0.1.0-rc.1",
		"--expected-commit", "9190af11a6188fd17a614d2b9d9833d08f164188",
		"--trials", "2",
	}, strings.NewReader(""), &stdout, &stderr, httpClient, func(_ context.Context, address string, timeout time.Duration) error {
		if timeout != 3*time.Second {
			return fmt.Errorf("cleanup endpoint check = %s, %s", address, timeout)
		}
		checkedEndpoints[address] = true
		return nil
	})
	if err != nil {
		t.Fatalf("run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	for _, expected := range []string{
		"Segment 1 — clean deterministic core",
		"Segment 2 — enterprise trust path",
		"204 No Content",
		"Segment 3 — deterministic AI-agent governance controls",
		"cleanup verified",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output missing %q:\n%s", expected, stdout.String())
		}
	}
	commands, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"up --state-dir " + stateDir + " walking-skeleton",
		"up --state-dir " + stateDir + " acquisition-agent",
		"agent campaign unsafe --state-dir " + stateDir + " --trials 2",
		"agent campaign safe --state-dir " + stateDir + " --trials 2",
		"down --state-dir " + stateDir,
	} {
		if !strings.Contains(string(commands), expected) {
			t.Fatalf("command log missing %q:\n%s", expected, commands)
		}
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("state directory not clean: %+v", entries)
	}
	for _, address := range []string{"127.0.0.1:61231", "127.0.0.1:61232", "127.0.0.1:61233", "127.0.0.1:61234"} {
		if !checkedEndpoints[address] {
			t.Fatalf("runtime endpoint cleanup was not checked for %s: %+v", address, checkedEndpoints)
		}
	}
}

func TestParseRuntimeEndpoints(t *testing.T) {
	output := `Run:       run:test
Runtime:   aws/docker http://127.0.0.1:4566 (ready)
Runtime:   microsoft/native http://127.0.0.1:61234 (ready)
Runtime:   google/native http://127.0.0.1:61235 (ready)
Runtime:   oidc/native http://127.0.0.1:61236 (ready)
`
	endpoints, err := parseRuntimeEndpoints(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 4 || endpoints["microsoft"].String() != "http://127.0.0.1:61234" {
		t.Fatalf("endpoints = %+v", endpoints)
	}
	if err := validateFlagshipRuntimes(endpoints); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLoopbackEndpointRejectsUnsafeTargets(t *testing.T) {
	for _, raw := range []string{
		"https://127.0.0.1:8000",
		"http://localhost:8000",
		"http://127.0.0.1",
		"http://127.0.0.1:0",
		"http://127.0.0.1:65536",
		"http://127.0.0.1:8000/path",
		"http://127.0.0.1:8000?query=true",
		"http://user@127.0.0.1:8000",
		"http://192.0.2.1:8000",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := validateLoopbackEndpoint(raw); err == nil {
				t.Fatalf("validateLoopbackEndpoint(%q) succeeded", raw)
			}
		})
	}
}

func TestValidateBuild(t *testing.T) {
	const commit = "9190af11a6188fd17a614d2b9d9833d08f164188"
	output := "cailab 0.1.0-rc.1 (commit " + commit + ", built 2026-07-13T09:56:37Z)\n"
	if err := validateBuild(output, "0.1.0-rc.1", commit); err != nil {
		t.Fatal(err)
	}
	if err := validateBuild(output, "0.1.0", commit); err == nil {
		t.Fatal("validateBuild accepted the wrong version")
	}
	if err := validateBuild(output, "0.1.0-rc.1", strings.Repeat("a", 40)); err == nil {
		t.Fatal("validateBuild accepted the wrong commit")
	}
}

func TestPrepareStateDirRejectsExistingContentAndLoosePermissions(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "existing"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareStateDir(directory); err == nil || !strings.Contains(err.Error(), "must be empty") {
		t.Fatalf("prepareStateDir content error = %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	loose := filepath.Join(t.TempDir(), "loose")
	if err := os.Mkdir(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareStateDir(loose); err == nil || !strings.Contains(err.Error(), "group or other") {
		t.Fatalf("prepareStateDir permissions error = %v", err)
	}
}

func TestReadBounded(t *testing.T) {
	data, err := readBounded(strings.NewReader("1234"), 4)
	if err != nil || string(data) != "1234" {
		t.Fatalf("readBounded = %q, %v", data, err)
	}
	if _, err := readBounded(strings.NewReader("12345"), 4); err == nil {
		t.Fatal("readBounded accepted an oversized response")
	}
}

func runHelper(args []string) int {
	if err := appendHelperLog(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(args) == 0 {
		return 2
	}
	stateDir := helperFlagValue(args, "--state-dir")
	scenarioPath := filepath.Join(stateDir, "helper-scenario")
	switch args[0] {
	case "version":
		fmt.Println("cailab 0.1.0-rc.1 (commit 9190af11a6188fd17a614d2b9d9833d08f164188, built 2026-07-13T09:56:37Z)")
	case "doctor":
		fmt.Println("✓ prerequisites")
	case "up":
		if stateDir == "" || len(args) == 0 {
			return 2
		}
		scenarioName := args[len(args)-1]
		if err := os.WriteFile(scenarioPath, []byte(scenarioName+"\n"), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		fmt.Printf("✓ run helper:%s is active\n", scenarioName)
	case "status":
		if _, err := os.Stat(scenarioPath); err != nil {
			fmt.Fprintln(os.Stderr, "error: no active run")
			return 1
		}
		fmt.Println("Run:       helper")
		fmt.Println("Runtime:   aws/floci http://127.0.0.1:61231 (ready)")
		fmt.Println("Runtime:   oidc/native http://127.0.0.1:61232 (ready)")
		fmt.Printf("Runtime:   microsoft/native %s (ready)\n", os.Getenv(helperEndpointEnvironment))
		fmt.Println("Runtime:   google/native http://127.0.0.1:61233 (ready)")
	case "verify":
		scenarioBytes, err := os.ReadFile(scenarioPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if strings.TrimSpace(string(scenarioBytes)) == "acquisition-agent" {
			if _, err := os.Stat(os.Getenv(helperMarkerEnvironment)); err != nil {
				fmt.Println("1 passed, 1 failed")
				return verificationFailedExit
			}
			fmt.Println("2 passed, 0 failed")
		} else {
			fmt.Println("1 passed, 0 failed")
		}
	case "down":
		if stateDir != "" {
			_ = os.Remove(scenarioPath)
		}
		fmt.Println("✓ run stopped")
	case "mission", "graph", "reset", "agent":
		fmt.Printf("✓ %s complete\n", args[0])
	default:
		fmt.Fprintf(os.Stderr, "unknown helper command %q\n", args[0])
		return 2
	}
	return 0
}

func helperFlagValue(args []string, name string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
}

func appendHelperLog(args []string) error {
	path := os.Getenv(helperLogEnvironment)
	if path == "" {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, strings.Join(args, " ")+"\n")
	return err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
