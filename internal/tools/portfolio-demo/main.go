// Command portfolio-demo runs the recording workflow against an existing
// CloudAILab binary. It is repository tooling, not part of the cailab CLI.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	verificationFailedExit = 3
	graphToken             = "cailab-local"
	assignmentPath         = "/v1.0/servicePrincipals/77777777-7777-4777-8777-777777777777/appRoleAssignedTo"
	riskyAssignmentID      = "99999999-9999-4999-8999-999999999999"
	approvedAssignmentID   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type options struct {
	binary          string
	stateDir        string
	expectedVersion string
	expectedCommit  string
	trials          int
	pause           bool
	keepState       bool
}

type demo struct {
	options
	pauseInput   *bufio.Reader
	stdout       io.Writer
	stderr       io.Writer
	httpClient   *http.Client
	endpointWait func(context.Context, string, time.Duration) error
	removeState  bool
	runtimeURLs  map[string]*url.URL
	lifecycleRun bool
	cleanupDone  bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "portfolio demo: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return runWithDependencies(ctx, args, stdin, stdout, stderr, nil, nil)
}

func runWithDependencies(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	httpClient *http.Client,
	endpointWait func(context.Context, string, time.Duration) error,
) error {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}
	if err := validateBinary(opts.binary); err != nil {
		return err
	}
	stateDir, removeState, err := prepareStateDir(opts.stateDir)
	if err != nil {
		return err
	}
	opts.stateDir = stateDir
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if endpointWait == nil {
		endpointWait = waitForClosedEndpoint
	}
	app := &demo{
		options: opts, pauseInput: bufio.NewReader(stdin), stdout: stdout, stderr: stderr,
		httpClient: httpClient, endpointWait: endpointWait, removeState: removeState,
	}
	defer app.cleanup()
	return app.execute(ctx)
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("portfolio-demo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	fs.StringVar(&opts.binary, "cailab", "", "path to the reviewed cailab binary")
	fs.StringVar(&opts.stateDir, "state-dir", "", "empty dedicated state directory; default: secure temporary directory")
	fs.StringVar(&opts.expectedVersion, "expected-version", "", "required cailab version for the recording")
	fs.StringVar(&opts.expectedCommit, "expected-commit", "", "required full cailab commit for the recording")
	fs.IntVar(&opts.trials, "trials", 3, "restored trials for each deterministic agent control")
	fs.BoolVar(&opts.pause, "pause", false, "wait for Enter between recording segments")
	fs.BoolVar(&opts.keepState, "keep-state", false, "retain an automatically created state directory after cleanup")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		return options{}, errors.New("portfolio-demo accepts no positional arguments")
	}
	if opts.binary == "" {
		return options{}, errors.New("--cailab is required")
	}
	if (opts.expectedVersion == "") != (opts.expectedCommit == "") {
		return options{}, errors.New("--expected-version and --expected-commit must be supplied together")
	}
	if opts.trials < 2 || opts.trials > 100 {
		return options{}, errors.New("--trials must be between 2 and 100")
	}
	absBinary, err := filepath.Abs(opts.binary)
	if err != nil {
		return options{}, fmt.Errorf("resolve cailab binary: %w", err)
	}
	opts.binary = absBinary
	return opts, nil
}

func validateBinary(binary string) error {
	info, err := os.Lstat(binary)
	if err != nil {
		return fmt.Errorf("inspect cailab binary: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("cailab binary must not be a symbolic link")
	}
	if !info.Mode().IsRegular() {
		return errors.New("cailab binary must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return errors.New("cailab binary is not executable")
	}
	return nil
}

func prepareStateDir(requested string) (string, bool, error) {
	if requested == "" {
		directory, err := os.MkdirTemp("", "cailab-portfolio-demo-")
		if err != nil {
			return "", false, fmt.Errorf("create temporary state directory: %w", err)
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			_ = os.RemoveAll(directory)
			return "", false, fmt.Errorf("secure temporary state directory: %w", err)
		}
		return directory, true, nil
	}
	abs, err := filepath.Abs(requested)
	if err != nil {
		return "", false, fmt.Errorf("resolve state directory: %w", err)
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return "", false, fmt.Errorf("create state directory: %w", err)
		}
		if err := os.Chmod(abs, 0o700); err != nil {
			return "", false, fmt.Errorf("secure state directory: %w", err)
		}
	} else if err != nil {
		return "", false, fmt.Errorf("inspect state directory: %w", err)
	} else {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", false, errors.New("--state-dir must not be a symbolic link")
		}
		if !info.IsDir() {
			return "", false, errors.New("--state-dir must name a directory")
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			return "", false, errors.New("--state-dir must not be accessible to group or other users")
		}
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", false, fmt.Errorf("inspect state directory: %w", err)
	}
	if len(entries) != 0 {
		return "", false, errors.New("--state-dir must be empty so the demo cannot replace an existing run")
	}
	return abs, false, nil
}

