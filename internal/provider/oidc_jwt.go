package provider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const oidcSigningAlgorithm = "RS256"

type OIDCJWK struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

type OIDCJWKSet struct {
	Keys []OIDCJWK `json:"keys"`
}

type OIDCClaims struct {
	Issuer      string            `json:"iss"`
	Subject     string            `json:"sub"`
	Audience    OIDCAudienceClaim `json:"aud"`
	ExpiresAt   int64             `json:"exp"`
	IssuedAt    int64             `json:"iat"`
	TokenID     string            `json:"jti"`
	Nonce       string            `json:"nonce,omitempty"`
	ClientID    string            `json:"client_id,omitempty"`
	Scope       string            `json:"scope,omitempty"`
	Tenant      string            `json:"tenant,omitempty"`
	PrincipalID string            `json:"principal_id,omitempty"`
	Email       string            `json:"email,omitempty"`
	Groups      []string          `json:"groups,omitempty"`
}

type OIDCAudienceClaim []string

func (a *OIDCAudienceClaim) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return errors.New("aud must be a string or string array")
	}
	*a = multiple
	return nil
}

func generateRSAKey() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA signing key: %w", err)
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("validate RSA signing key: %w", err)
	}
	return key, nil
}

func publicJWK(key *rsa.PublicKey) (OIDCJWK, error) {
	if key == nil || key.N == nil || key.N.Sign() <= 0 || key.E <= 0 {
		return OIDCJWK{}, errors.New("RSA public key is invalid")
	}
	jwk := OIDCJWK{
		KeyType: "RSA", Use: "sig", Algorithm: oidcSigningAlgorithm,
		Modulus:  base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		Exponent: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
	thumbprintJSON, err := json.Marshal(struct {
		Exponent string `json:"e"`
		KeyType  string `json:"kty"`
		Modulus  string `json:"n"`
	}{Exponent: jwk.Exponent, KeyType: jwk.KeyType, Modulus: jwk.Modulus})
	if err != nil {
		return OIDCJWK{}, fmt.Errorf("encode JWK thumbprint input: %w", err)
	}
	sum := sha256.Sum256(thumbprintJSON)
	jwk.KeyID = base64.RawURLEncoding.EncodeToString(sum[:])
	return jwk, nil
}

func signJWT(key *rsa.PrivateKey, tokenType string, claims OIDCClaims) (string, error) {
	jwk, err := publicJWK(&key.PublicKey)
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]string{"alg": oidcSigningAlgorithm, "kid": jwk.KeyID, "typ": tokenType})
	if err != nil {
		return "", fmt.Errorf("encode JWT header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode JWT claims: %w", err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func ValidateOIDCToken(token, expectedType, expectedIssuer, expectedAudience string, now time.Time, keys OIDCJWKSet) (OIDCClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return OIDCClaims{}, errors.New("JWT must contain three segments")
	}
	headerData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return OIDCClaims{}, errors.New("JWT header is not valid base64url")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerData, &header); err != nil {
		return OIDCClaims{}, errors.New("JWT header is not valid JSON")
	}
	if subtle.ConstantTimeCompare([]byte(header.Algorithm), []byte(oidcSigningAlgorithm)) != 1 {
		return OIDCClaims{}, fmt.Errorf("JWT algorithm %q is not allowed", header.Algorithm)
	}
	if header.Type != expectedType {
		return OIDCClaims{}, fmt.Errorf("JWT type %q does not match %q", header.Type, expectedType)
	}
	if header.KeyID == "" {
		return OIDCClaims{}, errors.New("JWT kid is required")
	}
	var selected *OIDCJWK
	for i := range keys.Keys {
		if keys.Keys[i].KeyID == header.KeyID {
			if selected != nil {
				return OIDCClaims{}, errors.New("JWKS contains duplicate kid values")
			}
			selected = &keys.Keys[i]
		}
	}
	if selected == nil {
		return OIDCClaims{}, fmt.Errorf("JWT kid %q is not present in JWKS", header.KeyID)
	}
	publicKey, err := rsaKeyFromJWK(*selected)
	if err != nil {
		return OIDCClaims{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return OIDCClaims{}, errors.New("JWT signature is not valid base64url")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return OIDCClaims{}, errors.New("JWT signature validation failed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return OIDCClaims{}, errors.New("JWT payload is not valid base64url")
	}
	var claims OIDCClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return OIDCClaims{}, errors.New("JWT claims are not valid JSON")
	}
	if claims.Issuer != expectedIssuer {
		return OIDCClaims{}, errors.New("JWT issuer does not match expected issuer")
	}
	if claims.Subject == "" || claims.TokenID == "" {
		return OIDCClaims{}, errors.New("JWT sub and jti claims are required")
	}
	if !oidcContains([]string(claims.Audience), expectedAudience) {
		return OIDCClaims{}, errors.New("JWT audience does not include expected recipient")
	}
	unixNow := now.Unix()
	if claims.ExpiresAt <= unixNow {
		return OIDCClaims{}, errors.New("JWT is expired")
	}
	if claims.IssuedAt > unixNow+30 {
		return OIDCClaims{}, errors.New("JWT iat is in the future")
	}
	if claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return OIDCClaims{}, errors.New("JWT time claims are invalid")
	}
	return claims, nil
}

func oidcContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func rsaKeyFromJWK(jwk OIDCJWK) (*rsa.PublicKey, error) {
	if jwk.KeyType != "RSA" || jwk.Use != "sig" || jwk.Algorithm != oidcSigningAlgorithm {
		return nil, errors.New("JWK is not an RS256 signature key")
	}
	modulus, err := base64.RawURLEncoding.DecodeString(jwk.Modulus)
	if err != nil || len(modulus) == 0 {
		return nil, errors.New("JWK modulus is invalid")
	}
	exponent, err := base64.RawURLEncoding.DecodeString(jwk.Exponent)
	if err != nil || len(exponent) == 0 || len(exponent) > 4 {
		return nil, errors.New("JWK exponent is invalid")
	}
	e := 0
	for _, value := range exponent {
		e = e<<8 | int(value)
	}
	if e < 3 || e%2 == 0 {
		return nil, errors.New("JWK exponent is invalid")
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: e}
	if key.Size() < 256 {
		return nil, errors.New("JWK RSA key must be at least 2048 bits")
	}
	return key, nil
}
