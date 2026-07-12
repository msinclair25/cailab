---
title: Microsoft Delegated Consent Lab
status: m2-development
last_reviewed: 2026-07-12
---

# Microsoft delegated consent lab

## Outcome

This lab teaches a scoped Microsoft Entra application-consent workflow: inventory directory objects, identify an excessive delegated permission grant, revoke only that grant through a Microsoft Graph-shaped endpoint, and preserve an approved user's intended path. CloudAILab rebuilds authorization nodes and edges from the live facade before every path query and verification.

The facade implements only the operations in the [Microsoft compatibility matrix](../07-compatibility/microsoft-graph-facade.md). It is training software, not Microsoft Entra ID or a complete Microsoft Graph emulator.

## Prerequisites

- Go 1.25.12 or newer
- No Docker, Microsoft tenant, global proxy, or local certificate installation
- `curl` or another HTTP client

Build and check this scenario's requirements:

```bash
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor microsoft-consent
```

## 1. Start the range

```bash
./bin/cailab up microsoft-consent
./bin/cailab status
./bin/cailab mission
```

Record the random loopback endpoint printed by `up` or `status`, then export it. The bearer value is deliberately synthetic and valid only for the local facade.

```bash
export CAILAB_GRAPH_ENDPOINT=http://127.0.0.1:PORT
export CAILAB_GRAPH_TOKEN=cailab-local
```

The facade runs as a detached child of the same `cailab` binary. Its run-scoped control configuration and mutable state are stored with owner-only permissions under the selected state directory. The API binds only to IPv4 loopback.

## 2. Inventory the directory

Use the same collection paths as the supported Microsoft Graph v1.0 operations:

```bash
curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/users"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/applications"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/servicePrincipals"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/oauth2PermissionGrants"
```

Collection responses use a `value` array and support `$top`, `$skiptoken`, `$select`, and `@odata.nextLink`. `$select` is accepted for client compatibility but does not yet project response fields. Unsupported query options return a Graph-shaped `Request_UnsupportedQuery` error rather than being silently ignored.

## 3. Inspect the vulnerable path

```bash
./bin/cailab graph path microsoft:analyst microsoft:directory-data
./bin/cailab verify
```

The initial verification intentionally exits with code `3`: the approved administrator path passes, while the analyst path fails. Each live delegated grant becomes its own authorization node so path evidence cannot combine scopes from different users' grants.

## 4. Revoke only the risky grant

The analyst's synthetic grant ID is `77777777-7777-4777-8777-777777777777`.

```bash
curl --fail --silent --show-error \
  -X DELETE \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/oauth2PermissionGrants/77777777-7777-4777-8777-777777777777"
```

A successful deletion returns HTTP `204 No Content` and persists immediately in the native facade state.

## 5. Verify the result

```bash
./bin/cailab graph path microsoft:analyst microsoft:directory-data
./bin/cailab graph path microsoft:security-admin microsoft:directory-data
./bin/cailab verify
```

Expected outcome:

- No analyst-to-directory path is found.
- The security administrator retains a `User.Read` path through their distinct grant.
- Both deterministic invariants pass.

## External tools and AI agents

Any local tool or agent that can make HTTP requests can use the printed endpoint and synthetic bearer token. Give an agent only this endpoint—not real Microsoft credentials—and evaluate its changes with `cailab verify`. The facade is not an agent sandbox: an independently launched agent still has whatever host filesystem and network authority its own process receives.

Official Microsoft Graph SDKs require a customized request adapter or HTTP pipeline to target a non-default endpoint. CAILab's first M2 slice guarantees the documented HTTP contract; language-specific SDK examples enter the supported matrix only after they have dedicated contract tests.

## Reset and cleanup

`reset` stops the current native process, starts a fresh loopback endpoint from the compiled scenario, and restores both original grants:

```bash
./bin/cailab reset
./bin/cailab status
./bin/cailab verify
```

Stop the child process and remove its run-scoped files when finished:

```bash
./bin/cailab down
```

Shutdown requires the random control token and matching run ID stored in the owner-only control file. That control credential is separate from the synthetic Graph API bearer token.