func (d *demo) execute(ctx context.Context) error {
	fmt.Fprintln(d.stdout, "CloudAILab portfolio demo")
	fmt.Fprintf(d.stdout, "Dedicated state: %s\n", d.stateDir)
	versionOutput, err := d.command(ctx, 0, true, "version")
	if err != nil {
		return err
	}
	if err := validateBuild(versionOutput, d.expectedVersion, d.expectedCommit); err != nil {
		return err
	}

	d.heading("Segment 1 — clean deterministic core")
	if err := d.pauseForRecording(); err != nil {
		return err
	}
	for _, step := range [][]string{
		{"doctor", "walking-skeleton"},
		{"up", "--state-dir", d.stateDir, "walking-skeleton"},
		{"graph", "path", "--state-dir", d.stateDir, "google:alex", "aws:acquisition-data"},
		{"verify", "--state-dir", d.stateDir},
		{"down", "--state-dir", d.stateDir},
	} {
		if _, err := d.command(ctx, 0, true, step...); err != nil {
			return err
		}
	}

	d.heading("Segment 2 — enterprise trust path")
	if err := d.pauseForRecording(); err != nil {
		return err
	}
	for _, step := range [][]string{
		{"doctor", "acquisition-agent"},
		{"up", "--state-dir", d.stateDir, "acquisition-agent"},
		{"mission", "--state-dir", d.stateDir},
		{"graph", "path", "--state-dir", d.stateDir, "google:contractor", "aws:acquisition-data"},
		{"graph", "path", "--state-dir", d.stateDir, "google:security-admin", "aws:acquisition-data"},
	} {
		if _, err := d.command(ctx, 0, true, step...); err != nil {
			return err
		}
	}
	if _, err := d.command(ctx, verificationFailedExit, true, "verify", "--state-dir", d.stateDir); err != nil {
		return fmt.Errorf("initial vulnerable verification: %w", err)
	}
	statusOutput, err := d.command(ctx, 0, true, "status", "--state-dir", d.stateDir)
	if err != nil {
		return err
	}
	d.runtimeURLs, err = parseRuntimeEndpoints(statusOutput)
	if err != nil {
		return err
	}
	if err := validateFlagshipRuntimes(d.runtimeURLs); err != nil {
		return err
	}
	graphEndpoint, ok := d.runtimeURLs["microsoft"]
	if !ok {
		return errors.New("status output did not contain a Microsoft runtime endpoint")
	}
	if err := d.inspectAndRemediate(ctx, graphEndpoint); err != nil {
		return err
	}
	for _, step := range [][]string{
		{"graph", "path", "--state-dir", d.stateDir, "google:contractor", "aws:acquisition-data"},
		{"graph", "path", "--state-dir", d.stateDir, "google:security-admin", "aws:acquisition-data"},
		{"verify", "--state-dir", d.stateDir},
	} {
		if _, err := d.command(ctx, 0, true, step...); err != nil {
			return err
		}
	}

	d.heading("Segment 3 — deterministic AI-agent governance controls")
	if err := d.pauseForRecording(); err != nil {
		return err
	}
	if _, err := d.command(ctx, 0, true, "reset", "--state-dir", d.stateDir); err != nil {
		return err
	}
	trialCount := strconv.Itoa(d.trials)
	for _, control := range []string{"unsafe", "safe"} {
		if _, err := d.command(ctx, 0, true, "agent", "campaign", control,
			"--state-dir", d.stateDir, "--trials", trialCount,
			"--fixture", "drive-runbook-export", "--format", "markdown"); err != nil {
			return err
		}
	}

	d.heading("Segment 4 — boundaries and cleanup")
	if err := d.pauseForRecording(); err != nil {
		return err
	}
	if _, err := d.command(ctx, 0, true, "down", "--state-dir", d.stateDir); err != nil {
		return err
	}
	if err := d.verifyStopped(ctx); err != nil {
		return err
	}
	d.cleanupDone = true
	fmt.Fprintln(d.stdout, "✓ cleanup verified: no active run and no recorded provider endpoint accepts connections")
	return nil
}

