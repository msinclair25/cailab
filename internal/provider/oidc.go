package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/msinclair25/cailab/internal/scenario"
)

type OIDCRuntimeConfig struct {
	NativeRuntimeControl
	Provider scenario.OIDCProvider `json:"provider"`
}

type oidcAuthorizationCode struct {
	ClientID    string
	RedirectURI string
	Subject     string
	Scopes      []string
	Nonce       string
	ExpiresAt   time.Time
}

type oidcFacade struct {
	mu           sync.Mutex
	provider     scenario.OIDCProvider
	statePath    string
	runID        string
	controlToken string
	issuer       string
	shutdown     func()
	now          func() time.Time
	generateKey  func() (*rsa.PrivateKey, error)
	codes        map[string]oidcAuthorizationCode
	keys         []oidcSigningKey
}

// ServeOIDCRuntime runs the private child-process entrypoint used by the native facade manager.
func ServeOIDCRuntime(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read OIDC runtime config: %w", err)
	}
	var config OIDCRuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("decode OIDC runtime config: %w", err)
	}
	if config.RunID == "" || config.StatePath == "" || config.ReadyPath == "" || config.ControlToken == "" {
		return errors.New("OIDC runtime config is incomplete")
	}
	if config.Listen == "" {
		config.Listen = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp4", config.Listen)
	if err != nil {
		return fmt.Errorf("listen for OIDC issuer: %w", err)
	}
	defer listener.Close()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return fmt.Errorf("resolve OIDC issuer address: %w", err)
	}
	if host != "127.0.0.1" {
		return fmt.Errorf("OIDC issuer must bind to IPv4 loopback, got %q", host)
	}
	endpoint := "http://127.0.0.1:" + port
	runtimeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	facade := &oidcFacade{
		provider: config.Provider, statePath: config.StatePath, runID: config.RunID,
		controlToken: config.ControlToken, issuer: endpoint, shutdown: cancel,
		now: time.Now, generateKey: generateRSAKey, codes: make(map[string]oidcAuthorizationCode),
	}
	if err := facade.loadOrInitializeKeys(); err != nil {
		return err
	}
	server := &http.Server{
		Handler: facade, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
	}
	readyData, err := json.Marshal(nativeReady{RunID: config.RunID, Endpoint: endpoint, PID: os.Getpid()})
	if err != nil {
		return fmt.Errorf("encode OIDC readiness: %w", err)
	}
	if err := os.WriteFile(config.ReadyPath, readyData, 0o600); err != nil {
		return fmt.Errorf("write OIDC readiness: %w", err)
	}
	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()
	select {
	case <-runtimeCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down OIDC issuer: %w", err)
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}

func (f *oidcFacade) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	switch r.URL.Path {
	case "/_cailab/health":
		f.handleHealth(w, r)
	case "/_cailab/shutdown":
		f.handleShutdown(w, r)
	case "/_cailab/rotate":
		f.handleRotate(w, r)
	case "/_cailab/reset":
		f.handleReset(w, r)
	case "/.well-known/openid-configuration", "/.well-known/oauth-authorization-server":
		f.handleDiscovery(w, r)
	case "/jwks":
		f.handleJWKS(w, r)
	case "/authorize":
		f.handleAuthorize(w, r)
	case "/token":
		f.handleToken(w, r)
	default:
		writeOAuthError(w, http.StatusNotFound, "invalid_request", "The requested local issuer resource was not found.")
	}
}

func (f *oidcFacade) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed.")
		return
	}
	writeJSONContent(w, http.StatusOK, map[string]any{"ready": true, "runId": f.runID})
}

func (f *oidcFacade) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed.")
		return
	}
	if !f.validControlRequest(r) {
		writeOAuthError(w, http.StatusForbidden, "access_denied", "Invalid runtime control credentials.")
		return
	}
	writeJSONContent(w, http.StatusAccepted, map[string]any{"status": "stopping"})
	go f.shutdown()
}

