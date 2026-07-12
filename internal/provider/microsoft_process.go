package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/msinclair25/cailab/internal/scenario"
)

type MicrosoftProcessManager struct {
	stateDir   string
	executable func() (string, error)
	command    func(string, string) *exec.Cmd
	httpClient *http.Client
	now        func() time.Time
}

func NewMicrosoftProcessManager(stateDir string) *MicrosoftProcessManager {
	absolute, err := filepath.Abs(stateDir)
	if err != nil {
		absolute = stateDir
	}
	return &MicrosoftProcessManager{
		stateDir: absolute, executable: os.Executable, command: microsoftRuntimeCommand,
		httpClient: &http.Client{Timeout: 2 * time.Second}, now: time.Now,
	}
}

func (m *MicrosoftProcessManager) Start(ctx context.Context, runID string, compiled scenario.Compiled) ([]Instance, error) {
	if compiled.Runtimes.Microsoft == nil {
		return nil, nil
	}
	if compiled.Runtimes.Microsoft.Engine != "native" || compiled.Providers.Microsoft == nil {
		return nil, errors.New("Microsoft runtime is not supported by this CloudAILab build")
	}
	runtimeDir := m.runtimeDir(runID)
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create Microsoft runtime directory: %w", err)
	}
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("secure Microsoft runtime directory: %w", err)
	}
	configPath := filepath.Join(runtimeDir, "control.json")
	readyPath := filepath.Join(runtimeDir, "ready.json")
	statePath := filepath.Join(runtimeDir, "microsoft-state.json")
	logPath := filepath.Join(runtimeDir, "runtime.log")
	_ = os.Remove(readyPath)
	token, err := randomControlToken()
	if err != nil {
		return nil, err
	}
	config := MicrosoftRuntimeConfig{
		RunID: runID, Listen: "127.0.0.1:0", StatePath: statePath,
		ReadyPath: readyPath, ControlToken: token, Provider: *compiled.Providers.Microsoft,
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Microsoft runtime config: %w", err)
	}
	if err := os.WriteFile(configPath, append(configData, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Microsoft runtime config: %w", err)
	}
	executable, err := m.executable()
	if err != nil {
		return nil, fmt.Errorf("locate CloudAILab executable: %w", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open Microsoft runtime log: %w", err)
	}
	command := m.command(executable, configPath)
	command.Stdout = logFile
	command.Stderr = logFile
	configureDetachedCommand(command)
	if err := command.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start Microsoft runtime: %w", err)
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		_ = command.Process.Kill()
		logFile.Close()
		return nil, fmt.Errorf("release Microsoft runtime process: %w", err)
	}
	_ = logFile.Close()

	ready, err := m.waitReady(ctx, readyPath, runID)
	if err != nil {
		if process, findErr := os.FindProcess(pid); findErr == nil {
			_ = process.Kill()
		}
		logs, _ := os.ReadFile(logPath)
		return nil, fmt.Errorf("Microsoft runtime readiness: %w; logs: %s", err, strings.TrimSpace(string(logs)))
	}
	return []Instance{{
		Provider: "microsoft", Engine: "native", ProcessID: ready.PID,
		Name: filepath.Base(runtimeDir), Endpoint: ready.Endpoint,
		ControlPath: configPath, Status: "ready",
	}}, nil
}

func microsoftRuntimeCommand(executable, configPath string) *exec.Cmd {
	return exec.Command(executable, "_runtime", "microsoft", "--config", configPath)
}

func (m *MicrosoftProcessManager) Snapshot(ctx context.Context, instances []Instance, compiled scenario.Compiled) (scenario.Compiled, error) {
	if compiled.Providers.Microsoft == nil {
		return compiled, nil
	}
	for _, instance := range instances {
		if instance.Provider == "microsoft" && instance.Engine == "native" {
			return snapshotMicrosoft(ctx, instance.Endpoint, compiled)
		}
	}
	return scenario.Compiled{}, errors.New("active scenario requires a Microsoft runtime, but none is recorded")
}

func (m *MicrosoftProcessManager) Stop(ctx context.Context, runID string, instances []Instance) error {
	if len(instances) == 0 {
		configPath := filepath.Join(m.runtimeDir(runID), "control.json")
		readyPath := filepath.Join(m.runtimeDir(runID), "ready.json")
		if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		readyData, err := os.ReadFile(readyPath)
		if err != nil {
			return fmt.Errorf("read stale Microsoft runtime readiness: %w", err)
		}
		var ready microsoftReady
		if err := json.Unmarshal(readyData, &ready); err != nil {
			return fmt.Errorf("decode stale Microsoft runtime readiness: %w", err)
		}
		instances = []Instance{{Provider: "microsoft", Engine: "native", Endpoint: ready.Endpoint, ProcessID: ready.PID, ControlPath: configPath}}
	}
	var stopErrors []error
	for _, instance := range instances {
		if instance.Provider != "microsoft" || instance.Engine != "native" {
			continue
		}
		if err := m.stopInstance(ctx, runID, instance); err != nil {
			stopErrors = append(stopErrors, err)
		}
	}
	return errors.Join(stopErrors...)
}

