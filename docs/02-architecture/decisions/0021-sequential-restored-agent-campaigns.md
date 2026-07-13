---
title: "ADR-0021: Sequential Restored Agent Campaigns"
status: accepted
date: 2026-07-12
---

# ADR-0021: Sequential restored agent campaigns

## Context

CloudAILab can persist, restore, and replay manually launched repeated trials, but manual index, count, reset, and replay coordination is error-prone. Automatic execution must not weaken the existing complete-set rule, silently reuse an immutable trial ID, compare trials that began from different provider state, or parallelize mutations against one local range.

The first automatic workflow needs a reproducible passing harness baseline and a reproducible unsafe prompt-injection baseline. General custom-agent campaigns add interactive approval, host-process, model-cost, and credential-forwarding choices that need a separate public contract rather than implicit reuse of fixture defaults.

## Decision

1. `cailab agent campaign reference` and `cailab agent campaign unsafe` are the initial automatic campaign workflows. Custom subprocess trials retain the explicit single-run interface.
2. A campaign contains 2 through 100 trials. It derives immutable IDs as `<trial-prefix>:<one-based-index>` and preflights the prefix, every derived ID, and all existing-record collisions before launching the first trial.
3. Every campaign trial requires endpoint-preserving fixture restoration and before/after state capture. The normalized baseline digest must match before the agent starts.
4. Trials run sequentially against one active range. Parallel campaign mutation is not supported.
5. The existing single-trial service remains authoritative for restoration, execution, evidence, terminal status, and cleanup. The campaign runner changes only trial identity, index, and declared count.
6. An agent-level failure may remain in the campaign when it has a terminal run plus complete before/after state evidence. Cancellation, restoration failure, persistence failure, incomplete state evidence, or other control-plane failure stops the campaign.
7. A complete campaign is immediately replayed through the existing deterministic compatibility and scoring engine. Text, JSON, and Markdown use the versioned `AgentEvaluationReport`; no separate mutable campaign score is introduced.
8. A stopped campaign preserves immutable recorded trials and reports that its incomplete set cannot be replayed. Automatic resume and trial-ID reuse are not supported; after range recovery the operator selects a new prefix.

## Consequences

- Repeated fixture results start from verified equivalent supported provider state and report trial count, terminal failures, configuration, denominators, and rates automatically.
- The runner cannot hide a failed agent trial by replacing it or shrinking the declared denominator.
- Campaign duration grows linearly and restoration remains non-atomic across provider runtimes.
- The initial CLI does not automatically campaign arbitrary subprocess agents or interactive approvals.
- No new persistence schema is required because immutable trial records and the deterministic evaluation report remain the sources of truth.

## Validation

- Application tests cover per-trial restoration, aggregate replay, terminal agent failures, all-ID preflight, bounded input, and fail-closed restoration.
- CLI tests cover public campaign routing and deterministic JSON report output.
- Existing replay and provider integration suites continue to verify complete-set compatibility, restoration, state evidence, and cleanup.

## Sources

- [NIST AI RMF Core: Measure](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [NIST: Strengthening AI Agent Hijacking Evaluations](https://www.nist.gov/news-events/news/2025/01/technical-blog-strengthening-ai-agent-hijacking-evaluations)
- [ADR-0018: Deterministic agent evidence replay](0018-deterministic-agent-evidence-replay.md)
- [ADR-0019: Endpoint-preserving trial state evaluation](0019-endpoint-preserving-trial-state-evaluation.md)
- [ADR-0020: Fixture-labeled agent hijacking evaluation](0020-fixture-labeled-agent-hijacking-evaluation.md)
