package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msinclair25/cailab/internal/scenario"
)

const (
	testOIDCIssuer   = "http://127.0.0.1:61999"
	testOIDCClientID = "cailab-automation-client"
	testOIDCSecret   = "cailab-synthetic-local-oidc-secret"
	testOIDCRedirect = "http://127.0.0.1:7777/callback"
	testOIDCAudience = "https://identity-api.cailab.local"
)

func TestOIDCAuthorizationCodeContract(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	facade := newTestOIDCFacade(t, &now)

	discovery := oidcRequest(t, facade, http.MethodGet, testOIDCIssuer+"/.well-known/openid-configuration", "", "", "")
	var metadata struct {
		Issuer        string `json:"issuer"`
		TokenEndpoint string `json:"token_endpoint"`
		JWKSURI       string `json:"jwks_uri"`
	}
	decodeOIDCResponse(t, discovery, http.StatusOK, &metadata)
	if metadata.Issuer != testOIDCIssuer || metadata.TokenEndpoint != testOIDCIssuer+"/token" || metadata.JWKSURI != testOIDCIssuer+"/jwks" {
		t.Fatalf("metadata = %+v", metadata)
	}

	set := fetchTestJWKS(t, facade)
	if len(set.Keys) != 1 || set.Keys[0].Algorithm != oidcSigningAlgorithm || set.Keys[0].Use != "sig" {
		t.Fatalf("JWKS = %+v", set)
	}

	code := authorizeTestSubject(t, facade, "state-123", "nonce-123")
	tokens := exchangeTestCode(t, facade, code, testOIDCRedirect, testOIDCClientID, testOIDCSecret, http.StatusOK)
	idToken := tokens["id_token"].(string)
	accessToken := tokens["access_token"].(string)
	idClaims, err := ValidateOIDCToken(idToken, "JWT", testOIDCIssuer, testOIDCClientID, now, set)
	if err != nil {
		t.Fatal(err)
	}
	if idClaims.Subject != "identity-engineer" || idClaims.Nonce != "nonce-123" || idClaims.Email != "identity.engineer@example.test" || idClaims.PrincipalID != "principal:identity-engineer" || len(idClaims.Groups) != 1 {
		t.Fatalf("ID claims = %+v", idClaims)
	}
	accessClaims, err := ValidateOIDCToken(accessToken, "at+jwt", testOIDCIssuer, testOIDCAudience, now, set)
	if err != nil {
		t.Fatal(err)
	}
	if accessClaims.ClientID != testOIDCClientID || accessClaims.Scope != "email openid profile" || accessClaims.Tenant != "tenant:identity-lab" || accessClaims.Email != "identity.engineer@example.test" || len(accessClaims.Groups) != 1 {
		t.Fatalf("access claims = %+v", accessClaims)
	}

	replay := exchangeTestCode(t, facade, code, testOIDCRedirect, testOIDCClientID, testOIDCSecret, http.StatusBadRequest)
	if replay["error"] != "invalid_grant" {
		t.Fatalf("replay error = %+v", replay)
	}
}

