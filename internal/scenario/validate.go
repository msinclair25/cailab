package scenario

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9._:-]{0,127}$`)
var awsAccountPattern = regexp.MustCompile(`^[0-9]{12}$`)
var awsRoleNamePattern = regexp.MustCompile(`^[A-Za-z0-9_+=,.@-]{1,64}$`)
var awsBucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
var awsRegionPattern = regexp.MustCompile(`^[a-z]{2}(-gov)?-[a-z]+-[0-9]$`)
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var googleObjectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)
var oidcAudiencePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,253}[a-z0-9])?$`)

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
	validateRuntimes(&issues, s.Spec.Runtimes)

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
	principalIDs := make(map[string]struct{}, len(s.Spec.Principals))
	resourceIDs := make(map[string]struct{}, len(s.Spec.Resources))
	principalTenants := make(map[string]string, len(s.Spec.Principals))
	principalKinds := make(map[string]string, len(s.Spec.Principals))
	resourceTenants := make(map[string]string, len(s.Spec.Resources))
	for id := range tenantIDs {
		nodeIDs[id] = struct{}{}
	}
	for i, principal := range s.Spec.Principals {
		prefix := fmt.Sprintf("spec.principals[%d]", i)
		checkUniqueID(&issues, prefix+".id", principal.ID, nodeIDs)
		principalIDs[principal.ID] = struct{}{}
		principalTenants[principal.ID] = principal.Tenant
		principalKinds[principal.ID] = principal.Type
		checkReference(&issues, prefix+".tenant", principal.Tenant, tenantIDs)
		if _, ok := principalTypes[principal.Type]; !ok {
			issues = append(issues, fmt.Sprintf("%s.type has unsupported value %q", prefix, principal.Type))
		}
		requireText(&issues, prefix+".displayName", principal.DisplayName)
	}
	for i, resource := range s.Spec.Resources {
		prefix := fmt.Sprintf("spec.resources[%d]", i)
		checkUniqueID(&issues, prefix+".id", resource.ID, nodeIDs)
		resourceIDs[resource.ID] = struct{}{}
		resourceTenants[resource.ID] = resource.Tenant
		checkReference(&issues, prefix+".tenant", resource.Tenant, tenantIDs)
		requireText(&issues, prefix+".type", resource.Type)
		requireText(&issues, prefix+".displayName", resource.DisplayName)
		if _, ok := classifications[resource.Classification]; !ok {
			issues = append(issues, fmt.Sprintf("%s.classification has unsupported value %q", prefix, resource.Classification))
		}
	}
	validateProviders(&issues, s.Spec.Providers, s.Spec.Runtimes, tenantIDs, principalIDs, resourceIDs, principalTenants, resourceTenants, principalKinds)
	validateEvaluation(&issues, s.Spec.Evaluation, resourceIDs)

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

func validateEvaluation(issues *[]string, evaluation Evaluation, resources map[string]struct{}) {
	fixtureIDs := make(map[string]struct{}, len(evaluation.PromptInjections))
	for index, fixture := range evaluation.PromptInjections {
		prefix := fmt.Sprintf("spec.evaluation.promptInjections[%d]", index)
		checkUniqueID(issues, prefix+".id", fixture.ID, fixtureIDs)
		requireText(issues, prefix+".description", fixture.Description)
		checkReference(issues, prefix+".untrustedContentResource", fixture.UntrustedContentResource, resources)
		validateEvaluationAction(issues, prefix+".exposure", fixture.Exposure, resources)
		if fixture.Exposure.Resource != fixture.UntrustedContentResource {
			*issues = append(*issues, prefix+".exposure.resource must match untrustedContentResource")
		}
		if len(fixture.Prohibited) == 0 {
			*issues = append(*issues, prefix+".prohibited must contain at least one action")
		}
		seen := make(map[string]struct{}, len(fixture.Prohibited))
		for actionIndex, action := range fixture.Prohibited {
			actionPrefix := fmt.Sprintf("%s.prohibited[%d]", prefix, actionIndex)
			validateEvaluationAction(issues, actionPrefix, action, resources)
			key := action.Tool + "\x00" + action.Action + "\x00" + action.Resource
			if _, exists := seen[key]; exists {
				*issues = append(*issues, actionPrefix+" duplicates an earlier prohibited action")
			}
			seen[key] = struct{}{}
			if action == fixture.Exposure {
				*issues = append(*issues, actionPrefix+" must differ from the exposure action")
			}
		}
	}
}

func validateEvaluationAction(issues *[]string, prefix string, action EvaluationAction, resources map[string]struct{}) {
	checkID(issues, prefix+".tool", action.Tool)
	requireText(issues, prefix+".action", action.Action)
	if strings.ContainsAny(action.Action, "\r\n\x00") {
		*issues = append(*issues, prefix+".action must not contain control line breaks or NUL")
	}
	checkReference(issues, prefix+".resource", action.Resource, resources)
}

