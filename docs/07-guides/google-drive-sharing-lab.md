---
title: Google Drive Sharing Lab
status: active
last_reviewed: 2026-07-12
---

# Google Drive sharing lab

## Outcome

This lab teaches a scoped Google Workspace access-review workflow: inventory directory users and group membership, inspect a restricted Drive file and its direct permissions, remove a contractor's direct grant, and preserve access inherited through an approved group.

The facade implements only the operations in the [Google compatibility matrix](../07-compatibility/google-workspace-facade.md). It is training software, not Google Workspace or a complete API emulator.

## Prerequisites

- Go 1.25.12 or newer
- No Docker, Google Workspace account, proxy, or certificate installation
- `curl` or another HTTP client

```bash
mkdir -p ./bin
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor google-drive-sharing
```

## 1. Start the range

```bash
./bin/cailab up google-drive-sharing
./bin/cailab status
./bin/cailab mission
```

Export the random Google endpoint printed by `up` or `status`. The API token is synthetic and is not a Google credential.

```bash
export CAILAB_GOOGLE_ENDPOINT=http://127.0.0.1:PORT
export CAILAB_GOOGLE_TOKEN=cailab-google-local
```

## 2. Inventory Directory state

```bash
curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/admin/directory/v1/users?customer=my_customer"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/admin/directory/v1/groups?customer=my_customer"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/admin/directory/v1/groups/group_records_team/members"
```

The records administrator is a direct member of `records-team@example.test`. Collection pagination uses the documented parameter names, but local page tokens are deterministic integer cursors.

## 3. Inspect Drive state and the vulnerable path

```bash
curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/drive/v3/files"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/drive/v3/files/file_retention_plan/permissions"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/drive/v3/files/file_retention_plan?alt=media"

./bin/cailab graph path principal:contractor resource:retention-plan
./bin/cailab graph path principal:records-admin resource:retention-plan
./bin/cailab verify
```

Initial verification intentionally exits with code `3`: the contractor-removal invariant fails while the administrator-preservation invariant passes.

## 4. Remove only the contractor grant

```bash
curl --fail --silent --show-error \
  -X DELETE \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/drive/v3/files/file_retention_plan/permissions/permission_contractor"
```

A successful deletion returns HTTP 200 with `{}` and persists immediately. Do not delete `permission_records_team`; that grant is the intended access path.

## 5. Verify the result

```bash
./bin/cailab graph path principal:contractor resource:retention-plan
./bin/cailab graph path principal:records-admin resource:retention-plan
./bin/cailab verify
```

Expected outcome:

- No contractor-to-file path is found.
- The records administrator retains access through group membership and the group permission.
- Both deterministic invariants pass.

## External tools and AI agents

Any local tool or agent that can make HTTP requests can use the endpoint and synthetic bearer token. Point the agent only at this local API surface, then use `cailab verify` as the deterministic result. The facade does not sandbox the agent: a separately launched process retains whatever host filesystem and network authority you grant it.

Official Google client libraries commonly support custom root or base URLs, but this slice guarantees the documented HTTP contract only. Language-specific SDK examples become supported after dedicated contract tests.

## Reset and cleanup

`reset` stops the process, restores the original direct and group permissions, and starts a new random endpoint:

```bash
./bin/cailab reset
./bin/cailab status
./bin/cailab verify
```

Stop the native process and remove its run-scoped files when finished:

```bash
./bin/cailab down
```
