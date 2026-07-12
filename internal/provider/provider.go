package provider

import (
	"context"

	"github.com/msinclair25/cailab/internal/scenario"
)

// Instance is the persisted handle needed to inspect and clean up one provider runtime.
type Instance struct {
	Provider    string `json:"provider"`
	Engine      string `json:"engine"`
	ContainerID string `json:"containerId"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Image       string `json:"image"`
	Status      string `json:"status"`
}

// Manager owns the external provider-runtime lifecycle for a scenario run.
type Manager interface {
	Start(context.Context, string, scenario.Compiled) ([]Instance, error)
	Stop(context.Context, string, []Instance) error
	Snapshot(context.Context, []Instance, scenario.Compiled) (scenario.Compiled, error)
}