func validateRuntimes(issues *[]string, runtimes Runtimes) {
	if runtimes.AWS != nil {
		if runtimes.AWS.Engine != "floci" {
			*issues = append(*issues, fmt.Sprintf("spec.runtimes.aws.engine has unsupported value %q", runtimes.AWS.Engine))
		}
		if runtimes.AWS.Image != FlociImage {
			*issues = append(*issues, fmt.Sprintf("spec.runtimes.aws.image must be the supported pinned image %q", FlociImage))
		}
	}
	if runtimes.Microsoft != nil && runtimes.Microsoft.Engine != "native" {
		*issues = append(*issues, fmt.Sprintf("spec.runtimes.microsoft.engine has unsupported value %q", runtimes.Microsoft.Engine))
	}
	if runtimes.Google != nil && runtimes.Google.Engine != "native" {
		*issues = append(*issues, fmt.Sprintf("spec.runtimes.google.engine has unsupported value %q", runtimes.Google.Engine))
	}
	if runtimes.OIDC != nil && runtimes.OIDC.Engine != "native" {
		*issues = append(*issues, fmt.Sprintf("spec.runtimes.oidc.engine has unsupported value %q", runtimes.OIDC.Engine))
	}
}

func validateProviders(
	issues *[]string,
	providers Providers,
	runtimes Runtimes,
	tenants, principals, resources map[string]struct{},
	principalTenants, resourceTenants, principalKinds map[string]string,
) {
	validateMicrosoftProvider(issues, providers.Microsoft, runtimes.Microsoft, tenants, principals, resources, principalTenants, resourceTenants)
	validateGoogleProvider(issues, providers.Google, runtimes.Google, tenants, principals, resources, principalTenants, resourceTenants)
	validateOIDCProvider(issues, providers.OIDC, runtimes.OIDC, tenants, principals, resources, principalTenants, resourceTenants, principalKinds)
	if providers.AWS == nil {
		return
	}
	if runtimes.AWS == nil {
		*issues = append(*issues, "spec.providers.aws requires spec.runtimes.aws")
	} else if !runtimes.AWS.IAMEnforcement {
		*issues = append(*issues, "spec.providers.aws requires IAM enforcement")
	}
	awsProvider := providers.AWS
	if !awsRegionPattern.MatchString(awsProvider.Region) {
		*issues = append(*issues, fmt.Sprintf("spec.providers.aws.region has invalid value %q", awsProvider.Region))
	}
	if len(awsProvider.Accounts) == 0 {
		*issues = append(*issues, "spec.providers.aws.accounts must contain at least one account")
	}
	accounts := make(map[string]AWSAccount)
	accountPrincipals := make(map[string]AWSAccount)
	for i, account := range awsProvider.Accounts {
		prefix := fmt.Sprintf("spec.providers.aws.accounts[%d]", i)
		if !awsAccountPattern.MatchString(account.ID) {
			*issues = append(*issues, fmt.Sprintf("%s.id must be a 12-digit account ID", prefix))
		}
		if _, exists := accounts[account.ID]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.id %q is duplicated", prefix, account.ID))
		}
		accounts[account.ID] = account
		checkReference(issues, prefix+".tenant", account.Tenant, tenants)
		checkReference(issues, prefix+".principal", account.Principal, principals)
		if principalTenant, ok := principalTenants[account.Principal]; ok && principalTenant != account.Tenant {
			*issues = append(*issues, fmt.Sprintf("%s.principal %q belongs to tenant %q, not %q", prefix, account.Principal, principalTenant, account.Tenant))
		}
		if _, exists := accountPrincipals[account.Principal]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.principal %q is duplicated", prefix, account.Principal))
		}
		accountPrincipals[account.Principal] = account
	}

	roleKeys := make(map[string]struct{})
	for i, role := range awsProvider.Roles {
		prefix := fmt.Sprintf("spec.providers.aws.roles[%d]", i)
		checkReference(issues, prefix+".node", role.Node, principals)
		account, accountExists := accounts[role.Account]
		if !accountExists {
			*issues = append(*issues, fmt.Sprintf("%s.account references unknown AWS account %q", prefix, role.Account))
		} else if nodeTenant, ok := principalTenants[role.Node]; ok && nodeTenant != account.Tenant {
			*issues = append(*issues, fmt.Sprintf("%s.node %q belongs to tenant %q, not AWS account tenant %q", prefix, role.Node, nodeTenant, account.Tenant))
		}
		if !awsRoleNamePattern.MatchString(role.Name) {
			*issues = append(*issues, fmt.Sprintf("%s.name has invalid IAM role name %q", prefix, role.Name))
		}
		roleKey := role.Account + ":" + role.Name
		if _, exists := roleKeys[roleKey]; exists {
			*issues = append(*issues, fmt.Sprintf("%s duplicates role %q in account %q", prefix, role.Name, role.Account))
		}
		roleKeys[roleKey] = struct{}{}
		if len(role.Trust) == 0 {
			*issues = append(*issues, prefix+".trust must contain at least one principal")
		}
		seenTrust := make(map[string]struct{})
		for _, trusted := range role.Trust {
			if _, ok := accountPrincipals[trusted]; !ok {
				*issues = append(*issues, fmt.Sprintf("%s.trust references unsupported principal %q", prefix, trusted))
			}
			if _, exists := seenTrust[trusted]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.trust contains duplicate %q", prefix, trusted))
			}
			seenTrust[trusted] = struct{}{}
		}
		validateAWSPolicies(issues, prefix+".policies", role.Policies)
		if role.WebIdentity != nil {
			webPrefix := prefix + ".webIdentity"
			checkReference(issues, webPrefix+".clientNode", role.WebIdentity.ClientNode, principals)
			checkReference(issues, webPrefix+".audienceNode", role.WebIdentity.AudienceNode, resources)
			if !validOIDCAudienceValue(role.WebIdentity.Audience) {
				*issues = append(*issues, fmt.Sprintf("%s.audience has invalid value %q", webPrefix, role.WebIdentity.Audience))
			}
			if !oidcClientAudienceExists(providers.OIDC, role.WebIdentity.ClientNode, role.WebIdentity.AudienceNode, role.WebIdentity.Audience) {
				*issues = append(*issues, webPrefix+" must match a declared OIDC client and audience")
			}
		}
	}

	bucketNames := make(map[string]struct{})
	for i, bucket := range awsProvider.Buckets {
		prefix := fmt.Sprintf("spec.providers.aws.buckets[%d]", i)
		checkReference(issues, prefix+".node", bucket.Node, resources)
		account, accountExists := accounts[bucket.Account]
		if !accountExists {
			*issues = append(*issues, fmt.Sprintf("%s.account references unknown AWS account %q", prefix, bucket.Account))
		} else if nodeTenant, ok := resourceTenants[bucket.Node]; ok && nodeTenant != account.Tenant {
			*issues = append(*issues, fmt.Sprintf("%s.node %q belongs to tenant %q, not AWS account tenant %q", prefix, bucket.Node, nodeTenant, account.Tenant))
		}
		if !awsBucketNamePattern.MatchString(bucket.Name) {
			*issues = append(*issues, fmt.Sprintf("%s.name has invalid S3 bucket name %q", prefix, bucket.Name))
		}
		if _, exists := bucketNames[bucket.Name]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.name %q is duplicated", prefix, bucket.Name))
		}
		bucketNames[bucket.Name] = struct{}{}
		objectKeys := make(map[string]struct{})
		for j, object := range bucket.Objects {
			objectPrefix := fmt.Sprintf("%s.objects[%d]", prefix, j)
			requireText(issues, objectPrefix+".key", object.Key)
			if _, exists := objectKeys[object.Key]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.key %q is duplicated", objectPrefix, object.Key))
			}
			objectKeys[object.Key] = struct{}{}
		}
	}
}

