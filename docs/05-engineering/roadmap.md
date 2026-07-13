---
title: Delivery Roadmap
status: draft
---

# Delivery roadmap

Milestones are capability gates, not date commitments. This document is the concise roadmap; detailed scope, workstreams, evidence, risks, and exit criteria are maintained in the [CloudAILab Master Plan](../00-project/master-plan.md).

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

**Status:** in development.

- Cross-platform binary packaging with an embedded built-in scenario catalog, checksums, SPDX SBOM generation, provenance/SBOM attestations, and native install smoke tests is implemented. A digest-pinned, non-root clean-demo container using Docker's `none` network is built and exercised in CI but intentionally not published.
- Reproducible demo and architecture walkthrough.
- Threat-model review and dependency provenance.
- SBOM, checksums, versioned releases, and contribution guidance.
- Optional evidence-grounded AI coaching.

**Exit:** a new user can reproduce the documented demo from a clean machine using the supported prerequisites.

## Explicitly later

- Transparent interception and host certificate automation
- Web administration console
- Additional scenarios and provider APIs
- Production multi-user hosting
- Live-cloud differential contract tests
