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

type NativeRuntimeControl struct {
	RunID        string `json:"runId"`
	Listen       string `json:"listen"`
	StatePath    string `json:"statePath"`
	ReadyPath    string `json:"readyPath"`
	ControlToken string `json:"controlToken"`
}

type nativeReady struct {
	RunID    string `json:"runId"`
	Endpoint string `json:"endpoint"`
	PID      int    `json:"pid"`
}

type NativeProcessManager struct {
	stateDir   string
	executable func() (string, error)
	command    func(string, string, string) *exec.Cmd
	httpClient *http.Client
	now        func() time.Time
}

func NewNativeProcessManager(stateDir string) *NativeProcessManager {
	absolute, err := filepath.Abs(stateDir)
	if err != nil {
		absolute = stateDir
	}
	return &NativeProcessManager{
		stateDir: absolute, executable: os.Executable, command: nativeRuntimeCommand,
		httpClient: &http.Client{Timeout: 2 * time.Second}, now: time.Now,
	}
}

// NewMicrosoftProcessManager is retained for source compatibility with existing tests and embedders.
func NewMicrosoftProcessManager(stateDir string) *NativeProcessManager {
	return NewNativeProcessManager(stateDir)
}

func (m *NativeProcessManager) Start(ctx context.Context, runID string, compiled scenario.Compiled) ([]Instance, error) {
	var instances []Instance
	for _, name := range []string{"oidc", "microsoft", "google"} {
		if !nativeRuntimeConfigured(compiled, name) {
			continue
		}
		instance, err := m.startProvider(ctx, runID, name, compiled)
		if err != nil {
			_ = m.Stop(context.Background(), runID, instances, compiled)
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func nativeRuntimeConfigured(compiled scenario.Compiled, name string) bool {
	switch name {
	case "microsoft":
		return compiled.Runtimes.Microsoft != nil
	case "google":
		return compiled.Runtimes.Google != nil
	case "oidc":
		return compiled.Runtimes.OIDC != nil
	default:
		return false
	}
}

func (m *NativeProcessManager) startProvider(ctx context.Context, runID, name string, compiled scenario.Compiled) (Instance, error) {
	runtimeDir := m.runtimeDir(runID, name)
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return Instance{}, fmt.Errorf("create %s runtime directory: %w", name, err)
	}
	started := false
	defer func() {
		if !started {
			_ = os.RemoveAll(runtimeDir)
		}
	}()
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		return Instance{}, fmt.Errorf("secure %s runtime directory: %w", name, err)
	}
	control := NativeRuntimeControl{
		RunID: runID, Listen: "127.0.0.1:0",
		StatePath: filepath.Join(runtimeDir, name+"-state.json"),
		ReadyPath: filepath.Join(runtimeDir, "ready.json"),
	}
	_ = os.Remove(control.ReadyPath)
	token, err := randomControlToken()
	if err != nil {
		return Instance{}, err
	}
	control.ControlToken = token

	var config any
	switch name {
	case "microsoft":
		if compiled.Runtimes.Microsoft.Engine != "native" || compiled.Providers.Microsoft == nil {
			return Instance{}, errors.New("Microsoft runtime is not supported by this CloudAILab build")
		}
		config = MicrosoftRuntimeConfig{NativeRuntimeControl: control, Provider: *compiled.Providers.Microsoft}
	case "google":
		if compiled.Runtimes.Google.Engine != "native" || compiled.Providers.Google == nil {
			return Instance{}, errors.New("Google runtime is not supported by this CloudAILab build")
		}
		config = GoogleRuntimeConfig{NativeRuntimeControl: control, Provider: *compiled.Providers.Google}
	case "oidc":
		if compiled.Runtimes.OIDC.Engine != "native" || compiled.Providers.OIDC == nil {
			return Instance{}, errors.New("OIDC runtime is not supported by this CloudAILab build")
		}
		config = OIDCRuntimeConfig{NativeRuntimeControl: control, Provider: *compiled.Providers.OIDC}
	default:
		return Instance{}, fmt.Errorf("unsupported native provider %q", name)
	}

	configPath := filepath.Join(runtimeDir, "control.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return Instance{}, fmt.Errorf("encode %s runtime config: %w", name, err)
	}
	if err := os.WriteFile(configPath, append(configData, '\n'), 0o600); err != nil {
		return Instance{}, fmt.Errorf("write %s runtime config: %w", name, err)
	}
	executable, err := m.executable()
	if err != nil {
		return Instance{}, fmt.Errorf("locate CloudAILab executable: %w", err)
	}
	logPath := filepath.Join(runtimeDir, "runtime.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Instance{}, fmt.Errorf("open %s runtime log: %w", name, err)
	}
	command := m.command(executable, name, configPath)
	command.Stdout = logFile
	command.Stderr = logFile
	configureDetachedCommand(command)
	if err := command.Start(); err != nil {
		logFile.Close()
		return Instance{}, fmt.Errorf("start %s runtime: %w", name, err)
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		_ = command.Process.Kill()
		logFile.Close()
		return Instance{}, fmt.Errorf("release %s runtime process: %w", name, err)
	}
	_ = logFile.Close()
	ready, err := m.waitReady(ctx, control.ReadyPath, runID)
	if err != nil {
		if process, findErr := os.FindProcess(pid); findErr == nil {
			_ = process.Kill()
		}
		logs, _ := os.ReadFile(logPath)
		return Instance{}, fmt.Errorf("%s runtime readiness: %w; logs: %s", name, err, strings.TrimSpace(string(logs)))
	}
	started = true
	return Instance{Provider: name, Engine: "native", ProcessID: ready.PID, Name: filepath.Base(runtimeDir), Endpoint: ready.Endpoint, ControlPath: configPath, Status: "ready"}, nil
}

func nativeRuntimeCommand(executable, provider, configPath string) *exec.Cmd {
	return exec.Command(executable, "_runtime", provider, "--config", configPath)
}

func (m *NativeProcessManager) Snapshot(ctx context.Context, instances []Instance, compiled scenario.Compiled) (scenario.Compiled, error) {
	snapshot := compiled
	for _, name := range []string{"oidc", "microsoft", "google"} {
		if !nativeRuntimeConfigured(compiled, name) {
			continue
		}
		instance, found := nativeInstance(instances, name)
		if !found {
			return scenario.Compiled{}, fmt.Errorf("active scenario requires a %s runtime, but none is recorded", name)
		}
		var err error
		if name == "microsoft" {
			snapshot, err = snapshotMicrosoft(ctx, instance.Endpoint, snapshot)
		} else if name == "google" {
			snapshot, err = snapshotGoogle(ctx, instance.Endpoint, snapshot)
		}
		if err != nil {
			return scenario.Compiled{}, err
		}
	}
	return snapshot, nil
}

func nativeInstance(instances []Instance, provider string) (Instance, bool) {
	for _, instance := range instances {
		if instance.Provider == provider && instance.Engine == "native" {
			return instance, true
		}
	}
	return Instance{}, false
}

func (m *NativeProcessManager) Stop(ctx context.Context, runID string, instances []Instance, compiled scenario.Compiled) error {
	var stopErrors []error
	for _, name := range []string{"google", "microsoft", "oidc"} {
		providerInstances := make([]Instance, 0, 1)
		for _, instance := range instances {
			if instance.Provider == name && instance.Engine == "native" {
				providerInstances = append(providerInstances, instance)
			}
		}
		if len(providerInstances) == 0 && nativeRuntimeConfigured(compiled, name) {
			stale, err := m.staleInstance(runID, name)
			if err != nil {
				stopErrors = append(stopErrors, err)
				continue
			}
			if stale != nil {
				providerInstances = append(providerInstances, *stale)
			}
		}
		for _, instance := range providerInstances {
			if err := m.stopInstance(ctx, runID, name, instance); err != nil {
				stopErrors = append(stopErrors, err)
			}
		}
	}
	return errors.Join(stopErrors...)
}

func (m *NativeProcessManager) RotateOIDC(ctx context.Context, runID string, instances []Instance) (OIDCJWKSet, error) {
	instance, found := nativeInstance(instances, "oidc")
	if !found {
		return OIDCJWKSet{}, errors.New("active scenario has no OIDC runtime")
	}
	expectedDir := m.runtimeDir(runID, "oidc")
	expectedControl := filepath.Join(expectedDir, "control.json")
	controlPath, err := filepath.Abs(instance.ControlPath)
	if err != nil || controlPath != expectedControl {
		return OIDCJWKSet{}, fmt.Errorf("refuse to rotate OIDC runtime: control path does not match run %q", runID)
	}
	data, err := os.ReadFile(controlPath)
	if err != nil {
		return OIDCJWKSet{}, fmt.Errorf("read OIDC runtime control: %w", err)
	}
	var control NativeRuntimeControl
	if err := json.Unmarshal(data, &control); err != nil {
		return OIDCJWKSet{}, fmt.Errorf("decode OIDC runtime control: %w", err)
	}
	if control.RunID != runID || control.ControlToken == "" || !isIPv4LoopbackEndpoint(instance.Endpoint) {
		return OIDCJWKSet{}, fmt.Errorf("refuse to rotate OIDC runtime: ownership does not match run %q", runID)
	}
	readyData, err := os.ReadFile(filepath.Join(expectedDir, "ready.json"))
	if err != nil {
		return OIDCJWKSet{}, fmt.Errorf("read OIDC runtime ownership record: %w", err)
	}
	var ready nativeReady
	if err := json.Unmarshal(readyData, &ready); err != nil {
		return OIDCJWKSet{}, fmt.Errorf("decode OIDC runtime ownership record: %w", err)
	}
	if ready.RunID != runID || ready.Endpoint != instance.Endpoint || !isIPv4LoopbackEndpoint(ready.Endpoint) {
		return OIDCJWKSet{}, fmt.Errorf("refuse to rotate OIDC runtime: endpoint ownership does not match run %q", runID)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(ready.Endpoint, "/")+"/_cailab/rotate", nil)
	if err != nil {
		return OIDCJWKSet{}, fmt.Errorf("build OIDC rotation request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+control.ControlToken)
	request.Header.Set("X-CloudAILab-Run", runID)
	response, err := m.httpClient.Do(request)
	if err != nil {
		return OIDCJWKSet{}, fmt.Errorf("rotate OIDC signing key: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return OIDCJWKSet{}, fmt.Errorf("OIDC runtime rejected rotation with status %d", response.StatusCode)
	}
	var set OIDCJWKSet
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&set); err != nil {
		return OIDCJWKSet{}, fmt.Errorf("decode rotated OIDC key set: %w", err)
	}
	return set, nil
}

func (m *NativeProcessManager) staleInstance(runID, provider string) (*Instance, error) {
	runtimeDir := m.runtimeDir(runID, provider)
	configPath := filepath.Join(runtimeDir, "control.json")
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	readyData, err := os.ReadFile(filepath.Join(runtimeDir, "ready.json"))
	if err != nil {
		return nil, fmt.Errorf("read stale %s runtime readiness: %w", provider, err)
	}
	var ready nativeReady
	if err := json.Unmarshal(readyData, &ready); err != nil {
		return nil, fmt.Errorf("decode stale %s runtime readiness: %w", provider, err)
	}
	return &Instance{Provider: provider, Engine: "native", Endpoint: ready.Endpoint, ProcessID: ready.PID, ControlPath: configPath}, nil
}

func (m *NativeProcessManager) stopInstance(ctx context.Context, runID, provider string, instance Instance) error {
	expectedDir := m.runtimeDir(runID, provider)
	expectedControl := filepath.Join(expectedDir, "control.json")
	controlPath, err := filepath.Abs(instance.ControlPath)
	if err != nil || controlPath != expectedControl {
		return fmt.Errorf("refuse to stop %s runtime: control path does not match run %q", provider, runID)
	}
	data, err := os.ReadFile(controlPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s runtime control: %w", provider, err)
	}
	var control NativeRuntimeControl
	if err := json.Unmarshal(data, &control); err != nil {
		return fmt.Errorf("decode %s runtime control: %w", provider, err)
	}
	if control.RunID != runID || control.ControlToken == "" {
		return fmt.Errorf("refuse to stop %s runtime: run identity does not match %q", provider, runID)
	}
	readyData, err := os.ReadFile(filepath.Join(expectedDir, "ready.json"))
	if err != nil {
		return fmt.Errorf("read %s runtime ownership record: %w", provider, err)
	}
	var ready nativeReady
	if err := json.Unmarshal(readyData, &ready); err != nil {
		return fmt.Errorf("decode %s runtime ownership record: %w", provider, err)
	}
	if ready.RunID != runID || ready.Endpoint != instance.Endpoint || !isIPv4LoopbackEndpoint(ready.Endpoint) {
		return fmt.Errorf("refuse to stop %s runtime: endpoint ownership does not match run %q", provider, runID)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(instance.Endpoint, "/")+"/_cailab/shutdown", nil)
	if err != nil {
		return fmt.Errorf("build %s shutdown request: %w", provider, err)
	}
	request.Header.Set("Authorization", "Bearer "+control.ControlToken)
	request.Header.Set("X-CloudAILab-Run", runID)
	response, err := m.httpClient.Do(request)
	shutdownAccepted := false
	if err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode != http.StatusAccepted {
			return fmt.Errorf("%s runtime rejected shutdown with status %d", provider, response.StatusCode)
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
			return fmt.Errorf("%s runtime accepted shutdown but remained reachable", provider)
		}
		return fmt.Errorf("%s runtime could not be authenticated for shutdown and remains reachable", provider)
	}
	removeDeadline := m.now().Add(5 * time.Second)
	for {
		if err := os.RemoveAll(expectedDir); err == nil {
			return nil
		} else if !m.now().Before(removeDeadline) {
			return fmt.Errorf("remove %s runtime directory: %w", provider, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (m *NativeProcessManager) waitReady(ctx context.Context, path, runID string) (nativeReady, error) {
	deadline := m.now().Add(15 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var ready nativeReady
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
			return nativeReady{}, err
		}
		if !m.now().Before(deadline) {
			return nativeReady{}, errors.New("timed out waiting for native facade")
		}
		select {
		case <-ctx.Done():
			return nativeReady{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (m *NativeProcessManager) runtimeDir(runID, provider string) string {
	return filepath.Join(m.stateDir, "runtimes", containerName(runID, provider))
}

func randomControlToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate native runtime control token: %w", err)
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
