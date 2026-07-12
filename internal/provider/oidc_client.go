package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func ValidateOIDCRuntimeToken(ctx context.Context, endpoint, token, tokenType, audience string) (OIDCClaims, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return validateOIDCRuntimeTokenWithClient(ctx, endpoint, token, tokenType, audience, time.Now(), client)
}

func validateOIDCRuntimeTokenWithClient(ctx context.Context, endpoint, token, tokenType, audience string, now time.Time, client *http.Client) (OIDCClaims, error) {
	endpoint = strings.TrimRight(endpoint, "/")
	if !isIPv4LoopbackEndpoint(endpoint) {
		return OIDCClaims{}, errors.New("OIDC endpoint must be an IPv4 loopback HTTP origin")
	}
	metadataURL := endpoint + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return OIDCClaims{}, fmt.Errorf("build OIDC discovery request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return OIDCClaims{}, fmt.Errorf("retrieve OIDC discovery: %w", err)
	}
	var metadata struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&metadata)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return OIDCClaims{}, fmt.Errorf("retrieve OIDC discovery: status %d", response.StatusCode)
	}
	if decodeErr != nil {
		return OIDCClaims{}, fmt.Errorf("decode OIDC discovery: %w", decodeErr)
	}
	if metadata.Issuer != endpoint {
		return OIDCClaims{}, errors.New("OIDC discovery issuer does not exactly match the active endpoint")
	}
	if err := validateLocalJWKSURI(endpoint, metadata.JWKSURI); err != nil {
		return OIDCClaims{}, err
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, metadata.JWKSURI, nil)
	if err != nil {
		return OIDCClaims{}, fmt.Errorf("build JWKS request: %w", err)
	}
	response, err = client.Do(request)
	if err != nil {
		return OIDCClaims{}, fmt.Errorf("retrieve JWKS: %w", err)
	}
	var set OIDCJWKSet
	decodeErr = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&set)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return OIDCClaims{}, fmt.Errorf("retrieve JWKS: status %d", response.StatusCode)
	}
	if decodeErr != nil {
		return OIDCClaims{}, fmt.Errorf("decode JWKS: %w", decodeErr)
	}
	expectedType := ""
	switch tokenType {
	case "id":
		expectedType = "JWT"
	case "access":
		expectedType = "at+jwt"
	default:
		return OIDCClaims{}, fmt.Errorf("unsupported token type %q", tokenType)
	}
	return ValidateOIDCToken(strings.TrimSpace(token), expectedType, metadata.Issuer, audience, now, set)
}

func validateLocalJWKSURI(endpoint, jwksURI string) error {
	base, err := url.Parse(endpoint)
	if err != nil {
		return errors.New("active OIDC endpoint is invalid")
	}
	parsed, err := url.Parse(jwksURI)
	if err != nil || parsed.Scheme != base.Scheme || parsed.Host != base.Host || parsed.Path != "/jwks" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return errors.New("OIDC discovery jwks_uri must be the active loopback issuer's /jwks endpoint")
	}
	return nil
}