func (f *oidcFacade) handleRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed.")
		return
	}
	if !f.validControlRequest(r) {
		writeOAuthError(w, http.StatusForbidden, "access_denied", "Invalid runtime control credentials.")
		return
	}
	key, err := f.generateKey()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not generate a signing key.")
		return
	}
	jwk, err := publicJWK(&key.PublicKey)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not prepare the signing key.")
		return
	}
	f.mu.Lock()
	now := f.now()
	f.pruneLocked(now)
	previous := append([]oidcSigningKey(nil), f.keys...)
	for i := range f.keys {
		if f.keys[i].RetireAt.IsZero() {
			f.keys[i].RetireAt = now.Add(time.Duration(f.provider.TokenTTLSeconds+30) * time.Second)
			f.keys[i].PublicKey = &f.keys[i].PrivateKey.PublicKey
			f.keys[i].PrivateKey = nil
		}
	}
	f.keys = append(f.keys, oidcSigningKey{PrivateKey: key, PublicKey: &key.PublicKey, KeyID: jwk.KeyID})
	err = f.persistLocked()
	if err != nil {
		f.keys = previous
	}
	set := f.jwksLocked(now)
	f.mu.Unlock()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not persist key rotation.")
		return
	}
	writeJSONContent(w, http.StatusOK, set)
}

func (f *oidcFacade) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed.")
		return
	}
	if !f.validControlRequest(r) {
		writeOAuthError(w, http.StatusForbidden, "access_denied", "Invalid runtime control credentials.")
		return
	}
	key, err := f.generateKey()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not refresh the signing key.")
		return
	}
	jwk, err := publicJWK(&key.PublicKey)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not prepare the signing key.")
		return
	}
	f.mu.Lock()
	previousKeys := f.keys
	previousCodes := f.codes
	f.keys = []oidcSigningKey{{PrivateKey: key, PublicKey: &key.PublicKey, KeyID: jwk.KeyID}}
	f.codes = make(map[string]oidcAuthorizationCode)
	err = f.persistLocked()
	if err != nil {
		f.keys = previousKeys
		f.codes = previousCodes
	}
	f.mu.Unlock()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not persist the restored issuer state.")
		return
	}
	writeJSONContent(w, http.StatusOK, map[string]any{"status": "restored"})
}

func (f *oidcFacade) validControlRequest(r *http.Request) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+f.controlToken)) == 1 && r.Header.Get("X-CloudAILab-Run") == f.runID
}

func (f *oidcFacade) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.RawQuery != "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Discovery requires GET without query parameters.")
		return
	}
	writeJSONContent(w, http.StatusOK, map[string]any{
		"issuer": f.issuer, "authorization_endpoint": f.issuer + "/authorize",
		"token_endpoint": f.issuer + "/token", "jwks_uri": f.issuer + "/jwks",
		"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code"},
		"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{oidcSigningAlgorithm},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
	})
}

func (f *oidcFacade) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.RawQuery != "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "JWKS requires GET without query parameters.")
		return
	}
	f.mu.Lock()
	now := f.now()
	f.pruneLocked(now)
	set := f.jwksLocked(now)
	f.mu.Unlock()
	writeJSONContent(w, http.StatusOK, set)
}

