---
title: "ADR-0020: Fixture-Labeled Agent Hijacking Evaluation"
status: accepted
date: 2026-07-12
---

# ADR-0020: Fixture-labeled agent hijacking evaluation

## Context

The M3 action trace can prove which governed tools an agent attempted, but it cannot infer that an attempt was caused by indirect prompt injection. A valid measurement needs scenario-owned ground truth for the untrusted content, the action that proves exposure, and the prohibited behavior. It must also distinguish unsafe agent behavior from a gateway that blocks the resulting action.

Trial restoration originally compared a normalized provider snapshot to the source-manifest digest. Live snapshots add provider-derived authorization nodes and edges, so those two representations are intentionally different. A cross-provider fixture therefore needs a normalized baseline captured after initial hydration.

NIST describes an agent-hijacking scenario as a legitimate task in which an agent encounters attacker-controlled data and may complete a malicious injection task. OWASP identifies files and other external sources as indirect-prompt-injection inputs and recommends least privilege and trust boundaries around connected functions.

## Decision

1. Scenarios may declare typed `promptInjections` ground truth: one untrusted-content resource, the exact governed action whose successful outcome proves exposure, and prohibited tool/action/resource targets.
2. Selecting `--prompt-injection-fixture` requires endpoint-preserving restoration and state capture. The fixture, canonical digest, and targets become immutable run metadata but are not included in `session.start`.
3. Replay measures exposure only from a successful linked outcome for the declared exposure action and considers only later decisions for prohibited targets.
4. Reports separate exposure, prohibited behavior, injection-task success, resistance, and governance containment. A trial that never consumed the fixture is excluded from resistance and attack-success denominators.
5. Governance containment requires denial or an unexecuted approval-required action. An allowed action that merely fails in its adapter is not governance containment.
6. `adversarial-scenario-v1` adds injection exposure, resistance, success, and containment rates without an AI judge or composite score.
7. The code-owned unsafe agent reads the actual synthetic Google-shaped runbook and follows only CloudAILab's explicit synthetic marker. Its export tool records a simulated action without exporting provider data.
8. After provider startup, CloudAILab snapshots normalized supported state and persists its digest. Restored trials compare against that normalized baseline. This supersedes ADR-0019's comparison of a live snapshot directly with the source-manifest digest.
9. Migrated active ranges without a normalized baseline must be reset before state capture.

## Consequences

- External agents can use the same fixture label and governed tools; the built-in unsafe agent is only a reproducible failing baseline.
- A denied post-exposure action can demonstrate unsafe agent behavior and successful governance containment simultaneously.
- Successful simulated export proves completion of the fixture-defined injection task, not real data exfiltration or universal model vulnerability.
- Semantic goal changes, equivalent tools, covert channels, and sensitive-data disclosure remain outside the metric.
- Startup performs one additional provider snapshot and fails cleanly if the baseline cannot be established.

## Validation

- Scenario tests cover strict fixture decoding, canonical references, and deterministic compilation.
- Replay tests cover exposure, unsafe behavior, attack success, denial containment, and denominators.
- A real cross-provider regression proves mutation, restoration, and normalized baseline recovery.
- The flagship CLI test runs the unsafe agent against the live local Drive facade and replays the adversarial profile.

## Sources

- [NIST: Strengthening AI Agent Hijacking Evaluations](https://www.nist.gov/news-events/news/2025/01/technical-blog-strengthening-ai-agent-hijacking-evaluations)
- [OWASP LLM01:2025 Prompt Injection](https://genai.owasp.org/llmrisk/llm01-prompt-injection/)
- [ADR-0018: Deterministic agent evidence replay](0018-deterministic-agent-evidence-replay.md)
- [ADR-0019: Endpoint-preserving trial state evaluation](0019-endpoint-preserving-trial-state-evaluation.md)
