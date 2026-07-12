package scenario

const (
	APIVersion = "cloudailab.dev/v1alpha1"
	Kind       = "Scenario"
)

// Scenario is the author-facing, versioned description of a CloudAILab lab.
type Scenario struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Spec       Spec     `json:"spec" yaml:"spec"`
}

type Metadata struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Title   string `json:"title" yaml:"title"`
}

type Spec struct {
	Seed          int64          `json:"seed" yaml:"seed"`
	Briefing      string         `json:"briefing" yaml:"briefing"`
	Objectives    []Objective    `json:"objectives" yaml:"objectives"`
	Tenants       []Tenant       `json:"tenants" yaml:"tenants"`
	Principals    []Principal    `json:"principals" yaml:"principals"`
	Resources     []Resource     `json:"resources" yaml:"resources"`
	Relationships []Relationship `json:"relationships" yaml:"relationships"`
	Verification  Verification   `json:"verification" yaml:"verification"`
}

type Objective struct {
	ID          string `json:"id" yaml:"id"`
	Description string `json:"description" yaml:"description"`
}

type Tenant struct {
	ID        string   `json:"id" yaml:"id"`
	Name      string   `json:"name" yaml:"name"`
	Providers []string `json:"providers" yaml:"providers"`
}

type Principal struct {
	ID          string `json:"id" yaml:"id"`
	Tenant      string `json:"tenant" yaml:"tenant"`
	Type        string `json:"type" yaml:"type"`
	DisplayName string `json:"displayName" yaml:"displayName"`
}

type Resource struct {
	ID             string `json:"id" yaml:"id"`
	Tenant         string `json:"tenant" yaml:"tenant"`
	Type           string `json:"type" yaml:"type"`
	DisplayName    string `json:"displayName" yaml:"displayName"`
	Classification string `json:"classification" yaml:"classification"`
}

type Relationship struct {
	ID      string   `json:"id" yaml:"id"`
	From    string   `json:"from" yaml:"from"`
	To      string   `json:"to" yaml:"to"`
	Type    string   `json:"type" yaml:"type"`
	Actions []string `json:"actions,omitempty" yaml:"actions,omitempty"`
}

type Verification struct {
	Invariants []Invariant `json:"invariants" yaml:"invariants"`
}

type Invariant struct {
	ID          string `json:"id" yaml:"id"`
	Type        string `json:"type" yaml:"type"`
	From        string `json:"from" yaml:"from"`
	To          string `json:"to" yaml:"to"`
	Severity    string `json:"severity" yaml:"severity"`
	Description string `json:"description" yaml:"description"`
}

// Compiled is the normalized, deterministic representation persisted for a run.
type Compiled struct {
	SchemaVersion   string         `json:"schemaVersion"`
	ScenarioName    string         `json:"scenarioName"`
	ScenarioVersion string         `json:"scenarioVersion"`
	Title           string         `json:"title"`
	Seed            int64          `json:"seed"`
	Briefing        string         `json:"briefing"`
	Objectives      []Objective    `json:"objectives"`
	Nodes           []Node         `json:"nodes"`
	Edges           []Relationship `json:"edges"`
	Invariants      []Invariant    `json:"invariants"`
	Digest          string         `json:"digest"`
}

type Node struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Tenant         string `json:"tenant,omitempty"`
	Type           string `json:"type"`
	DisplayName    string `json:"displayName"`
	Classification string `json:"classification,omitempty"`
}
