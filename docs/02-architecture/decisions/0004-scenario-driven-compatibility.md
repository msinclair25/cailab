---
title: "ADR-0004: Scenario-Driven Compatibility"
status: accepted
date: 2026-07-11
---

# ADR-0004: Scenario-driven compatibility

## Context

AWS, Microsoft Graph, and Google Workspace expose far more operations and semantics than a portfolio-scale local range can faithfully reproduce. Broad endpoint counts create an attractive but misleading measure of progress. The product needs enough compatibility for credible learning workflows while remaining testable and honest about differences.

## Decision

Provider operations enter the implementation only when required by an accepted scenario, public workflow, or explicit compatibility contract.

Each operation receives one or more fidelity claims:

1. **API-compatible:** supported request and response contract.
2. **Authorization-compatible:** supported access-decision semantics.
3. **Behavior-compatible:** supported side effects and audit events.
4. **Training-only:** scenario-useful behavior with no provider-parity claim.

Claims are operation-specific and link to contract tests and known limitations. Unsupported behavior fails clearly rather than silently approximating success.

## Consequences

### Positive

- Scope follows user value and testable scenarios.
- Compatibility claims remain reviewable and credible.
- Semantic gaps are visible to learners and agent evaluators.
- Implementation effort concentrates on end-to-end depth.

### Negative

- Some familiar tools will call unsupported auxiliary operations.
- Provider support may appear smaller than endpoint-oriented emulators.
- Compatibility documentation becomes a required maintenance artifact.

## Validation

- No operation is marked supported without a contract test.
- The compatibility matrix identifies fidelity and limitations for every supported operation.
- Scenario acceptance tests use only declared operations.
- Unexpected operations return a stable unsupported-operation error with diagnostic guidance.
