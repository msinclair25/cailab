package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInvalidPolicy        = errors.New("invalid governance policy")
	ErrInvalidAuthorization = errors.New("invalid authorization request")
)

type AuthorizationRequest struct {
	Run           AgentRun
	Actor         ActorRef
	Manifest      ToolManifest
	Action        string
	Resource      ResourceRef
	CorrelationID string
	Arguments     json.RawMessage
}

type Evaluation struct {
	Decision  Decision
	Tool      ToolRef
	InputHash string
	Arguments json.RawMessage
}

func ValidateGovernancePolicy(policy GovernancePolicy) error {
	var issues []string
	requireEqual(&issues, "apiVersion", policy.APIVersion, APIVersion)
	requireEqual(&issues, "kind", policy.Kind, GovernancePolicyKind)
	validateVersion(&issues, "version", policy.Version)
	if policy.DefaultEffect != "deny" {
		issues = append(issues, "defaultEffect must be \"deny\"")
	}
	if policy.Rules == nil {
		issues = append(issues, "rules must be an array")
	}
	seen := make(map[string]struct{}, len(policy.Rules))
	for i, rule := range policy.Rules {
		prefix := fmt.Sprintf("rules[%d]", i)
		validateID(&issues, prefix+".id", rule.ID)
		if _, exists := seen[rule.ID]; exists {
			issues = append(issues, fmt.Sprintf("%s.id duplicates %q", prefix, rule.ID))
		}
		seen[rule.ID] = struct{}{}
		if !contains([]string{"allow", "deny", "redact", "require_approval"}, rule.Effect) {
			issues = append(issues, fmt.Sprintf("%s.effect has unsupported value %q", prefix, rule.Effect))
		}
		validateID(&issues, prefix+".agentId", rule.AgentID)
		validateID(&issues, prefix+".tool", rule.Tool)
		requireSafeText(&issues, prefix+".action", rule.Action)
		validateID(&issues, prefix+".resource", rule.Resource)
		validateID(&issues, prefix+".resourceTenant", rule.ResourceTenant)
		if !contains([]string{"public", "internal", "confidential", "restricted"}, rule.ResourceClassification) {
			issues = append(issues, fmt.Sprintf("%s.resourceClassification has unsupported value %q", prefix, rule.ResourceClassification))
		}
		if rule.Effect == "redact" {
			if len(rule.Redactions) == 0 {
				issues = append(issues, prefix+".redactions must contain at least one pointer for redact")
			}
			seenPointers := make(map[string]struct{}, len(rule.Redactions))
			for j, pointer := range rule.Redactions {
				if pointer == "" || !validJSONPointer(pointer) {
					issues = append(issues, fmt.Sprintf("%s.redactions[%d] must be a non-root RFC 6901 JSON Pointer", prefix, j))
				}
				if _, exists := seenPointers[pointer]; exists {
					issues = append(issues, fmt.Sprintf("%s.redactions[%d] duplicates %q", prefix, j, pointer))
				}
				seenPointers[pointer] = struct{}{}
			}
		} else if len(rule.Redactions) > 0 {
			issues = append(issues, prefix+".redactions are allowed only for redact")
		}
	}
	if err := validationResult(issues); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPolicy, err)
	}
	return nil
}

func DigestGovernancePolicy(policy GovernancePolicy) (string, error) {
	if err := ValidateGovernancePolicy(policy); err != nil {
		return "", err
	}
	data, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("encode governance policy: %w", err)
	}
	return DigestJSON(data)
}

func ValidateAuthorizationRequest(request AuthorizationRequest) error {
	var issues []string
	if err := ValidateAgentRun(request.Run); err != nil {
		appendNestedIssues(&issues, "run", err)
	}
	if request.Run.Status != "planned" && request.Run.Status != "running" {
		issues = append(issues, "run.status must be planned or running")
	}
	if err := ValidateToolManifest(request.Manifest); err != nil {
		appendNestedIssues(&issues, "manifest", err)
	}
	validateID(&issues, "actor.id", request.Actor.ID)
	validateID(&issues, "actor.tenant", request.Actor.Tenant)
	if request.Actor.Type != "agent" {
		issues = append(issues, fmt.Sprintf("actor.type has unsupported value %q", request.Actor.Type))
	}
	if request.Actor.ID != request.Run.Agent.ID {
		issues = append(issues, "actor.id must match run.agent.id")
	}
	requireSafeText(&issues, "action", request.Action)
	validateID(&issues, "resource.id", request.Resource.ID)
	validateID(&issues, "resource.tenant", request.Resource.Tenant)
	if !contains([]string{"public", "internal", "confidential", "restricted"}, request.Resource.Classification) {
		issues = append(issues, fmt.Sprintf("resource.classification has unsupported value %q", request.Resource.Classification))
	}
	validateID(&issues, "correlationId", request.CorrelationID)
	validateJSONObject(&issues, "arguments", request.Arguments)
	if err := validationResult(issues); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAuthorization, err)
	}
	return nil
}

