---
title: "ADR-0013: Deterministic Tool Policy and Decision Evidence"
status: accepted
date: 2026-07-12
---

# ADR-0013: Deterministic tool policy and decision evidence

## Context

The M3 session controller can receive a declared tool call, but process supervision and protocol validity do not authorize an action. CloudAILab needs a deterministic boundary that cannot be widened by a model, a permissive provider emulator, rule order, or a missing audit write. It also needs useful decision evidence without persisting raw tool arguments or claiming that an authorization-only slice executed a tool.

## Decision

CloudAILab will evaluate governed tool calls and append decision evidence under these rules:

1. A versioned `GovernancePolicy` uses exact agent, tool, action, resource, resource-tenant, and classification matches. The only default effect is `deny`.
2. The validated tool manifest is an independent permission ceiling. A policy rule cannot grant an action, resource, or tenant absent from the manifest.
3. Matching rules are order-independent. Precedence is `deny` → `require_approval` → `redact` → `allow`; the lexically first matching rule ID at the winning effect is the stable reason code.
4. Redaction pointers are merged, deduplicated, and sorted. If any required pointer cannot be applied, the evaluator returns a stable deny rather than passing unredacted data.
5. Approval identifiers derive deterministically from the run, trial, correlation, and winning rule. Approval remains `not_executed` until a later explicit resolution flow re-evaluates it.
6. The gateway hashes canonical original arguments but does not persist them. Redacted arguments are prepared in memory for a later executor.
7. A tool response is eligible for emission only after its complete decision event commits. Evidence failure fails the call closed.
8. SQLite assigns a contiguous per-run, per-trial sequence in the same transaction that appends the event. A correlation can have only one decision event.
9. Stored event JSON is canonical and chained through record hashes plus a transactional head. Reads verify schema, sequence, correlation, chain, and head consistency.
10. The hash chain detects accidental mutation, deletion, or incomplete writes. It is not a cryptographic defense against an attacker who can rewrite the database and recompute the chain.
11. This slice does not execute tools. `allow` and `redact` decisions return `not_executed` until a separately tested executor records the actual outcome.

## Consequences

### Positive

- Explicit deny and manifest limits cannot be bypassed by rule order.
- Every successful gateway response has durable, validated decision evidence.
- Audit storage contains hashes and metadata rather than raw tool arguments.
- Repeated evaluations of the same inputs produce the same decision, redactions, reason, and approval identifier.
- Transactional sequence assignment supports deterministic replay order independently of timestamps.

### Negative

- Exact-match rules are intentionally verbose and do not yet support conditions, wildcards, or hierarchical resources.
- Full JSON Schema input-instance validation remains part of the executor slice.
- The SQLite chain is integrity diagnostics, not non-repudiation or protection from the local OS account.
- Allowed and redacted results are useful authorization evidence but not proof that a tool ran.
- A public policy-registration and agent-run workflow remains to be designed.

## Validation

- Unit tests cover default deny, manifest permission ceilings, all four effects, deny precedence, order independence, stable approvals, merged redactions, and fail-closed redaction errors.
- Gateway tests prove evidence commits before responses and persistence failure returns no result.
- A real SQLite composition test proves the gateway stores hashed `not_executed` evidence without raw arguments.
- Migration and persistence tests cover reopen, monotonic sequence, duplicate correlation, inactive runs, invalid drafts, stored mutation, record-hash mutation, and record deletion.
- Session integration tests exercise the gateway through the subprocess controller; race tests cover the affected packages.

## Sources

- [NIST: Building Evaluation Probes into Agentic AI](https://www.nist.gov/programs-projects/building-evaluation-probes-agentic-ai)
- [SQLite transactions](https://www.sqlite.org/lang_transaction.html)
- [SQLite table constraints](https://www.sqlite.org/lang_createtable.html)
- [SQLite is transactional](https://www.sqlite.org/transactional.html)
- [ADR-0011: Versioned agent JSON Lines protocol](0011-versioned-agent-json-lines-protocol.md)
- [ADR-0012: Owned agent subprocess sessions](0012-owned-agent-subprocess-sessions.md)
