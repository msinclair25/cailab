---
title: Product Requirements
status: active
last_reviewed: 2026-07-13
---

# Product requirements

Requirement identifiers are stable. Removed requirements are deprecated rather than renumbered.

## Status and evidence

This is the active requirements index. A requirement states the product contract; its presence does not by itself prove implementation. Current capability claims require supporting implementation, tests, schemas, and the applicable [compatibility record](../07-compatibility/README.md).

Known open requirements in the current M4.1 slice are explicit:

| Requirement | Current evidence boundary |
|---|---|
| FR-014 | Implemented for deterministic verification as text, Markdown, JSON, and timestamp-free JUnit with one testcase per invariant. Agent evaluation remains text/Markdown/JSON because its evidence rates intentionally do not define one universal pass/fail verdict; an agent JUnit gate is explicitly deferred until a scenario declares such criteria. |
| FR-018 | Optional evidence-grounded coaching is not implemented and is deferred beyond the first public release. |
| NFR-USE-008, NFR-USE-009, and NFR-USE-011 | Implemented in M4.1 with the public help matrix, versioned evidence-safe status projection, and exact-commit release-link tests. |
| NFR-USE-010 | The guided no-Docker lifecycle is implemented and covered by unit and cross-platform CI workflow steps. A clean maintainer archive rehearsal completed in 0.07 seconds without Go or Docker, but the ten-minute unfamiliar-user target remains pending the documented RC2 participant walkthrough. |
| NFR-USE-012 | Implemented with the dependency-free external-agent starter, fixed facade-backed governed tool, generated scenario-bound registrations, public flagship integration, release packaging/smoke checks, expected evidence, and explicit host/isolation limitations. |
| NFR-USE-013 | Implemented with the public validate-without-starting command, release-packaged no-runtime starter, strict hook/runtime rejection tests, public lifecycle coverage, authoring/contribution guidance, and an explicit compatibility record. |
| NFR-USE-014 | Implemented with the versioned data-only learning catalog, semantic validator, common-core and mission-layer metadata, one-time adaptation provenance, initial focused path, CI validation, and release bundling. |

## Functional requirements

| ID | Requirement | Priority |
|---|---|---|
| FR-001 | The system shall validate a versioned scenario manifest before changing runtime state. | Must |
| FR-002 | The system shall create multiple isolated organizational tenants and provider accounts. | Must |
| FR-003 | The system shall use a provider-neutral identity, resource, policy, and trust model. | Must |
| FR-004 | The system shall compile manifests into deterministic execution plans without requiring an LLM. | Must |
| FR-005 | The system shall deploy and collect supported AWS resources through a Floci adapter. | Must |
| FR-006 | The system shall expose a scoped Microsoft Graph-compatible facade backed by canonical state. | Must |
| FR-007 | The system shall expose a scoped Google Workspace-compatible facade backed by canonical state. | Must |
| FR-008 | The system shall issue local test identities and tokens for supported federation flows. | Must |
| FR-009 | The system shall normalize provider state into one current-state graph. | Must |
| FR-010 | The system shall evaluate scenario invariants deterministically and attach evidence to results. | Must |
| FR-011 | The system shall record actor identity, request, policy decision, side effect, and outcome for supported actions. | Must |
| FR-012 | The system shall support lifecycle commands for diagnostics, startup, status, reset, verification, and shutdown. | Must |
| FR-013 | The system shall support seeded data generation and reproducible initial state. | Must |
| FR-014 | The system shall export human-readable Markdown and machine-readable JSON for deterministic verification and agent evaluation, plus JUnit testcases for deterministic verification invariants; agent evaluation shall not invent a JUnit verdict without declared pass/fail criteria. | Should |
| FR-015 | The system shall execute or connect an agent under test through a governed tool boundary. | Must |
| FR-016 | The agent boundary shall support allow, deny, redact, and require-approval decisions. | Must |
| FR-017 | The system shall support repeated agent trials and aggregate behavioral results. | Should |
| FR-018 | The system may generate evidence-grounded coaching through an optional model provider. | Could |
| FR-019 | The system shall publish an operation-level compatibility record for each supported provider operation. | Must |
| FR-020 | The system shall return a stable diagnostic error for unsupported provider operations. | Must |
| FR-021 | The system shall preserve enough run metadata to reproduce or explain each agent evaluation. | Must |
| FR-022 | The system should support replaying a captured agent trace against compatible verification rules. | Should |
| FR-023 | The system shall allow users to register versioned custom tool adapters with declared schemas, permissions, risk classification, timeout, transport, and isolation requirements. | Must |
| FR-024 | The system shall authorize each supported local federation exchange from a validated signed token, current provider state, and typed trust contract before invoking a permissive emulator. | Must |
| FR-025 | The CLI shall validate scenario-bound policy and tool registrations and run a reference or protocol-compatible subprocess agent against an active scenario. | Must |
| FR-026 | The system shall optionally capture deterministic provider-state digests and invariant results before and after an agent trial and link them to that trial. | Must |
| FR-027 | The system shall restore supported provider fixtures without changing their recorded loopback endpoints and shall verify the canonical baseline digest before launching an evaluated agent. | Should |
| FR-028 | The system shall support scenario-labeled indirect prompt-injection fixtures and deterministically distinguish fixture exposure, prohibited behavior, successful injection tasks, and gateway containment. | Must |
| FR-029 | The system shall provide paired deterministic safe and unsafe control behaviors for the flagship prompt-injection fixture and shall label the safe result as fixture-specific rather than model-general resistance. | Should |

