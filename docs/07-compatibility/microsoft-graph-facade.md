---
title: Microsoft Graph Facade Compatibility Matrix
status: m2-complete
last_reviewed: 2026-07-12
facade_version: 0.1.0
---

# Microsoft Graph facade compatibility matrix

## Claim boundary

CloudAILab supports only the operations below for the `microsoft-consent` and `acquisition-agent` scenarios. The facade is shaped like a narrow Microsoft Graph v1.0 surface; it does not claim to emulate Microsoft Entra ID, token issuance, role enforcement, replication, throttling, or the complete OData query language.

The authoritative tests are `TestMicrosoftFacadeAuthPaginationAndGrantDeletion`, `TestMicrosoftSnapshotTracksLivePermissionGrants`, `TestFlagshipCrossProviderPathAndRemediation`, and the cross-platform `TestMicrosoftNativeIntegration` in [`internal/provider`](../../internal/provider).

## Runtime contract

| Capability | Fidelity | Evidence | Limits |
|---|---|---|---|
| Persistent native facade | CloudAILab behavior contract | Compiled-binary lifecycle and cross-platform integration test | Runs as the current OS user; it is not a sandbox or service manager. |
| Random IPv4 loopback endpoint | CloudAILab security contract | Readiness and lifecycle tests | HTTP only; safe because the supported endpoint is loopback-only and uses synthetic data. |
| Detached process lifecycle | CloudAILab behavior contract | Unix session and Windows process-group implementations exercised in the build matrix | Abrupt host shutdown can leave stale files; authenticated stale-runtime cleanup is bounded to the run directory. |
| Reset | CloudAILab behavior contract | Manual lifecycle and provider tests | Recreates the original scenario state at a new random endpoint. |
| Cleanup | CloudAILab security contract | Run ID, owner-only control file, random control token, and post-run directory check | The PID is not treated as sufficient ownership evidence. |

## HTTP and directory operations

| Resource | Method | Fidelity | Exercised behavior | Known limits |
|---|---|---|---|---|
| `users` | `GET /v1.0/users` and `GET /v1.0/users/{id-or-upn}` | API-shaped, training-only | Returns `id`, `displayName`, and `userPrincipalName`. | No create/update/delete, directory roles, licenses, or advanced filters. |
| `groups` | `GET /v1.0/groups` and `GET /v1.0/groups/{id}` | API-shaped, training-only | Returns declared `id` and `displayName`. | No membership routes, nested/dynamic groups, owners, or group mutation. |
| `applications` | `GET /v1.0/applications` and `GET /v1.0/applications/{id}` | API-shaped, training-only | Returns `id`, `appId`, and `displayName`. | No credentials, owners, manifests, or application mutation. |
| `servicePrincipals` | `GET /v1.0/servicePrincipals` and `GET /v1.0/servicePrincipals/{id}` | API-shaped, training-only | Returns `id`, `appId`, `displayName`, and declared `appRoles`. | No owners, credentials, or service-principal mutation. |
| `appRoleAssignedTo` | `GET /v1.0/servicePrincipals/{resourceId}/appRoleAssignedTo` | API- and behavior-shaped for the scenario | Returns current group assignments with assignment, principal, resource, and app-role IDs; supports local pagination. | No user or service-principal assignees in the current scenario contract. |
| `appRoleAssignments` | `GET /v1.0/groups/{groupId}/appRoleAssignments` | API-shaped, training-only | Returns assignments for the addressed group. | Direct group assignments only; no transitive effective-assignment calculation. |
| `appRoleAssignedTo` | `DELETE /v1.0/servicePrincipals/{resourceId}/appRoleAssignedTo/{assignmentId}` | API- and behavior-shaped for the scenario | Returns 204, persists deletion, and changes subsequent graph snapshots, federation decisions, and verification. | No create/update in this slice. |
| `oauth2PermissionGrants` | `GET` collection and item | API- and behavior-shaped for the scenario | Returns the documented grant identifiers, consent type, principal, resource, and space-separated scopes. | Only `Principal` consent is accepted by the scenario schema; delta and filter are not supported. |
| `oauth2PermissionGrants` | `DELETE /v1.0/oauth2PermissionGrants/{id}` | API- and behavior-shaped for the scenario | Returns 204, persists deletion, and changes subsequent graph snapshots and verification. | No create/update in this slice. |

## Collection and error behavior

| Behavior | Supported | Notes |
|---|---|---|
| `value` collection envelope | Yes | Includes an `@odata.context` string. |
| `$top` | Yes | Integer range 1–999. |
| `$skiptoken` and `@odata.nextLink` | Yes | Local integer cursor designed for deterministic training; not a Microsoft token format. |
| `$select` | Accepted | Does not yet project response fields; documented to prevent an implied OData claim. |
| Other OData options | No | Returns HTTP 400 with `Request_UnsupportedQuery`. |
| Missing/unknown object | Yes | Returns HTTP 404 with `Request_ResourceNotFound`. |
| Missing API bearer | Yes | Returns HTTP 401 with `InvalidAuthenticationToken`. |

## Authorization and identity limits

- `Bearer cailab-local` is a synthetic facade access token, not a JWT and not proof of a specific user or application.
- The random runtime control token is separate and is never printed as an API credential.
- The static API bearer is still not caller identity. The separate flagship federation gateway validates signed local OIDC identity and current app-role assignment state before AWS exchange.
- The canonical graph, not the facade token, is authoritative for the current scenario's deterministic verification.
- Each delegated grant normalizes through a distinct authorization node so evidence cannot mix scopes across grants that share a client and resource.

## Primary sources

- [List users](https://learn.microsoft.com/en-us/graph/api/user-list?view=graph-rest-1.0)
- [List applications](https://learn.microsoft.com/en-us/graph/api/application-list?view=graph-rest-1.0)
- [List groups](https://learn.microsoft.com/en-us/graph/api/group-list?view=graph-rest-1.0)
- [List service principals](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list?view=graph-rest-1.0)
- [App-role assignment resource](https://learn.microsoft.com/en-us/graph/api/resources/approleassignment?view=graph-rest-1.0)
- [List app-role assignments for a resource service principal](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list-approleassignedto?view=graph-rest-1.0)
- [Delete appRoleAssignedTo](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-delete-approleassignedto?view=graph-rest-1.0)
- [OAuth2 permission grant resource](https://learn.microsoft.com/en-us/graph/api/resources/oauth2permissiongrant?view=graph-rest-1.0)
- [Customize the Microsoft Graph SDK client](https://learn.microsoft.com/en-us/graph/sdks/customize-client)
- [Dev Proxy mock responses](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/mock-responses)
- [Dev Proxy certificate troubleshooting](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/troubleshooting)