func validateOIDCProvider(
	issues *[]string,
	provider *OIDCProvider,
	runtime *OIDCRuntime,
	tenants, principals, resources map[string]struct{},
	principalTenants, resourceTenants, principalKinds map[string]string,
) {
	if provider == nil {
		if runtime != nil {
			*issues = append(*issues, "spec.runtimes.oidc requires spec.providers.oidc")
		}
		return
	}
	if runtime == nil {
		*issues = append(*issues, "spec.providers.oidc requires spec.runtimes.oidc")
	}
	checkReference(issues, "spec.providers.oidc.tenant", provider.Tenant, tenants)
	if provider.CodeTTLSeconds < 15 || provider.CodeTTLSeconds > 300 {
		*issues = append(*issues, "spec.providers.oidc.codeTtlSeconds must be from 15 through 300")
	}
	if provider.TokenTTLSeconds < 60 || provider.TokenTTLSeconds > 3600 {
		*issues = append(*issues, "spec.providers.oidc.tokenTtlSeconds must be from 60 through 3600")
	}
	if len(provider.Clients) == 0 {
		*issues = append(*issues, "spec.providers.oidc.clients must contain at least one client")
	}
	if len(provider.Subjects) == 0 {
		*issues = append(*issues, "spec.providers.oidc.subjects must contain at least one subject")
	}

	clientIDs := make(map[string]struct{})
	clientNodes := make(map[string]struct{})
	for i, client := range provider.Clients {
		prefix := fmt.Sprintf("spec.providers.oidc.clients[%d]", i)
		checkReference(issues, prefix+".node", client.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", client.Node, provider.Tenant, principalTenants)
		if kind := principalKinds[client.Node]; kind != "application" && kind != "workload" && kind != "agent" {
			*issues = append(*issues, fmt.Sprintf("%s.node must reference an application, workload, or agent principal", prefix))
		}
		if _, exists := clientNodes[client.Node]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.node %q is duplicated", prefix, client.Node))
		}
		clientNodes[client.Node] = struct{}{}
		checkID(issues, prefix+".clientId", client.ClientID)
		if client.ClientID == "" {
			*issues = append(*issues, prefix+".clientId must not be empty")
		} else if _, exists := clientIDs[client.ClientID]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.clientId %q is duplicated", prefix, client.ClientID))
		}
		clientIDs[client.ClientID] = struct{}{}
		if !strings.HasPrefix(client.ClientSecret, "cailab-synthetic-") || len(client.ClientSecret) < 32 {
			*issues = append(*issues, prefix+".clientSecret must be visibly synthetic and at least 32 characters")
		}
		validateOIDCStringSet(issues, prefix+".redirectUris", client.RedirectURIs, func(value string) bool {
			parsed, err := url.Parse(value)
			return err == nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.Port() != "" && parsed.User == nil && parsed.Fragment == ""
		}, "must contain only IPv4 loopback HTTP redirect URIs with explicit ports and no fragments")
		if len(client.Audiences) != 1 {
			*issues = append(*issues, prefix+".audiences must contain exactly one default audience in this profile")
		}
		audienceValues := make(map[string]struct{})
		audienceNodes := make(map[string]struct{})
		for j, audience := range client.Audiences {
			audiencePrefix := fmt.Sprintf("%s.audiences[%d]", prefix, j)
			checkReference(issues, audiencePrefix+".node", audience.Node, resources)
			validateMicrosoftNodeTenant(issues, audiencePrefix+".node", audience.Node, provider.Tenant, resourceTenants)
			if !validOIDCAudienceValue(audience.Value) {
				*issues = append(*issues, fmt.Sprintf("%s.value must be an absolute HTTPS URI, URN, or DNS-style audience identifier without a fragment: %q", audiencePrefix, audience.Value))
			}
			if _, exists := audienceValues[audience.Value]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.value %q is duplicated", audiencePrefix, audience.Value))
			}
			audienceValues[audience.Value] = struct{}{}
			if _, exists := audienceNodes[audience.Node]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.node %q is duplicated", audiencePrefix, audience.Node))
			}
			audienceNodes[audience.Node] = struct{}{}
		}
		validateOIDCStringSet(issues, prefix+".scopes", client.Scopes, func(value string) bool {
			return value == "openid" || value == "profile" || value == "email"
		}, "contains an unsupported scope")
		if !containsString(client.Scopes, "openid") {
			*issues = append(*issues, prefix+".scopes must include openid")
		}
	}

	subjects := make(map[string]struct{})
	subjectNodes := make(map[string]struct{})
	for i, subject := range provider.Subjects {
		prefix := fmt.Sprintf("spec.providers.oidc.subjects[%d]", i)
		checkReference(issues, prefix+".node", subject.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", subject.Node, provider.Tenant, principalTenants)
		if kind := principalKinds[subject.Node]; kind != "human" && kind != "agent" && kind != "workload" {
			*issues = append(*issues, fmt.Sprintf("%s.node must reference a human, agent, or workload principal", prefix))
		}
		if _, exists := subjectNodes[subject.Node]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.node %q is duplicated", prefix, subject.Node))
		}
		subjectNodes[subject.Node] = struct{}{}
		checkID(issues, prefix+".subject", subject.Subject)
		if subject.Subject == "" {
			*issues = append(*issues, prefix+".subject must not be empty")
		} else if _, exists := subjects[subject.Subject]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.subject %q is duplicated", prefix, subject.Subject))
		}
		subjects[subject.Subject] = struct{}{}
		validateGoogleEmail(issues, prefix+".email", subject.Email)
		seenGroups := make(map[string]struct{})
		for _, group := range subject.Groups {
			checkReference(issues, prefix+".groups[]", group, principals)
			validateMicrosoftNodeTenant(issues, prefix+".groups[]", group, provider.Tenant, principalTenants)
			if principalKinds[group] != "group" {
				*issues = append(*issues, fmt.Sprintf("%s.groups references non-group principal %q", prefix, group))
			}
			if _, exists := seenGroups[group]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.groups contains duplicate %q", prefix, group))
			}
			seenGroups[group] = struct{}{}
		}
	}
}

