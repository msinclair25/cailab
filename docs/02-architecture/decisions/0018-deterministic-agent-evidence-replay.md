---
title: "ADR-0018: Deterministic Agent Evidence Replay"
status: accepted
date: 2026-07-12
---

# ADR-0018: Deterministic agent evidence replay

## Context

CloudAILab persists immutable agent-run configuration plus linked decision, approval, and tool-outcome evidence. M3 still needs a repeatable way to convert that evidence into trial metrics without executing the agent or trusting timestamps, narrative model output, or mutable provider state.

The current trace does not contain raw protocol messages, tool arguments, successful tool content, per-trial scenario snapshots, or invariant results. Those omissions are intentional security controls, but they also limit which claims replay can support. Treating terminal process completion as task success, observed targets as effective blast radius, or protected hashes as proof of no data exposure would produce attractive but invalid scores.

## Decision

1. Define versioned `AgentTrace` and `AgentEvaluationReport` contracts. A trace contains one terminal run plus its evidence-safe decision, approval, and outcome records. It never contains raw protocol frames, tool arguments, tool content, or child diagnostics.
2. Add replay profile `governed-evidence-v1`. Replay reads existing records through their store integrity checks, validates all run/trial/correlation, sequence, decision, approval, and outcome links again, and performs no agent, tool, provider, policy, or model execution.
3. Aggregate only an explicitly selected complete trial set. Trials must share the same range run, scenario digest and seed, agent identity/version/provider/model, policy digest, prompt hash, ordered tool references, execution profile, and declared trial count. Their one-based indices must be unique and contiguous.
4. Sort trials by declared index before scoring. Wall-clock timestamps remain validated diagnostic evidence but do not affect compatibility, ordering, digests of the replay configuration, counts, or rates.
5. Report primitive counts and transparent numerator/denominator/rate objects. The initial measured facts are terminal completion, final authorization dispositions, policy denials, approval decisions, tool outcomes, missing outcome evidence, and distinct confidential/restricted action-resource pairs observed in the trace.
6. Do not calculate a composite score. Report task success, prompt-injection resistance, remediation quality, sensitive-data exposure, and effective blast radius as `notMeasured` with a stable reason until the trace includes the evidence required for those constructs.
7. Hash each canonical trace and the compatible replay configuration for reproducibility. These hashes detect accidental differences; they are not signatures and do not make a local SQLite database tamper-proof.
8. `cailab agent replay` accepts one or more explicit trial IDs and emits deterministic text, JSON, or Markdown. Replaying stopped runs requires the recorded range run ID.

## Consequences

### Positive

- Repeating replay over equivalent stored evidence produces byte-stable JSON without running untrusted code or changing range state.
- Counts, denominators, rates, configuration, failures, and unavailable measurements remain visible rather than hidden behind one score.
- Incomplete or cherry-picked declared trial sets fail instead of silently changing denominators.
- Cross-record corruption and inconsistent evidence linkage fail closed before any metric is emitted.

### Negative

- Users must currently launch each repeated trial and restore any required provider state themselves.
- Compatible configuration does not prove that mutable provider state was equivalent at the start of each trial.
- The first profile cannot claim mission success, prompt-injection resistance, data-loss prevention, remediation quality, or effective blast radius.
- Trace/configuration hashes provide reproducibility identifiers, not provenance or protection against a local user who can replace the database and recompute output.

## Validation

- Domain tests cover deterministic sorting/digests, count and rate semantics, approval dispositions, missing outcomes, incomplete sets, incompatible configuration, duplicate indices/IDs, and broken links.
- State and application tests read traces through existing hash-chain checks and aggregate real persisted subprocess trials.
- CLI tests exercise JSON replay and deterministic text/Markdown/JSON rendering without unmeasured claims.
- Schemas define both public contracts, and documentation checks cover the compatibility record and workflow.

## Sources

- [NIST AI RMF Core: Measure](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [NIST AI RMF Playbook: Measure 2.1](https://airc.nist.gov/airmf-resources/playbook/measure/)
- [NIST Metrics and Measures](https://www.nist.gov/itl/ai/ai-standards-and-guidelines-group/metrics-and-measures)
- [ADR-0013: Deterministic tool policy and decision evidence](0013-deterministic-tool-policy-and-evidence.md)
- [ADR-0015: Scenario-bound public agent runs](0015-scenario-bound-public-agent-runs.md)
