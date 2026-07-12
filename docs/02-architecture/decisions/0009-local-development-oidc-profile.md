---
title: "ADR-0009: Local Development OIDC Profile"
status: accepted
date: 2026-07-12
---

# ADR-0009: Local development OIDC profile

## Context

M2 needs signed local identities that future Microsoft- and AWS-shaped trust contracts can validate without a cloud account or hosted identity provider. Discovery metadata, JWKS, issuer consistency, audiences, expiry, and key rotation are security-sensitive and must be testable. A token-mint endpoint with arbitrary claims would bypass authentication semantics and make confused-deputy exercises less credible.

OpenID Connect Discovery and OAuth Authorization Server Metadata require HTTPS issuer identifiers and TLS-protected endpoints. CloudAILab's default local mode intentionally uses random IPv4 loopback HTTP so it can avoid host certificate installation and global trust changes. The default mode therefore cannot claim production OpenID Provider conformance.

## Decision

CloudAILab will implement a deliberately scoped **local development OIDC profile** as a managed native process. It will provide:

- OpenID Connect and OAuth authorization-server metadata at their standard well-known paths;
- an RSA 2048-bit JWKS and RS256 signatures;
- a synthetic Authorization Code flow for declared subjects and confidential clients;
- exact registered redirect-URI matching and HTTP Basic client authentication;
- one-time, short-lived authorization codes;
- short-lived ID tokens and RFC 9068-shaped JWT access tokens with fixed issuer, subject, audience, expiry, issued-at, token ID, client, scope, tenant, and canonical-principal claims;
- owner-authorized key rotation that discards retired private keys and retains their public keys until every token they could have signed has expired;
- strict issuer, algorithm, type, signature, time, and audience validation in the reference verifier.

Subjects, clients, redirect URIs, audiences, scopes, and lifetimes are typed scenario data. Scenario text cannot choose signing algorithms, key sizes, arbitrary claims, network listeners, or executable hooks.

The authorization endpoint's `cailab_subject` parameter is a training-only subject selector, not authentication. All credentials and tokens are synthetic. Compatibility documentation must distinguish standard-shaped protocol fields from this non-production authentication and transport profile.

## Consequences

### Positive

- Future federation scenarios can validate signed claims against a discoverable key set.
- Authorization codes, client binding, redirect binding, audiences, expiry, and rollover are executable learning concepts.
- The default remains one binary with no certificate or proxy changes.
- Keys and codes remain run-scoped and lifecycle-owned.

### Negative

- Loopback HTTP diverges from the HTTPS requirements in OIDC Discovery, OAuth metadata, and OAuth token transport.
- `cailab_subject` lets an authorized synthetic client select any subject declared for that scenario; it is not proof that a human authenticated.
- The first slice omits login UI, consent UI, PKCE, refresh tokens, UserInfo, logout, dynamic registration, token revocation, and token introspection.
- Broad OIDC SDK compatibility cannot be claimed until each client accepts the documented development issuer and passes a contract test.

## Validation

- Contract tests cover discovery, issuer equality, JWKS fields, authorization redirects, client authentication, one-time code use, exact redirect matching, signed claim sets, expiry, audience rejection, signature rejection, and OAuth-shaped errors.
- Rotation tests prove newly issued tokens use a new key while unexpired older tokens remain verifiable.
- Native lifecycle tests cover readiness, reset, authenticated shutdown, and run-directory cleanup on Linux, macOS, and Windows.
- The compatibility matrix labels loopback HTTP and synthetic subject selection as explicit non-conformities.

## Sources

- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
- [RFC 8414: OAuth 2.0 Authorization Server Metadata](https://www.rfc-editor.org/rfc/rfc8414)
- [RFC 6749: The OAuth 2.0 Authorization Framework](https://www.rfc-editor.org/rfc/rfc6749)
- [RFC 7517: JSON Web Key](https://www.rfc-editor.org/rfc/rfc7517)
- [RFC 7519: JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519)
- [RFC 7638: JSON Web Key Thumbprint](https://www.rfc-editor.org/rfc/rfc7638)
- [RFC 8725: JSON Web Token Best Current Practices](https://www.rfc-editor.org/rfc/rfc8725)
- [RFC 9068: JWT Profile for OAuth 2.0 Access Tokens](https://www.rfc-editor.org/rfc/rfc9068)