func TestOIDCRejectsRedirectClientAndTokenTampering(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	facade := newTestOIDCFacade(t, &now)

	badRedirect := oidcRequest(t, facade, http.MethodGet, authorizationURL("http://127.0.0.1:8888/callback", "identity-engineer", "", ""), "", "", "")
	if badRedirect.Code != http.StatusBadRequest || badRedirect.Header().Get("Location") != "" {
		t.Fatalf("bad redirect status=%d location=%q", badRedirect.Code, badRedirect.Header().Get("Location"))
	}

	denied := oidcRequest(t, facade, http.MethodGet, authorizationURL(testOIDCRedirect, "missing-subject", "state-denied", ""), "", "", "")
	if denied.Code != http.StatusFound {
		t.Fatalf("denied status = %d", denied.Code)
	}
	deniedLocation, _ := url.Parse(denied.Header().Get("Location"))
	if deniedLocation.Query().Get("error") != "access_denied" || deniedLocation.Query().Get("state") != "state-denied" {
		t.Fatalf("denied location = %s", deniedLocation)
	}

	code := authorizeTestSubject(t, facade, "", "")
	invalidClient := exchangeTestCode(t, facade, code, testOIDCRedirect, testOIDCClientID, "wrong-secret", http.StatusUnauthorized)
	if invalidClient["error"] != "invalid_client" {
		t.Fatalf("invalid client response = %+v", invalidClient)
	}
	// Client authentication happens before code consumption.
	tokens := exchangeTestCode(t, facade, code, testOIDCRedirect, testOIDCClientID, testOIDCSecret, http.StatusOK)
	accessToken := tokens["access_token"].(string)
	set := fetchTestJWKS(t, facade)

	parts := strings.Split(accessToken, ".")
	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	signature[0] ^= 1
	tampered := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(signature)
	if _, err := ValidateOIDCToken(tampered, "at+jwt", testOIDCIssuer, testOIDCAudience, now, set); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("tampered token error = %v", err)
	}

	headerData, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var header map[string]any
	_ = json.Unmarshal(headerData, &header)
	header["alg"] = "none"
	changedHeader, _ := json.Marshal(header)
	wrongAlgorithm := base64.RawURLEncoding.EncodeToString(changedHeader) + "." + parts[1] + "." + parts[2]
	if _, err := ValidateOIDCToken(wrongAlgorithm, "at+jwt", testOIDCIssuer, testOIDCAudience, now, set); err == nil || !strings.Contains(err.Error(), "algorithm") {
		t.Fatalf("wrong algorithm error = %v", err)
	}
	if _, err := ValidateOIDCToken(accessToken, "JWT", testOIDCIssuer, testOIDCAudience, now, set); err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("wrong type error = %v", err)
	}
	if _, err := ValidateOIDCToken(accessToken, "at+jwt", testOIDCIssuer, "https://wrong.example", now, set); err == nil || !strings.Contains(err.Error(), "audience") {
		t.Fatalf("wrong audience error = %v", err)
	}
	if _, err := ValidateOIDCToken(accessToken, "at+jwt", testOIDCIssuer, testOIDCAudience, now.Add(6*time.Minute), set); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired token error = %v", err)
	}
}

