---
title: Delivery Roadmap
status: active
last_reviewed: 2026-07-13
---

# Delivery roadmap

Milestones are capability gates, not date commitments. This document is the concise roadmap; detailed scope, workstreams, evidence, risks, and exit criteria are maintained in the [CloudAILab Master Plan](../00-project/master-plan.md). Product development is depth-first in identity and security and gains breadth through connected end-to-end scenarios, not through untested tool or provider catalogs.

CloudAILab remains a sandbox with self-guided missions and portable contracts. Classroom administration, instructor tooling, LMS integration, accreditation, rosters, and institution-specific delivery remain outside this roadmap and may be developed independently against stable CloudAILab interfaces.

CloudAILab supports job readiness through realistic practice and portable evidence of recorded work. It does not issue credentials, rank users, make hiring recommendations, or promise employment. Product direction is reviewed at least annually and at major milestone boundaries against workforce, cloud-native, agent-identity, security, maintenance, and observed community evidence.

## M0 — Contracts and walking skeleton

**Status:** complete.

- Accept the initial charter, requirements, and ADRs.
- Define the `v1alpha1` scenario schema.
- Define canonical graph types and stable identifiers.
- Scaffold `cailab doctor`, `up`, `status`, `verify`, and `down`.
- Establish CI, linting, unit tests, release metadata, and documentation checks.

**Exit:** a minimal scenario compiles, starts a local control plane, verifies one invariant, and cleans up.

## M1 — AWS identity vertical slice

**Status:** complete; the IAM/STS/S3 trust-remediation workflow is executable and CI-backed.

- Integrate pinned Floci runtime.
- Support two AWS accounts and selected IAM, STS, and S3 operations.
- Normalize AWS principals, policies, roles, and resources.
- Document Floci fidelity and limitations.

**Exit:** the learner can discover and close a supported cross-account path with familiar AWS tooling.

## M2 — Microsoft and Google identity facades

**Status:** complete; the flagship lifecycle connects live Google membership and Microsoft assignment state to signed local OIDC identity and an AWS-shaped temporary session.

- Implement scenario-required users, groups, applications, roles, and memberships.
- Add local issuer and selected federation claims.
- Complete cross-provider graph normalization.
- Publish a tested compatibility matrix.

**Exit:** the flagship attack path crosses Google, Microsoft, and AWS representations.

## M3 — Agent governance harness

**Status:** complete; versioned protocols, supported inert reference/safe/unsafe/custom subprocess runs, deterministic governance, optional Docker agent isolation, protected linked evidence, endpoint-preserving restoration, normalized baselines, scenario evidence, paired fixture controls, fixture-labeled injection scoring, and automatic restored reference/safe/unsafe campaigns satisfy the exit gate. Tool subprocess isolation is deferred until required by a scenario.

- Add agent identity and run metadata.
- Supervise protocol-compatible direct subprocesses with bounded lifecycle and diagnostics.
- Enforce manifest permission ceilings and deterministic allow, deny, redact, and approval-required policy decisions.
- Persist ordered decision evidence without raw tool arguments.
- Execute only allowed or redacted one-shot tools and persist linked actual outcomes.
- Implement governed tool gateway decisions: allow, deny, redact, approve.
- Record complete action traces.
- Replay complete compatible trial sets into transparent counts, denominators, rates, and explicit unavailable metrics.
- Restore supported provider fixtures at stable endpoints and link before/after invariant evidence to each evaluated trial.
- Add prompt-injection fixtures and repeated trials. (paired safe/unsafe controls, repeated-trial scoring, and automatic reference/safe/unsafe campaigns implemented)
- Report evidence-supported task success and policy violations while explicitly labeling effective blast radius as unmeasured.

**Exit:** a local or external agent can be evaluated deterministically against the flagship scenario.

## M4 — Portfolio-quality release

**Status:** release foundation and RC1 validation complete; bounded M4.1 adoption pass in progress before RC2.

- Cross-platform binary packaging with an embedded built-in scenario catalog, checksums, SPDX SBOM generation, provenance/SBOM attestations, and native install smoke tests is implemented. A digest-pinned, non-root clean-demo container using Docker's `none` network is built and exercised in CI but intentionally not published.
- Apache-2.0 project licensing, linked-component notice inventory, archive legal-bundle enforcement, public policies, installation/architecture/upgrade/troubleshooting guides, recording-ready demo runbook, and the [release-candidate security/compatibility audit](release-readiness-audit.md) are implemented; RC1 artifacts passed the manual exercise.
- M4.1 has reconciled stale documentation, fixed CLI help, added machine-readable endpoint discovery and a guided no-Docker first run, packaged tested external-agent and data-only scenario starters, added the validated focused learning contract/path, and closed CI reporting with invariant JUnit plus a least-privilege synthetic workflow. A clean maintainer rehearsal now covers the complete packaged journeys and records resolved friction; unfamiliar-participant acceptance against RC2 and RC2 validation remain.
- The M4.1 lesson contract records common-core dependencies and covered mission layers so post-release role-oriented tracks can build on it without adding Azure, Kubernetes, or other platform scope to v0.1.
- The separately owned DevOps course is a read-only source for a one-time adaptation only; CloudAILab will have no dependency or synchronization relationship with it.
- User-facing changes require a fully revalidated `0.1.0-rc.2`; the portfolio recording, changelog promotion, owner license/risk approval, exact-lineage audit, and verified `v0.1.0` tag follow RC2.
- Web console, MCP ecosystem work, registries, broad provider/IaC support, and optional AI coaching remain outside the first-release gate.

