package provider

import (
	"context"
	"errors"

	"github.com/msinclair25/cailab/internal/scenario"
)

// Instance is the persisted handle needed to inspect and clean up one provider runtime.
type Instance struct {
	Provider    string `json:"provider"`
	Engine      string `json:"engine"`
	ContainerID string `json:"containerId,omitempty"`
	ProcessID   int    `json:"processId,omitempty"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Image       string `json:"image,omitempty"`
	ControlPath string `json:"controlPath,omitempty"`
	Status      string `json:"status"`
}

// Manager owns the external provider-runtime lifecycle for a scenario run.
type Manager interface {
	Start(context.Context, string, scenario.Compiled) ([]Instance, error)
	Stop(context.Context, string, []Instance, scenario.Compiled) error
	Snapshot(context.Context, []Instance, scenario.Compiled) (scenario.Compiled, error)
}

type CompositeManager struct {
	docker *DockerManager
	native *NativeProcessManager
}

func NewManager(stateDir string) *CompositeManager {
	return &CompositeManager{
		docker: NewDockerManager(), native: NewNativeProcessManager(stateDir),
	}
}

func (m *CompositeManager) Start(ctx context.Context, runID string, compiled scenario.Compiled) ([]Instance, error) {
	instances, err := m.docker.Start(ctx, runID, compiled)
	if err != nil {
		return nil, err
	}
	nativeInstances, err := m.native.Start(ctx, runID, compiled)
	if err != nil {
		_ = m.docker.Stop(context.Background(), runID, instances)
		return nil, err
	}
	return append(instances, nativeInstances...), nil
}

func (m *CompositeManager) Stop(ctx context.Context, runID string, instances []Instance, compiled scenario.Compiled) error {
	var nativeInstances, dockerInstances []Instance
	for _, instance := range instances {
		switch instance.Provider {
		case "microsoft", "google":
			nativeInstances = append(nativeInstances, instance)
		case "aws":
			dockerInstances = append(dockerInstances, instance)
		}
	}
	nativeErr := m.native.Stop(ctx, runID, nativeInstances, compiled)
	var dockerErr error
	if compiled.Runtimes.AWS != nil || len(dockerInstances) > 0 {
		dockerErr = m.docker.Stop(ctx, runID, dockerInstances)
	}
	return errors.Join(nativeErr, dockerErr)
}

func (m *CompositeManager) Snapshot(ctx context.Context, instances []Instance, compiled scenario.Compiled) (scenario.Compiled, error) {
	snapshot, err := m.docker.Snapshot(ctx, instances, compiled)
	if err != nil {
		return scenario.Compiled{}, err
	}
	return m.native.Snapshot(ctx, instances, snapshot)
}
