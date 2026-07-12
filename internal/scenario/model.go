package scenario

const (
	APIVersion = "cloudailab.dev/v1alpha1"
	Kind       = "Scenario"
	FlociImage = "floci/floci:1.5.32@sha256:4f69631e560120d79ad82d2af9f7dda8c6ef7ecbbae0c43ddcffa109c6588a15"
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
	Runtimes      Runtimes       `json:"runtimes,omitempty" yaml:"runtimes,omitempty"`
	Providers     Providers      `json:"providers,omitempty" yaml:"providers,omitempty"`
	Objectives    []Objective    `json:"objectives" yaml:"objectives"`
	Tenants       []Tenant       `json:"tenants" yaml:"tenants"`
	Principals    []Principal    `json:"principals" yaml:"principals"`
	Resources     []Resource     `json:"resources" yaml:"resources"`
	Relationships []Relationship `json:"relationships" yaml:"relationships"`
	Verification  Verification   `json:"verification" yaml:"verification"`
}

type Runtimes struct {
	AWS       *AWSRuntime       `json:"aws,omitempty" yaml:"aws,omitempty"`
	Microsoft *MicrosoftRuntime `json:"microsoft,omitempty" yaml:"microsoft,omitempty"`
}

type AWSRuntime struct {
	Engine         string `json:"engine" yaml:"engine"`
	Image          string `json:"image" yaml:"image"`
	IAMEnforcement bool   `json:"iamEnforcement" yaml:"iamEnforcement"`
}

type MicrosoftRuntime struct {
	Engine string `json:"engine" yaml:"engine"`
}

type Providers struct {
	AWS       *AWSProvider       `json:"aws,omitempty" yaml:"aws,omitempty"`
	Microsoft *MicrosoftProvider `json:"microsoft,omitempty" yaml:"microsoft,omitempty"`
}

type MicrosoftProvider struct {
	Tenant                 string                      `json:"tenant" yaml:"tenant"`
	TenantID               string                      `json:"tenantId" yaml:"tenantId"`
	Users                  []MicrosoftUser             `json:"users" yaml:"users"`
	Applications           []MicrosoftApplication      `json:"applications" yaml:"applications"`
	ServicePrincipals      []MicrosoftServicePrincipal `json:"servicePrincipals" yaml:"servicePrincipals"`
	OAuth2PermissionGrants []MicrosoftPermissionGrant  `json:"oauth2PermissionGrants" yaml:"oauth2PermissionGrants"`
}

type MicrosoftUser struct {
	Node              string `json:"node" yaml:"node"`
	ID                string `json:"id" yaml:"id"`
	DisplayName       string `json:"displayName" yaml:"displayName"`
	UserPrincipalName string `json:"userPrincipalName" yaml:"userPrincipalName"`
}

type MicrosoftApplication struct {
	Node        string `json:"node" yaml:"node"`
	ID          string `json:"id" yaml:"id"`
	AppID       string `json:"appId" yaml:"appId"`
	DisplayName string `json:"displayName" yaml:"displayName"`
}

type MicrosoftServicePrincipal struct {
	Node         string `json:"node,omitempty" yaml:"node,omitempty"`
	ResourceNode string `json:"resourceNode,omitempty" yaml:"resourceNode,omitempty"`
	ID           string `json:"id" yaml:"id"`
	AppID        string `json:"appId" yaml:"appId"`
	DisplayName  string `json:"displayName" yaml:"displayName"`
}

type MicrosoftPermissionGrant struct {
	ID          string `json:"id" yaml:"id"`
	ClientID    string `json:"clientId" yaml:"clientId"`
	ConsentType string `json:"consentType" yaml:"consentType"`
	PrincipalID string `json:"principalId,omitempty" yaml:"principalId,omitempty"`
	ResourceID  string `json:"resourceId" yaml:"resourceId"`
	Scope       string `json:"scope" yaml:"scope"`
}

type AWSProvider struct {
	Region   string       `json:"region" yaml:"region"`
	Accounts []AWSAccount `json:"accounts" yaml:"accounts"`
	Roles    []AWSRole    `json:"roles" yaml:"roles"`
	Buckets  []AWSBucket  `json:"buckets" yaml:"buckets"`
}

type AWSAccount struct {
	ID        string `json:"id" yaml:"id"`
	Tenant    string `json:"tenant" yaml:"tenant"`
	Principal string `json:"principal" yaml:"principal"`
}

type AWSRole struct {
	Node     string            `json:"node" yaml:"node"`
	Account  string            `json:"account" yaml:"account"`
	Name     string            `json:"name" yaml:"name"`
	Trust    []string          `json:"trust" yaml:"trust"`
	Policies []AWSInlinePolicy `json:"policies,omitempty" yaml:"policies,omitempty"`
}

type AWSInlinePolicy struct {
	Name       string               `json:"name" yaml:"name"`
	Statements []AWSPolicyStatement `json:"statements" yaml:"statements"`
}

type AWSPolicyStatement struct {
	Effect    string   `json:"effect" yaml:"effect"`
	Actions   []string `json:"actions" yaml:"actions"`
	Resources []string `json:"resources" yaml:"resources"`
}

type AWSBucket struct {
	Node    string      `json:"node" yaml:"node"`
	Account string      `json:"account" yaml:"account"`
	Name    string      `json:"name" yaml:"name"`
	Objects []AWSObject `json:"objects,omitempty" yaml:"objects,omitempty"`
}

type AWSObject struct {
	Key  string `json:"key" yaml:"key"`
	Data string `json:"data" yaml:"data"`
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
	Runtimes        Runtimes       `json:"runtimes,omitempty"`
	Providers       Providers      `json:"providers,omitempty"`
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
