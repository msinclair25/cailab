---
title: Google Workspace Facade Compatibility Matrix
status: active
last_reviewed: 2026-07-13
facade_version: 0.1.0
---

# Google Workspace facade compatibility matrix

## Claim boundary

CloudAILab supports only the operations below for the `google-drive-sharing` scenario. The native facade is shaped like selected Admin SDK Directory API v1 and Drive API v3 resources. It does not emulate Google Workspace authentication, admin roles, sharing policy, inherited permissions, shared drives, replication, quotas, or the full query language.

The authoritative tests are `TestGoogleFacadeContractAndSnapshot`, `TestGoogleFacadeRejectsUnsupportedQuery`, and the cross-platform `TestGoogleNativeIntegration` in [`internal/provider`](../../internal/provider).

## Runtime contract

| Capability | Fidelity | Evidence | Limits |
|---|---|---|---|
| Persistent native facade | CloudAILab behavior contract | Handler, persistence, compiled-binary lifecycle, and cross-platform integration tests | Runs as the current OS user; it is not an agent sandbox. |
| Random IPv4 loopback endpoint | CloudAILab security contract | Readiness and lifecycle tests | HTTP only and synthetic data only. |
| Reset and cleanup | CloudAILab behavior/security contract | Shared native manager tests | Reset creates a new endpoint; shutdown requires a run-matched owner-only control document and random token. |

## Directory operations

| Resource | Method | Fidelity | Exercised behavior | Known limits |
|---|---|---|---|---|
| Users | `GET /admin/directory/v1/users` | API-shaped, training-only | `users` envelope, `customer` or `domain`, `maxResults`, `pageToken`, `nextPageToken` | No mutation, aliases, schemas, search, ordering, projection, or authorization semantics. |
| User | `GET /admin/directory/v1/users/{id-or-email}` | API-shaped, training-only | Returns synthetic ID, primary email, and full name | No aliases or custom schemas. |
| Groups | `GET /admin/directory/v1/groups` | API-shaped, training-only | `groups` envelope and deterministic pagination | No mutation, aliases, search, ordering, or admin roles. |
| Group | `GET /admin/directory/v1/groups/{id-or-email}` | API-shaped, training-only | Returns synthetic group metadata | No settings or nested groups. |
| Members | `GET /admin/directory/v1/groups/{id-or-email}/members` | API-shaped and behavior-shaped for this scenario | Returns direct `USER` members with `OWNER`, `MANAGER`, or `MEMBER` roles | No create/delete, indirect membership, nested groups, delivery settings, or suspended members. |

Directory collection maxima follow the documented 500-user and 200-group/member limits. Local page tokens are deterministic integer cursors, not Google-issued tokens.

## Drive operations

| Resource | Method | Fidelity | Exercised behavior | Known limits |
|---|---|---|---|---|
| Files | `GET /drive/v3/files` | API-shaped, training-only | `files` envelope, `pageSize`, `pageToken`, `nextPageToken`; `fields` is accepted without projection | No query search, hierarchy, owners, revisions, shared drives, or mutation. |
| File | `GET /drive/v3/files/{fileId}` | API-shaped, training-only | Returns synthetic file metadata; `alt=media` returns stored content | No export, range requests, abuse acknowledgement, or Google-native document conversion. |
| Permissions | `GET /drive/v3/files/{fileId}/permissions` | API- and behavior-shaped for the scenario | Returns direct user/group permission ID, type, email, and role; deterministic pagination | No inherited permissions, domains, anyone grants, expiration, details, or shared drives. |
| Permission | `DELETE /drive/v3/files/{fileId}/permissions/{permissionId}` | API- and behavior-shaped for the scenario | Returns HTTP 200 with `{}`, persists deletion, and changes snapshots and verification | No create/update or ownership transfer. |

## Authorization and normalization limits

- `Bearer cailab-google-local` is a synthetic facade access token, not OAuth, a JWT, or caller identity.
- Each live direct Drive permission becomes a distinct authorization node; group membership becomes a separate `member_of` edge. This prevents evidence from mixing unrelated grants.
- The facade does not evaluate real Google sharing policy. Deterministic verification evaluates only the normalized scenario graph.
- Unsupported query options return a Google-shaped HTTP 400 error instead of being ignored.
- External agents may call the loopback endpoint, but CloudAILab does not isolate independently launched agents.

## Primary sources

- [Directory users.list](https://developers.google.com/workspace/admin/directory/reference/rest/v1/users/list)
- [Directory groups.list](https://developers.google.com/workspace/admin/directory/reference/rest/v1/groups/list)
- [Directory members.list](https://developers.google.com/workspace/admin/directory/reference/rest/v1/members/list)
- [Drive files.list](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/list)
- [Drive files.get](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/get)
- [Drive permissions.list](https://developers.google.com/workspace/drive/api/reference/rest/v3/permissions/list)
- [Drive permissions.delete](https://developers.google.com/workspace/drive/api/reference/rest/v3/permissions/delete)
