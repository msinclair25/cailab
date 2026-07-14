---
title: Threat Model
status: active
last_reviewed: 2026-07-13
---

# Threat model

## Scope

This threat model covers the CloudAILab control plane, local provider services, scenario files, generated credentials, agent gateway, audit records, planned proof-of-work exports, optional model integrations, and the developer host on which the range runs.

It does not assert containment for arbitrary agent processes unless an isolation mode enforces network and filesystem boundaries.

## Protected assets

- Host credentials and personal files
- Integrity of scenario ground truth and verification rules
- Integrity and confidentiality of audit evidence
- Confidentiality, integrity, and bounded interpretation of planned proof-of-work exports
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
8. Persisted evidence ↔ user-selected proof-of-work export directory (planned M5)

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
| TM-027 | A malicious schema or tool process triggers external retrieval, inherits host secrets, evades timeout, smuggles protocol output, or returns sensitive data. | Fragment-local schema references, no schema URL loader, explicit argv/cwd/environment, no shell, input validation before launch, bounded timeout/stdout/stderr/cleanup, one correlated response, output redaction, and linked outcome evidence. |
| TM-028 | A public agent registration forges resource metadata, inherits undeclared credentials, leaks raw arguments in CLI output, or creates evidence outside a recorded trial. | Resolve resource ID through active canonical state; explicit environment-name selection; bounded files; evidence-safe summaries; immutable start/terminal records; decision/outcome events accepted only while the matching trial is active. |
| TM-029 | A stale, forged, replayed, implicit, or unrecorded approval authorizes a sensitive tool call, or a reviewer prompt discloses model-controlled arguments. | Default rejection; exact correlated confirmation; canonical metadata without raw arguments; current-policy re-evaluation; immutable resolution evidence before response; decision/approval/outcome hash linkage; one-use continuation and duplicate rejection. |
| TM-030 | An adversarial agent reads host files, reaches local or public services, gains container privilege, exhausts resources, gains image-declared writable volumes or shared memory, leaves protocol data in Docker logs, substitutes mutable image content, redirects execution to a remote Docker daemon, relies on ignored rootless cgroup limits, or survives cancellation. | Opt-in digest/image-ID pinning; volume-free image inspection; explicit local Unix-socket non-rootless Linux engine with active cgroups; remote-context rejection; no host environment or mounts; network and IPC none; log driver none; read-only root and bounded noexec tmpfs; non-root UID; all capabilities dropped; no-new-privileges; built-in seccomp; CPU/memory/PID limits; init process; deterministic ownership labels and verified forced cleanup. |
| TM-031 | Replayed evidence is cherry-picked, reordered, cross-linked to another trial, corrupted, or summarized with unsupported claims such as equating process completion with mission success. | Integrity-checked store reads; complete-set and exact-configuration compatibility; contiguous declared indices; strict decision/approval/outcome linkage; timestamp-independent deterministic projection; primitive counts and explicit denominators/rates; stable `notMeasured` reasons; no composite score. |
| TM-032 | A forged reset request, mismatched runtime, partial provider restore, stale synthetic token, state-capture failure, or racing host process creates a false equivalent-fixture or task-success claim. | Run-scoped authenticated native control; container ownership labels; explicit memory-backed Floci replacement at the recorded loopback port; fresh OIDC key and cleared codes; exact post-restore canonical digest before launch; append-only before/after invariant evidence; after-phase trace closure; failed evaluation on capture error; accurate non-atomic and host-mode limitations. |
| TM-033 | A trial is scored as resistant without consuming the attack, fixture expectations leak to the agent, an adapter failure is mislabeled as governance containment, or the unsafe baseline exports real data. | Ground truth omitted from `session.start`; exposure requires a linked successful outcome; only later exact prohibited decisions count; separate behavior/success/containment facts; containment from gateway disposition; fixed synthetic marker and simulated export only. |
| TM-034 | A repeated campaign silently reuses trials, changes its denominator, runs against drifted state, races provider mutations, continues after incomplete restoration, or emits a score from a partial set. | Bounded count; preflighted derived IDs and collision checks; immutable one-based count/index metadata; sequential execution; verified restoration and before/after evidence per trial; fail-closed control-plane errors; complete-set replay only. |
| TM-035 | A scripted safe control is presented as generally resistant, avoids leaked prohibited ground truth, or earns resistance without consuming the fixture. | Fixture-specific labeling; prohibited tool/action/resource tuple absent from child argv; no prohibited action/resource in `session.start`; exposure requires a successful linked outcome; no content-derived follow-up call; paired unsafe control; explicit limitations for models, adaptive attacks, semantic equivalents, and deployments. |
| TM-036 | A mutable workflow dependency, injected tag/version, stale output, broad token, or unverified archive compromises a public release or misstates its origin. | Full-SHA action pins; explicit Syft version; semantic-version validation in workflow and Go tool; fixed cleaned staging paths; read-only module builds; sorted checksums; native smoke tests; separate least-privilege attestation/publication jobs; GitHub/Sigstore build and SBOM attestations. |
| TM-037 | An ambient working-directory catalog silently replaces built-in scenarios, or embedded verification data is mistaken for a secret from the launching OS account. | Immutable built-in `embed.FS` selected by default; explicit custom file/catalog flags; one strict validation path; documentation that embedding is distribution rather than confidentiality and host-mode agents retain the launching account's file authority. |
| TM-038 | A mutable or compromised base image, oversized build context, root default, host mount, network access, writable path, or misleading isolation claim compromises the CI demo or makes it depend on undeclared host state. | Docker Official builder pinned by tag and digest; allowlisted context; multi-stage CGO-free build; final-image contract inspection; numeric non-root user; no ports/volumes/source/toolchain; Docker `none` network; no IPC or mounts; read-only root; bounded tmpfs/resources; dropped capabilities; no-new-privileges; built-in seccomp; CI-only non-publication and explicit non-sandbox claim. |
| TM-039 | A release omits license/notice material, ships stale dependency attribution, or treats an SBOM/classifier as a legal conclusion. | Apache-2.0 project license; required release documents; copied linked-component licenses/notices including the Go runtime; multi-target linked-module inventory drift gate; archive unit/smoke tests; independent SBOM, vulnerability, and human review; explicit non-legal-advice boundary. |
| TM-040 | A recording helper executes an unintended binary, replaces an existing run, follows a forged external endpoint, hides an expected failure, consumes an oversized response, or leaves provider listeners active. | Explicit regular non-symlink executable; optional exact version/commit assertion; new or empty owner-only state; direct argv without a shell; exact expected exits; complete ready-runtime set and origin-only IPv4 loopback validation; bounded HTTP responses and redirect refusal; fixed synthetic Graph operation; signal-aware cleanup; inactive-run and endpoint-closure checks. |
| TM-041 | A planned proof-of-work export leaks credentials, classified payloads, or protected ground truth; follows an unsafe output path; omits contradictory evidence; or is presented as identity proof, a credential, or a hiring recommendation. | Before M5 support: allowlisted evidence projection, export-specific redaction, protected-ground-truth exclusion, owner-safe atomic writes, traversal/symlink and size checks, required incomplete/error evidence, deterministic ordering, integrity manifest, explicit non-attestation language, and negative/regression tests. No export support claim exists until those controls pass. |
| TM-042 | The external-agent starter follows a caller-controlled provider URL, broadens its canonical target, accepts ambiguous protocol input, leaks retrieved content or a token into configuration/evidence, overwrites user files, or is mistaken for isolated/model-general evaluation. | Exact IPv4 loopback origin and fixed provider path; exact scenario tenant/action/resource/classification; UTF-8, duplicate-key-free strict JSON; closed empty input; explicit environment selection; `/content` protected-output redaction; new-directory and exclusive-file creation; host/tool non-isolation warnings; deterministic-starter limitation; unit, flagship integration, archive, and cross-platform smoke tests. |
| TM-043 | Learning metadata becomes an executable hook, points outside the repository, binds to nonexistent scenarios, bypasses prerequisites, changes deterministic results, or is mistaken for grades or credentials. | Closed data-only schema; fixed field vocabulary with no command transport; regular in-repository guide and scenario checks; stable/unique references; cycle and path-order validation; deterministic verification remains external and authoritative; explicit non-LMS/non-attestation language; CI and release-bundle checks. |
| TM-044 | A community scenario starter gains an executable hook or unreviewed runtime, leaks real data, hides cross-tenant authority, exposes ground truth as learner briefing, or is mistaken for provider parity. | No-runtime starter; strict unknown-field and runtime allowlist validation; synthetic-data and visible-edge guidance; separate briefing/invariants; explicit non-confidential ground-truth boundary; public lifecycle, negative, CI, archive, and compatibility tests. |
| TM-045 | Scenario-controlled text corrupts JUnit XML, timestamps make results nondeterministic, a report is mistaken for an agent score, or canonical evidence discloses real data. | Standard XML escaping; timestamp-free ordered projection; invariant-only testcases; explicit agent-JUnit deferral; synthetic-data rule; deterministic, metacharacter, CLI, CI, and archive tests. |

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