func TestOIDCRotationRetainsUnexpiredKeys(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	facade := newTestOIDCFacade(t, &now)
	firstCode := authorizeTestSubject(t, facade, "", "")
	firstTokens := exchangeTestCode(t, facade, firstCode, testOIDCRedirect, testOIDCClientID, testOIDCSecret, http.StatusOK)
	firstToken := firstTokens["access_token"].(string)
	firstKeyID, err := keyIDFromToken(firstToken)
	if err != nil {
		t.Fatal(err)
	}

	rotation := oidcRequest(t, facade, http.MethodPost, testOIDCIssuer+"/_cailab/rotate", "", "Bearer control-token", "oidc-test")
	var rotated OIDCJWKSet
	decodeOIDCResponse(t, rotation, http.StatusOK, &rotated)
	if len(rotated.Keys) != 2 {
		t.Fatalf("rotated JWKS = %+v", rotated)
	}
	stateData, err := os.ReadFile(facade.statePath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted oidcPersistentState
	if err := json.Unmarshal(stateData, &persisted); err != nil {
		t.Fatal(err)
	}
	privateKeys, publicKeys := 0, 0
	for _, key := range persisted.Keys {
		if key.PrivateKeyPEM != "" {
			privateKeys++
		}
		if key.PublicKeyPEM != "" {
			publicKeys++
		}
	}
	if privateKeys != 1 || publicKeys != 1 {
		t.Fatalf("persisted key classes: private=%d public=%d", privateKeys, publicKeys)
	}
	reloaded := &oidcFacade{
		provider: facade.provider, statePath: facade.statePath, runID: facade.runID,
		controlToken: facade.controlToken, issuer: facade.issuer, now: facade.now,
		generateKey: generateRSAKey, codes: make(map[string]oidcAuthorizationCode),
	}
	if err := reloaded.loadOrInitializeKeys(); err != nil {
		t.Fatal(err)
	}
	if reloadedSet := fetchTestJWKS(t, reloaded); len(reloadedSet.Keys) != 2 {
		t.Fatalf("reloaded JWKS = %+v", reloadedSet)
	}
	secondCode := authorizeTestSubject(t, reloaded, "", "")
	secondTokens := exchangeTestCode(t, reloaded, secondCode, testOIDCRedirect, testOIDCClientID, testOIDCSecret, http.StatusOK)
	secondToken := secondTokens["access_token"].(string)
	secondKeyID, err := keyIDFromToken(secondToken)
	if err != nil {
		t.Fatal(err)
	}
	if firstKeyID == secondKeyID {
		t.Fatal("rotation did not change the signing key")
	}
	if _, err := ValidateOIDCToken(firstToken, "at+jwt", testOIDCIssuer, testOIDCAudience, now, rotated); err != nil {
		t.Fatalf("old unexpired token after rotation: %v", err)
	}
	if _, err := ValidateOIDCToken(secondToken, "at+jwt", testOIDCIssuer, testOIDCAudience, now, rotated); err != nil {
		t.Fatalf("new token after rotation: %v", err)
	}

	now = now.Add(6 * time.Minute)
	pruned := fetchTestJWKS(t, reloaded)
	if len(pruned.Keys) != 1 || pruned.Keys[0].KeyID != secondKeyID {
		t.Fatalf("pruned JWKS = %+v", pruned)
	}
}

func TestOIDCRuntimeValidatorUsesBoundDiscoveryAndJWKS(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	facade := newTestOIDCFacade(t, &now)
	code := authorizeTestSubject(t, facade, "", "")
	tokens := exchangeTestCode(t, facade, code, testOIDCRedirect, testOIDCClientID, testOIDCSecret, http.StatusOK)
	client := &http.Client{Transport: oidcHandlerTransport{handler: facade}}
	claims, err := validateOIDCRuntimeTokenWithClient(context.Background(), testOIDCIssuer, tokens["access_token"].(string), "access", testOIDCAudience, now, client)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "identity-engineer" {
		t.Fatalf("validated claims = %+v", claims)
	}
	for _, candidate := range []string{
		"https://attacker.example/jwks",
		testOIDCIssuer + "/other",
		testOIDCIssuer + "/jwks?redirect=https://attacker.example",
	} {
		if err := validateLocalJWKSURI(testOIDCIssuer, candidate); err == nil {
			t.Fatalf("validateLocalJWKSURI(%q) error = nil", candidate)
		}
	}
}

func TestOIDCRuntimeValidatorRefusesDiscoveryRedirects(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://attacker.example/metadata", http.StatusFound)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	_, err = ValidateOIDCRuntimeToken(context.Background(), server.URL, "not-a-token", "access", testOIDCAudience)
	if err == nil || !strings.Contains(err.Error(), "status 302") {
		t.Fatalf("redirect refusal error = %v", err)
	}
}

func TestOIDCRotationRequiresReadyEndpointOwnership(t *testing.T) {
	manager := NewNativeProcessManager(t.TempDir())
	runID := "oidc-ownership-test"
	runtimeDir := manager.runtimeDir(runID, "oidc")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	controlPath := filepath.Join(runtimeDir, "control.json")
	controlData, _ := json.Marshal(NativeRuntimeControl{RunID: runID, ControlToken: "secret"})
	if err := os.WriteFile(controlPath, controlData, 0o600); err != nil {
		t.Fatal(err)
	}
	readyData, _ := json.Marshal(nativeReady{RunID: runID, Endpoint: "http://127.0.0.1:62001", PID: 123})
	if err := os.WriteFile(filepath.Join(runtimeDir, "ready.json"), readyData, 0o600); err != nil {
		t.Fatal(err)
	}
	instances := []Instance{{Provider: "oidc", Engine: "native", Endpoint: "http://127.0.0.1:62002", ControlPath: controlPath}}
	_, err := manager.RotateOIDC(context.Background(), runID, instances)
	if err == nil || !strings.Contains(err.Error(), "endpoint ownership") {
		t.Fatalf("RotateOIDC() ownership error = %v", err)
	}
}

func TestOIDCNativeIntegration(t *testing.T) {
	if os.Getenv("CAILAB_NATIVE_INTEGRATION") != "1" {
		t.Skip("set CAILAB_NATIVE_INTEGRATION=1 to run the native issuer lifecycle")
	}
	compiled := loadOIDCScenario(t)
	manager := NewNativeProcessManager(t.TempDir())
	manager.command = func(executable, providerName, configPath string) *exec.Cmd {
		if providerName != "oidc" {
			t.Fatalf("providerName = %q, want oidc", providerName)
		}
		command := exec.Command(executable, "-test.run=^TestOIDCRuntimeHelper$", "--", configPath)
		command.Env = append(os.Environ(), "CAILAB_OIDC_RUNTIME_HELPER=1")
		return command
	}
	runID := "oidc-native-integration"
	instances, err := manager.Start(context.Background(), runID, compiled)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = manager.Stop(context.Background(), runID, instances, compiled)
		}
	}()
	if len(instances) != 1 || instances[0].Provider != "oidc" {
		t.Fatalf("instances = %+v", instances)
	}
	response, err := http.Get(instances[0].Endpoint + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("discovery status = %d", response.StatusCode)
	}
	rotated, err := manager.RotateOIDC(context.Background(), runID, instances)
	if err != nil {
		t.Fatal(err)
	}
	if len(rotated.Keys) != 2 {
		t.Fatalf("rotated keys = %+v", rotated)
	}
	if _, err := manager.Snapshot(context.Background(), instances, compiled); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(context.Background(), runID, instances, compiled); err != nil {
		t.Fatal(err)
	}
	stopped = true
	if _, err := os.Stat(manager.runtimeDir(runID, "oidc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime directory still exists: %v", err)
	}
}

