---
title: Threat Model
status: active
---

# Threat model

## Scope

This threat model covers the CloudAILab control plane, local provider services, scenario files, generated credentials, agent gateway, audit records, optional model integrations, and the developer host on which the range runs.

It does not assert containment for arbitrary agent processes unless an isolation mode enforces network and filesystem boundaries.

## Protected assets

- Host credentials and personal files
- Integrity of scenario ground truth and verification rules
- Integrity and confidentiality of audit evidence
- Isolation between simulated tenants and accounts
- Authenticity of downloaded runtime dependencies
- Availability of the local control plane
- Secrets supplied for optional hosted model providers

## Trust boundaries

1. User shell ↔ CloudAILab CLI
2. CloudAILab control plane ↔ external containers
3. Agent under test ↔ governed tool gateway
4. Provider facade ↔ canonical state
5. Local issuer ↔ token-consuming services
6. Local range ↔ optional hosted model API
7. Workspace files ↔ downloaded third-party artifacts

## Initial threats and controls

| ID | Threat | Primary controls |
|---|---|---|
| TM-001 | A generated credential is accidentally sent to a real cloud endpoint. | Visibly fake credentials, endpoint-scoped environment, network isolation option, no production credential discovery in range processes. |
| TM-002 | An agent reads host files or accesses the public internet. | Explicit isolated execution mode, minimal mounts, deny-by-default egress, accurate documentation of non-isolated mode. |
| TM-003 | Prompt-injected content causes unauthorized tool use. | Treat content as untrusted, action-level policy gateway, approvals, scoped credentials, complete audit trail. |
| TM-004 | A malicious scenario executes arbitrary host code. | Declarative schema, no implicit shell evaluation, capability allowlist, validation before apply. |
| TM-005 | Compromised dependency images or binaries execute locally. | Pinned versions, checksums/signatures where available, provenance metadata, SBOM, update policy. |
| TM-006 | Global proxy or certificate changes weaken the host. | Endpoint override by default, explicit consent for advanced proxy mode, reversible changes, diagnostics and cleanup. |
| TM-007 | Tenant state leaks across simulated boundaries. | Tenant-scoped keys, authorization tests, negative contract tests, unique identifiers, graph invariants. |
| TM-008 | Verification rules are disclosed or modified during a mission. | Separate public objectives from protected ground truth, integrity hashes, read-only mission mounts in isolated mode. |
| TM-009 | Hosted AI receives sensitive traces or secrets. | Opt-in adapters, field-level redaction, outbound preview, provider configuration, no implicit uploads. |
| TM-010 | An AI-generated explanation contradicts evidence. | Evidence citations, deterministic score precedence, explicit non-authoritative labeling. |
| TM-011 | A scenario causes CAL to pull and execute an attacker-selected container. | Code-owned runtime allowlist, immutable OCI digest, schema rejection of custom images, no implicit shell hooks. |
| TM-012 | Cleanup removes an unrelated host container. | CAL ownership label, per-run label, deterministic names, label verification before removal. |
| TM-013 | The provider container gains unnecessary host authority. | Non-root UID, all capabilities dropped, `no-new-privileges`, CPU/memory limits, no Docker socket mount for M1, loopback-only published API. |
| TM-014 | Native-facade cleanup stops an unrelated process after PID reuse or control-file tampering. | Owner-only run directory, random control token, matching run ID, authenticated shutdown endpoint, exact control-path validation, PID treated as diagnostic only after startup. |
| TM-015 | Another local process mutates the training facade. | IPv4 loopback binding, explicit synthetic bearer requirement, no production credentials, separate unprinted runtime-control credential; local OS-account isolation remains a documented boundary. |
| TM-016 | A forged, replayed, expired, wrong-audience, or algorithm-confused local token crosses a trust boundary. | RS256 allowlist, asymmetric signatures, exact issuer and audience validation, explicit token types, one-time expiring codes, bounded token lifetime, RFC 7638 key IDs, and negative contract tests. |
| TM-017 | Discovery redirects a validator to attacker-controlled signing keys. | HTTP redirect refusal, exact active issuer match, IPv4 loopback endpoint validation, and same-origin fixed `/jwks` enforcement in the reference validator. |
| TM-018 | Key rotation invalidates legitimate unexpired tokens or exposes private signing material. | Owner-authenticated rotation, owner-only active private-key state, immediate retired-private-key discard, retired-public-key overlap through maximum token lifetime, public-only JWKS, persistence rollback, and run-scoped cleanup. |
| TM-019 | A forged token or stale group assignment obtains a temporary AWS-shaped credential because Floci accepts the request. | The federation command validates the active issuer/JWKS, signed claims, declared subject, current Microsoft app-role assignment, and typed AWS trust before calling Floci; negative and lifecycle regression tests cover each decision class. |
| TM-020 | Federation credentials leak through terminal output or permissive file modes. | The CLI requires an explicit output path, writes through an owner-only temporary file, atomically publishes it, and prints only the path and expiration. |
| TM-021 | An external tool bypasses the federation command and calls the permissive Floci endpoint directly. | Direct Floci web-identity calls are excluded from authorization evidence; agent-facing wrappers must withhold the endpoint, and no containment claim is made until M3 enforces network/tool boundaries. |
| TM-022 | A malicious agent frame exhausts memory, exploits parser ambiguity, or smuggles a conflicting field. | UTF-8 JSON Lines objects are capped at 1 MiB and reject empty frames, duplicate keys, unknown typed fields, malformed payloads, and trailing values before execution. |
| TM-023 | A tool manifest hides shell behavior or is mistaken for authorization/isolation. | Commands are explicit argv vectors with no implicit shell concatenation; manifests are inert until explicit registration; authority and isolation declarations require separate deterministic evaluation and enforcement. |
| TM-024 | Raw tool inputs, outputs, or credentials leak into an audit trace. | Decision events store canonical payload hashes by default; sensitive fields use fail-closed RFC 6901 redaction before persistence; later persistence requires dedicated regression tests. |
| TM-025 | An agent inherits host credentials, floods diagnostics, violates protocol order, or survives a canceled session. | Absolute direct execution without a shell, explicit non-inherited environment, frame/message/stderr/time limits, direction and lifecycle validation, direct-child cancellation, bounded pipe cleanup, and wait-on-all-started-paths tests. |
| TM-026 | Rule ordering, permissive manifests, failed audit writes, or stored-record mutation hides or widens an agent action. | Default deny, manifest permission ceiling, fixed deny-first precedence, evidence-before-response, transactional sequence/correlation constraints, canonical event hashes, chain/head verification, and mutation/deletion regression tests. |

