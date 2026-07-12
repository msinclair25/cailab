---
title: Project Charter
status: active
---

# Project charter

## Mission

Enable engineers, security practitioners, students, and AI-agent developers to learn and test enterprise identity, authorization, cyberattack paths, and AI governance across AWS, Microsoft, and Google-compatible environments without requiring production tenants.

## Problem

Cloud IAM training is commonly provider-specific and resource-centric. Real organizations combine human identities, workload identities, applications, contractors, directory synchronization, federation, and data across multiple providers. AI agents add another acting principal whose delegated authority, tool access, data exposure, and approval boundaries must be tested.

CloudAILab will model those relationships as one inspectable system while presenting deliberately scoped provider-compatible interfaces.

## Primary users

1. Engineers learning enterprise IAM and federation.
2. Security practitioners investigating and remediating attack paths.
3. Agent developers evaluating permissions, prompt injection, and governance controls.
4. Teams benchmarking security automation in local development and CI.

## Intended outcomes

- A learner can launch a reproducible enterprise scenario with one command.
- Familiar SDKs and command-line tools can inspect and mutate supported resources.
- A learner can trace why access is allowed across tenant and provider boundaries.
- An agent can be evaluated as an identity with explicit authority and tools.
- Verification produces deterministic findings with evidence and remediation guidance.
- Scenario results can be exported for humans and CI systems.

## Non-goals

- Reimplementing every AWS, Microsoft 365, or Google Workspace API.
- Claiming behavioral parity where only schema or API compatibility exists.
- Replacing testing against real provider sandboxes before production deployment.
- Using a language model as the source of truth for authorization or grading.
- Running offensive activity against external systems.
- Providing host isolation without an explicitly enabled isolation mode.

## Product principles

### Depth before breadth

The first release will implement one credible cross-provider scenario end to end. Additional APIs are added only when a scenario or compatibility contract requires them.

### Evidence before narrative

Policy decisions and scores come from typed state, audit events, and deterministic invariants. AI-generated coaching is optional and must cite that evidence.

### Compatibility is a contract

Each supported operation is assigned a documented fidelity level:

1. **API-compatible:** accepted request and response shapes are compatible.
2. **Authorization-compatible:** relevant access decisions are evaluated.
3. **Behavior-compatible:** expected side effects and audit events are modeled.
4. **Training-only:** useful for the scenario but not asserted to match the provider.

## Initial success criteria

- A new user with the documented prerequisites can run the flagship scenario locally.
- The same scenario seed produces the same initial canonical state.
- Supported provider operations pass contract tests.
- The evaluator identifies the intended attack path and proves when it is closed.
- An agent run produces a trace of identity, inputs, tool calls, policy decisions, and outcomes.
- Linux CI can run the scenario without hosted AI or cloud credentials.
