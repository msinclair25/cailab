---
title: "ADR-0019: Endpoint-Preserving Trial State Evaluation"
status: accepted
date: 2026-07-12
---

# ADR-0019: Endpoint-preserving trial state evaluation

## Context

The initial replay profile can measure governed actions but cannot claim task success or remediation quality because an agent run does not record scenario state before and after execution. Repeated trial metadata also does not prove equivalent starting state.

The existing public `reset` lifecycle stops and recreates provider runtimes on random loopback ports. Using it between trials would invalidate endpoint values explicitly forwarded to registered tools. Restoration must therefore preserve each recorded endpoint while still proving that supported mutable provider state returned to the compiled fixture.

## Decision

1. Add opt-in `scenario-state-v1` capture to agent runs. `--capture-state` records deterministic before/after provider snapshots and invariant reports. `--restore-fixture` implies capture and restores the compiled fixture before launch.
2. Persist the immutable agent start record before restoration. A restore or initial-capture failure produces a failed terminal trial and never launches the agent.
3. Restore native Microsoft and Google facades through a new authenticated, run-scoped `POST /_cailab/reset` control route. Each process deep-copies its startup provider state, replaces current state under its lock, persists it, and keeps the same listener and control identity.
4. Restore the local OIDC runtime through the same control route. It clears one-time codes and replaces signing material with one fresh ephemeral key, invalidating credentials from an earlier trial while retaining the issuer endpoint.
5. Restore AWS by replacing only the ownership-checked Floci container, republishing it at the exact recorded loopback port, and rehydrating the compiled topology. CloudAILab explicitly configures `FLOCI_STORAGE_MODE=memory`; it does not mount provider state. Managed and run labels must match before removal, and the replacement runtime handle is persisted before capture.
6. After restoration, snapshot every configured provider and calculate the canonical compiled-state digest. The digest must equal the original compiled scenario digest before `session.start` is sent.
7. Append one `TrialStateEvidence` record with phase `before` before any governed decision. After the session ends, capture and append phase `after` with a cancellation-independent bounded context. The after record closes action-evidence appends for that trial.
8. Each state record contains a snapshot digest and the deterministic invariant report, but no provider credentials, raw provider payloads, agent frames, tool arguments, or tool output. Records are bounded, canonical, append-only, and hash-verified on read.
9. Replay promotes only complete compatible state-captured sets to profile `scenario-outcome-v1`. Task success means every declared scenario invariant passes after the trial. Remediation success applies only when at least one invariant failed before and every invariant passes after. Completion remains a separate process metric.
10. The snapshot and score cover only provider operations and invariants already in CloudAILab's compatibility contracts. They do not establish real-cloud parity, complete blast radius, prompt-injection resistance, or sensitive-data exposure.

## Consequences

### Positive

- Registered tools keep stable loopback endpoint values across restoration.
- An agent cannot start after an unproven or partially observed restore.
- Task and remediation rates are grounded in the same deterministic invariants used by `cailab verify`.
- Prior OIDC codes and tokens do not remain valid merely because the issuer endpoint is reused.
- Repeated sets can report how often their initial snapshot matched the baseline.

### Negative

- Provider restoration is not atomic across processes. A later provider failure can leave an earlier provider restored; the failed trial does not launch, and the operator may need the existing full `reset` recovery path.
- Replacing Floci briefly makes its endpoint unavailable even though the address remains unchanged.
- State capture adds provider snapshot latency and can turn an otherwise completed agent session into a failed evaluation if terminal evidence cannot be captured.
- Host-mode agents or detached descendants retain ambient authority and could race the after snapshot; Docker isolation remains the stronger agent boundary.
- Full invariant reports are protected from the live agent but become user-visible after termination and in explicitly requested JSON replay output.

## Validation

- Facade tests mutate Microsoft and Google state, call the authenticated reset route, and prove the baseline returns at the same endpoint.
- OIDC tests prove reset clears codes and replaces the signing key.
- Docker unit and real Floci integration tests prove ownership checks, explicit memory storage, same-port replacement, endpoint stability, rehydration, and baseline digest recovery.
- State tests cover migration, before/after order, duplicates, size bounds, mutation detection, terminal requirements, and refusal of action evidence after closure.
- Application and CLI tests cover restore-before-launch, failed restoration, before/after capture, measured task/remediation output, and deterministic replay.

## Sources

- [Floci environment variables](https://floci.io/floci/configuration/environment-variables/)
- [Docker container run and published ports](https://docs.docker.com/reference/cli/docker/container/run/#published-ports)
- [NIST AI RMF Core: Measure](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [ADR-0008: Managed native facade processes](0008-managed-native-facade-processes.md)
- [ADR-0018: Deterministic agent evidence replay](0018-deterministic-agent-evidence-replay.md)