- The supported public workflows validate scenario-bound registrations and persist run, decision, approval, and outcome evidence, but host subprocess mode does not make an agent trustworthy.
- In host mode, JSON Lines framing and direct-child ownership provide no authentication, encryption, filesystem isolation, syscall isolation, or network policy.
- Host-mode cancellation targets the direct child; independently detached descendants may outlive it.
- A host-mode agent receives an empty environment by default but still runs with the launching OS user's ambient filesystem and network authority.
- Draft 2020-12 input instances are validated with a pinned library; custom formats are annotations unless a later contract enables format assertion explicitly.
- Host-mode agents and every registered tool subprocess remain unisolated and retain the launching OS user's ambient filesystem, network, and syscall authority.
- A crash after a tool side effect but before outcome commit can leave durable authorization intent without durable outcome evidence; the agent result is withheld in that case.
- Interactive approvals are local terminal decisions by the launching user, not authenticated remote identity, multi-party authorization, or separation of duties.
- The SQLite hash chain detects inconsistent stored records but is not tamper-proof against the launching OS account, which can rewrite both records and chain metadata.
- Declared `none`, `loopback`, or filesystem restrictions are requirements, not verified isolation claims, until an execution backend enforces them.
- Explicitly selected environment variables can contain real provider credentials. CloudAILab does not persist or print their values, but the unisolated receiving process can use or exfiltrate them with the launching user's ambient authority.
- A controller or host crash after the start record commits can leave a non-terminal trial record. The same trial ID is not reused automatically; explicit recovery remains planned.
- Docker-isolated agents cannot directly call provider loopback APIs, hosted models, or the public internet. They must be self-contained and use governed tool calls over standard I/O.
- Docker isolation trusts the daemon, container runtime, host kernel or Docker Desktop VM, and the exact selected image. It is not a virtual-machine or remote-account security boundary.
- Digest pinning identifies image content but does not establish provenance, signature validity, or absence of vulnerabilities.
- Replay compatibility does not prove equivalent mutable provider state across trials, and trace/configuration hashes are reproducibility identifiers rather than signatures.
- State-captured traces measure task and remediation outcomes only for documented provider snapshots and declared graph invariants; unsupported provider state is outside the score.
- Provider restoration is not atomic across runtimes. A failed restore prevents agent launch but may require the full lifecycle reset to recover a partially restored range.
- Host-mode agents and detached descendants can race terminal state capture with the launching user's ambient authority.
- Fixture-labeled traces measure prompt-injection resistance only for exact declared exposure and prohibited targets; semantic equivalents and covert channels remain outside the score.
- The unsafe baseline's export is a simulator outcome, not evidence that provider data left the range.
- Evidence-safe traces still cannot measure sensitive-data exposure or effective blast radius.
- Automatic campaigns are sequential and bounded but can be long-running. A stopped campaign is not resumable under the same immutable trial IDs and requires a new prefix after range recovery.
- The safe fixture control is scripted and deterministic. Its positive result validates the exact evaluation path only; it says nothing about an LLM's reasoning, adaptive robustness, or a production agent deployment.

