package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return "agent protocol validation failed: " + strings.Join(e.Issues, "; ")
}

func ValidateToolManifest(manifest ToolManifest) error {
	var issues []string
	requireEqual(&issues, "apiVersion", manifest.APIVersion, APIVersion)
	requireEqual(&issues, "kind", manifest.Kind, ToolManifestKind)
	validateID(&issues, "metadata.name", manifest.Metadata.Name)
	validateVersion(&issues, "metadata.version", manifest.Metadata.Version)
	requireText(&issues, "metadata.description", manifest.Metadata.Description)
	if manifest.Spec.Transport.Type != "subprocess" {
		issues = append(issues, fmt.Sprintf("spec.transport.type has unsupported value %q", manifest.Spec.Transport.Type))
	}
	if len(manifest.Spec.Transport.Command) == 0 {
		issues = append(issues, "spec.transport.command must contain an executable")
	}
	for i, value := range manifest.Spec.Transport.Command {
		requireSafeText(&issues, fmt.Sprintf("spec.transport.command[%d]", i), value)
	}
	validateToolInputSchema(&issues, manifest.Spec.InputSchema)
	if len(manifest.Spec.Permissions) == 0 {
		issues = append(issues, "spec.permissions must contain at least one permission")
	}
	for i, permission := range manifest.Spec.Permissions {
		prefix := fmt.Sprintf("spec.permissions[%d]", i)
		validateID(&issues, prefix+".tenant", permission.Tenant)
		validateUniqueText(&issues, prefix+".actions", permission.Actions)
		validateUniqueIDs(&issues, prefix+".resources", permission.Resources)
	}
	if !contains([]string{"low", "medium", "high", "critical"}, manifest.Spec.Risk) {
		issues = append(issues, fmt.Sprintf("spec.risk has unsupported value %q", manifest.Spec.Risk))
	}
	if manifest.Spec.TimeoutMillis < 100 || manifest.Spec.TimeoutMillis > 300_000 {
		issues = append(issues, "spec.timeoutMillis must be between 100 and 300000")
	}
	if !contains([]string{"none", "loopback", "host"}, manifest.Spec.Isolation.Network) {
		issues = append(issues, fmt.Sprintf("spec.isolation.network has unsupported value %q", manifest.Spec.Isolation.Network))
	}
	if !contains([]string{"none", "read_only", "workspace_write", "host"}, manifest.Spec.Isolation.Filesystem) {
		issues = append(issues, fmt.Sprintf("spec.isolation.filesystem has unsupported value %q", manifest.Spec.Isolation.Filesystem))
	}
	seenPointers := make(map[string]struct{})
	for i, pointer := range manifest.Spec.SensitiveFields {
		field := fmt.Sprintf("spec.sensitiveFields[%d]", i)
		if pointer == "" || !validJSONPointer(pointer) {
			issues = append(issues, fmt.Sprintf("%s must be a non-root RFC 6901 JSON Pointer", field))
		}
		if _, exists := seenPointers[pointer]; exists {
			issues = append(issues, fmt.Sprintf("%s duplicates %q", field, pointer))
		}
		seenPointers[pointer] = struct{}{}
	}
	return validationResult(issues)
}

func ValidateAgentRun(run AgentRun) error {
	var issues []string
	requireEqual(&issues, "apiVersion", run.APIVersion, APIVersion)
	requireEqual(&issues, "kind", run.Kind, AgentRunKind)
	validateID(&issues, "runId", run.RunID)
	validateID(&issues, "trialId", run.TrialID)
	validateID(&issues, "scenario.name", run.Scenario.Name)
	validateVersion(&issues, "scenario.version", run.Scenario.Version)
	validateDigest(&issues, "scenario.digest", run.Scenario.Digest)
	validateID(&issues, "agent.id", run.Agent.ID)
	validateVersion(&issues, "agent.version", run.Agent.Version)
	if run.Agent.Adapter != "subprocess" {
		issues = append(issues, fmt.Sprintf("agent.adapter has unsupported value %q", run.Agent.Adapter))
	}
	requireText(&issues, "agent.provider", run.Agent.Provider)
	requireText(&issues, "agent.model", run.Agent.Model)
	validateVersion(&issues, "policy.version", run.Policy.Version)
	validateDigest(&issues, "policy.digest", run.Policy.Digest)
	validateDigest(&issues, "promptHash", run.PromptHash)
	if len(run.Tools) == 0 {
		issues = append(issues, "tools must contain at least one tool reference")
	}
	seenTools := make(map[string]struct{})
	for i, tool := range run.Tools {
		validateToolRef(&issues, fmt.Sprintf("tools[%d]", i), tool)
		if _, exists := seenTools[tool.Name]; exists {
			issues = append(issues, fmt.Sprintf("tools[%d].name duplicates %q", i, tool.Name))
		}
		seenTools[tool.Name] = struct{}{}
	}
	if run.Trial.Index < 1 || run.Trial.Count < 1 || run.Trial.Index > run.Trial.Count {
		issues = append(issues, "trial index and count must satisfy 1 <= index <= count")
	}
	terminal := contains([]string{"completed", "failed", "canceled"}, run.Status)
	if !terminal && !contains([]string{"planned", "running"}, run.Status) {
		issues = append(issues, fmt.Sprintf("status has unsupported value %q", run.Status))
	}
	validateTimestamp(&issues, "startedAt", run.StartedAt)
	if terminal && run.EndedAt == nil {
		issues = append(issues, "endedAt is required for a terminal run")
	}
	if !terminal && run.EndedAt != nil {
		issues = append(issues, "endedAt must be absent for a non-terminal run")
	}
	if run.EndedAt != nil {
		validateTimestamp(&issues, "endedAt", *run.EndedAt)
		if run.EndedAt.Before(run.StartedAt) {
			issues = append(issues, "endedAt must not precede startedAt")
		}
	}
	return validationResult(issues)
}

