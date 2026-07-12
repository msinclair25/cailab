package scenario

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9._:-]{0,127}$`)

var (
	providerTypes     = setOf("aws", "microsoft", "google", "local")
	principalTypes    = setOf("human", "group", "workload", "application", "service", "agent")
	classifications   = setOf("public", "internal", "confidential", "restricted")
	relationshipTypes = setOf("member_of", "synchronized_to", "assigned_to", "federates_as", "can_access", "owns")
	invariantTypes    = setOf("path_exists", "path_absent")
	severities        = setOf("low", "medium", "high", "critical")
)

type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return "scenario validation failed: " + strings.Join(e.Issues, "; ")
}

func Validate(s Scenario) error {
	var issues []string
	requireEqual(&issues, "apiVersion", s.APIVersion, APIVersion)
	requireEqual(&issues, "kind", s.Kind, Kind)
	requireText(&issues, "metadata.name", s.Metadata.Name)
	checkID(&issues, "metadata.name", s.Metadata.Name)
	requireText(&issues, "metadata.version", s.Metadata.Version)
	requireText(&issues, "metadata.title", s.Metadata.Title)
	requireText(&issues, "spec.briefing", s.Spec.Briefing)

	if len(s.Spec.Objectives) == 0 {
		issues = append(issues, "spec.objectives must contain at least one objective")
	}
	if len(s.Spec.Tenants) == 0 {
		issues = append(issues, "spec.tenants must contain at least one tenant")
	}
	if len(s.Spec.Principals) == 0 {
		issues = append(issues, "spec.principals must contain at least one principal")
	}
	if len(s.Spec.Resources) == 0 {
		issues = append(issues, "spec.resources must contain at least one resource")
	}
	if len(s.Spec.Verification.Invariants) == 0 {
		issues = append(issues, "spec.verification.invariants must contain at least one invariant")
	}

	tenantIDs := make(map[string]struct{})
	for i, tenant := range s.Spec.Tenants {
		prefix := fmt.Sprintf("spec.tenants[%d]", i)
		checkUniqueID(&issues, prefix+".id", tenant.ID, tenantIDs)
		requireText(&issues, prefix+".name", tenant.Name)
		if len(tenant.Providers) == 0 {
			issues = append(issues, prefix+".providers must not be empty")
		}
		seenProviders := make(map[string]struct{})
		for _, provider := range tenant.Providers {
			if _, ok := providerTypes[provider]; !ok {
				issues = append(issues, fmt.Sprintf("%s.providers contains unsupported provider %q", prefix, provider))
			}
			if _, exists := seenProviders[provider]; exists {
				issues = append(issues, fmt.Sprintf("%s.providers contains duplicate %q", prefix, provider))
			}
			seenProviders[provider] = struct{}{}
		}
	}

	nodeIDs := make(map[string]struct{}, len(tenantIDs)+len(s.Spec.Principals)+len(s.Spec.Resources))
	for id := range tenantIDs {
		nodeIDs[id] = struct{}{}
	}
	for i, principal := range s.Spec.Principals {
		prefix := fmt.Sprintf("spec.principals[%d]", i)
		checkUniqueID(&issues, prefix+".id", principal.ID, nodeIDs)
		checkReference(&issues, prefix+".tenant", principal.Tenant, tenantIDs)
		if _, ok := principalTypes[principal.Type]; !ok {
			issues = append(issues, fmt.Sprintf("%s.type has unsupported value %q", prefix, principal.Type))
		}
		requireText(&issues, prefix+".displayName", principal.DisplayName)
	}
	for i, resource := range s.Spec.Resources {
		prefix := fmt.Sprintf("spec.resources[%d]", i)
		checkUniqueID(&issues, prefix+".id", resource.ID, nodeIDs)
		checkReference(&issues, prefix+".tenant", resource.Tenant, tenantIDs)
		requireText(&issues, prefix+".type", resource.Type)
		requireText(&issues, prefix+".displayName", resource.DisplayName)
		if _, ok := classifications[resource.Classification]; !ok {
			issues = append(issues, fmt.Sprintf("%s.classification has unsupported value %q", prefix, resource.Classification))
		}
	}

	objectiveIDs := make(map[string]struct{})
	for i, objective := range s.Spec.Objectives {
		prefix := fmt.Sprintf("spec.objectives[%d]", i)
		checkUniqueID(&issues, prefix+".id", objective.ID, objectiveIDs)
		requireText(&issues, prefix+".description", objective.Description)
	}

	relationshipIDs := make(map[string]struct{})
	for i, relationship := range s.Spec.Relationships {
		prefix := fmt.Sprintf("spec.relationships[%d]", i)
		checkUniqueID(&issues, prefix+".id", relationship.ID, relationshipIDs)
		checkReference(&issues, prefix+".from", relationship.From, nodeIDs)
		checkReference(&issues, prefix+".to", relationship.To, nodeIDs)
		if relationship.From == relationship.To && relationship.From != "" {
			issues = append(issues, prefix+" must not be a self-reference")
		}
		if _, ok := relationshipTypes[relationship.Type]; !ok {
			issues = append(issues, fmt.Sprintf("%s.type has unsupported value %q", prefix, relationship.Type))
		}
		seenActions := make(map[string]struct{})
		for _, action := range relationship.Actions {
			if strings.TrimSpace(action) == "" {
				issues = append(issues, prefix+".actions must not contain empty values")
				continue
			}
			if _, exists := seenActions[action]; exists {
				issues = append(issues, fmt.Sprintf("%s.actions contains duplicate %q", prefix, action))
			}
			seenActions[action] = struct{}{}
		}
	}

	invariantIDs := make(map[string]struct{})
	for i, invariant := range s.Spec.Verification.Invariants {
		prefix := fmt.Sprintf("spec.verification.invariants[%d]", i)
		checkUniqueID(&issues, prefix+".id", invariant.ID, invariantIDs)
		if _, ok := invariantTypes[invariant.Type]; !ok {
			issues = append(issues, fmt.Sprintf("%s.type has unsupported value %q", prefix, invariant.Type))
		}
		checkReference(&issues, prefix+".from", invariant.From, nodeIDs)
		checkReference(&issues, prefix+".to", invariant.To, nodeIDs)
		if _, ok := severities[invariant.Severity]; !ok {
			issues = append(issues, fmt.Sprintf("%s.severity has unsupported value %q", prefix, invariant.Severity))
		}
		requireText(&issues, prefix+".description", invariant.Description)
	}

	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return &ValidationError{Issues: issues}
}

func setOf(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func requireEqual(issues *[]string, field, actual, expected string) {
	if actual != expected {
		*issues = append(*issues, fmt.Sprintf("%s must be %q", field, expected))
	}
}

func requireText(issues *[]string, field, value string) {
	if strings.TrimSpace(value) == "" {
		*issues = append(*issues, field+" must not be empty")
	}
}

func checkID(issues *[]string, field, id string) {
	if id != "" && !idPattern.MatchString(id) {
		*issues = append(*issues, fmt.Sprintf("%s %q must match %s", field, id, idPattern.String()))
	}
}

func checkUniqueID(issues *[]string, field, id string, existing map[string]struct{}) {
	checkID(issues, field, id)
	if id == "" {
		*issues = append(*issues, field+" must not be empty")
		return
	}
	if _, ok := existing[id]; ok {
		*issues = append(*issues, fmt.Sprintf("%s %q is duplicated", field, id))
		return
	}
	existing[id] = struct{}{}
}

func checkReference(issues *[]string, field, id string, known map[string]struct{}) {
	if id == "" {
		*issues = append(*issues, field+" must not be empty")
		return
	}
	if _, ok := known[id]; !ok {
		*issues = append(*issues, fmt.Sprintf("%s references unknown id %q", field, id))
	}
}
