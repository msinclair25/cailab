---
title: Agent Trial State Compatibility
status: implemented
profile: scenario-state-v1
last_reviewed: 2026-07-12
---

# Agent trial state compatibility

## Supported contract

`cailab agent run reference` and `cailab agent run subprocess` accept `--capture-state`. This records deterministic invariant results before and after the agent session. `--restore-fixture` additionally restores supported provider state before capture and refuses to launch unless the resulting canonical digest exactly matches the compiled fixture.

| Provider | Restoration behavior | Endpoint behavior | Tested state |
|---|---|---|---|
| AWS / Floci | Replace the ownership-checked memory-backed container, publish the exact prior host port, wait for readiness, and rehydrate the compiled IAM/S3 topology. | Recorded loopback endpoint remains; the container identity changes and is persisted. | Selected IAM roles, trust/policies, buckets, and objects in the AWS compatibility matrix. |
| Microsoft facade | Authenticated in-process replacement from a deep-copied startup provider model. | Existing loopback listener remains. | Selected users, groups, applications, service principals, delegated grants, and app-role assignments. |
| Google facade | Authenticated in-process replacement from a deep-copied startup provider model. | Existing loopback listener remains. | Selected users, groups/members, Drive files, and direct permissions. |
| Local OIDC | Clear codes and replace all signing material with one new ephemeral key. | Existing issuer endpoint remains. | Selected development-profile clients, subjects, codes, and signing-key lifecycle. |

## Evidence and scoring

Each participating trial has exactly one `before` and one `after` `TrialStateEvidence` record. Replay validates their identity, phase order, fixture-restoration flag, snapshot digests, invariant set, and hashes.

Complete compatible sets use replay profile `scenario-outcome-v1` and add:

- initial-state match rate;
- task success rate, where all after-state invariants pass;
- remediation success rate, where a trial that began with at least one failed invariant ends with all invariants passing.

Agent process completion remains separate. A failed process can still have a successful scenario outcome, and a completed process can still fail the mission.

## Explicit limitations

- Capture and restore are opt-in.
- Restoration is not atomic across provider processes.
- Only documented provider surfaces and current graph invariants participate.
- Mutable state outside supported snapshots is not measured.
- A stable endpoint does not authenticate a registered host tool or isolate it.
- A host-mode agent or detached descendant can race provider state because host execution remains unisolated.
- The OIDC endpoint is stable, but signing keys and synthetic credentials intentionally change on restore.
- The CLI still launches repeated trials individually; automatic campaign execution remains planned.

## Tests

- Native facade restoration: `internal/provider/microsoft_test.go`, `internal/provider/google_test.go`, `internal/provider/oidc_test.go`
- Docker restoration: `internal/provider/docker_test.go`, `internal/provider/docker_integration_test.go`
- Evidence persistence: `internal/state/trial_state_evidence_test.go`
- Capture and outcome scoring: `internal/app/agent_run_test.go`, `internal/agent/replay_test.go`
- Public workflow: `internal/cli/cli_test.go`