func ValidateDecision(decision Decision) error {
	var issues []string
	if !contains([]string{"allow", "deny", "redact", "require_approval"}, decision.Effect) {
		issues = append(issues, fmt.Sprintf("effect has unsupported value %q", decision.Effect))
	}
	validateID(&issues, "reasonCode", decision.ReasonCode)
	validateVersion(&issues, "policyVersion", decision.PolicyVersion)
	switch decision.Effect {
	case "redact":
		if len(decision.Redactions) == 0 {
			issues = append(issues, "redact decision requires at least one redaction")
		}
		seen := make(map[string]struct{}, len(decision.Redactions))
		for i, pointer := range decision.Redactions {
			if pointer == "" || !validJSONPointer(pointer) {
				issues = append(issues, fmt.Sprintf("redactions[%d] must be a non-root RFC 6901 JSON Pointer", i))
			}
			if _, exists := seen[pointer]; exists {
				issues = append(issues, fmt.Sprintf("redactions[%d] duplicates %q", i, pointer))
			}
			seen[pointer] = struct{}{}
		}
		if decision.ApprovalID != "" {
			issues = append(issues, "redact decision must not set approvalId")
		}
	case "require_approval":
		validateID(&issues, "approvalId", decision.ApprovalID)
		if len(decision.Redactions) > 0 {
			issues = append(issues, "require_approval decision must not set redactions")
		}
	default:
		if len(decision.Redactions) > 0 || decision.ApprovalID != "" {
			issues = append(issues, "allow and deny decisions must not set redactions or approvalId")
		}
	}
	return validationResult(issues)
}

func ValidateDecisionEvent(event DecisionEvent) error {
	var issues []string
	requireEqual(&issues, "apiVersion", event.APIVersion, APIVersion)
	requireEqual(&issues, "kind", event.Kind, DecisionEventKind)
	validateID(&issues, "eventId", event.EventID)
	if event.Sequence == 0 {
		issues = append(issues, "sequence must be greater than zero")
	}
	validateTimestamp(&issues, "occurredAt", event.OccurredAt)
	validateID(&issues, "runId", event.RunID)
	validateID(&issues, "trialId", event.TrialID)
	validateID(&issues, "correlationId", event.CorrelationID)
	validateID(&issues, "actor.id", event.Actor.ID)
	validateID(&issues, "actor.tenant", event.Actor.Tenant)
	if event.Actor.Type != "agent" {
		issues = append(issues, fmt.Sprintf("actor.type has unsupported value %q", event.Actor.Type))
	}
	validateToolRef(&issues, "tool", event.Tool)
	requireText(&issues, "action", event.Action)
	validateID(&issues, "resource.id", event.Resource.ID)
	validateID(&issues, "resource.tenant", event.Resource.Tenant)
	if !contains([]string{"public", "internal", "confidential", "restricted"}, event.Resource.Classification) {
		issues = append(issues, fmt.Sprintf("resource.classification has unsupported value %q", event.Resource.Classification))
	}
	if err := ValidateDecision(event.Decision); err != nil {
		appendNestedIssues(&issues, "decision", err)
	}
	if !contains([]string{"not_executed", "succeeded", "failed"}, event.Outcome.Status) {
		issues = append(issues, fmt.Sprintf("outcome.status has unsupported value %q", event.Outcome.Status))
	}
	if event.Outcome.Status == "failed" {
		validateID(&issues, "outcome.errorCode", event.Outcome.ErrorCode)
	} else if event.Outcome.ErrorCode != "" {
		issues = append(issues, "outcome.errorCode is allowed only for failed outcomes")
	}
	validateDigest(&issues, "inputHash", event.InputHash)
	if event.OutputHash != "" {
		validateDigest(&issues, "outputHash", event.OutputHash)
	}
	if event.Outcome.Status == "succeeded" && event.OutputHash == "" {
		issues = append(issues, "outputHash is required for a succeeded outcome")
	}
	if event.Outcome.Status == "not_executed" && event.OutputHash != "" {
		issues = append(issues, "outputHash must be absent for a not_executed outcome")
	}
	if event.Decision.Effect == "deny" || event.Decision.Effect == "require_approval" {
		if event.Outcome.Status != "not_executed" {
			issues = append(issues, "deny and require_approval decisions must have a not_executed outcome")
		}
	}
	return validationResult(issues)
}

