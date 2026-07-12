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

**Status:** in development; the versioned protocol, metadata, decision, and redaction contracts are complete, while execution and governance enforcement remain.

- Add agent identity and run metadata.
- Implement governed tool gateway decisions: allow, deny, redact, approve.
- Record complete action traces.
- Add prompt-injection fixtures and repeated trials.
- Report task success, policy violations, and blast radius.

**Exit:** a local or external agent can be evaluated deterministically against the flagship scenario.

## M4 — Portfolio-quality release

- Cross-platform packaging and CI container.
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
