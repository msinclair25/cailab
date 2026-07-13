---
title: Cross-Provider Federation Compatibility Matrix
status: m2-complete
last_reviewed: 2026-07-13
scenario_version: acquisition-agent@0.1.0
---

# Cross-provider federation compatibility matrix

## Claim boundary

CloudAILab supports one deliberately scoped Google → Microsoft → local OIDC → AWS federation workflow for `acquisition-agent@0.1.0`. It does not emulate Google Cloud Directory Sync, Microsoft federation services, an AWS IAM OIDC provider resource, or general identity brokering. Each edge below has a distinct source of truth and fidelity claim.

The authoritative lifecycle exercise is `TestCrossProviderFederationIntegration`; the hermetic authorization regression is `TestFlagshipCrossProviderPathAndRemediation` in [`internal/provider`](../../internal/provider), and `TestCrossProviderCLIE2E` exercises the documented public commands in [`internal/cli`](../../internal/cli).

## Chain contract

| Stage | Source of truth | Supported behavior | Fidelity and limits |
|---|---|---|---|
| Google user → Google group | Live Google facade membership | Direct declared `USER` membership is collected and normalized before path evaluation. | API-shaped, training-only; no nested groups, dynamic membership, or real Workspace policy. |
| Google group → Microsoft group | Scenario relationship | An explicit `synchronized_to` edge connects two declared canonical groups. | CloudAILab scenario contract; no sync engine, provisioning protocol, delay, conflict handling, or mutable sync configuration. |
| Microsoft group → application | Live Microsoft facade app-role assignment | `principalId`, `resourceId`, and `appRoleId` normalize through a unique authorization node. Deleting the assignment changes subsequent paths and decisions. | API- and behavior-shaped for the scenario; no real Entra token service, directory roles, transitive membership, or assignment policy. |
| Application → OIDC audience | Declared local issuer client | The application node is a confidential local client with exactly one canonical audience. | CloudAILab development profile; client secret and HTTP loopback transport are synthetic. |
| Subject identity | Signed local access token plus declared subject | Gateway pins active discovery/JWKS and checks RS256, `at+jwt`, issuer, audience, time, client, tenant, principal, subject, email, and exact group set. | CloudAILab authorization contract; `cailab_subject` selects a declared identity and is not human authentication. |
| OIDC audience → AWS role | Typed scenario web-identity trust | Exact client node, audience node, and audience value must match the requested role. | CloudAILab authorization contract; not a hydrated or evaluated Floci IAM OIDC-provider policy. |
| AWS role session | Pinned Floci STS response | After all CloudAILab checks pass, `AssumeRoleWithWebIdentity` returns temporary local credentials routed to the role account. | API-shaped response only. Measured Floci behavior accepts an invalid JWT and ignores the declared web trust, so direct calls are not authorization evidence. |
| AWS role → S3 object | Declared role policy and live Floci object | Temporary credentials retrieve the seeded restricted object through the local S3 endpoint. | Same limited IAM/S3 contract as the [AWS matrix](aws-floci-1.5.32.md). |

## Federation gateway

`cailab federation assume-aws` requires a raw token file, canonical role node, and output path. On success it:

1. validates the token only against the active run's loopback issuer;
2. snapshots current Google, Microsoft, OIDC, and AWS state;
3. evaluates the declared subject, live app-role assignment, and typed web trust deterministically;
4. invokes the pinned Floci STS endpoint;
5. writes the credential JSON through an owner-only temporary file and atomic rename;
6. prints only the output path and expiration, never the credential values.

Any validation or authorization failure occurs before the Floci STS call. Existing credentials are bearer material until they expire; remediation blocks new exchanges and canonical paths but does not revoke an already-issued Floci credential.

## Tested negative behavior

- Invalid signature, algorithm, token type, issuer, audience, or expiry is rejected.
- A token client not bound to the role is rejected.
- Token identity or group claims that differ from the declared scenario subject are rejected.
- A group without a current Microsoft app-role assignment is rejected.
- Deleting the risky assignment closes the contractor path while preserving the approved group path.
- AWS, OIDC, and native runtime endpoints must be recorded loopback instances owned by the active run.

## External agent boundary

Agents and scripts may call the supported loopback APIs and execute the CLI gateway as ordinary external tools. M2 makes no claim that those processes are sandboxed, governed, approved, traced, or repeatedly scored. The M3 agent protocol and governance gateway remain separate work.

## Primary sources

- [Microsoft Graph appRoleAssignment resource](https://learn.microsoft.com/en-us/graph/api/resources/approleassignment?view=graph-rest-1.0)
- [List app-role assignments granted for a resource service principal](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list-approleassignedto?view=graph-rest-1.0)
- [Delete appRoleAssignedTo](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-delete-approleassignedto?view=graph-rest-1.0)
- [AWS web identity temporary credentials](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_request.html)
- [AWS federated principals in role trust](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html)
- [Floci STS operations and limitations](https://floci.io/floci/services/sts/)
- [Floci multi-account isolation](https://floci.io/floci/configuration/multi-account/)
