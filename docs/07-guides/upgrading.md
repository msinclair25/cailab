---
title: Upgrading
status: active
last_reviewed: 2026-07-13
---

# Upgrading

CloudAILab has no public release yet. Development commits and release candidates may change the scenario schema, evidence contracts, embedded catalog, runtime layout, or SQLite representation without an automated migration path. Do not upgrade an active range in place.

## Before replacing a binary

1. Record `cailab version`, active run ID, scenario, and state directory.
2. Export or copy any evidence you need to retain.
3. Run `cailab down` with the current binary and verify owned provider resources are gone.
4. Back up the complete state directory.
5. Read [CHANGELOG.md](../../CHANGELOG.md), the release notes, compatibility records, and security advisories.
6. Verify the new archive using the [release verification guide](release-verification.md).

## Pre-1.0 policy

- The latest `0.x` release is the only supported line unless an advisory says otherwise.
- Minor `0.x` versions may make incompatible scenario, CLI, state, or agent-contract changes.
- Patch versions are intended for compatible fixes, but the changelog and release notes remain authoritative.
- Built-in scenario changes are part of the executable and take effect only after replacement.
- Provider runtime digest changes require a CloudAILab release and compatibility review.

If the new version cannot read old state, start with a new state directory and retain the backup for evidence/reference. Never copy control tokens, PIDs, runtime endpoints, or container ownership records into a new run manually.

## After upgrading

```bash
cailab version
cailab doctor walking-skeleton
cailab scenario list
cailab up walking-skeleton
cailab verify
cailab down
```

Then exercise the scenario and agent profile you actually use. A successful walking-skeleton run does not establish provider or agent compatibility beyond that workflow.

## Rollback

Stop the new run before rollback. Restore the previous executable and its matching complete state-directory backup together. Mixing a previous binary with state already changed by a newer version is unsupported.
