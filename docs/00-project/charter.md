---
title: Project Charter
status: active
---

# Project charter

## Mission

Enable engineers, security practitioners, students, career-transitioning technologists, and AI-agent developers to learn and test how modern enterprise systems are delivered, connected, secured, operated, and governed across AWS, Microsoft, and Google-compatible environments without requiring production tenants. Identity and authorization are the technical spine connecting those layers, and portable evidence helps users demonstrate the work they actually completed.

## Problem

Cloud, DevOps, IAM, security-operations, and AI-governance training are commonly taught as disconnected tool or provider topics. Real organizations connect source control, delivery identities, human and workload identities, applications, runtime platforms, federation, secrets, data, telemetry, and response across multiple providers. AI agents add another acting principal whose delegated authority, tool access, data exposure, and approval boundaries must be tested.

CloudAILab will model those relationships as one inspectable system while presenting deliberately scoped provider-compatible interfaces.

## Primary users

1. Cloud, DevOps, and platform engineers learning secure enterprise delivery and operations.
2. Engineers learning enterprise IAM and federation.
3. Security practitioners investigating and remediating attack paths.
4. Agent developers evaluating permissions, prompt injection, and governance controls.
5. Teams benchmarking security automation in local development and CI.
6. People entering, returning to, or transforming their role in technology who need realistic practice and portfolio evidence beyond isolated tutorials.

## Intended outcomes

- A learner can launch a reproducible enterprise scenario with one command.
- Familiar SDKs and command-line tools can inspect and mutate supported resources.
- A learner can trace why access is allowed across tenant and provider boundaries.
- A learner can follow one system from source and delivery identity through workload, protected data, telemetry, investigation, and remediation.
- An agent can be evaluated as an identity with explicit authority and tools.
- Verification produces deterministic findings with evidence and remediation guidance.
- Scenario results can be exported for humans and CI systems.
- A user can export a redacted, versioned proof-of-work bundle showing the recorded scenario, investigation, decisions, remediation, preserved access, verification, limitations, and cleanup.

## Non-goals

- Reimplementing every AWS, Microsoft 365, or Google Workspace API.
- Becoming a broad catalog of disconnected cloud, DevOps, Kubernetes, or AI tutorials.
- Providing classroom administration, instructor dashboards, student rosters, LMS integration, accreditation, or institution-specific course delivery.
- Issuing credentials, certifying general competence, ranking people, making hiring recommendations, or guaranteeing employment outcomes.
- Claiming behavioral parity where only schema or API compatibility exists.
- Replacing testing against real provider sandboxes before production deployment.
- Using a language model as the source of truth for authorization or grading.
- Running offensive activity against external systems.
- Providing host isolation without an explicitly enabled isolation mode.

## Product principles

### Depth-first, breadth through scenarios

Identity and authorization receive deep treatment. Breadth comes from end-to-end missions that connect delivery, runtime/resource, data, evidence/governance, and remediation layers. Additional platforms, tools, and APIs are added only when a documented scenario and compatibility contract require them.

### Evidence before narrative

Policy decisions and scores come from typed state, audit events, and deterministic invariants. AI-generated coaching is optional and must cite that evidence.

### Compatibility is a contract

Each supported operation is assigned a documented fidelity level:

1. **API-compatible:** accepted request and response shapes are compatible.
2. **Authorization-compatible:** relevant access decisions are evaluated.
3. **Behavior-compatible:** expected side effects and audit events are modeled.
4. **Training-only:** useful for the scenario but not asserted to match the provider.

### Sandbox, not classroom platform

CloudAILab owns an excellent local sandbox, self-guided missions, deterministic verification, portable evidence, and extension contracts. Educational institutions may use or build on those capabilities, but classroom and institutional delivery layers are separate products outside this repository.

### Demonstration, not credentialing

Job readiness is supported through realistic practice and evidence of recorded work. A CloudAILab export documents what occurred in a bounded synthetic scenario; it does not establish identity, independent authorship, general competence, or employability and is not a certificate.

## Initial success criteria

- A new user with the documented prerequisites can run the flagship scenario locally.
- The same scenario seed produces the same initial canonical state.
- Supported provider operations pass contract tests.
- The evaluator identifies the intended attack path and proves when it is closed.
- An agent run produces a trace of identity, inputs, tool calls, policy decisions, and outcomes.
- Linux CI can run the scenario without hosted AI or cloud credentials.
