package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/msinclair25/cailab/internal/scenario"
)

// FederatedCredentials is the owner-only credential artifact returned by the
// local federation gateway. Callers must not write it to shared logs or stdout.
type FederatedCredentials struct {
	AccessKeyID     string    `json:"accessKeyId"`
	SecretAccessKey string    `json:"secretAccessKey"`
	SessionToken    string    `json:"sessionToken"`
	Expiration      time.Time `json:"expiration"`
	RoleARN         string    `json:"roleArn"`
}

// AuthorizeAWSWebIdentity evaluates the provider-neutral authorization inputs
// in a live normalized snapshot. It does not trust Floci to validate the JWT or
// the web-identity trust policy.
func AuthorizeAWSWebIdentity(compiled scenario.Compiled, claims OIDCClaims, roleNode string) (scenario.AWSRole, error) {
	if compiled.Providers.OIDC == nil || compiled.Providers.Microsoft == nil || compiled.Providers.AWS == nil {
		return scenario.AWSRole{}, errors.New("active scenario does not configure the OIDC, Microsoft, and AWS federation providers")
	}
	role, ok := awsRoleByNode(compiled.Providers.AWS.Roles, roleNode)
	if !ok || role.WebIdentity == nil {
		return scenario.AWSRole{}, fmt.Errorf("AWS role %q has no declared web-identity trust", roleNode)
	}
	if !oidcContains([]string(claims.Audience), role.WebIdentity.Audience) {
		return scenario.AWSRole{}, errors.New("token audience is not trusted by the requested AWS role")
	}
	client, ok := oidcClientForTrust(compiled.Providers.OIDC.Clients, *role.WebIdentity)
	if !ok || client.ClientID != claims.ClientID {
		return scenario.AWSRole{}, errors.New("token client is not trusted by the requested AWS role")
	}
	if claims.Tenant != compiled.Providers.OIDC.Tenant {
		return scenario.AWSRole{}, errors.New("token tenant does not match the active local issuer")
	}
	subject, ok := oidcSubjectByPrincipal(compiled.Providers.OIDC.Subjects, claims.PrincipalID)
	if !ok || subject.Subject != claims.Subject || subject.Email != claims.Email || !sameStrings(subject.Groups, claims.Groups) {
		return scenario.AWSRole{}, errors.New("token identity claims do not match the active scenario subject")
	}
	if !hasLiveMicrosoftAppRole(compiled.Providers.Microsoft, claims.Groups, role.WebIdentity.ClientNode) {
		return scenario.AWSRole{}, errors.New("token groups have no live Microsoft app-role assignment to the trusted client")
	}
	return role, nil
}

// AssumeAWSWebIdentity invokes the permissive Floci STS response emulator only
// after AuthorizeAWSWebIdentity and signed-token validation have succeeded.
func AssumeAWSWebIdentity(ctx context.Context, endpoint, region string, role scenario.AWSRole, token, sessionName string) (FederatedCredentials, error) {
	if !isIPv4LoopbackEndpoint(endpoint) {
		return FederatedCredentials{}, errors.New("AWS endpoint must be an IPv4 loopback HTTP origin")
	}
	if role.WebIdentity == nil {
		return FederatedCredentials{}, errors.New("AWS role has no web-identity trust")
	}
	if sessionName == "" {
		sessionName = "cailab-federation"
	}
	roleARN := "arn:aws:iam::" + role.Account + ":role/" + role.Name
	output, err := stsClient(endpoint, region, role.Account, localSecret, "").AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
		RoleArn: aws.String(roleARN), RoleSessionName: aws.String(sessionName), WebIdentityToken: aws.String(token),
	})
	if err != nil {
		return FederatedCredentials{}, fmt.Errorf("obtain temporary credentials from AWS emulator: %w", err)
	}
	if output.Credentials == nil || output.Credentials.Expiration == nil {
		return FederatedCredentials{}, errors.New("AWS emulator returned incomplete temporary credentials")
	}
	credentials := FederatedCredentials{
		AccessKeyID: aws.ToString(output.Credentials.AccessKeyId), SecretAccessKey: aws.ToString(output.Credentials.SecretAccessKey),
		SessionToken: aws.ToString(output.Credentials.SessionToken), Expiration: *output.Credentials.Expiration, RoleARN: roleARN,
	}
	if credentials.AccessKeyID == "" || credentials.SecretAccessKey == "" || credentials.SessionToken == "" {
		return FederatedCredentials{}, errors.New("AWS emulator returned incomplete temporary credentials")
	}
	return credentials, nil
}

func awsRoleByNode(roles []scenario.AWSRole, node string) (scenario.AWSRole, bool) {
	for _, role := range roles {
		if role.Node == node {
			return role, true
		}
	}
	return scenario.AWSRole{}, false
}

func oidcClientForTrust(clients []scenario.OIDCClient, trust scenario.AWSWebIdentityTrust) (scenario.OIDCClient, bool) {
	for _, client := range clients {
		if client.Node != trust.ClientNode {
			continue
		}
		for _, audience := range client.Audiences {
			if audience.Node == trust.AudienceNode && audience.Value == trust.Audience {
				return client, true
			}
		}
	}
	return scenario.OIDCClient{}, false
}

func oidcSubjectByPrincipal(subjects []scenario.OIDCSubject, principal string) (scenario.OIDCSubject, bool) {
	for _, subject := range subjects {
		if subject.Node == principal {
			return subject, true
		}
	}
	return scenario.OIDCSubject{}, false
}

func hasLiveMicrosoftAppRole(provider *scenario.MicrosoftProvider, groupNodes []string, clientNode string) bool {
	groupIDs := make(map[string]struct{}, len(groupNodes))
	for _, group := range provider.Groups {
		for _, node := range groupNodes {
			if group.Node == node {
				groupIDs[group.ID] = struct{}{}
			}
		}
	}
	resources := make(map[string]map[string]struct{})
	for _, servicePrincipal := range provider.ServicePrincipals {
		if servicePrincipal.Node != clientNode {
			continue
		}
		roles := make(map[string]struct{}, len(servicePrincipal.AppRoles))
		for _, appRole := range servicePrincipal.AppRoles {
			roles[appRole.ID] = struct{}{}
		}
		resources[servicePrincipal.ID] = roles
	}
	for _, assignment := range provider.AppRoleAssignments {
		_, groupOK := groupIDs[assignment.PrincipalID]
		roles, resourceOK := resources[assignment.ResourceID]
		_, roleOK := roles[assignment.AppRoleID]
		if groupOK && resourceOK && roleOK {
			return true
		}
	}
	return false
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}
