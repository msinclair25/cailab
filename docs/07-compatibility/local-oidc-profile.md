---
title: Local Development OIDC Profile Compatibility Matrix
status: m2-development
last_reviewed: 2026-07-12
profile_version: 0.1.0
---

# Local development OIDC profile compatibility matrix

## Claim boundary

CloudAILab implements a deliberately scoped local development identity issuer for the `local-oidc` scenario. It uses standard-shaped OpenID Connect Discovery, OAuth Authorization Code, JWKS, ID-token, and JWT access-token fields. It is **not a conformant production OpenID Provider**: the random issuer uses HTTP on IPv4 loopback, and `cailab_subject` selects a declared synthetic identity without authenticating a human.

The authoritative tests are `TestOIDCAuthorizationCodeContract`, `TestOIDCRejectsRedirectClientAndTokenTampering`, `TestOIDCRotationRetainsUnexpiredKeys`, `TestOIDCRuntimeValidatorUsesBoundDiscoveryAndJWKS`, and the cross-platform `TestOIDCNativeIntegration` in [`internal/provider`](../../internal/provider).

## Runtime and transport

| Capability | Fidelity | Evidence | Limits |
|---|---|---|---|
| Managed native issuer | CloudAILab behavior/security contract | Shared native lifecycle and compiled-binary tests | Runs as the current OS user; not an agent sandbox. |
| Random IPv4 loopback endpoint | Intentional development divergence | Binding and endpoint-ownership tests | HTTP violates the HTTPS requirements of OIDC Discovery, RFC 8414, and OAuth token transport. Never expose it as a network service. |
| Owner-only signing state | CloudAILab security contract | File-mode, persistence, rotation, reset, and cleanup tests | Private keys are run-scoped local files, not HSM-backed. |
| Reset and cleanup | CloudAILab behavior/security contract | Native lifecycle integration | Reset creates a new issuer URL and key; existing tokens from the old issuer no longer belong to the active run. |

## Protocol operations

| Operation | Supported contract | Known limits |
|---|---|---|
| Discovery | `GET /.well-known/openid-configuration` and `GET /.well-known/oauth-authorization-server`; exact `issuer`, authorization/token endpoints, `jwks_uri`, supported flow, scopes, auth method, subject type, and RS256 | No WebFinger, dynamic registration, UserInfo, logout, revocation, or introspection. |
| JWKS | `GET /jwks`; RSA `kty`, `sig` use, RS256 `alg`, RFC 7638-derived unique `kid`, modulus, and exponent | RSA 2048 only; no certificates, encryption keys, or algorithm negotiation. |
| Authorization | `GET /authorize`; `response_type=code`, declared client, exact registered redirect, `openid` scope, optional state/nonce, declared `cailab_subject`; 302 response | `cailab_subject` is synthetic selection, not authentication. No login/consent UI, PKCE, prompt, claims parameter, response modes, or hybrid/implicit flow. |
| Token | `POST /token`; form-encoded Authorization Code grant, `client_secret_basic`, one-time code, client and redirect binding | No public clients, refresh tokens, client credentials, token exchange, private-key JWT, or mTLS. |
| ID token | RS256 compact JWT with `typ=JWT`, issuer, subject, client audience, expiry, issued-at, token ID, nonce, tenant, canonical principal, and scope-controlled email/groups | Public subjects only; no `auth_time`, `acr`, `amr`, `azp`, pairwise subjects, encryption, or distributed claims. |
| Access token | RFC 9068-shaped RS256 compact JWT with `typ=at+jwt`, one default resource audience, subject, expiry, issued-at, token ID, client ID, scope, tenant, and canonical principal | No authorization-server access-policy engine or token introspection. The canonical graph remains authoritative for scenario verification. |
| Rotation | `cailab identity rotate`; retired private keys are discarded, new tokens use a new `kid`, and retired public keys remain published through the maximum prior token lifetime plus 30 seconds | Rotation is local run administration, not an OIDC endpoint. No scheduled or external KMS rotation. |
| Validation | `cailab identity validate`; refuses HTTP redirects and pins active loopback discovery, exact issuer, same-origin `/jwks`, RS256, token type, signature, audience, issue/expiry times, subject, and token ID | Validates only this profile; it is not a general JWT or OIDC validation utility. |

## OAuth errors and security behavior

- Token errors use `error` and `error_description` with `invalid_request`, `invalid_client`, `invalid_grant`, `unsupported_grant_type`, or `server_error` as applicable.
- Basic-authentication failures return HTTP 401 and `WWW-Authenticate`; malformed or invalid grants return HTTP 400.
- Redirect URIs are scenario-validated IPv4 loopback HTTP URLs with explicit ports and are compared as exact strings.
- Authorization codes contain 256 random bits, expire after the scenario-bounded lifetime, are held only in memory, and are deleted on successful client-authenticated redemption attempts.
- Scenarios cannot choose signing algorithms, key sizes, arbitrary JWT claims, listeners, or external redirect origins.
- Tokens and client secrets are synthetic but should still be kept out of logs and shared terminals to preserve the learning exercise.

## Primary sources

- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
- [RFC 8414: OAuth 2.0 Authorization Server Metadata](https://www.rfc-editor.org/rfc/rfc8414)
- [RFC 6749: OAuth 2.0](https://www.rfc-editor.org/rfc/rfc6749)
- [RFC 7517: JSON Web Key](https://www.rfc-editor.org/rfc/rfc7517)
- [RFC 7519: JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519)
- [RFC 7638: JWK Thumbprint](https://www.rfc-editor.org/rfc/rfc7638)
- [RFC 8725: JWT Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725)
- [RFC 9068: JWT Profile for OAuth Access Tokens](https://www.rfc-editor.org/rfc/rfc9068)