func validateToolInputSchema(issues *[]string, raw json.RawMessage) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		*issues = append(*issues, "spec.inputSchema is invalid: "+err.Error())
		return
	}
	var schema struct {
		Schema               string `json:"$schema"`
		Type                 string `json:"type"`
		AdditionalProperties *bool  `json:"additionalProperties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		*issues = append(*issues, "spec.inputSchema must be a JSON object")
		return
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		*issues = append(*issues, "spec.inputSchema.$schema must select JSON Schema Draft 2020-12")
	}
	if schema.Type != "object" {
		*issues = append(*issues, "spec.inputSchema.type must be object")
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		*issues = append(*issues, "spec.inputSchema.additionalProperties must be false")
	}
	if _, err := compileToolInputSchema(raw); err != nil {
		*issues = append(*issues, "spec.inputSchema does not compile: "+err.Error())
	}
}

func validateToolRef(issues *[]string, prefix string, tool ToolRef) {
	validateID(issues, prefix+".name", tool.Name)
	validateVersion(issues, prefix+".version", tool.Version)
	validateDigest(issues, prefix+".digest", tool.Digest)
}

func validateUniqueText(issues *[]string, field string, values []string) {
	if len(values) == 0 {
		*issues = append(*issues, field+" must not be empty")
	}
	seen := make(map[string]struct{})
	for i, value := range values {
		requireSafeText(issues, fmt.Sprintf("%s[%d]", field, i), value)
		if _, exists := seen[value]; exists {
			*issues = append(*issues, fmt.Sprintf("%s[%d] duplicates %q", field, i, value))
		}
		seen[value] = struct{}{}
	}
}

func validateUniqueIDs(issues *[]string, field string, values []string) {
	if len(values) == 0 {
		*issues = append(*issues, field+" must not be empty")
	}
	seen := make(map[string]struct{})
	for i, value := range values {
		validateID(issues, fmt.Sprintf("%s[%d]", field, i), value)
		if _, exists := seen[value]; exists {
			*issues = append(*issues, fmt.Sprintf("%s[%d] duplicates %q", field, i, value))
		}
		seen[value] = struct{}{}
	}
}

func validateID(issues *[]string, field, value string) {
	if len(value) == 0 || len(value) > 128 || value[0] < 'a' || value[0] > 'z' {
		*issues = append(*issues, fmt.Sprintf("%s has invalid identifier %q", field, value))
		return
	}
	for _, character := range value[1:] {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || strings.ContainsRune("._:-", character) {
			continue
		}
		*issues = append(*issues, fmt.Sprintf("%s has invalid identifier %q", field, value))
		return
	}
}

func validateVersion(issues *[]string, field, value string) {
	if !versionPattern.MatchString(value) {
		*issues = append(*issues, fmt.Sprintf("%s has invalid semantic version %q", field, value))
	}
}

func validateDigest(issues *[]string, field, value string) {
	if !digestPattern.MatchString(value) {
		*issues = append(*issues, fmt.Sprintf("%s must be a lowercase SHA-256 digest", field))
	}
}

func validateTimestamp(issues *[]string, field string, value time.Time) {
	if value.IsZero() {
		*issues = append(*issues, field+" must not be zero")
		return
	}
	_, offset := value.Zone()
	if offset != 0 {
		*issues = append(*issues, field+" must use UTC")
	}
}

func validJSONPointer(pointer string) bool {
	if pointer == "" {
		return true
	}
	if !strings.HasPrefix(pointer, "/") {
		return false
	}
	for i := 0; i < len(pointer); i++ {
		if pointer[i] != '~' {
			continue
		}
		if i+1 >= len(pointer) || (pointer[i+1] != '0' && pointer[i+1] != '1') {
			return false
		}
		i++
	}
	return true
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

func requireSafeText(issues *[]string, field, value string) {
	requireText(issues, field, value)
	if strings.ContainsRune(value, 0) || strings.ContainsAny(value, "\r\n") {
		*issues = append(*issues, field+" must not contain NUL or line breaks")
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validationResult(issues []string) error {
	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return &ValidationError{Issues: issues}
}

func appendNestedIssues(issues *[]string, prefix string, err error) {
	validation, ok := err.(*ValidationError)
	if !ok {
		*issues = append(*issues, prefix+": "+err.Error())
		return
	}
	for _, issue := range validation.Issues {
		*issues = append(*issues, prefix+"."+issue)
	}
}

func decodeStrict[T any](data []byte, target *T) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected trailing JSON value")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("unexpected trailing JSON value")
	}
	return nil
}