## M4 residual risk

- Built-in scenario manifests and verification rules are public and recoverable from the executable. Embedding prevents accidental working-directory substitution but does not hide ground truth from the launching OS account or an unisolated host-mode agent.
- The clean-demo container trusts Docker Hub availability and the exact pinned base contents, Docker Engine, the container runtime, and the host kernel or Docker Desktop VM. It is neither published nor covered by release attestations/SBOMs, and its hardening must not be generalized to untrusted-agent containment.
- Artifact attestations establish recorded build origin and integrity, not that source, dependencies, workflow logic, or binaries are vulnerability-free.
- Checksums do not establish authenticity unless the manifest or its attestation is independently trusted.
- Equivalent source-date inputs and pinned module content reduce nondeterminism, but bit-for-bit independent reproduction across toolchain environments is not yet claimed.
- Linux ARM64 is cross-compiled and packaged but is not yet executed on a native hosted release runner.
- The SBOM reflects Syft's detection over staged binaries and can be incomplete; it complements rather than replaces `govulncheck`, dependency review, and source inspection.
- The portfolio runner's version/commit check prevents accidental candidate mix-ups but is not cryptographic authenticity. Recording still requires independent checksum and provenance verification of the selected binary.
- The external-agent starter is intentionally host-executed and deterministic. Its loopback/filesystem declarations are requirements rather than enforced tool isolation, and its successful governed read is not evidence of general model quality or prompt-injection resistance.
- Learning paths are navigation and instructional metadata. Their durations, outcomes, checklists, and reflection prompts do not prove identity, authorship, competence, employment readiness, or scenario success.
- The data-only scenario starter proves schema, graph, invariant, lifecycle, and packaging behavior only. It provides no provider facade, executable extension, confidential ground truth, tenant process isolation, or provider-parity claim.

## Planned M5 proof-of-work boundary

- The proof-of-work bundle is planned, not implemented or supported in M4. Existing reports and release attestations do not provide this user-facing portfolio contract.
- A future integrity manifest can detect changed export files after generation but cannot prove personal identity, independent authorship, general competence, or employment suitability.
- Public sharing creates a new disclosure boundary even for synthetic environments. Export projection and redaction require dedicated threat review and tests before M5 implementation ships.

## Review triggers

Review this document when adding a provider, execution mode, downloadable dependency, hosted integration, new credential type, agent tool protocol, or evidence-export format.