func (f *oidcFacade) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Authorization requires GET.")
		return
	}
	query := r.URL.Query()
	if err := validateSingleValues(query, map[string]bool{"response_type": true, "client_id": true, "redirect_uri": true, "scope": true, "state": true, "nonce": true, "cailab_subject": true}); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	client, ok := f.client(query.Get("client_id"))
	if !ok {
		writeOAuthError(w, http.StatusBadRequest, "unauthorized_client", "The client is not declared by this scenario.")
		return
	}
	redirectURI := query.Get("redirect_uri")
	if !oidcContains(client.RedirectURIs, redirectURI) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "redirect_uri does not exactly match a registered URI.")
		return
	}
	state := query.Get("state")
	if len(state) > 256 || len(query.Get("nonce")) > 256 {
		f.redirectAuthorizationError(w, redirectURI, state, "invalid_request", "state and nonce are limited to 256 bytes.")
		return
	}
	if query.Get("response_type") != "code" {
		f.redirectAuthorizationError(w, redirectURI, state, "unsupported_response_type", "Only response_type=code is supported.")
		return
	}
	scopes, err := validateRequestedScopes(query.Get("scope"), client.Scopes)
	if err != nil {
		f.redirectAuthorizationError(w, redirectURI, state, "invalid_scope", err.Error())
		return
	}
	subject, ok := f.subject(query.Get("cailab_subject"))
	if !ok {
		f.redirectAuthorizationError(w, redirectURI, state, "access_denied", "The synthetic subject is not declared by this scenario.")
		return
	}
	code, err := randomOpaqueValue(32)
	if err != nil {
		f.redirectAuthorizationError(w, redirectURI, state, "server_error", "Could not create an authorization code.")
		return
	}
	f.mu.Lock()
	now := f.now()
	f.pruneCodesLocked(now)
	f.codes[code] = oidcAuthorizationCode{
		ClientID: client.ClientID, RedirectURI: redirectURI, Subject: subject.Subject,
		Scopes: scopes, Nonce: query.Get("nonce"), ExpiresAt: now.Add(time.Duration(f.provider.CodeTTLSeconds) * time.Second),
	}
	f.mu.Unlock()
	location, _ := url.Parse(redirectURI)
	values := location.Query()
	values.Set("code", code)
	if state != "" {
		values.Set("state", state)
	}
	location.RawQuery = values.Encode()
	http.Redirect(w, r, location.String(), http.StatusFound)
}

func (f *oidcFacade) redirectAuthorizationError(w http.ResponseWriter, redirectURI, state, code, description string) {
	location, _ := url.Parse(redirectURI)
	values := location.Query()
	values.Set("error", code)
	values.Set("error_description", description)
	if state != "" {
		values.Set("state", state)
	}
	location.RawQuery = values.Encode()
	w.Header().Set("Location", location.String())
	w.WriteHeader(http.StatusFound)
}

func (f *oidcFacade) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.RawQuery != "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Token requests require POST without query parameters.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Content-Type must be application/x-www-form-urlencoded.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "The token request form is invalid.")
		return
	}
	if err := validateSingleValues(r.PostForm, map[string]bool{"grant_type": true, "code": true, "redirect_uri": true}); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := requireParameters(r.PostForm, "grant_type", "code", "redirect_uri"); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	clientID, clientSecret, ok := r.BasicAuth()
	client, clientOK := f.client(clientID)
	if !ok || !clientOK || subtle.ConstantTimeCompare([]byte(clientSecret), []byte(client.ClientSecret)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="CloudAILab OIDC"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed.")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "Only authorization_code is supported.")
		return
	}
	codeValue := r.PostForm.Get("code")
	f.mu.Lock()
	now := f.now()
	f.pruneCodesLocked(now)
	code, exists := f.codes[codeValue]
	if exists {
		delete(f.codes, codeValue)
	}
	f.mu.Unlock()
	if !exists || !now.Before(code.ExpiresAt) || code.ClientID != clientID || code.RedirectURI != r.PostForm.Get("redirect_uri") {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "The authorization code is invalid, expired, consumed, or not bound to this client and redirect URI.")
		return
	}
	subject, subjectOK := f.subject(code.Subject)
	if !subjectOK {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "The authorization code subject is no longer available.")
		return
	}
	response, err := f.issueTokens(now, client, subject, code)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "Could not issue signed tokens.")
		return
	}
	writeJSONContent(w, http.StatusOK, response)
}