func (m *MicrosoftProcessManager) stopInstance(ctx context.Context, runID string, instance Instance) error {
	expectedDir := m.runtimeDir(runID)
	expectedControl := filepath.Join(expectedDir, "control.json")
	control, err := filepath.Abs(instance.ControlPath)
	if err != nil || control != expectedControl {
		return fmt.Errorf("refuse to stop Microsoft runtime: control path does not match run %q", runID)
	}
	data, err := os.ReadFile(control)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read Microsoft runtime control: %w", err)
	}
	var config MicrosoftRuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("decode Microsoft runtime control: %w", err)
	}
	if config.RunID != runID || config.ControlToken == "" {
		return fmt.Errorf("refuse to stop Microsoft runtime: run identity does not match %q", runID)
	}
	readyData, err := os.ReadFile(filepath.Join(expectedDir, "ready.json"))
	if err != nil {
		return fmt.Errorf("read Microsoft runtime ownership record: %w", err)
	}
	var ready microsoftReady
	if err := json.Unmarshal(readyData, &ready); err != nil {
		return fmt.Errorf("decode Microsoft runtime ownership record: %w", err)
	}
	if ready.RunID != runID || ready.Endpoint != instance.Endpoint || !isIPv4LoopbackEndpoint(ready.Endpoint) {
		return fmt.Errorf("refuse to stop Microsoft runtime: endpoint ownership does not match run %q", runID)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(instance.Endpoint, "/")+"/_cailab/shutdown", nil)
	if err != nil {
		return fmt.Errorf("build Microsoft shutdown request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+config.ControlToken)
	request.Header.Set("X-CloudAILab-Run", runID)
	response, err := m.httpClient.Do(request)
	shutdownAccepted := false
	if err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode != http.StatusAccepted {
			return fmt.Errorf("Microsoft runtime rejected shutdown with status %d", response.StatusCode)
		}
		shutdownAccepted = true
	}
	deadline := m.now().Add(5 * time.Second)
	stopped := false
	for m.now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		healthRequest, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(instance.Endpoint, "/")+"/_cailab/health", nil)
		if requestErr != nil {
			return requestErr
		}
		healthResponse, healthErr := m.httpClient.Do(healthRequest)
		if healthErr != nil {
			stopped = true
			break
		}
		healthResponse.Body.Close()
		time.Sleep(50 * time.Millisecond)
	}
	if !stopped {
		if shutdownAccepted {
			return errors.New("Microsoft runtime accepted shutdown but remained reachable")
		}
		return errors.New("Microsoft runtime could not be authenticated for shutdown and remains reachable")
	}
	removeDeadline := m.now().Add(5 * time.Second)
	for {
		if err := os.RemoveAll(expectedDir); err == nil {
			return nil
		} else if !m.now().Before(removeDeadline) {
			return fmt.Errorf("remove Microsoft runtime directory: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (m *MicrosoftProcessManager) waitReady(ctx context.Context, path, runID string) (microsoftReady, error) {
	deadline := m.now().Add(15 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var ready microsoftReady
			if decodeErr := json.Unmarshal(data, &ready); decodeErr == nil && ready.RunID == runID && ready.Endpoint != "" && ready.PID > 0 {
				request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, ready.Endpoint+"/_cailab/health", nil)
				if requestErr == nil {
					response, responseErr := m.httpClient.Do(request)
					if responseErr == nil {
						var health struct {
							Ready bool   `json:"ready"`
							RunID string `json:"runId"`
						}
						decodeErr = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&health)
						response.Body.Close()
						if response.StatusCode == http.StatusOK && decodeErr == nil && health.Ready && health.RunID == runID {
							return ready, nil
						}
					}
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return microsoftReady{}, err
		}
		if !m.now().Before(deadline) {
			return microsoftReady{}, errors.New("timed out waiting for native facade")
		}
		select {
		case <-ctx.Done():
			return microsoftReady{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (m *MicrosoftProcessManager) runtimeDir(runID string) string {
	return filepath.Join(m.stateDir, "runtimes", containerName(runID, "microsoft"))
}

func randomControlToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate Microsoft runtime control token: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func isIPv4LoopbackEndpoint(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" {
		return false
	}
	return parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}
