---
title: "ADR-0016: Immutable Approval Resolution"
status: accepted
date: 2026-07-12
---

# ADR-0016: Immutable approval resolution

## Context

`require_approval` already stopped a governed tool call and returned a deterministic approval identifier, but it did not define who could resolve the request, how stale policy was handled, or what evidence authorized a later side effect. Treating a wire message or a bare boolean as authorization would permit replay, lose human-accountability context, and let an approval override a later deny or redaction requirement.

Approval prompts also sit on a sensitive boundary. They need enough canonical context for a reviewer to decide without disclosing raw model-controlled arguments, and unattended runs must not block or approve implicitly.

## Decision

1. An initial `require_approval` decision remains immutable and `not_executed`. Its identifier covers the run, trial, call correlation, and every matching approval rule at the winning precedence.
2. The local approver receives the agent, tool, action, canonical resource metadata, reason code, input hash, and decision identifier, but not raw tool arguments.
3. `cailab agent run subprocess` defaults to `--approval-mode reject`. Interactive resolution requires `--approval-mode prompt`, a recorded `--approver`, and the exact text `approve <approval-id>`; every other response rejects.
4. After a human response, the gateway re-evaluates the same canonical request against the current policy and manifest ceiling. A changed or mismatched requirement fails closed. Approval never overrides deny precedence, and applicable redaction still applies.
5. Rejection produces a deterministic `approval:rejected` deny. Approval produces only an `allow` or `redact` decision.
6. The store appends a strict `ApprovalResolutionEvent` linked to the original decision record before `approval.resolved` is emitted or execution continues. The record includes the resolver identity, resulting decision, and original input hash, but no raw arguments.
7. An approved tool outcome must link both the original decision and the exact approved resolution. A rejected resolution cannot authorize execution. A resolution is consumed once in the live gateway, and duplicate persisted resolutions are rejected.
8. Approval handling remains local controller behavior in protocol `1.1`; it does not create process, network, filesystem, or identity isolation.

## Consequences

### Positive

- Unattended and CI runs fail closed without waiting for input.
- Reviewers see stable canonical context while model-controlled arguments remain outside prompts and stored evidence.
- Policy drift, mismatched resolution messages, persistence failures, and replay attempts cannot widen authority.
- Evidence distinguishes the original policy requirement, the accountable resolution, and any resulting tool outcome.
- Redaction and deny precedence remain deterministic after approval.

### Negative

- The first interactive approver is a local terminal workflow rather than a remote or multi-party approval service.
- A reviewer sees an input hash instead of the full request; richer protected previews require a separate disclosure design.
- Approval waits count against the whole agent-session timeout.
- The SQLite linkage detects inconsistent records but is not tamper-proof against the local OS account.

## Validation

- Policy tests cover stable multi-rule identifiers, approve/reject behavior, retained redaction, mismatches, and policy drift.
- Session tests cover the required → resolved → result sequence, mismatched resolutions, and cancellation.
- Gateway tests cover persistence-before-response, no execution on rejection or persistence failure, and one-use continuation.
- State tests cover linked persistence, reopen, duplicates, rejected-outcome refusal, approved outcomes, and mutation detection.
- App and CLI tests exercise real subprocess continuation, default rejection, evidence-safe summaries, and exact prompt confirmation without raw arguments.

## Sources

- [NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [NIST AI RMF Appendix C: Human-AI Interaction](https://airc.nist.gov/airmf-resources/airmf/appendices/app-c-ai-risk-management-and-human-ai-interaction/)
- [NIST AI RMF Playbook: Measure](https://airc.nist.gov/airmf-resources/playbook/measure/)
- [ADR-0013: Deterministic tool policy and decision evidence](0013-deterministic-tool-policy-and-evidence.md)
- [ADR-0015: Scenario-bound public agent runs](0015-scenario-bound-public-agent-runs.md)