// EvaluatePolicy makes an order-independent, fail-closed decision. Manifest
// permissions are a mandatory ceiling and policy rules cannot expand them.
func EvaluatePolicy(policy GovernancePolicy, request AuthorizationRequest) (Evaluation, error) {
	if err := ValidateGovernancePolicy(policy); err != nil {
		return Evaluation{}, err
	}
	if err := ValidateAuthorizationRequest(request); err != nil {
		return Evaluation{}, err
	}
	policyDigest, err := DigestGovernancePolicy(policy)
	if err != nil {
		return Evaluation{}, err
	}
	if request.Run.Policy.Version != policy.Version || request.Run.Policy.Digest != policyDigest {
		return Evaluation{}, fmt.Errorf("%w: run policy reference does not match evaluated policy", ErrInvalidAuthorization)
	}
	manifestDigest, err := DigestToolManifest(request.Manifest)
	if err != nil {
		return Evaluation{}, err
	}
	tool, ok := findRunTool(request.Run.Tools, request.Manifest.Metadata.Name)
	if !ok || tool.Version != request.Manifest.Metadata.Version || tool.Digest != manifestDigest {
		return Evaluation{}, fmt.Errorf("%w: run tool reference does not match manifest", ErrInvalidAuthorization)
	}
	canonicalArguments, err := CanonicalJSON(request.Arguments)
	if err != nil {
		return Evaluation{}, fmt.Errorf("%w: canonicalize arguments: %v", ErrInvalidAuthorization, err)
	}
	inputHash, err := DigestJSON(canonicalArguments)
	if err != nil {
		return Evaluation{}, fmt.Errorf("hash authorization input: %w", err)
	}
	evaluation := Evaluation{Tool: tool, InputHash: inputHash}
	if err := ValidateToolInput(request.Manifest.Spec.InputSchema, canonicalArguments); err != nil {
		if errors.Is(err, ErrToolInput) {
			evaluation.Decision = Decision{Effect: "deny", ReasonCode: "schema:invalid_input", PolicyVersion: policy.Version}
			return evaluation, nil
		}
		return Evaluation{}, err
	}
	if !manifestAllows(request.Manifest, request.Action, request.Resource) {
		evaluation.Decision = Decision{Effect: "deny", ReasonCode: "manifest:permission_denied", PolicyVersion: policy.Version}
		return evaluation, nil
	}

	matches := make([]PolicyRule, 0, len(policy.Rules))
	for _, rule := range policy.Rules {
		if policyRuleMatches(rule, request) {
			matches = append(matches, rule)
		}
	}
	if len(matches) == 0 {
		evaluation.Decision = Decision{Effect: "deny", ReasonCode: "policy:default_deny", PolicyVersion: policy.Version}
		return evaluation, nil
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	selectedEffect := highestPrecedenceEffect(matches)
	selected := matchingEffect(matches, selectedEffect)
	decision := Decision{Effect: selectedEffect, ReasonCode: selected[0].ID, PolicyVersion: policy.Version}
	switch selectedEffect {
	case "allow":
		evaluation.Arguments = canonicalArguments
	case "redact":
		pointers := uniqueSortedPointers(selected)
		redacted, err := RedactJSON(canonicalArguments, pointers)
		if err != nil {
			evaluation.Decision = Decision{Effect: "deny", ReasonCode: "policy:redaction_failed", PolicyVersion: policy.Version}
			return evaluation, nil
		}
		decision.Redactions = pointers
		evaluation.Arguments = redacted
	case "require_approval":
		parts := []string{request.Run.RunID, request.Run.TrialID, request.CorrelationID}
		for _, rule := range selected {
			parts = append(parts, rule.ID)
		}
		decision.ApprovalID = deterministicIdentifier("approval", parts...)
	}
	evaluation.Decision = decision
	return evaluation, nil
}

// ReevaluateApprovedPolicy verifies that the current request still produces the
// expected approval requirement, then applies an explicit resolution without
// allowing it to override deny or redact rules.
func ReevaluateApprovedPolicy(policy GovernancePolicy, request AuthorizationRequest, approvalID string, approved bool) (Evaluation, error) {
	initial, err := EvaluatePolicy(policy, request)
	if err != nil {
		return Evaluation{}, err
	}
	if initial.Decision.Effect != "require_approval" || initial.Decision.ApprovalID != approvalID {
		return Evaluation{}, fmt.Errorf("%w: approval no longer matches current policy evaluation", ErrInvalidAuthorization)
	}
	if !approved {
		initial.Decision = Decision{Effect: "deny", ReasonCode: "approval:rejected", PolicyVersion: policy.Version}
		return initial, nil
	}

	canonicalArguments, err := CanonicalJSON(request.Arguments)
	if err != nil {
		return Evaluation{}, fmt.Errorf("%w: canonicalize approved arguments: %v", ErrInvalidAuthorization, err)
	}
	matches := make([]PolicyRule, 0, len(policy.Rules))
	for _, rule := range policy.Rules {
		if policyRuleMatches(rule, request) && rule.Effect != "require_approval" {
			matches = append(matches, rule)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	decision := Decision{Effect: "allow", ReasonCode: initial.Decision.ReasonCode, PolicyVersion: policy.Version}
	arguments := canonicalArguments
	if len(matches) > 0 {
		selectedEffect := highestPrecedenceEffect(matches)
		selected := matchingEffect(matches, selectedEffect)
		decision = Decision{Effect: selectedEffect, ReasonCode: selected[0].ID, PolicyVersion: policy.Version}
		switch selectedEffect {
		case "deny":
			arguments = nil
		case "redact":
			pointers := uniqueSortedPointers(selected)
			redacted, err := RedactJSON(canonicalArguments, pointers)
			if err != nil {
				decision = Decision{Effect: "deny", ReasonCode: "policy:redaction_failed", PolicyVersion: policy.Version}
				arguments = nil
			} else {
				decision.Redactions = pointers
				arguments = redacted
			}
		}
	}
	initial.Decision = decision
	initial.Arguments = arguments
	return initial, nil
}

func findRunTool(tools []ToolRef, name string) (ToolRef, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return ToolRef{}, false
}

func manifestAllows(manifest ToolManifest, action string, resource ResourceRef) bool {
	for _, permission := range manifest.Spec.Permissions {
		if permission.Tenant == resource.Tenant && contains(permission.Actions, action) && contains(permission.Resources, resource.ID) {
			return true
		}
	}
	return false
}

func policyRuleMatches(rule PolicyRule, request AuthorizationRequest) bool {
	return rule.AgentID == request.Actor.ID &&
		rule.Tool == request.Manifest.Metadata.Name &&
		rule.Action == request.Action &&
		rule.Resource == request.Resource.ID &&
		rule.ResourceTenant == request.Resource.Tenant &&
		rule.ResourceClassification == request.Resource.Classification
}

func highestPrecedenceEffect(rules []PolicyRule) string {
	precedence := map[string]int{"allow": 0, "redact": 1, "require_approval": 2, "deny": 3}
	effect := "allow"
	for _, rule := range rules {
		if precedence[rule.Effect] > precedence[effect] {
			effect = rule.Effect
		}
	}
	return effect
}

func matchingEffect(rules []PolicyRule, effect string) []PolicyRule {
	selected := make([]PolicyRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Effect == effect {
			selected = append(selected, rule)
		}
	}
	return selected
}

func uniqueSortedPointers(rules []PolicyRule) []string {
	seen := make(map[string]struct{})
	for _, rule := range rules {
		for _, pointer := range rule.Redactions {
			seen[pointer] = struct{}{}
		}
	}
	pointers := make([]string, 0, len(seen))
	for pointer := range seen {
		pointers = append(pointers, pointer)
	}
	sort.Strings(pointers)
	return pointers
}

func deterministicIdentifier(prefix string, parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return prefix + ":" + hex.EncodeToString(hash.Sum(nil))[:24]
}
