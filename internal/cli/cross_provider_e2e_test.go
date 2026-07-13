package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCrossProviderCLIE2E(t *testing.T) {
	if os.Getenv("CAILAB_CROSS_PROVIDER_E2E") != "1" {
		t.Skip("set CAILAB_CROSS_PROVIDER_E2E=1 to run the public flagship workflow")
	}
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	binary := filepath.Join(workspace, "cailab")
	build := exec.Command("go", "build", "-o", binary, "./cmd/cailab")
	build.Dir = repository
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}
	stateDir := filepath.Join(workspace, "state")
	output, code := runE2ECLI(t, repository, binary, "up", "--state-dir", stateDir, "--scenario-root", "scenarios", "acquisition-agent")
	if code != ExitOK {
		t.Fatalf("up exit=%d output=%s", code, output)
	}
	defer func() { _, _ = runE2ECLI(t, repository, binary, "down", "--state-dir", stateDir) }()
	endpoints := parseE2EEndpoints(output)
	for _, providerName := range []string{"AWS", "MICROSOFT", "GOOGLE", "OIDC"} {
		if endpoints[providerName] == "" {
			t.Fatalf("up output has no %s endpoint: %s", providerName, output)
		}
	}
	output, code = runE2ECLI(t, repository, binary, "graph", "path", "--state-dir", stateDir, "google:contractor", "aws:acquisition-data")
	if code != ExitOK || !strings.Contains(output, "sts:AssumeRoleWithWebIdentity") {
		t.Fatalf("initial graph exit=%d output=%s", code, output)
	}
	if output, code = runE2ECLI(t, repository, binary, "verify", "--state-dir", stateDir); code != ExitVerificationFailed {
		t.Fatalf("initial verify exit=%d output=%s", code, output)
	}
	output, code = runE2ECLI(t, repository, binary, "agent", "run", "unsafe", "--state-dir", stateDir, "--fixture", "drive-runbook-export")
	if code != ExitOK || !strings.Contains(output, "2 state snapshot(s)") {
		t.Fatalf("unsafe agent exit=%d output=%s", code, output)
	}
	output, code = runE2ECLI(t, repository, binary, "agent", "replay", "--state-dir", stateDir, "--trial-id", "trial:unsafe", "--format", "json")
	if code != ExitOK {
		t.Fatalf("unsafe replay exit=%d output=%s", code, output)
	}
	var unsafeReport struct {
		Profile   string `json:"profile"`
		Aggregate struct {
			InjectionSuccessRate struct {
				Numerator int `json:"numerator"`
			} `json:"injectionSuccessRate"`
		} `json:"aggregate"`
	}
	if err := json.Unmarshal([]byte(output), &unsafeReport); err != nil {
		t.Fatal(err)
	}
	if unsafeReport.Profile != "adversarial-scenario-v1" || unsafeReport.Aggregate.InjectionSuccessRate.Numerator != 1 {
		t.Fatalf("unsafe report = %+v", unsafeReport)
	}
	output, code = runE2ECLI(t, repository, binary, "agent", "campaign", "unsafe", "--state-dir", stateDir,
		"--trials", "2", "--trial-prefix", "campaign:e2e-unsafe", "--fixture", "drive-runbook-export", "--format", "json")
	if code != ExitOK {
		t.Fatalf("unsafe campaign exit=%d output=%s", code, output)
	}
	var campaignReport struct {
		Profile   string `json:"profile"`
		Aggregate struct {
			Trials          int `json:"trials"`
			CompletedTrials struct {
				Numerator int `json:"numerator"`
			} `json:"completedTrials"`
			InjectionSuccessRate struct {
				Numerator int `json:"numerator"`
			} `json:"injectionSuccessRate"`
		} `json:"aggregate"`
	}
	jsonStart := strings.IndexByte(output, '{')
	if jsonStart < 0 {
		t.Fatalf("unsafe campaign output has no JSON report: %s", output)
	}
	if err := json.Unmarshal([]byte(output[jsonStart:]), &campaignReport); err != nil {
		t.Fatal(err)
	}
	if campaignReport.Profile != "adversarial-scenario-v1" || campaignReport.Aggregate.Trials != 2 ||
		campaignReport.Aggregate.CompletedTrials.Numerator != 2 || campaignReport.Aggregate.InjectionSuccessRate.Numerator != 2 {
		t.Fatalf("unsafe campaign report = %+v", campaignReport)
	}

	contractorToken := issueE2EAccessToken(t, endpoints["OIDC"], "northstar-contractor")
	contractorTokenPath := filepath.Join(workspace, "contractor.jwt")
	if err := os.WriteFile(contractorTokenPath, []byte(contractorToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	contractorCredentials := filepath.Join(workspace, "contractor-aws.json")
	output, code = runE2ECLI(t, repository, binary, "federation", "assume-aws", "--state-dir", stateDir, "--token-file", contractorTokenPath, "--role-node", "aws:acquisition-reader", "--output", contractorCredentials)
	if code != ExitOK || !strings.Contains(output, "temporary AWS credentials written") {
		t.Fatalf("contractor exchange exit=%d output=%s", code, output)
	}
	assertE2ECredentialFile(t, contractorCredentials)

	request, err := http.NewRequest(http.MethodDelete, endpoints["MICROSOFT"]+"/v1.0/servicePrincipals/77777777-7777-4777-8777-777777777777/appRoleAssignedTo/99999999-9999-4999-8999-999999999999", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer cailab-local")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("remediation status=%d", response.StatusCode)
	}
	deniedCredentials := filepath.Join(workspace, "denied-aws.json")
	output, code = runE2ECLI(t, repository, binary, "federation", "assume-aws", "--state-dir", stateDir, "--token-file", contractorTokenPath, "--role-node", "aws:acquisition-reader", "--output", deniedCredentials)
	if code != ExitError || !strings.Contains(output, "no live Microsoft app-role assignment") {
		t.Fatalf("remediated contractor exchange exit=%d output=%s", code, output)
	}
	if _, err := os.Stat(deniedCredentials); !os.IsNotExist(err) {
		t.Fatalf("denied exchange created a credential file: %v", err)
	}

	adminToken := issueE2EAccessToken(t, endpoints["OIDC"], "northstar-security-admin")
	adminTokenPath := filepath.Join(workspace, "admin.jwt")
	if err := os.WriteFile(adminTokenPath, []byte(adminToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	adminCredentials := filepath.Join(workspace, "admin-aws.json")
	output, code = runE2ECLI(t, repository, binary, "federation", "assume-aws", "--state-dir", stateDir, "--token-file", adminTokenPath, "--role-node", "aws:acquisition-reader", "--output", adminCredentials)
	if code != ExitOK {
		t.Fatalf("admin exchange exit=%d output=%s", code, output)
	}
	assertE2ECredentialFile(t, adminCredentials)
	if output, code = runE2ECLI(t, repository, binary, "verify", "--state-dir", stateDir); code != ExitOK || !strings.Contains(output, "2 passed, 0 failed") {
		t.Fatalf("final verify exit=%d output=%s", code, output)
	}
	if output, code = runE2ECLI(t, repository, binary, "down", "--state-dir", stateDir); code != ExitOK {
		t.Fatalf("down exit=%d output=%s", code, output)
	}
}

func runE2ECLI(t *testing.T, directory, binary string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("cailab %v timed out", args)
	}
	if err == nil {
		return string(output), ExitOK
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return string(output), exitError.ExitCode()
	}
	t.Fatalf("execute cailab %v: %v", args, err)
	return "", ExitError
}

func parseE2EEndpoints(output string) map[string]string {
	endpoints := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 3 && fields[1] == "endpoint:" {
			endpoints[fields[0]] = fields[2]
		}
	}
	return endpoints
}

func issueE2EAccessToken(t *testing.T, issuer, subject string) string {
	t.Helper()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	query := url.Values{
		"response_type": {"code"}, "client_id": {"cailab-acquisition-automation"},
		"redirect_uri": {"http://127.0.0.1:7777/callback"}, "scope": {"openid profile email"},
		"state": {"cli-e2e"}, "cailab_subject": {subject},
	}
	response, err := client.Get(issuer + "/authorize?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || response.StatusCode != http.StatusFound || location.Query().Get("code") == "" {
		t.Fatalf("authorization status=%d location=%q err=%v", response.StatusCode, response.Header.Get("Location"), err)
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
		t.Fatalf("token response status=%d", response.StatusCode)
	}
	return tokens.AccessToken
}

func assertE2ECredentialFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode=%o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var credential map[string]any
	if err := json.Unmarshal(data, &credential); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"accessKeyId", "secretAccessKey", "sessionToken", "expiration", "roleArn"} {
		value, exists := credential[key]
		if !exists || fmt.Sprint(value) == "" {
			t.Fatalf("credential file missing %s", key)
		}
	}
}
