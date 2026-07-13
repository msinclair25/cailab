---
title: Troubleshooting
status: active
last_reviewed: 2026-07-13
---

# Troubleshooting

Start with the scenario-specific diagnostic:

```bash
cailab version
cailab doctor SCENARIO
cailab status
```

Use synthetic data in every reproduction and remove credentials, signing material, control tokens, provider data, and raw agent/tool content before sharing logs.

## Common failures

| Symptom | Likely cause | Safe action |
|---|---|---|
| `no active run` | The selected state directory has no active range. | Confirm `CAILAB_HOME` or pass the intended `--state-dir`, then run `up`. |
| Scenario name is unknown | A custom catalog was assumed implicitly. | Use a built-in name from `scenario list`, or explicitly pass `--root`/`--scenario-root`. |
| Docker CLI or engine check fails | Docker is stopped, too old, remote, rootless, or incompatible with the selected workflow. | Start a local supported engine and rerun `doctor`; do not expose a Docker TCP socket to make the check pass. |
| Floci never becomes ready | Image pull, port publication, or engine health failed. | Inspect Docker events/logs for the run-labeled container, then run `down` or `reset`; do not remove unrelated containers. |
| Loopback endpoint refuses connections | A native child exited or stale state points to a dead process. | Run `status`, then `reset`; cleanup requires the stored run ID and control token. |
| `verify` exits with code 3 | One or more scenario invariants failed. | This is expected for intentionally vulnerable initial states; inspect the evidence and follow the scenario guide. |
| Federation rejects a token | Issuer, signature, audience, time claims, subject/group state, Microsoft assignment, or AWS trust no longer matches. | Issue a new synthetic token from the active run and inspect current paths; never bypass the CloudAILab gateway with direct Floci calls. |
| Agent process fails before a report | Protocol, registration, timeout, output, persistence, or fixture restoration failed. | Use the evidence-safe CLI summary and selected trial ID; avoid publishing raw protocol frames or child diagnostics. |
| Docker-isolated agent is rejected | Image reference, local-engine identity, cgroups, volume declaration, user, or runtime flags violate the supported boundary. | Follow the [Docker isolation compatibility record](../07-compatibility/agent-docker-isolation.md); CloudAILab does not fall back to host mode. |
| Checksum or attestation verification fails | Asset bytes, repository identity, workflow, commit, or tag does not match. | Stop installation and reacquire assets from the intended GitHub release. Do not override the verification result. |

## Recover an interrupted run

1. Preserve the state directory if evidence matters.
2. Run `cailab status` with the same `CAILAB_HOME` or `--state-dir`.
3. Try `cailab down`; cleanup verifies native control tokens and Docker ownership labels.
4. If the run must continue, use `reset` to recreate the compiled baseline at supported endpoints.
5. Report a cleanup defect before manually deleting process or container state.

Never kill a persisted PID or remove a similarly named container solely because it appears in CloudAILab state. Ownership checks exist to prevent cross-run or unrelated-resource cleanup.

## Installation and state problems

The executable can list and start built-in scenarios from any working directory. If behavior changes by directory, check for an explicit custom catalog flag or a different `CAILAB_HOME`; ambient `./scenarios` content is not loaded.

Pre-release state has no cross-version migration promise. Back up evidence, stop the range, and follow [upgrading](upgrading.md) before replacing a development build.

## Ask for help

Follow [SUPPORT.md](../../SUPPORT.md) for ordinary issues and [SECURITY.md](../../SECURITY.md) for private vulnerability reporting.
