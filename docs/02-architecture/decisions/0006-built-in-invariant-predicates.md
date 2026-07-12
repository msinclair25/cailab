---
title: "ADR-0006: Built-In Invariant Predicates Before a Policy DSL"
status: accepted
date: 2026-07-11
---

# ADR-0006: Built-in invariant predicates before a policy DSL

## Context

CloudAILab needs deterministic, explainable verification, but M0 has only two required graph assertions: a directed path must exist or must be absent. Introducing a general policy language now would add another parser, runtime, dependency, and security boundary before the domain semantics are stable.

## Decision

M0 uses typed, built-in invariant predicates compiled from the versioned scenario schema. The accepted predicates are `path_exists` and `path_absent`. Each result includes a stable invariant ID, pass/fail status, severity, message, and path evidence.

OPA or another policy engine is deferred. A general policy DSL may be introduced only after multiple accepted scenarios demonstrate requirements that cannot be expressed cleanly as typed predicates. That change requires a new ADR, sandbox and resource-limit analysis, deterministic tests, and a migration path for existing scenarios.

## Consequences

### Positive

- The M0 evaluator is small, deterministic, and directly testable.
- Scenario authors receive schema and type errors instead of runtime policy failures.
- No general-purpose evaluator is exposed to untrusted scenario input.

### Negative

- Every new assertion type requires a versioned code and schema change.
- Complex authorization semantics will eventually need richer typed predicates or a policy engine.

## Validation

- Unit tests cover passing and failing path assertions with evidence.
- Scenario validation rejects unknown predicate types.
- Recompiling the same manifest and seed produces the same digest and ordered verification inputs.
- A future policy engine must produce equivalent results for all predicates it supersedes.