## Non-functional requirements

### Security

| ID | Requirement |
|---|---|
| NFR-SEC-001 | Runtime services shall bind to loopback by default. |
| NFR-SEC-002 | Generated credentials shall be synthetic and visibly unsuitable for production. |
| NFR-SEC-003 | Global certificate stores and system proxies shall not be modified without explicit consent. |
| NFR-SEC-004 | Downloads shall use pinned versions and integrity verification. |
| NFR-SEC-005 | Hosted model calls shall be disabled by default and shall not receive secrets or scenario data implicitly. |
| NFR-SEC-006 | Agent execution shall not be described as isolated unless network and filesystem boundaries are enforced. |
| NFR-SEC-007 | Declarative scenarios shall not select executable provider images outside a code-owned allowlist. |
| NFR-SEC-008 | Runtime cleanup shall verify CloudAILab ownership and run identity before removing host resources. |
| NFR-SEC-009 | A persisted process identifier alone shall not authorize native-runtime cleanup. |
| NFR-SEC-010 | Temporary federation credentials shall be written only to an owner-only file and shall not be printed to standard output. |
| NFR-SEC-011 | A permissive provider emulator shall not be treated as the authoritative source for a CloudAILab authorization decision. |
| NFR-SEC-012 | Agent protocol inputs shall reject oversized frames, invalid UTF-8, duplicate JSON keys, unknown typed fields, and malformed message contracts before side effects. |
| NFR-SEC-013 | Tool isolation declarations shall not be described as enforced unless the runtime verifies the corresponding network and filesystem boundaries. |
| NFR-SEC-014 | An unisolated agent subprocess shall use explicit argv, working directory, and environment; enforce bounded protocol, diagnostics, and time limits; and be waited for after success, failure, or cancellation. |
| NFR-SEC-015 | A governed tool response shall not be emitted unless manifest and policy evaluation succeed and its decision event commits; persistence failure shall prevent execution and response. |
| NFR-SEC-016 | Tool schemas shall validate Draft 2020-12 input instances without external reference loading; only allow or redact decisions may launch a tool subprocess. |
| NFR-SEC-017 | Successful tool output shall apply declared sensitive-field redaction before return and hashing, and a result shall not be emitted until linked outcome evidence commits. |
| NFR-SEC-018 | A public agent run shall resolve declared resource identifiers from active canonical scenario state, forward only explicitly selected environment variables, and omit raw protocol transcripts and child diagnostics from its default and JSON summaries. |
| NFR-SEC-019 | An approval-required call shall remain unexecuted until an exact local resolution is re-evaluated and durably linked; unattended runs shall reject by default, and approval prompts and evidence shall omit raw tool arguments. |
| NFR-SEC-020 | Docker-isolated agent execution shall require a present content-addressed image without declared volumes and an explicitly pinned local Unix-socket non-rootless Linux engine with active cgroups, reject remote contexts, forward no host environment or mounts, disable external networking, shared-memory IPC, and Docker log persistence, enforce a read-only root with bounded temporary storage, drop privileges and capabilities, select the built-in seccomp profile, apply CPU/memory/PID limits, and never fall back silently to host execution. |
| NFR-SEC-021 | Agent replay shall consume only integrity-checked evidence-safe records, reject inconsistent decision/approval/outcome linkage, and shall not expose raw protocol frames, tool arguments, successful tool content, or child diagnostics. |
| NFR-SEC-022 | Provider fixture restoration shall require run-scoped runtime ownership, authenticated native control, and matching container labels; a failed or unverified restore shall not launch the agent. |
| NFR-SEC-023 | Prompt-injection scoring ground truth shall be scenario-owned, immutable for a trial, omitted from the agent start message, and evaluated only from linked governed-action evidence after proven fixture exposure. |
| NFR-SEC-024 | A fixture-specific safe control shall not receive the prohibited tool/action/resource target through its command configuration, shall receive no prohibited action/resource ground truth in `session.start`, shall make no content-derived follow-up call, and shall not be presented as evidence of general model or framework resistance. |
| NFR-SEC-025 | The public repository shall publish supported-version and private vulnerability-reporting instructions through an enabled private disclosure channel. |

