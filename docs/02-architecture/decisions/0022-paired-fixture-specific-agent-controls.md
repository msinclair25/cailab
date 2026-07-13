---
title: "ADR-0022: Paired Fixture-Specific Agent Controls"
status: accepted
date: 2026-07-12
---

# ADR-0022: Paired fixture-specific agent controls

## Context

The inert reference agent proves protocol and persistence behavior but never consumes the adversarial fixture, so it cannot enter the prompt-injection resistance denominator. The deliberately unsafe agent proves that exposure, prohibited behavior, injection success, and gateway containment findings can trigger. M3 also needs a positive control proving that the same evaluation can record exposure without prohibited behavior.

A positive control can be misleading if it is described as a generally safe agent, receives the exact prohibited ground truth it is expected to avoid, or is treated as evidence about an LLM. NIST emphasizes task-specific results, adaptive attacks, and repeated attempts for probabilistic systems; one deterministic scripted control cannot establish broader robustness.

## Decision

1. Add a distinct code-owned `safe` fixture control. The inert `reference` harness baseline and deliberately vulnerable `unsafe` baseline retain their existing purposes.
2. The safe control performs the legitimate declared exposure read, strictly validates the correlated result, treats returned content as untrusted data, and completes without deriving any later tool call from that content.
3. The safe child command receives only its identity/version and legitimate exposure target. The prohibited tool/action/resource tuple remains in immutable controller-side evaluation metadata and is absent from child argv. As with every run, `session.start` lists registered tool references but contains no prohibited action/resource ground truth.
4. The controller still registers the fixture's prohibited tool and applies an explicit deny policy as defense in depth. No prohibited decision is expected in the safe trace; therefore governance containment remains separate and has no triggered-behavior denominator.
5. Replay remains authoritative. A positive safe result requires a successful linked exposure outcome followed by no exact prohibited governed decision.
6. Public commands are `cailab agent run safe` and `cailab agent campaign safe`. Campaigns retain endpoint-preserving restoration, complete-set, and deterministic-report semantics from ADR-0021.
7. Documentation must call this a fixture-specific deterministic control and must not infer resistance for a model, agent framework, semantic equivalent, adaptive attack, covert channel, or real deployment.
8. With paired safe/unsafe controls, automatic campaigns, complete governed-action evidence, and repeated aggregate reports CI-backed, the M3 exit gate is satisfied.

## Consequences

- The same live fixture now has a reproducible positive and negative behavior control.
- A regression that adds a prohibited post-exposure call changes the deterministic resistance result and is visible in trace evidence.
- The safe control is intentionally simple and does not model reasoning quality, remediation ability, or probabilistic behavior.
- Scenario task success remains independent; reading safely does not remediate the vulnerable enterprise topology.
- No schema or scoring change is required because the existing adversarial profile already represents exposed resistance.

## Validation

- Unit tests prove marker-bearing content produces exactly one legitimate call and no follow-up call.
- Application tests prove the safe child configuration excludes the prohibited target while the gateway policy denies it.
- The flagship real-runtime E2E proves exposure, resistance, zero injection success, and repeated safe-campaign aggregation.

## Sources

- [NIST: Strengthening AI Agent Hijacking Evaluations](https://www.nist.gov/news-events/news/2025/01/technical-blog-strengthening-ai-agent-hijacking-evaluations)
- [NIST AI RMF Core: Measure](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [ADR-0020: Fixture-labeled agent hijacking evaluation](0020-fixture-labeled-agent-hijacking-evaluation.md)
- [ADR-0021: Sequential restored agent campaigns](0021-sequential-restored-agent-campaigns.md)