## Security invariants

- An action without an authenticated simulated principal is denied unless explicitly public.
- Explicit deny takes precedence over allow in the canonical evaluator.
- Tenant boundary crossings require a visible trust edge.
- Agent authority cannot exceed the credential and policy attached to its current run.
- Every governed agent tool call produces a decision record.
- Optional coaching cannot change verification state or score.
- A scenario cannot select a provider image outside the release allowlist.
- CAL removes a provider container only when its persisted or discovered run label matches the active run.
- CAL stops a native facade only through a run-matched control document and authenticated control endpoint; a persisted PID alone is insufficient.
- The supported federation command returns credentials only after signed-token, live-assignment, and typed-trust checks succeed.
- An agent protocol frame or valid tool manifest cannot by itself authorize or execute a host action.

## M1 residual risk

- The Floci container uses Docker's bridge network and may have outbound network access. It is not an agent sandbox.
- Docker daemon access by the `cailab` process is host-equivalent authority even though the child container is unprivileged.
- Digest pinning establishes artifact identity, not that the artifact is vulnerability-free or independently reproducible.
- Floci signature validation remains disabled for the synthetic M1 workflow, and unknown local access keys are permissive emulator behavior.

## M2 residual risk

- The native Microsoft and Google facades run as the current OS user and are not agent sandboxes.
- `Bearer cailab-local` and `Bearer cailab-google-local` gate training APIs but are not signed tokens or caller identities; the federation gateway separately validates signed local OIDC tokens.
- Facades use HTTP on IPv4 loopback and must never be advertised as network services.
- The supported Graph, Directory, and Drive surfaces are intentionally incomplete and do not enforce real provider roles, consent policy, sharing policy, inherited permissions, or OAuth semantics.
- Synthetic Drive content is readable by any local process that receives the endpoint and static API token; scenarios must not contain real secrets.
- The local development issuer's loopback HTTP transport does not meet OIDC/OAuth TLS requirements and must not carry real credentials or tokens.
- `cailab_subject` is synthetic subject selection, not user authentication; any declared confidential client with its scenario secret can request a code for a declared subject.
- Client secrets, codes, tokens, and RSA private keys are run-local synthetic credentials; local processes under the same OS account remain inside the documented trust boundary.
- Floci's direct `AssumeRoleWithWebIdentity` route accepts invalid tokens in the pinned release, and unknown access keys are permissive. Any process given or able to discover the Floci endpoint can bypass the M2 CLI gateway. M2 is a learning range, not an enforced agent sandbox.
- Removing an app-role assignment prevents new gateway exchanges but does not revoke already-issued Floci credentials before expiration.

## M3 residual risk

- The internal gateway evaluates resolved tool metadata and persists `not_executed` decision evidence, but no supported public agent-run workflow or tool execution path exists yet.
- JSON Lines framing and direct-child ownership provide no authentication, encryption, filesystem isolation, syscall isolation, or network policy.
- Cancellation targets the direct child. Independently detached descendants may outlive it until an isolation backend owns a process boundary.
- An agent receives an empty environment by default but still runs with the launching OS user's ambient filesystem and network authority.
- Tool input schemas are structurally constrained and hashed, but full input-instance evaluation enters with the executor.
- The SQLite hash chain detects inconsistent stored records but is not tamper-proof against the launching OS account, which can rewrite both records and chain metadata.
- Declared `none`, `loopback`, or filesystem restrictions are requirements, not verified isolation claims, until an execution backend enforces them.

## Review triggers

Review this document when adding a provider, execution mode, downloadable dependency, hosted integration, new credential type, or new agent tool protocol.