func validateOIDCStringSet(issues *[]string, field string, values []string, valid func(string) bool, invalidMessage string) {
	if len(values) == 0 {
		*issues = append(*issues, field+" must not be empty")
		return
	}
	seen := make(map[string]struct{})
	for _, value := range values {
		if !valid(value) {
			*issues = append(*issues, fmt.Sprintf("%s %s: %q", field, invalidMessage, value))
		}
		if _, exists := seen[value]; exists {
			*issues = append(*issues, fmt.Sprintf("%s contains duplicate %q", field, value))
		}
		seen[value] = struct{}{}
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validateGoogleProvider(
	issues *[]string,
	provider *GoogleProvider,
	runtime *GoogleRuntime,
	tenants, principals, resources map[string]struct{},
	principalTenants, resourceTenants map[string]string,
) {
	if provider == nil {
		if runtime != nil {
			*issues = append(*issues, "spec.runtimes.google requires spec.providers.google")
		}
		return
	}
	if runtime == nil {
		*issues = append(*issues, "spec.providers.google requires spec.runtimes.google")
	}
	checkReference(issues, "spec.providers.google.tenant", provider.Tenant, tenants)
	validateGoogleID(issues, "spec.providers.google.customerId", provider.CustomerID)
	if len(provider.Users) == 0 {
		*issues = append(*issues, "spec.providers.google.users must contain at least one user")
	}

	objectIDs := make(map[string]string)
	usersByEmail := make(map[string]struct{})
	for i, user := range provider.Users {
		prefix := fmt.Sprintf("spec.providers.google.users[%d]", i)
		validateGoogleObjectID(issues, prefix+".id", user.ID, objectIDs)
		checkReference(issues, prefix+".node", user.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", user.Node, provider.Tenant, principalTenants)
		email := validateGoogleEmail(issues, prefix+".primaryEmail", user.PrimaryEmail)
		if _, exists := usersByEmail[email]; exists && email != "" {
			*issues = append(*issues, fmt.Sprintf("%s.primaryEmail %q is duplicated", prefix, user.PrimaryEmail))
		}
		usersByEmail[email] = struct{}{}
		requireText(issues, prefix+".displayName", user.DisplayName)
	}

	groupsByEmail := make(map[string]struct{})
	for i, group := range provider.Groups {
		prefix := fmt.Sprintf("spec.providers.google.groups[%d]", i)
		validateGoogleObjectID(issues, prefix+".id", group.ID, objectIDs)
		checkReference(issues, prefix+".node", group.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", group.Node, provider.Tenant, principalTenants)
		email := validateGoogleEmail(issues, prefix+".email", group.Email)
		if _, exists := groupsByEmail[email]; exists && email != "" {
			*issues = append(*issues, fmt.Sprintf("%s.email %q is duplicated", prefix, group.Email))
		}
		groupsByEmail[email] = struct{}{}
		requireText(issues, prefix+".name", group.Name)
		memberIDs := make(map[string]struct{})
		memberEmails := make(map[string]struct{})
		for j, member := range group.Members {
			memberPrefix := fmt.Sprintf("%s.members[%d]", prefix, j)
			validateGoogleID(issues, memberPrefix+".id", member.ID)
			if _, exists := memberIDs[member.ID]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.id %q is duplicated in group", memberPrefix, member.ID))
			}
			memberIDs[member.ID] = struct{}{}
			memberEmail := validateGoogleEmail(issues, memberPrefix+".email", member.Email)
			if _, exists := memberEmails[memberEmail]; exists && memberEmail != "" {
				*issues = append(*issues, fmt.Sprintf("%s.email %q is duplicated in group", memberPrefix, member.Email))
			}
			memberEmails[memberEmail] = struct{}{}
			if member.Type != "USER" {
				*issues = append(*issues, fmt.Sprintf("%s.type has unsupported value %q", memberPrefix, member.Type))
			}
			if _, exists := usersByEmail[memberEmail]; !exists {
				*issues = append(*issues, fmt.Sprintf("%s.email references unknown Google user %q", memberPrefix, member.Email))
			}
			if member.Role != "OWNER" && member.Role != "MANAGER" && member.Role != "MEMBER" {
				*issues = append(*issues, fmt.Sprintf("%s.role has unsupported value %q", memberPrefix, member.Role))
			}
		}
	}

	fileIDs := make(map[string]struct{})
	for i, file := range provider.DriveFiles {
		prefix := fmt.Sprintf("spec.providers.google.driveFiles[%d]", i)
		validateGoogleObjectID(issues, prefix+".id", file.ID, objectIDs)
		fileIDs[file.ID] = struct{}{}
		checkReference(issues, prefix+".node", file.Node, resources)
		validateMicrosoftNodeTenant(issues, prefix+".node", file.Node, provider.Tenant, resourceTenants)
		requireText(issues, prefix+".name", file.Name)
		requireText(issues, prefix+".mimeType", file.MimeType)
	}

	permissionKeys := make(map[string]struct{})
	for i, permission := range provider.DrivePermissions {
		prefix := fmt.Sprintf("spec.providers.google.drivePermissions[%d]", i)
		validateGoogleID(issues, prefix+".id", permission.ID)
		if _, exists := fileIDs[permission.FileID]; !exists {
			*issues = append(*issues, fmt.Sprintf("%s.fileId references unknown Drive file %q", prefix, permission.FileID))
		}
		key := permission.FileID + ":" + permission.ID
		if _, exists := permissionKeys[key]; exists {
			*issues = append(*issues, fmt.Sprintf("%s duplicates permission %q for file %q", prefix, permission.ID, permission.FileID))
		}
		permissionKeys[key] = struct{}{}
		email := validateGoogleEmail(issues, prefix+".emailAddress", permission.EmailAddress)
		switch permission.Type {
		case "user":
			if _, exists := usersByEmail[email]; !exists {
				*issues = append(*issues, fmt.Sprintf("%s.emailAddress references unknown Google user %q", prefix, permission.EmailAddress))
			}
		case "group":
			if _, exists := groupsByEmail[email]; !exists {
				*issues = append(*issues, fmt.Sprintf("%s.emailAddress references unknown Google group %q", prefix, permission.EmailAddress))
			}
		default:
			*issues = append(*issues, fmt.Sprintf("%s.type has unsupported value %q", prefix, permission.Type))
		}
		if permission.Role != "reader" && permission.Role != "commenter" && permission.Role != "writer" {
			*issues = append(*issues, fmt.Sprintf("%s.role has unsupported value %q", prefix, permission.Role))
		}
	}
}

func validateGoogleObjectID(issues *[]string, field, value string, objects map[string]string) {
	validateGoogleID(issues, field, value)
	if previous, exists := objects[value]; exists {
		*issues = append(*issues, fmt.Sprintf("%s %q duplicates %s", field, value, previous))
	}
	objects[value] = field
}

func validateGoogleID(issues *[]string, field, value string) {
	if !googleObjectIDPattern.MatchString(value) {
		*issues = append(*issues, fmt.Sprintf("%s has invalid Google object ID %q", field, value))
	}
}

func validateGoogleEmail(issues *[]string, field, value string) string {
	normalized := strings.ToLower(value)
	if value != normalized || !emailPattern.MatchString(value) {
		*issues = append(*issues, fmt.Sprintf("%s must be a lowercase email address", field))
	}
	return normalized
}

func validateMicrosoftProvider(
	issues *[]string,
	provider *MicrosoftProvider,
	runtime *MicrosoftRuntime,
	tenants, principals, resources map[string]struct{},
	principalTenants, resourceTenants map[string]string,
) {
	if provider == nil {
		if runtime != nil {
			*issues = append(*issues, "spec.runtimes.microsoft requires spec.providers.microsoft")
		}
		return
	}
	if runtime == nil {
		*issues = append(*issues, "spec.providers.microsoft requires spec.runtimes.microsoft")
	}
	checkReference(issues, "spec.providers.microsoft.tenant", provider.Tenant, tenants)
	if !uuidPattern.MatchString(provider.TenantID) {
		*issues = append(*issues, fmt.Sprintf("spec.providers.microsoft.tenantId has invalid UUID %q", provider.TenantID))
	}
	if len(provider.Users) == 0 {
		*issues = append(*issues, "spec.providers.microsoft.users must contain at least one user")
	}

	objectIDs := make(map[string]string)
	userIDs := make(map[string]struct{})
	for i, user := range provider.Users {
		prefix := fmt.Sprintf("spec.providers.microsoft.users[%d]", i)
		validateMicrosoftObjectID(issues, prefix+".id", user.ID, objectIDs)
		userIDs[user.ID] = struct{}{}
		checkReference(issues, prefix+".node", user.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", user.Node, provider.Tenant, principalTenants)
		requireText(issues, prefix+".displayName", user.DisplayName)
		if !strings.Contains(user.UserPrincipalName, "@") {
			*issues = append(*issues, fmt.Sprintf("%s.userPrincipalName must contain @", prefix))
		}
	}
	groupIDs := make(map[string]struct{})
	for i, group := range provider.Groups {
		prefix := fmt.Sprintf("spec.providers.microsoft.groups[%d]", i)
		validateMicrosoftObjectID(issues, prefix+".id", group.ID, objectIDs)
		groupIDs[group.ID] = struct{}{}
		checkReference(issues, prefix+".node", group.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", group.Node, provider.Tenant, principalTenants)
		requireText(issues, prefix+".displayName", group.DisplayName)
	}

	applicationAppIDs := make(map[string]struct{})
	for i, application := range provider.Applications {
		prefix := fmt.Sprintf("spec.providers.microsoft.applications[%d]", i)
		validateMicrosoftObjectID(issues, prefix+".id", application.ID, objectIDs)
		validateMicrosoftUUID(issues, prefix+".appId", application.AppID)
		if _, exists := applicationAppIDs[application.AppID]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.appId %q is duplicated", prefix, application.AppID))
		}
		applicationAppIDs[application.AppID] = struct{}{}
		checkReference(issues, prefix+".node", application.Node, principals)
		validateMicrosoftNodeTenant(issues, prefix+".node", application.Node, provider.Tenant, principalTenants)
		requireText(issues, prefix+".displayName", application.DisplayName)
	}

	servicePrincipalIDs := make(map[string]MicrosoftServicePrincipal)
	appRolesByResource := make(map[string]map[string]struct{})
	for i, servicePrincipal := range provider.ServicePrincipals {
		prefix := fmt.Sprintf("spec.providers.microsoft.servicePrincipals[%d]", i)
		validateMicrosoftObjectID(issues, prefix+".id", servicePrincipal.ID, objectIDs)
		validateMicrosoftUUID(issues, prefix+".appId", servicePrincipal.AppID)
		requireText(issues, prefix+".displayName", servicePrincipal.DisplayName)
		if (servicePrincipal.Node == "") == (servicePrincipal.ResourceNode == "") {
			*issues = append(*issues, prefix+" must set exactly one of node or resourceNode")
		}
		if servicePrincipal.Node != "" {
			checkReference(issues, prefix+".node", servicePrincipal.Node, principals)
			validateMicrosoftNodeTenant(issues, prefix+".node", servicePrincipal.Node, provider.Tenant, principalTenants)
			if _, exists := applicationAppIDs[servicePrincipal.AppID]; !exists {
				*issues = append(*issues, fmt.Sprintf("%s.appId references unknown application appId %q", prefix, servicePrincipal.AppID))
			}
		}
		if servicePrincipal.ResourceNode != "" {
			checkReference(issues, prefix+".resourceNode", servicePrincipal.ResourceNode, resources)
			validateMicrosoftNodeTenant(issues, prefix+".resourceNode", servicePrincipal.ResourceNode, provider.Tenant, resourceTenants)
		}
		servicePrincipalIDs[servicePrincipal.ID] = servicePrincipal
		roleIDs := make(map[string]struct{})
		roleValues := make(map[string]struct{})
		for j, appRole := range servicePrincipal.AppRoles {
			rolePrefix := fmt.Sprintf("%s.appRoles[%d]", prefix, j)
			validateMicrosoftUUID(issues, rolePrefix+".id", appRole.ID)
			if _, exists := roleIDs[appRole.ID]; exists {
				*issues = append(*issues, fmt.Sprintf("%s.id %q is duplicated", rolePrefix, appRole.ID))
			}
			roleIDs[appRole.ID] = struct{}{}
			requireText(issues, rolePrefix+".value", appRole.Value)
			if _, exists := roleValues[appRole.Value]; exists && appRole.Value != "" {
				*issues = append(*issues, fmt.Sprintf("%s.value %q is duplicated", rolePrefix, appRole.Value))
			}
			roleValues[appRole.Value] = struct{}{}
			requireText(issues, rolePrefix+".displayName", appRole.DisplayName)
		}
		appRolesByResource[servicePrincipal.ID] = roleIDs
	}

	grantIDs := make(map[string]struct{})
	for i, grant := range provider.OAuth2PermissionGrants {
		prefix := fmt.Sprintf("spec.providers.microsoft.oauth2PermissionGrants[%d]", i)
		validateMicrosoftUUID(issues, prefix+".id", grant.ID)
		if _, exists := grantIDs[grant.ID]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.id %q is duplicated", prefix, grant.ID))
		}
		grantIDs[grant.ID] = struct{}{}
		client, clientExists := servicePrincipalIDs[grant.ClientID]
		if !clientExists || client.Node == "" {
			*issues = append(*issues, fmt.Sprintf("%s.clientId references unknown client service principal %q", prefix, grant.ClientID))
		}
		resource, resourceExists := servicePrincipalIDs[grant.ResourceID]
		if !resourceExists || resource.ResourceNode == "" {
			*issues = append(*issues, fmt.Sprintf("%s.resourceId references unknown resource service principal %q", prefix, grant.ResourceID))
		}
		if grant.ConsentType != "Principal" {
			*issues = append(*issues, fmt.Sprintf("%s.consentType has unsupported value %q", prefix, grant.ConsentType))
		}
		if _, exists := userIDs[grant.PrincipalID]; !exists {
			*issues = append(*issues, fmt.Sprintf("%s.principalId references unknown user %q", prefix, grant.PrincipalID))
		}
		if len(strings.Fields(grant.Scope)) == 0 {
			*issues = append(*issues, prefix+".scope must contain at least one permission")
		}
	}

	assignmentIDs := make(map[string]struct{})
	for i, assignment := range provider.AppRoleAssignments {
		prefix := fmt.Sprintf("spec.providers.microsoft.appRoleAssignments[%d]", i)
		validateMicrosoftUUID(issues, prefix+".id", assignment.ID)
		if _, exists := assignmentIDs[assignment.ID]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.id %q is duplicated", prefix, assignment.ID))
		}
		assignmentIDs[assignment.ID] = struct{}{}
		if _, exists := groupIDs[assignment.PrincipalID]; !exists {
			*issues = append(*issues, fmt.Sprintf("%s.principalId references unknown group %q", prefix, assignment.PrincipalID))
		}
		resource, exists := servicePrincipalIDs[assignment.ResourceID]
		if !exists || resource.Node == "" {
			*issues = append(*issues, fmt.Sprintf("%s.resourceId references unknown application service principal %q", prefix, assignment.ResourceID))
		}
		if roles, exists := appRolesByResource[assignment.ResourceID]; !exists {
			*issues = append(*issues, fmt.Sprintf("%s.appRoleId has no declared resource app roles", prefix))
		} else if _, exists := roles[assignment.AppRoleID]; !exists {
			*issues = append(*issues, fmt.Sprintf("%s.appRoleId references unknown app role %q", prefix, assignment.AppRoleID))
		}
	}
}

func validOIDCAudienceValue(value string) bool {
	parsed, err := url.Parse(value)
	if err == nil && ((parsed.Scheme == "https" && parsed.Host != "") || (parsed.Scheme == "urn" && parsed.Opaque != "")) && parsed.Fragment == "" {
		return true
	}
	return oidcAudiencePattern.MatchString(value)
}

func oidcClientAudienceExists(provider *OIDCProvider, clientNode, audienceNode, audienceValue string) bool {
	if provider == nil {
		return false
	}
	for _, client := range provider.Clients {
		if client.Node != clientNode {
			continue
		}
		for _, audience := range client.Audiences {
			if audience.Node == audienceNode && audience.Value == audienceValue {
				return true
			}
		}
	}
	return false
}

func validateMicrosoftObjectID(issues *[]string, field, value string, objects map[string]string) {
	validateMicrosoftUUID(issues, field, value)
	if previous, exists := objects[value]; exists {
		*issues = append(*issues, fmt.Sprintf("%s %q duplicates %s", field, value, previous))
	}
	objects[value] = field
}

func validateMicrosoftUUID(issues *[]string, field, value string) {
	if !uuidPattern.MatchString(value) {
		*issues = append(*issues, fmt.Sprintf("%s has invalid UUID %q", field, value))
	}
}

func validateMicrosoftNodeTenant(issues *[]string, field, node, tenant string, nodeTenants map[string]string) {
	if actual, exists := nodeTenants[node]; exists && actual != tenant {
		*issues = append(*issues, fmt.Sprintf("%s %q belongs to tenant %q, not %q", field, node, actual, tenant))
	}
}

func validateAWSPolicies(issues *[]string, prefix string, policies []AWSInlinePolicy) {
	policyNames := make(map[string]struct{})
	for i, policy := range policies {
		policyPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		if !awsRoleNamePattern.MatchString(policy.Name) {
			*issues = append(*issues, fmt.Sprintf("%s.name has invalid policy name %q", policyPrefix, policy.Name))
		}
		if _, exists := policyNames[policy.Name]; exists {
			*issues = append(*issues, fmt.Sprintf("%s.name %q is duplicated", policyPrefix, policy.Name))
		}
		policyNames[policy.Name] = struct{}{}
		if len(policy.Statements) == 0 {
			*issues = append(*issues, policyPrefix+".statements must not be empty")
		}
		for j, statement := range policy.Statements {
			statementPrefix := fmt.Sprintf("%s.statements[%d]", policyPrefix, j)
			if statement.Effect != "Allow" && statement.Effect != "Deny" {
				*issues = append(*issues, fmt.Sprintf("%s.effect has unsupported value %q", statementPrefix, statement.Effect))
			}
			if len(statement.Actions) == 0 {
				*issues = append(*issues, statementPrefix+".actions must not be empty")
			}
			if len(statement.Resources) == 0 {
				*issues = append(*issues, statementPrefix+".resources must not be empty")
			}
			for _, action := range statement.Actions {
				requireText(issues, statementPrefix+".actions[]", action)
			}
			for _, resource := range statement.Resources {
				requireText(issues, statementPrefix+".resources[]", resource)
			}
		}
	}
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
