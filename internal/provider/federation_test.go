package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/msinclair25/cailab/internal/scenario"
)

const (
	flagshipRiskyAssignment = "99999999-9999-4999-8999-999999999999"
	flagshipRoleNode        = "aws:acquisition-reader"
)

func TestFlagshipCrossProviderPathAndRemediation(t *testing.T) {
	compiled := loadFlagshipScenario(t)
	google := &googleFacade{provider: *compiled.Providers.Google, statePath: filepath.Join(t.TempDir(), "google.json"), runID: "flagship", controlToken: "control"}
	googleEndpoint := "http://google.test"
	googleClient := &http.Client{Transport: handlerRoundTripper{handler: google}}
	var err error
	compiled, err = snapshotGoogleWithClient(context.Background(), googleEndpoint, compiled, googleClient)
	if err != nil {
		t.Fatal(err)
	}
	microsoft := &microsoftFacade{provider: *compiled.Providers.Microsoft, statePath: filepath.Join(t.TempDir(), "microsoft.json"), runID: "flagship", controlToken: "control"}
	microsoft.baseURL = "http://microsoft.test"
	if err := microsoft.persist(); err != nil {
		t.Fatal(err)
	}
	microsoftClient := &http.Client{Transport: handlerRoundTripper{handler: microsoft}}
	response := graphRequest(t, microsoftClient, http.MethodGet, microsoft.baseURL+"/v1.0/groups", LocalGraphToken)
	var groupsPage struct {
		Value []scenario.MicrosoftGroup `json:"value"`
	}
	if err := json.NewDecoder(response.Body).Decode(&groupsPage); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(groupsPage.Value) != 2 {
		t.Fatalf("groups response status=%d value=%+v", response.StatusCode, groupsPage.Value)
	}
	response = graphRequest(t, microsoftClient, http.MethodGet, microsoft.baseURL+"/v1.0/groups/33333333-3333-4333-8333-333333333333/appRoleAssignments", LocalGraphToken)
	var assignmentPage struct {
		Value []scenario.MicrosoftAppRoleAssignment `json:"value"`
	}
	if err := json.NewDecoder(response.Body).Decode(&assignmentPage); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(assignmentPage.Value) != 1 || assignmentPage.Value[0].ID != flagshipRiskyAssignment {
		t.Fatalf("group assignments status=%d value=%+v", response.StatusCode, assignmentPage.Value)
	}
	compiled, err = snapshotMicrosoftWithClient(context.Background(), microsoft.baseURL, compiled, microsoftClient)
	if err != nil {
		t.Fatal(err)
	}
	compiled = snapshotOIDC(compiled)
	compiled = normalizeAWSWebIdentityTrust(compiled)
	assertPath(t, compiled, "google:contractor", "aws:acquisition-data", true)
	assertPath(t, compiled, "google:security-admin", "aws:acquisition-data", true)

	contractorClaims := flagshipClaims(compiled, "google:contractor", "cailab-acquisition-automation")
	if _, err := AuthorizeAWSWebIdentity(compiled, contractorClaims, flagshipRoleNode); err != nil {
		t.Fatalf("authorize contractor before remediation: %v", err)
	}
	adminClaims := flagshipClaims(compiled, "google:security-admin", "cailab-acquisition-automation")
	if _, err := AuthorizeAWSWebIdentity(compiled, adminClaims, flagshipRoleNode); err != nil {
		t.Fatalf("authorize admin before remediation: %v", err)
	}

	response = graphRequest(t, microsoftClient, http.MethodDelete, microsoft.baseURL+"/v1.0/servicePrincipals/77777777-7777-4777-8777-777777777777/appRoleAssignedTo/"+flagshipRiskyAssignment, LocalGraphToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete app-role assignment status = %d", response.StatusCode)
	}
	compiled, err = snapshotMicrosoftWithClient(context.Background(), microsoft.baseURL, compiled, microsoftClient)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, compiled, "google:contractor", "aws:acquisition-data", false)
	assertPath(t, compiled, "google:security-admin", "aws:acquisition-data", true)
	if _, err := AuthorizeAWSWebIdentity(compiled, contractorClaims, flagshipRoleNode); err == nil {
		t.Fatal("contractor federation remained authorized after remediation")
	}
	if _, err := AuthorizeAWSWebIdentity(compiled, adminClaims, flagshipRoleNode); err != nil {
		t.Fatalf("approved admin federation after remediation: %v", err)
	}
}

func TestAuthorizeAWSWebIdentityRejectsClaimEscalation(t *testing.T) {
	compiled := loadFlagshipScenario(t)
	claims := flagshipClaims(compiled, "google:contractor", "cailab-acquisition-automation")
	claims.Groups = []string{"microsoft:approved-automation"}
	if _, err := AuthorizeAWSWebIdentity(compiled, claims, flagshipRoleNode); err == nil {
		t.Fatal("authorization accepted claims that did not match the declared subject")
	}
}

func loadFlagshipScenario(t *testing.T) scenario.Compiled {
	t.Helper()
	definition, err := scenario.Load(filepath.Join("..", "..", "scenarios", "acquisition-agent", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := scenario.Compile(definition, definition.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func flagshipClaims(compiled scenario.Compiled, principal, clientID string) OIDCClaims {
	for _, subject := range compiled.Providers.OIDC.Subjects {
		if subject.Node == principal {
			return OIDCClaims{
				Subject: subject.Subject, ClientID: clientID, Tenant: compiled.Providers.OIDC.Tenant,
				Audience: OIDCAudienceClaim{"sts.amazonaws.com"}, PrincipalID: subject.Node,
				Email: subject.Email, Groups: append([]string(nil), subject.Groups...),
			}
		}
	}
	return OIDCClaims{}
}