**Exit:** an unfamiliar user can achieve a no-Docker first success in ten minutes or less, complete the flagship remediation, run/adapt the external-agent starter, consume machine-readable output, understand limitations, and clean up from a self-contained verified RC2 archive.

## M5 — Community extensibility and enterprise full-stack learning

**Status:** planned after `v0.1.0`.

- Stabilize versioned data-only scenario and lesson packaging, validation, integrity, common-core dependencies, mission-layer metadata, examples, and migration guidance.
- Preserve explicit trust tiers for scenarios, agent/tool adapters, and reviewed provider code; no implicit executable scenario hooks.
- Make Azure the first substantial post-v0.1 provider expansion while keeping Entra directory behavior distinct from Azure resource management and data planes.
- Deliver one Azure-centered enterprise mission that connects a GitHub Actions-shaped OIDC delivery identity, Entra workload identity, Azure RBAC, a workload/resource, protected secrets or data, agent tool use, evidence, investigation, remediation, and cleanup.
- Export that mission as a versioned, redacted Markdown/JSON proof-of-work bundle with evidence references, preserved-access results, compatibility limits, cleanup status, and an integrity manifest; state explicitly that the bundle is not identity proof, a credential, or a hiring recommendation.
- Bound the initial Azure surface to scenario-required resource hierarchy, principals, managed identities, RBAC, selected Storage/Key Vault/Policy behavior, and synthetic evidence with operation-specific contract and negative tests.
- Record infrastructure-as-code, OPA/Rego, Kubernetes workload identity, observability/Sentinel-style detection, data governance, supply-chain security, platform engineering, SPIFFE/SPIRE, and vendor agent-platform comparisons as post-M5 candidates; admit them only as optional scenario-driven specializations and do not require them for the M5 exit.
- Keep the no-Docker experience as the supported default; every optional runtime or hosted integration declares prerequisites, isolation, cost/data boundaries, compatibility, diagnostics, and cleanup.
- Add agent helpers, custom restored campaigns, reusable CI integrations, package-manager distribution, and an MCP bridge only after their architecture/security contracts are accepted and sustainable.

**Exit:** a contributor can safely create, validate, test, document, and distribute a data-only scenario; a learner can complete and remediate one deterministic Azure-centered mission spanning at least four declared enterprise layers and export its safe, integrity-checkable proof-of-work bundle; and every external integration preserves CloudAILab authorization, evidence, compatibility honesty, and cleanup.

## M6 — Local web learning console

**Status:** planned after the M5 contracts it consumes are stable.

- Loopback-only UI using the same application services, canonical graph, verifier, evidence, and cleanup controls as the CLI.
- Mission/curriculum navigation, progressive hints, graph visualization, supported provider state, agent traces, campaign comparisons, reports, and cleanup status.
- Run-scoped authentication, CSRF/origin/CSP controls, bounded input, accessibility, browser end-to-end tests, and CLI/console result equivalence.
- No independent authorization, mutation policy, grading, external telemetry, or required hosted asset.

**Exit:** supported learning journeys work visually without weakening CLI support, security boundaries, deterministic results, or cleanup.

## M7 — Optional evidence-grounded AI/ML

**Status:** planned after the deterministic product and console contracts are stable.

- Disabled-by-default evidence-grounded tutor, draft remediation/scenario assistance, model/framework comparison, and synthetic-trace analysis admitted one capability at a time.
- Local-model-first examples; hosted providers require explicit configuration, selected/redacted inputs, cost disclosure, provenance, and separate threat review.
- Model output remains supplemental and cannot authorize, mutate, approve, verify, score, or suppress deterministic findings by itself.

**Exit:** every shipped AI/ML feature is optional, bounded, evidence-linked, reproducible enough for its stated claim, and clearly separated from deterministic facts.

## Long-term deferrals

- Transparent interception and host certificate automation
- Production multi-user hosting
- Broad provider parity or live-cloud offensive testing
- Live-cloud differential tests without explicit credentials, cost controls, and isolation
- Arbitrary plugin execution or dynamic provider loading