func TestOIDCRuntimeHelper(t *testing.T) {
	if os.Getenv("CAILAB_OIDC_RUNTIME_HELPER") != "1" {
		return
	}
	configPath := os.Args[len(os.Args)-1]
	if err := ServeOIDCRuntime(context.Background(), configPath); err != nil {
		t.Fatal(err)
	}
}

func newTestOIDCFacade(t *testing.T, now *time.Time) *oidcFacade {
	t.Helper()
	compiled := loadOIDCScenario(t)
	facade := &oidcFacade{
		provider: *compiled.Providers.OIDC, statePath: filepath.Join(t.TempDir(), "oidc-state.json"),
		runID: "oidc-test", controlToken: "control-token", issuer: testOIDCIssuer,
		now: func() time.Time { return *now }, generateKey: generateRSAKey,
		codes: make(map[string]oidcAuthorizationCode),
	}
	if err := facade.loadOrInitializeKeys(); err != nil {
		t.Fatal(err)
	}
	return facade
}

func authorizationURL(redirectURI, subject, state, nonce string) string {
	values := url.Values{
		"response_type": {"code"}, "client_id": {testOIDCClientID}, "redirect_uri": {redirectURI},
		"scope": {"openid profile email"}, "cailab_subject": {subject},
	}
	if state != "" {
		values.Set("state", state)
	}
	if nonce != "" {
		values.Set("nonce", nonce)
	}
	return testOIDCIssuer + "/authorize?" + values.Encode()
}

func authorizeTestSubject(t *testing.T, facade http.Handler, state, nonce string) string {
	t.Helper()
	response := oidcRequest(t, facade, http.MethodGet, authorizationURL(testOIDCRedirect, "identity-engineer", state, nonce), "", "", "")
	if response.Code != http.StatusFound {
		t.Fatalf("authorization status = %d; body=%s", response.Code, response.Body.String())
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if state != "" && location.Query().Get("state") != state {
		t.Fatalf("authorization state = %q", location.Query().Get("state"))
	}
	code := location.Query().Get("code")
	if code == "" {
		t.Fatalf("authorization location = %s", location)
	}
	return code
}

func exchangeTestCode(t *testing.T, facade http.Handler, code, redirectURI, clientID, secret string, status int) map[string]any {
	t.Helper()
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}}
	request := httptest.NewRequest(http.MethodPost, testOIDCIssuer+"/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(clientID, secret)
	response := httptest.NewRecorder()
	facade.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("token status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func fetchTestJWKS(t *testing.T, facade http.Handler) OIDCJWKSet {
	t.Helper()
	response := oidcRequest(t, facade, http.MethodGet, testOIDCIssuer+"/jwks", "", "", "")
	var set OIDCJWKSet
	decodeOIDCResponse(t, response, http.StatusOK, &set)
	return set
}

func oidcRequest(t *testing.T, handler http.Handler, method, target, body, authorization, runID string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if runID != "" {
		request.Header.Set("X-CloudAILab-Run", runID)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeOIDCResponse(t *testing.T, response *httptest.ResponseRecorder, status int, target any) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("response status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}

func loadOIDCScenario(t *testing.T) scenario.Compiled {
	t.Helper()
	definition, err := scenario.Load(filepath.Join("..", "..", "scenarios", "local-oidc", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := scenario.Compile(definition, definition.Spec.Seed)
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}

func keyIDFromToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("JWT must contain three segments")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var header struct {
		KeyID string `json:"kid"`
	}
	if err := json.Unmarshal(data, &header); err != nil || header.KeyID == "" {
		return "", errors.New("JWT kid is missing")
	}
	return header.KeyID, nil
}

type oidcHandlerTransport struct{ handler http.Handler }

func (t oidcHandlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}