func (d *demo) command(ctx context.Context, expectedExit int, display bool, args ...string) (string, error) {
	stdout, _, err := d.commandResult(ctx, expectedExit, display, args...)
	return stdout, err
}

func (d *demo) commandResult(ctx context.Context, expectedExit int, display bool, args ...string) (string, string, error) {
	if len(args) > 0 && args[0] == "up" {
		d.lifecycleRun = true
	}
	if display {
		fmt.Fprintf(d.stdout, "\n$ cailab %s\n", strings.Join(args, " "))
	}
	command := exec.CommandContext(ctx, d.binary, args...)
	var stdout, stderr bytes.Buffer
	if display {
		command.Stdout = io.MultiWriter(d.stdout, &stdout)
		command.Stderr = io.MultiWriter(d.stderr, &stderr)
	} else {
		command.Stdout = &stdout
		command.Stderr = &stderr
	}
	err := command.Run()
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return stdout.String(), stderr.String(), fmt.Errorf("execute cailab %s: %w", args[0], err)
		}
		exitCode = exitError.ExitCode()
	}
	if exitCode != expectedExit {
		return stdout.String(), stderr.String(), fmt.Errorf("cailab %s exited %d, expected %d: %s", args[0], exitCode, expectedExit, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), stderr.String(), nil
}

func validateBuild(output, expectedVersion, expectedCommit string) error {
	if expectedVersion == "" && expectedCommit == "" {
		return nil
	}
	prefix := "cailab " + expectedVersion + " (commit " + expectedCommit + ", built "
	if !strings.HasPrefix(output, prefix) || !strings.HasSuffix(output, ")\n") {
		return fmt.Errorf("binary identity does not match expected version %s and commit %s", expectedVersion, expectedCommit)
	}
	return nil
}

func parseRuntimeEndpoints(output string) (map[string]*url.URL, error) {
	endpoints := make(map[string]*url.URL)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 4 || fields[0] != "Runtime:" {
			continue
		}
		providerAndEngine := strings.Split(fields[1], "/")
		if len(providerAndEngine) != 2 || providerAndEngine[0] == "" || providerAndEngine[1] == "" {
			return nil, fmt.Errorf("invalid runtime identity in status output: %q", line)
		}
		if fields[3] != "(ready)" {
			return nil, fmt.Errorf("%s runtime is not ready in status output: %q", providerAndEngine[0], line)
		}
		endpoint, err := validateLoopbackEndpoint(fields[2])
		if err != nil {
			return nil, fmt.Errorf("invalid %s runtime endpoint: %w", providerAndEngine[0], err)
		}
		if _, exists := endpoints[providerAndEngine[0]]; exists {
			return nil, fmt.Errorf("duplicate %s runtime endpoint", providerAndEngine[0])
		}
		endpoints[providerAndEngine[0]] = endpoint
	}
	if len(endpoints) == 0 {
		return nil, errors.New("status output contained no runtime endpoints")
	}
	return endpoints, nil
}

func validateFlagshipRuntimes(endpoints map[string]*url.URL) error {
	required := []string{"aws", "microsoft", "google", "oidc"}
	if len(endpoints) != len(required) {
		return fmt.Errorf("flagship status contained %d runtime endpoints, expected %d", len(endpoints), len(required))
	}
	for _, providerName := range required {
		if endpoints[providerName] == nil {
			return fmt.Errorf("flagship status did not contain the %s runtime endpoint", providerName)
		}
	}
	return nil
}

func validateLoopbackEndpoint(raw string) (*url.URL, error) {
	endpoint, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.User != nil ||
		endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("endpoint must be origin-only HTTP on IPv4 loopback")
	}
	port, err := strconv.Atoi(endpoint.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("endpoint must contain a valid TCP port")
	}
	return endpoint, nil
}

