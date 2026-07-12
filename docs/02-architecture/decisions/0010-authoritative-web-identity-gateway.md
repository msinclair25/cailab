---
title: "ADR-0010: Authoritative Web-Identity Gateway"
status: accepted
date: 2026-07-12
---

# ADR-0010: Authoritative web-identity gateway

## Context

The flagship scenario needs a signed local identity to cross Microsoft-shaped application authorization and obtain temporary credentials for an AWS-shaped role. AWS `AssumeRoleWithWebIdentity` normally validates the token signature against the registered issuer's JWKS and applies role trust conditions before returning credentials.

The pinned Floci 1.5.32 runtime exposes `AssumeRoleWithWebIdentity`, but a local compatibility spike submitted `not-a-signed-jwt` to a role whose trust policy contained only AWS principals. Floci returned temporary credentials, a fixed subject, provider, and audience. It therefore emulates response and account-routing behavior but does not provide the token or web-trust authorization required by CloudAILab's scenario.

## Decision

CloudAILab will own the authoritative web-identity decision before invoking Floci:

1. Load only the active run's recorded OIDC, Microsoft, and AWS endpoints.
2. Refuse redirects and validate the JWT through exact active-issuer discovery and same-origin JWKS.
3. Pin RS256, `at+jwt`, expiry, issuer, audience, token ID, subject, client ID, tenant, and canonical-principal claims.
4. Require the token client and audience to match a typed scenario OIDC client.
5. Require at least one token group to retain a live Microsoft app-role assignment to that client service principal.
6. Require the target AWS role's typed web-identity trust to match the OIDC client, audience value, and canonical audience node.
7. Only after those deterministic checks pass, call Floci `AssumeRoleWithWebIdentity` to obtain run-local temporary credentials.

The gateway returns credentials only to an explicitly requested owner-only output file. It never treats Floci's fixed web-identity subject/provider/audience response fields as authorization evidence.

Canonical normalization mirrors the same decision inputs: live Google membership, explicit directory synchronization, live Microsoft app-role assignment, OIDC client/audience registration, typed AWS web trust, and role access policy. Verification remains authoritative even when the emulator is permissive.

## Consequences

### Positive

- Invalid, expired, tampered, wrong-audience, unassigned, or untrusted tokens are denied before reaching the emulator.
- The executable exchange and canonical graph use the same provider-native identifiers and trust inputs.
- Floci remains useful for temporary credentials, account routing, and S3 access without becoming a false authorization oracle.
- The measured fidelity gap is explicit and regression-tested.

### Negative

- `cailab federation assume-aws` is a CloudAILab gateway, not an AWS endpoint or transparent SDK credential provider.
- Direct calls to Floci `AssumeRoleWithWebIdentity` remain permissive and must not be used as security evidence.
- The first gateway supports one local issuer, one Microsoft app-role hop, one configured AWS role, and one default audience per client.
- Production AWS OIDC-provider creation, HTTPS trust, IAM condition evaluation, session tags, and source identity remain outside this local profile.

## Validation

- A regression test proves an invalid token is rejected by CloudAILab even though the pinned emulator accepts it.
- Positive tests require a valid signature, exact audience/client, live Microsoft assignment, and matching AWS trust.
- Negative tests remove the risky Microsoft assignment and prove the contractor exchange is denied while the approved subject still succeeds.
- Temporary credentials retrieve only the configured synthetic S3 object through the pinned Floci runtime.
- Canonical verification closes the same contractor path while preserving the approved path.

## Sources

- [AWS temporary credentials through OIDC](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_request.html)
- [AWS OIDC federated principals](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html)
- [AWS CLI web-identity role configuration](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-role.html)
- [Floci STS](https://floci.io/floci/services/sts/)
- [Floci multi-account isolation](https://floci.io/floci/configuration/multi-account/)
