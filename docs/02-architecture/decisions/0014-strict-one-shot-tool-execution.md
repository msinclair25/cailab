---
title: "ADR-0014: Strict One-Shot Tool Execution"
status: accepted
date: 2026-07-12
---

# ADR-0014: Strict one-shot tool execution

## Context

ADR-0013 deliberately returned `not_executed` for allow and redact decisions. Executing user-registered tools adds two untrusted inputs: the manifest's JSON Schema and the tool process itself. Partial schema interpretation, remote schema loading, inherited credentials, shell evaluation, unbounded output, unredacted results, or rewriting the original authorization record would weaken the governance boundary.

## Decision

CloudAILab will execute an allowed or redacted tool call under these constraints:

1. Tool input instances are validated with pinned `github.com/santhosh-tekuri/jsonschema/v6` `v6.0.2` using Draft 2020-12. Schemas compile during manifest validation.
2. `$ref` and `$dynamicRef` must be fragment-local. The compiler has no external URL loader, so manifest validation cannot fetch schemas from the network or filesystem.
3. Schema-invalid input produces a stable deny and never launches a process.
4. A tool is one direct subprocess with an absolute executable, explicit argv, absolute working directory, complete explicit environment, manifest timeout, bounded stdout/stderr, and bounded cleanup. No shell is involved.
5. The tool receives exactly one versioned JSON request on stdin and must return exactly one correlated JSON response on stdout. Diagnostics use stderr. Extra frames, malformed UTF-8/JSON, wrong correlation, output overflow, timeout, cancellation, and non-zero exit fail with stable executor codes.
6. Only `allow` and `redact` decisions can reach the executor. Deny and approval-required decisions remain `not_executed`.
7. Policy redaction pointers apply to input arguments before execution. Tool-manifest `sensitiveFields` pointers apply to successful output before it can reach the agent or be hashed. Missing output pointers fail closed.
8. The immutable decision event commits before execution. A separate `ToolOutcomeEvent` records succeeded or failed execution and links to the decision event and its stored record hash.
9. Successful outcome evidence hashes the exact protected content returned to the agent. Raw input and raw tool output are not persisted.
10. A tool result is returned to the agent only after its outcome event commits. If outcome persistence fails after the subprocess ran, the result is withheld and the failure is surfaced; the pre-execution decision record remains durable.
11. Tool subprocess ownership is not isolation. The process retains the launching OS user's filesystem, network, and syscall authority, and direct-child cancellation does not contain independently detached descendants.

## Consequences

### Positive

- Draft 2020-12 behavior is delegated to a focused, pinned implementation rather than a partial validator.
- Declarative schemas cannot trigger external retrieval.
- Invalid, denied, and approval-pending calls cannot start a tool process.
- Tool timeouts and protocol failures are deterministic, correlated, and evidence-backed.
- Append-only decision intent and actual outcome remain distinguishable without mutating history.
- Sensitive successful output is redacted before return and evidence hashing.

### Negative

- The JSON Schema validator adds an Apache-2.0 dependency and its transitive `golang.org/x/text` dependency.
- Tool processes remain unisolated and therefore are not safe for arbitrary untrusted code.
- Output pointers must exist in successful content or the call fails closed.
- A host or disk failure after tool side effects but before outcome commit can leave durable authorization intent without a durable outcome.
- Public tool registration, runtime directory/environment UX, approval resolution, and isolated backends remain later work.

## Validation

- Schema tests cover required fields, types, patterns, additional properties, local references, and rejected remote references.
- Real subprocess tests cover success, declared failure, invalid input before launch, timeout, non-zero exit, malformed and extra output, wrong correlation, bounded diagnostics, and deterministic reference-tool behavior.
- Gateway tests prove deny/approval/invalid input do not execute, executor failures produce linked failed outcomes, sensitive output is redacted before response/hash, and outcome-persistence failure withholds the result.
- State tests cover linked success, duplicate outcome, non-executable decision rejection, migration, persistence, and stored-outcome mutation.
- A nested integration test exercises agent subprocess → gateway → tool subprocess → outcome evidence.

## Sources

- [`jsonschema/v6` v6.0.2 package documentation](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6@v6.0.2)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [Go `os/exec` package](https://pkg.go.dev/os/exec)
- [ADR-0013: Deterministic tool policy and decision evidence](0013-deterministic-tool-policy-and-evidence.md)