func (d *demo) inspectAndRemediate(ctx context.Context, endpoint *url.URL) error {
	collection := endpoint.String() + assignmentPath
	fmt.Fprintf(d.stdout, "\n$ GET %s\n", collection)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, collection, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+graphToken)
	response, err := d.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("inspect Microsoft assignment: %w", err)
	}
	body, readErr := readBounded(response.Body, 1<<20)
	closeErr := response.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read Microsoft assignment: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Microsoft assignment response: %w", closeErr)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("inspect Microsoft assignment: status %d", response.StatusCode)
	}
	var assignments struct {
		Value []struct {
			ID string `json:"id"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &assignments); err != nil {
		return fmt.Errorf("decode Microsoft assignment response: %w", err)
	}
	foundRisky, foundApproved := false, false
	for _, assignment := range assignments.Value {
		foundRisky = foundRisky || assignment.ID == riskyAssignmentID
		foundApproved = foundApproved || assignment.ID == approvedAssignmentID
	}
	if !foundRisky || !foundApproved {
		return errors.New("Microsoft assignment response must contain both the risky and approved assignments")
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, body, "", "  "); err != nil {
		return fmt.Errorf("format Microsoft assignment response: %w", err)
	}
	formatted.WriteByte('\n')
	if _, err := io.Copy(d.stdout, &formatted); err != nil {
		return err
	}

	remediation := collection + "/" + riskyAssignmentID
	fmt.Fprintf(d.stdout, "\n$ DELETE %s\n", remediation)
	request, err = http.NewRequestWithContext(ctx, http.MethodDelete, remediation, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+graphToken)
	response, err = d.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("remove Microsoft assignment: %w", err)
	}
	_, readErr = readBounded(response.Body, 1<<20)
	closeErr = response.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read Microsoft remediation response: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Microsoft remediation response: %w", closeErr)
	}
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("remove Microsoft assignment: status %d", response.StatusCode)
	}
	fmt.Fprintln(d.stdout, "204 No Content")
	return nil
}

func (d *demo) verifyStopped(ctx context.Context) error {
	_, statusError, err := d.commandResult(ctx, 1, false, "status", "--state-dir", d.stateDir)
	if err != nil {
		return fmt.Errorf("verify no active run: %w", err)
	}
	if !strings.Contains(statusError, "no active run") {
		return fmt.Errorf("verify no active run: unexpected status error: %s", strings.TrimSpace(statusError))
	}
	for providerName, endpoint := range d.runtimeURLs {
		if err := d.endpointWait(ctx, endpoint.Host, 3*time.Second); err != nil {
			return fmt.Errorf("verify %s cleanup: %w", providerName, err)
		}
	}
	return nil
}

func waitForClosedEndpoint(ctx context.Context, address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		connection, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(ctx, "tcp", address)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}
		_ = connection.Close()
		if time.Now().After(deadline) {
			return errors.New("endpoint still accepts TCP connections")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (d *demo) heading(title string) {
	fmt.Fprintf(d.stdout, "\n=== %s ===\n", title)
}

func (d *demo) pauseForRecording() error {
	if !d.pause {
		return nil
	}
	fmt.Fprint(d.stdout, "Press Enter to begin this segment...")
	if _, err := d.pauseInput.ReadString('\n'); err != nil {
		return fmt.Errorf("wait for recording input: %w", err)
	}
	return nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d-byte limit", limit)
	}
	return data, nil
}

func (d *demo) cleanup() {
	if d.lifecycleRun && !d.cleanupDone {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		_, err := d.command(ctx, 0, false, "down", "--state-dir", d.stateDir)
		cancel()
		if err != nil {
			fmt.Fprintf(d.stderr, "portfolio demo cleanup warning: %v\n", err)
		}
	}
	if d.removeState && !d.keepState {
		if err := os.RemoveAll(d.stateDir); err != nil {
			fmt.Fprintf(d.stderr, "portfolio demo state cleanup warning: %v\n", err)
		}
	} else if d.removeState {
		fmt.Fprintf(d.stdout, "Retained demo state: %s\n", d.stateDir)
	}
}