func (f *oidcFacade) issueTokens(now time.Time, client scenario.OIDCClient, subject scenario.OIDCSubject, code oidcAuthorizationCode) (map[string]any, error) {
	f.mu.Lock()
	f.pruneLocked(now)
	if len(f.keys) == 0 {
		f.mu.Unlock()
		return nil, errors.New("no active signing key")
	}
	key := f.keys[len(f.keys)-1].PrivateKey
	f.mu.Unlock()
	if key == nil {
		return nil, errors.New("active signing key has no private key")
	}
	idJTI, err := randomOpaqueValue(16)
	if err != nil {
		return nil, err
	}
	accessJTI, err := randomOpaqueValue(16)
	if err != nil {
		return nil, err
	}
	expires := now.Add(time.Duration(f.provider.TokenTTLSeconds) * time.Second).Unix()
	base := OIDCClaims{
		Issuer: f.issuer, Subject: subject.Subject, ExpiresAt: expires, IssuedAt: now.Unix(),
		Tenant: f.provider.Tenant, PrincipalID: subject.Node,
	}
	idClaims := base
	idClaims.Audience = OIDCAudienceClaim{client.ClientID}
	idClaims.TokenID = idJTI
	idClaims.Nonce = code.Nonce
	if oidcContains(code.Scopes, "email") {
		idClaims.Email = subject.Email
	}
	if oidcContains(code.Scopes, "profile") {
		idClaims.Groups = append([]string(nil), subject.Groups...)
	}
	idToken, err := signJWT(key, "JWT", idClaims)
	if err != nil {
		return nil, err
	}
	accessClaims := base
	accessClaims.Audience = OIDCAudienceClaim{client.Audiences[0].Value}
	accessClaims.TokenID = accessJTI
	accessClaims.ClientID = client.ClientID
	accessClaims.Scope = strings.Join(code.Scopes, " ")
	if oidcContains(code.Scopes, "email") {
		accessClaims.Email = subject.Email
	}
	if oidcContains(code.Scopes, "profile") {
		accessClaims.Groups = append([]string(nil), subject.Groups...)
	}
	accessToken, err := signJWT(key, "at+jwt", accessClaims)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"access_token": accessToken, "token_type": "Bearer", "expires_in": f.provider.TokenTTLSeconds,
		"scope": strings.Join(code.Scopes, " "), "id_token": idToken,
	}, nil
}

func (f *oidcFacade) client(clientID string) (scenario.OIDCClient, bool) {
	for _, client := range f.provider.Clients {
		if client.ClientID == clientID {
			return client, true
		}
	}
	return scenario.OIDCClient{}, false
}

func (f *oidcFacade) subject(subjectID string) (scenario.OIDCSubject, bool) {
	for _, subject := range f.provider.Subjects {
		if subject.Subject == subjectID {
			return subject, true
		}
	}
	return scenario.OIDCSubject{}, false
}

func validateSingleValues(values url.Values, allowed map[string]bool) error {
	for key, entries := range values {
		if !allowed[key] {
			return fmt.Errorf("parameter %q is not supported", key)
		}
		if len(entries) != 1 || entries[0] == "" {
			return fmt.Errorf("parameter %q must occur exactly once with a value", key)
		}
	}
	return nil
}

func requireParameters(values url.Values, required ...string) error {
	for _, key := range required {
		if values.Get(key) == "" {
			return fmt.Errorf("parameter %q is required", key)
		}
	}
	return nil
}

func validateRequestedScopes(raw string, allowed []string) ([]string, error) {
	requested := strings.Fields(raw)
	if len(requested) == 0 || !oidcContains(requested, "openid") {
		return nil, errors.New("scope must include openid")
	}
	seen := make(map[string]struct{})
	for _, scope := range requested {
		if !oidcContains(allowed, scope) {
			return nil, fmt.Errorf("scope %q is not allowed for this client", scope)
		}
		if _, exists := seen[scope]; exists {
			return nil, fmt.Errorf("scope %q is duplicated", scope)
		}
		seen[scope] = struct{}{}
	}
	sort.Strings(requested)
	return requested, nil
}

func randomOpaqueValue(size int) (string, error) {
	data := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	writeJSONContent(w, status, map[string]any{"error": code, "error_description": description})
}

func writeJSONContent(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (f *oidcFacade) pruneCodesLocked(now time.Time) {
	for code, value := range f.codes {
		if !now.Before(value.ExpiresAt) {
			delete(f.codes, code)
		}
	}
}
