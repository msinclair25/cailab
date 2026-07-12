package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestCrossProviderFederationIntegration(t *testing.T) {
	if os.Getenv("CAILAB_CROSS_PROVIDER_INTEGRATION") != "1" {
		t.Skip("set CAILAB_CROSS_PROVIDER_INTEGRATION=1 to run the flagship provider lifecycle")
	}
	ctx := context.Background()
	compiled := loadFlagshipScenario(t)
	manager := NewManager(t.TempDir())
	manager.native.command = func(executable, providerName, configPath string) *exec.Cmd {
		var testName, environment string
		switch providerName {
		case "microsoft":
			testName, environment = "TestMicrosoftRuntimeHelper", "CAILAB_MICROSOFT_RUNTIME_HELPER=1"
		case "google":
			testName, environment = "TestGoogleRuntimeHelper", "CAILAB_GOOGLE_RUNTIME_HELPER=1"
		case "oidc":
			testName, environment = "TestOIDCRuntimeHelper", "CAILAB_OIDC_RUNTIME_HELPER=1"
		default:
			t.Fatalf("unexpected native provider %q", providerName)
		}
		command := exec.Command(executable, "-test.run=^"+testName+"$", "--", configPath)
		command.Env = append(os.Environ(), environment)
		return command
	}
	runID := "flagship-cross-provider-integration"
	instances, err := manager.Start(ctx, runID, compiled)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = manager.Stop(context.Background(), runID, instances, compiled)
		}
	}()

	snapshot, err := manager.Snapshot(ctx, instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "google:contractor", "aws:acquisition-data", true)
	assertPath(t, snapshot, "google:security-admin", "aws:acquisition-data", true)
	oidcEndpoint := integrationEndpoint(t, instances, "oidc")
	awsEndpoint := integrationEndpoint(t, instances, "aws")
	microsoftEndpoint := integrationEndpoint(t, instances, "microsoft")
	declaredRole, ok := awsRoleByNode(compiled.Providers.AWS.Roles, flagshipRoleNode)
	if !ok {
		t.Fatal("flagship role is missing")
	}
	if _, err := ValidateOIDCRuntimeToken(ctx, oidcEndpoint, "not-a-signed-jwt", "access", "sts.amazonaws.com"); err == nil {
		t.Fatal("CloudAILab validator accepted an invalid web-identity token")
	}
	if _, err := AssumeAWSWebIdentity(ctx, awsEndpoint, compiled.Providers.AWS.Region, declaredRole, "not-a-signed-jwt", "floci-fidelity-spike"); err != nil {
		t.Fatalf("pinned Floci no longer exhibits the documented permissive web-identity behavior: %v", err)
	}

	contractorToken := issueFlagshipAccessToken(t, oidcEndpoint, "northstar-contractor")
	contractorClaims, err := ValidateOIDCRuntimeToken(ctx, oidcEndpoint, contractorToken, "access", "sts.amazonaws.com")
	if err != nil {
		t.Fatal(err)
	}
	role, err := AuthorizeAWSWebIdentity(snapshot, contractorClaims, flagshipRoleNode)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := AssumeAWSWebIdentity(ctx, awsEndpoint, compiled.Providers.AWS.Region, role, contractorToken, "cailab-integration")
	if err != nil {
		t.Fatal(err)
	}
	object, err := s3ClientWithCredentials(awsEndpoint, compiled.Providers.AWS.Region, credentials.AccessKeyID, credentials.SecretAccessKey, credentials.SessionToken).GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("cailab-acquisition-data"), Key: aws.String("restricted/acquisition-summary.txt"),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(object.Body)
	object.Body.Close()
	if err != nil || !strings.Contains(string(data), "SYNTHETIC TRAINING DATA") {
		t.Fatalf("federated S3 object = %q, err=%v", data, err)
	}

	response := graphRequest(t, http.DefaultClient, http.MethodDelete, microsoftEndpoint+"/v1.0/servicePrincipals/77777777-7777-4777-8777-777777777777/appRoleAssignedTo/"+flagshipRiskyAssignment, LocalGraphToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete app-role assignment status = %d", response.StatusCode)
	}
	snapshot, err = manager.Snapshot(ctx, instances, compiled)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, snapshot, "google:contractor", "aws:acquisition-data", false)
	assertPath(t, snapshot, "google:security-admin", "aws:acquisition-data", true)
	if _, err := AuthorizeAWSWebIdentity(snapshot, contractorClaims, flagshipRoleNode); err == nil {
		t.Fatal("remediated contractor token remained authorized")
	}
	adminToken := issueFlagshipAccessToken(t, oidcEndpoint, "northstar-security-admin")
	adminClaims, err := ValidateOIDCRuntimeToken(ctx, oidcEndpoint, adminToken, "access", "sts.amazonaws.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuthorizeAWSWebIdentity(snapshot, adminClaims, flagshipRoleNode); err != nil {
		t.Fatalf("approved admin authorization after remediation: %v", err)
	}

	if err := manager.Stop(ctx, runID, instances, compiled); err != nil {
		t.Fatal(err)
	}
	stopped = true
}

func integrationEndpoint(t *testing.T, instances []Instance, providerName string) string {
	t.Helper()
	for _, instance := range instances {
		if instance.Provider == providerName {
			return instance.Endpoint
		}
	}
	t.Fatalf("no %s runtime in %+v", providerName, instances)
	return ""
}

func issueFlagshipAccessToken(t *testing.T, issuer, subject string) string {
	t.Helper()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	query := url.Values{
		"response_type": {"code"}, "client_id": {"cailab-acquisition-automation"},
		"redirect_uri": {"http://127.0.0.1:7777/callback"}, "scope": {"openid profile email"},
		"state": {"integration"}, "cailab_subject": {subject},
	}
	response, err := client.Get(issuer + "/authorize?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("authorization status = %d", response.StatusCode)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || location.Query().Get("code") == "" {
		t.Fatalf("authorization redirect = %q, err=%v", response.Header.Get("Location"), err)
	}
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {location.Query().Get("code")},
		"redirect_uri": {"http://127.0.0.1:7777/callback"},
	}
	request, err := http.NewRequest(http.MethodPost, issuer+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth("cailab-acquisition-automation", "cailab-synthetic-acquisition-secret")
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&tokens); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || tokens.AccessToken == "" {
		t.Fatalf("token status = %d", response.StatusCode)
	}
	return tokens.AccessToken
}
