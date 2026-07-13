package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func Compile(s Scenario, seed int64) (Compiled, error) {
	if err := Validate(s); err != nil {
		return Compiled{}, err
	}

	nodes := make([]Node, 0, len(s.Spec.Tenants)+len(s.Spec.Principals)+len(s.Spec.Resources))
	for _, tenant := range s.Spec.Tenants {
		nodes = append(nodes, Node{
			ID: tenant.ID, Kind: "tenant", Type: "organization", DisplayName: tenant.Name,
		})
	}
	for _, principal := range s.Spec.Principals {
		nodes = append(nodes, Node{
			ID: principal.ID, Kind: "principal", Tenant: principal.Tenant,
			Type: principal.Type, DisplayName: principal.DisplayName,
		})
	}
	for _, resource := range s.Spec.Resources {
		nodes = append(nodes, Node{
			ID: resource.ID, Kind: "resource", Tenant: resource.Tenant,
			Type: resource.Type, DisplayName: resource.DisplayName,
			Classification: resource.Classification,
		})
	}

	edges := make([]Relationship, len(s.Spec.Relationships))
	for i, edge := range s.Spec.Relationships {
		edges[i] = edge
		edges[i].Actions = append([]string(nil), edge.Actions...)
	}
	invariants := append([]Invariant(nil), s.Spec.Verification.Invariants...)
	objectives := append([]Objective(nil), s.Spec.Objectives...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	sort.Slice(invariants, func(i, j int) bool { return invariants[i].ID < invariants[j].ID })
	sort.Slice(objectives, func(i, j int) bool { return objectives[i].ID < objectives[j].ID })
	for i := range edges {
		sort.Strings(edges[i].Actions)
	}

	compiled := Compiled{
		SchemaVersion: APIVersion,
		ScenarioName:  s.Metadata.Name, ScenarioVersion: s.Metadata.Version,
		Title: s.Metadata.Title, Seed: seed, Briefing: s.Spec.Briefing,
		Runtimes: copyRuntimes(s.Spec.Runtimes), Providers: copyProviders(s.Spec.Providers), Objectives: objectives,
		Nodes: nodes, Edges: edges, Invariants: invariants, Evaluation: copyEvaluation(s.Spec.Evaluation),
	}
	digest, err := digestCompiled(compiled)
	if err != nil {
		return Compiled{}, err
	}
	compiled.Digest = digest
	return compiled, nil
}

func copyEvaluation(evaluation Evaluation) Evaluation {
	result := Evaluation{PromptInjections: make([]PromptInjectionFixture, len(evaluation.PromptInjections))}
	for index, fixture := range evaluation.PromptInjections {
		result.PromptInjections[index] = fixture
		result.PromptInjections[index].Prohibited = append([]EvaluationAction(nil), fixture.Prohibited...)
		sort.Slice(result.PromptInjections[index].Prohibited, func(i, j int) bool {
			left, right := result.PromptInjections[index].Prohibited[i], result.PromptInjections[index].Prohibited[j]
			if left.Tool != right.Tool {
				return left.Tool < right.Tool
			}
			if left.Action != right.Action {
				return left.Action < right.Action
			}
			return left.Resource < right.Resource
		})
	}
	sort.Slice(result.PromptInjections, func(i, j int) bool {
		return result.PromptInjections[i].ID < result.PromptInjections[j].ID
	})
	return result
}

func copyProviders(providers Providers) Providers {
	result := Providers{}
	if providers.AWS != nil {
		awsProvider := *providers.AWS
		awsProvider.Accounts = append([]AWSAccount(nil), providers.AWS.Accounts...)
		sort.Slice(awsProvider.Accounts, func(i, j int) bool { return awsProvider.Accounts[i].ID < awsProvider.Accounts[j].ID })
		awsProvider.Roles = make([]AWSRole, len(providers.AWS.Roles))
		for i, role := range providers.AWS.Roles {
			awsProvider.Roles[i] = role
			if role.WebIdentity != nil {
				webIdentity := *role.WebIdentity
				awsProvider.Roles[i].WebIdentity = &webIdentity
			}
			awsProvider.Roles[i].Trust = append([]string(nil), role.Trust...)
			sort.Strings(awsProvider.Roles[i].Trust)
			awsProvider.Roles[i].Policies = make([]AWSInlinePolicy, len(role.Policies))
			for j, policy := range role.Policies {
				awsProvider.Roles[i].Policies[j] = policy
				awsProvider.Roles[i].Policies[j].Statements = make([]AWSPolicyStatement, len(policy.Statements))
				for k, statement := range policy.Statements {
					awsProvider.Roles[i].Policies[j].Statements[k] = statement
					awsProvider.Roles[i].Policies[j].Statements[k].Actions = append([]string(nil), statement.Actions...)
					awsProvider.Roles[i].Policies[j].Statements[k].Resources = append([]string(nil), statement.Resources...)
					sort.Strings(awsProvider.Roles[i].Policies[j].Statements[k].Actions)
					sort.Strings(awsProvider.Roles[i].Policies[j].Statements[k].Resources)
				}
			}
			sort.Slice(awsProvider.Roles[i].Policies, func(j, k int) bool {
				return awsProvider.Roles[i].Policies[j].Name < awsProvider.Roles[i].Policies[k].Name
			})
		}
		sort.Slice(awsProvider.Roles, func(i, j int) bool {
			left, right := awsProvider.Roles[i], awsProvider.Roles[j]
			return left.Account < right.Account || left.Account == right.Account && left.Name < right.Name
		})
		awsProvider.Buckets = make([]AWSBucket, len(providers.AWS.Buckets))
		for i, bucket := range providers.AWS.Buckets {
			awsProvider.Buckets[i] = bucket
			awsProvider.Buckets[i].Objects = append([]AWSObject(nil), bucket.Objects...)
			sort.Slice(awsProvider.Buckets[i].Objects, func(j, k int) bool {
				return awsProvider.Buckets[i].Objects[j].Key < awsProvider.Buckets[i].Objects[k].Key
			})
		}
		sort.Slice(awsProvider.Buckets, func(i, j int) bool {
			left, right := awsProvider.Buckets[i], awsProvider.Buckets[j]
			return left.Account < right.Account || left.Account == right.Account && left.Name < right.Name
		})
		result.AWS = &awsProvider
	}
	if providers.Microsoft != nil {
		microsoft := *providers.Microsoft
		microsoft.Users = append([]MicrosoftUser(nil), providers.Microsoft.Users...)
		microsoft.Groups = append([]MicrosoftGroup(nil), providers.Microsoft.Groups...)
		microsoft.Applications = append([]MicrosoftApplication(nil), providers.Microsoft.Applications...)
		microsoft.ServicePrincipals = make([]MicrosoftServicePrincipal, len(providers.Microsoft.ServicePrincipals))
		for i, servicePrincipal := range providers.Microsoft.ServicePrincipals {
			microsoft.ServicePrincipals[i] = servicePrincipal
			microsoft.ServicePrincipals[i].AppRoles = append([]MicrosoftAppRole(nil), servicePrincipal.AppRoles...)
			sort.Slice(microsoft.ServicePrincipals[i].AppRoles, func(j, k int) bool {
				return microsoft.ServicePrincipals[i].AppRoles[j].ID < microsoft.ServicePrincipals[i].AppRoles[k].ID
			})
		}
		microsoft.OAuth2PermissionGrants = append([]MicrosoftPermissionGrant(nil), providers.Microsoft.OAuth2PermissionGrants...)
		microsoft.AppRoleAssignments = append([]MicrosoftAppRoleAssignment(nil), providers.Microsoft.AppRoleAssignments...)
		sort.Slice(microsoft.Users, func(i, j int) bool { return microsoft.Users[i].ID < microsoft.Users[j].ID })
		sort.Slice(microsoft.Groups, func(i, j int) bool { return microsoft.Groups[i].ID < microsoft.Groups[j].ID })
		sort.Slice(microsoft.Applications, func(i, j int) bool { return microsoft.Applications[i].ID < microsoft.Applications[j].ID })
		sort.Slice(microsoft.ServicePrincipals, func(i, j int) bool { return microsoft.ServicePrincipals[i].ID < microsoft.ServicePrincipals[j].ID })
		sort.Slice(microsoft.OAuth2PermissionGrants, func(i, j int) bool {
			return microsoft.OAuth2PermissionGrants[i].ID < microsoft.OAuth2PermissionGrants[j].ID
		})
		sort.Slice(microsoft.AppRoleAssignments, func(i, j int) bool {
			return microsoft.AppRoleAssignments[i].ID < microsoft.AppRoleAssignments[j].ID
		})
		result.Microsoft = &microsoft
	}
	if providers.Google != nil {
		google := *providers.Google
		google.Users = append([]GoogleUser(nil), providers.Google.Users...)
		google.Groups = make([]GoogleGroup, len(providers.Google.Groups))
		for i, group := range providers.Google.Groups {
			google.Groups[i] = group
			google.Groups[i].Members = append([]GoogleGroupMember(nil), group.Members...)
			sort.Slice(google.Groups[i].Members, func(j, k int) bool {
				return google.Groups[i].Members[j].Email < google.Groups[i].Members[k].Email
			})
		}
		google.DriveFiles = append([]GoogleDriveFile(nil), providers.Google.DriveFiles...)
		google.DrivePermissions = append([]GoogleDrivePermission(nil), providers.Google.DrivePermissions...)
		sort.Slice(google.Users, func(i, j int) bool { return google.Users[i].ID < google.Users[j].ID })
		sort.Slice(google.Groups, func(i, j int) bool { return google.Groups[i].ID < google.Groups[j].ID })
		sort.Slice(google.DriveFiles, func(i, j int) bool { return google.DriveFiles[i].ID < google.DriveFiles[j].ID })
		sort.Slice(google.DrivePermissions, func(i, j int) bool {
			left, right := google.DrivePermissions[i], google.DrivePermissions[j]
			return left.FileID < right.FileID || left.FileID == right.FileID && left.ID < right.ID
		})
		result.Google = &google
	}
	if providers.OIDC != nil {
		oidc := *providers.OIDC
		oidc.Clients = make([]OIDCClient, len(providers.OIDC.Clients))
		for i, client := range providers.OIDC.Clients {
			oidc.Clients[i] = client
			oidc.Clients[i].RedirectURIs = append([]string(nil), client.RedirectURIs...)
			oidc.Clients[i].Audiences = append([]OIDCAudience(nil), client.Audiences...)
			oidc.Clients[i].Scopes = append([]string(nil), client.Scopes...)
			sort.Strings(oidc.Clients[i].RedirectURIs)
			sort.Slice(oidc.Clients[i].Audiences, func(j, k int) bool { return oidc.Clients[i].Audiences[j].Value < oidc.Clients[i].Audiences[k].Value })
			sort.Strings(oidc.Clients[i].Scopes)
		}
		oidc.Subjects = make([]OIDCSubject, len(providers.OIDC.Subjects))
		for i, subject := range providers.OIDC.Subjects {
			oidc.Subjects[i] = subject
			oidc.Subjects[i].Groups = append([]string(nil), subject.Groups...)
			sort.Strings(oidc.Subjects[i].Groups)
		}
		sort.Slice(oidc.Clients, func(i, j int) bool { return oidc.Clients[i].ClientID < oidc.Clients[j].ClientID })
		sort.Slice(oidc.Subjects, func(i, j int) bool { return oidc.Subjects[i].Subject < oidc.Subjects[j].Subject })
		result.OIDC = &oidc
	}
	return result
}

func copyRuntimes(runtimes Runtimes) Runtimes {
	result := Runtimes{}
	if runtimes.AWS != nil {
		awsRuntime := *runtimes.AWS
		result.AWS = &awsRuntime
	}
	if runtimes.Microsoft != nil {
		microsoftRuntime := *runtimes.Microsoft
		result.Microsoft = &microsoftRuntime
	}
	if runtimes.Google != nil {
		googleRuntime := *runtimes.Google
		result.Google = &googleRuntime
	}
	if runtimes.OIDC != nil {
		oidcRuntime := *runtimes.OIDC
		result.OIDC = &oidcRuntime
	}
	return result
}

func digestCompiled(compiled Compiled) (string, error) {
	compiled.Digest = ""
	data, err := json.Marshal(compiled)
	if err != nil {
		return "", fmt.Errorf("marshal compiled scenario: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// StateDigest returns the canonical digest of a compiled scenario snapshot.
// The embedded Digest field is excluded so live snapshots can be compared to
// the original compiled fixture without changing their source manifest ID.
func StateDigest(compiled Compiled) (string, error) {
	return digestCompiled(compiled)
}