### Reliability and reproducibility

| ID | Requirement |
|---|---|
| NFR-REL-001 | Applying the same scenario version and seed shall produce equivalent initial canonical state. |
| NFR-REL-002 | Failed startup shall identify the failed component and leave recoverable state. |
| NFR-REL-003 | Reset and shutdown operations shall be idempotent. |
| NFR-REL-004 | Verification output shall be stable for equivalent normalized state. |
| NFR-REL-005 | Persistent state changes shall use versioned, tested migrations. |
| NFR-REL-006 | Agent decision events shall use monotonic sequence numbers and stable correlation identifiers; timestamps shall not determine authorization or score. |
| NFR-REL-007 | Persisted agent decision events shall be append-only through the application API and shall verify contiguous order, canonical record hashes, and the stored chain head when read. |
| NFR-REL-008 | An agent trial shall persist a canonical immutable start record before launch and append exactly one terminal record without changing its run configuration. |
| NFR-REL-009 | Approval resolutions shall be append-only, linked to the exact original decision and input hash, consumed once for live continuation, and required as an integrity-checked predecessor for approved tool outcomes. |
| NFR-REL-010 | Agent-container cleanup shall verify run and trial ownership labels before forced removal and shall execute after success, failure, timeout, or cancellation using a bounded cleanup context. |
| NFR-REL-011 | Agent replay shall require a complete explicitly selected compatible trial set, order trials by contiguous declared index, exclude wall-clock timestamps from scoring, emit counts with denominators and rates, identify unavailable metrics, and produce equivalent output for equivalent evidence. |
| NFR-REL-012 | State-captured trials shall append bounded canonical before evidence prior to governed decisions and after evidence following session termination; the after record shall close further action evidence, and restored initial state shall match the normalized runtime baseline digest. |
| NFR-REL-013 | A range shall persist a normalized provider-state baseline after successful startup; migrated ranges without one shall require reset before state capture. |
| NFR-REL-014 | Automatic agent campaigns shall use a bounded preflighted trial set, restore and verify the fixture before every sequential trial, preserve terminal failures as evaluation evidence, stop on incomplete state or control-plane failure, and replay only a complete compatible set. |

### Usability and portability

