---
title: Learning Contract v1alpha1
status: active
last_reviewed: 2026-07-13
---

# Learning contract v1alpha1

The data-only [learning catalog](../../learning/catalog.json) is validated against the [Draft 2020-12 schema](../../schemas/learning/v1alpha1/learning-catalog.json) and semantic repository rules. It is navigation and instructional metadata. It cannot select an executable, provider runtime, policy, verification predicate, or protected ground truth.

## Lesson contract

Every lesson declares:

- a stable `lesson:` identifier, title, track, difficulty, and estimated duration;
- prerequisite lesson IDs and common-core outcome dependencies;
- one explicit safety boundary;
- a scenario or workflow binding and an existing repository guide;
- observable learning outcomes;
- coverage for delivery, identity, runtime/resource, data, evidence/governance, and remediation layers;
- one to three progressive hints;
- deterministic CLI or persisted-evidence verification commands;
- explicit cleanup requirements and commands;
- production context and reflection prompts.

Lesson IDs are not renumbered. Removed lessons are deprecated in documentation before deletion from a future schema version. `v1alpha1` may evolve before M5 package stabilization; a CLI or web console must not infer fields outside this schema.

## Common core

The common core is a dependency vocabulary, not a mandatory linear course. It covers:

- delivery fundamentals;
- CLI, APIs, and structured data;
- authentication and authorization;
- OAuth and OIDC;
- policy and scope;
- control and data planes;
- CI identity;
- secrets;
- evidence;
- deterministic verification;
- cleanup and ownership.

Tracks can reuse these outcomes without duplicating foundational explanations. Current track values leave room for cloud/DevOps, IAM/security, platform engineering, security operations, AI governance, and community authoring while the initial path stays focused on implemented CloudAILab behavior.

## Mission layers

Each lesson marks every layer as `covered`, `supporting`, or `not_applicable`:

| Layer | Meaning |
|---|---|
| Delivery | Source, build, CI identity, artifact, or release flow |
| Identity | Human, group, workload, application, role, token, or trust behavior |
| Runtime/resource | Provider facade, emulator, process, container, or protected resource |
| Data | Synthetic document, object, metadata, or classified payload |
| Evidence/governance | Decisions, reports, checksums, provenance, policy, or verification evidence |
| Remediation | A controlled change and proof that intended access survives |

Explicit `not_applicable` values prevent a narrow lesson from being mistaken for a full-stack exercise.

## Validation

From a source checkout:

```bash
go run ./internal/tools/learningcheck .
```

Validation rejects schema drift, unknown fields, duplicate IDs, unknown common-core or prerequisite references, prerequisite cycles, missing/non-regular guides, unordered hint levels, inconsistent cleanup metadata, unknown path lessons, and path order that violates prerequisites.

## Non-goals

The contract does not provide rosters, instructor dashboards, grades, certificates, employment claims, LMS integration, hosted progress tracking, model-generated scoring, or executable lesson hooks. A future consumer must reuse CloudAILab's deterministic application contracts rather than create separate authorization or grading logic.
