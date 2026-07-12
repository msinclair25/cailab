---
title: "ADR-0002: Deterministic Core with Optional AI"
status: accepted
date: 2026-07-11
---

# ADR-0002: Deterministic core with optional AI

## Context

Scenario hydration, authorization, and grading must be reproducible, explainable, usable offline, and suitable for CI. Language-model outputs can vary across providers, versions, prompts, and repeated runs.

## Decision

The following functions will not require a language model:

- Manifest parsing and validation
- Scenario compilation and hydration
- Authorization and policy decisions
- State normalization
- Attack-path discovery
- Verification and scoring
- Evidence generation

Optional model adapters may produce coaching, summaries, scenario-authoring assistance, or semantic analysis. Such output must consume structured evidence, remain clearly labeled, and never override deterministic findings.

Agent-under-test behavior is expected to be nondeterministic. It will be evaluated against deterministic action-level policies and scenario invariants across one or more trials.

## Consequences

### Positive

- Core operation is local-first and CI-friendly.
- Results remain explainable and testable.
- The project is not coupled to model availability or pricing.

### Negative

- The project must implement its own scenario compiler and evaluator.
- Some qualitative agent behavior will require carefully designed measurable proxies.

## Validation

The complete flagship scenario, including verification, must pass in CI with no API keys and no network access beyond required local containers.