| ID | Requirement |
|---|---|
| NFR-USE-001 | The default local deployment shall require at most the CloudAILab binary and, for container-backed workflows, one documented local container runtime; the initial supported runtime is Docker. |
| NFR-USE-002 | `cailab doctor` shall detect missing prerequisites and provide actionable remediation. |
| NFR-USE-003 | Linux CI shall run without cloud accounts, hosted models, or interactive prompts. |
| NFR-USE-004 | Provider compatibility and limitations shall be documented and test-backed. |
| NFR-USE-005 | The release executable shall list, show, validate, and start its built-in scenarios without a repository checkout or working-directory catalog; custom files and catalogs shall require explicit selection. |
| NFR-USE-006 | CI shall build and execute the documented walking-skeleton demo in an ephemeral non-root container without cloud accounts, hosted models, host mounts, external networking, or a writable root filesystem. |
| NFR-USE-007 | The public release documentation shall provide tested installation, architecture, upgrade, troubleshooting, verification, support, and recording-ready demo guidance without claiming an unpublished version. |
| NFR-USE-008 | `-h` and `--help` shall exit successfully for the root command, every public command, and every public command group; help shall be written to standard output and shall not be rendered as an operational error. |
| NFR-USE-009 | The CLI shall expose a stable versioned machine-readable status and endpoint representation that does not include provider control tokens, temporary credentials, or other secrets. |
| NFR-USE-010 | The release shall include a guided no-Docker workflow that uses the normal lifecycle, verification, and cleanup contracts and can produce a meaningful first success in ten minutes or less on a supported clean environment. |
| NFR-USE-011 | Every Markdown link in a release archive shall either resolve within that unpacked archive or target an intentionally durable versioned source. |
| NFR-USE-012 | The release shall include a tested protocol-compatible external-agent starter with a governed provider-backed tool, scenario-bound policy and registrations, expected deterministic evidence, and explicit host/isolation limitations. |
| NFR-USE-013 | The repository shall provide a validated data-only custom-scenario starter and authoring workflow that cannot select executable hooks or unreviewed provider runtimes. |
| NFR-USE-014 | The release shall include a schema- and semantics-validated data-only learning catalog with stable lesson/path IDs, common-core dependencies, complete mission-layer coverage, safety/verification/cleanup metadata, and an initial self-guided path bound only to implemented scenarios and workflows. |

### Maintainability and supply chain

| ID | Requirement |
|---|---|
| NFR-MNT-001 | Scenario, event, and report schemas shall be versioned from their first committed release. |
| NFR-MNT-002 | Dependency and generated-code changes shall be reproducible and reviewable. |
| NFR-MNT-003 | CI shall compare a versioned third-party notice inventory with the union of modules linked into every declared release target. |
| NFR-SUP-001 | Release artifacts shall include checksums, an SBOM, and build provenance by the M4 release. |
| NFR-SUP-002 | CI workflows shall use minimum permissions and pin third-party actions to immutable revisions. |
| NFR-SUP-003 | A tag release shall publish semantic-versioned CGO-free archives for the declared target matrix only after checksum verification and a native smoke test on each declared operating system, and shall attach an SPDX JSON SBOM plus signed build and SBOM attestations. |
| NFR-SUP-004 | Any CI-container base image shall use an immutable digest; the final image shall exclude the source tree and build toolchain, declare no ports or volumes, and shall not be published or represented as a release artifact. |
| NFR-SUP-005 | Every release archive shall include the project license/notice, changelog, third-party notice index, exact linked-module manifest, and copied license material for the Go runtime and linked dependencies. |

## Deferred requirements

- Versioned, redacted, integrity-manifested proof-of-work bundles generated from evidence-supported scenario state. These are portfolio artifacts, not identity proof, credentials, rankings, hiring recommendations, or employment guarantees.
- Full graphical administration console.
- High-fidelity email, collaboration, and endpoint emulation.
- Transparent interception for applications that cannot override endpoints.
- Production-grade multi-user hosting.
- Compliance certification or automated legal conclusions.
