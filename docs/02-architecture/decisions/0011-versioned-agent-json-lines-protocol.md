---
title: "ADR-0011: Versioned Agent JSON Lines Protocol"
status: accepted
date: 2026-07-12
---

# ADR-0011: Versioned agent JSON Lines protocol

## Context

M3 must evaluate agents implemented in different languages and frameworks without coupling deterministic authorization or evidence to a hosted model API. The boundary will eventually launch user-selected subprocesses and accept tool calls from untrusted output. Framing ambiguity, oversized messages, undeclared tool authority, hidden shell evaluation, incomplete run metadata, and inconsistent decision records would make both security and reproducibility brittle.

The protocol must stabilize before subprocess execution. JSON Lines provides a simple language-neutral stream in which each UTF-8 line is a complete JSON value. JSON Schema Draft 2020-12 provides a portable authoring contract, and MCP's tool contract demonstrates the usefulness of explicit JSON input schemas without requiring CloudAILab to claim MCP compatibility.

## Decision

CloudAILab will use a versioned UTF-8 JSON Lines subprocess protocol with these rules:

1. Protocol messages use `protocolVersion: "1.0"`; persisted manifests and records use `apiVersion: cloudailab.dev/agent/v1alpha1`.
2. Every line contains exactly one JSON object and is capped at 1 MiB. Empty, invalid UTF-8, duplicate-key, unknown-field, malformed, and oversized frames are rejected.
3. Standard output is reserved for protocol frames. Human-oriented subprocess diagnostics will use standard error when execution is added.
4. Initial messages are `session.start`, `agent.ready`, `tool.call`, `tool.result`, `approval.required`, `approval.resolved`, `session.complete`, and `protocol.error`.
5. Tool calls and their results use explicit message and correlation identifiers. Protocol ordering is not inferred from wall-clock timestamps.
6. Tool manifests declare a semantic version, strict JSON Schema 2020-12 object input, tenant/action/resource permissions, risk, timeout, subprocess argv, expected network/filesystem isolation, and RFC 6901 sensitive-field pointers.
7. A command is an argv vector. CloudAILab will not concatenate it into shell text or execute it merely because a manifest was decoded.
8. Isolation declarations describe requirements; they are not proof of enforcement. `host` explicitly means the tool expects ambient host authority.
9. Gateway decisions are one of `allow`, `deny`, `redact`, or `require_approval`. Denied and approval-pending actions have a `not_executed` outcome.
10. Redaction replaces selected RFC 6901 values before persistence or return. Missing or invalid pointers fail closed.
11. Agent-run records preserve scenario, agent/model, policy, prompt, tool, seed, trial, and lifecycle metadata. Decision events preserve actor, tenant, action, resource, decision, outcome, correlation, sequence, and input/output hashes.
12. Schema and manifest hashes use deterministic CloudAILab JSON normalization. This v1alpha1 contract does not claim full RFC 8785 JSON Canonicalization Scheme conformance.

HTTP and MCP adapters remain deferred until the subprocess contract and deterministic evidence model pass the M3 flagship evaluation.

## Consequences

### Positive

- Reference agents can be implemented without importing Go packages.
- Strict framing and resource limits bound an untrusted parsing surface.
- Tool authority and expected isolation are reviewable before execution.
- Decision effects cannot be conflated: approval is not allow, and deny cannot report execution success.
- Stable hashes and correlation identifiers support replay and repeated-trial comparison.
- The portable event model can later map to OpenTelemetry without depending on an exporter.

### Negative

- A custom protocol adds adapter work for existing agent frameworks.
- JSON Lines does not provide authentication, encryption, flow control, or process isolation.
- JSON Schema declaration does not itself authorize a tool call.
- The initial protocol is single-agent and request/response oriented; streaming tool content and multi-agent routing are deferred.
- Full input-schema instance evaluation, subprocess lifecycle, trace persistence, and approval UI remain later M3 slices.

## Validation

- Typed Go validation and committed JSON Schemas reject unknown fields and invalid contracts.
- Contract tests cover all four decision effects and correlation requirements.
- Negative tests cover duplicate JSON keys, invalid UTF-8, oversized frames, unsafe command text, open input schemas, invalid isolation declarations, and invalid redaction pointers.
- Stable-hash tests prove that insignificant input-schema whitespace and object-key order do not change a tool digest.
- Redaction tests cover escaped pointer tokens, arrays, missing paths, and input immutability.
- A fuzz target exercises arbitrary protocol frames without executing tools.

## Sources

- [JSON Lines](https://jsonlines.org/)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [RFC 6901: JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901.html)
- [RFC 8785: JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)
- [MCP tool contract](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
- [NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [NIST agentic evaluation probes](https://www.nist.gov/programs-projects/building-evaluation-probes-agentic-ai)
- [OpenTelemetry event conventions](https://opentelemetry.io/docs/specs/semconv/general/events/)
