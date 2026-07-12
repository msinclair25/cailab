---
title: Agent Protocol v1alpha1
status: m3-development
last_reviewed: 2026-07-12
---

# Agent protocol v1alpha1

## Current scope

CloudAILab now defines typed and schema-backed contracts for M3 agent runs, tool registration, governance policy, agent messages, tool execution, decisions, redaction, decision events, and tool outcomes. The internal harness owns agent and one-shot tool subprocesses, applies exact-match policy and manifest ceilings, validates Draft 2020-12 input offline, protects successful output, and commits linked decision/outcome evidence. Supported public registration/run commands, enforced isolation, full trace replay, interactive approval resolution, and aggregate metrics remain later M3 work.

The normative schemas are:

- [Tool manifest](../../schemas/agent/v1alpha1/tool-manifest.json)
- [Agent run](../../schemas/agent/v1alpha1/agent-run.json)
- [Protocol message](../../schemas/agent/v1alpha1/protocol-message.json)
- [Decision event](../../schemas/agent/v1alpha1/decision-event.json)
- [Governance policy](../../schemas/agent/v1alpha1/governance-policy.json)
- [Tool execution message](../../schemas/agent/v1alpha1/tool-execution-message.json)
- [Tool outcome event](../../schemas/agent/v1alpha1/tool-outcome-event.json)

The executable validation, policy, gateway, and session contracts are in [`internal/agent`](../../internal/agent). [ADR-0011](../02-architecture/decisions/0011-versioned-agent-json-lines-protocol.md) defines the wire contract; [ADR-0012](../02-architecture/decisions/0012-owned-agent-subprocess-sessions.md) defines the owned-process lifecycle; [ADR-0013](../02-architecture/decisions/0013-deterministic-tool-policy-and-evidence.md) defines policy and evidence semantics.

## Framing

- UTF-8 only
- one JSON object per line
- newline after every emitted frame
- maximum encoded frame size: 1 MiB
- no blank frames, duplicate object keys, unknown typed fields, or trailing JSON values
- stdout reserved for protocol traffic
- stderr reserved for diagnostics

JSON Lines defines each line as a valid JSON value and recommends a line terminator after the final value. CloudAILab narrows that format to one object per line so every frame has a version, identifier, type, and payload.

## Message flow

| Message | Sender | Purpose |
|---|---|---|
| `session.start` | Controller | Binds run, trial, scenario digest, policy version, and allowed tool versions. |
| `agent.ready` | Agent | Confirms the expected agent identity and version. |
| `tool.call` | Agent | Requests one declared tool with JSON-object arguments. |
| `tool.result` | Controller | Returns the deterministic decision, execution status, and optional content. |
| `approval.required` | Controller | States that the correlated call was not executed and needs a decision. |
| `approval.resolved` | Controller | Records the approval identity and outcome. It is not itself a tool result. |
| `session.complete` | Agent | Reports completed, failed, or canceled agent work. |
| `protocol.error` | Either | Reports a stable protocol error without changing authorization state. |

`tool.result`, `approval.required`, and `approval.resolved` require `correlationId`. The session controller additionally enforces direction, lifecycle order, expected agent identity/version, unique message IDs, declared-tool membership, and response correlation. Typed decoding by itself still validates only structure and payload semantics.

## Subprocess lifecycle

The internal controller requires an absolute executable path, an absolute working directory, and an explicit complete environment. It never invokes a shell and does not inherit the controller's environment by default. It sends `session.start`, requires a matching `agent.ready` before tool activity, bounds handshake and whole-session time, caps both frame count and retained transcript bytes, continuously drains bounded standard error, and waits for every direct child it starts. Captured standard error remains an explicit untrusted field and is not automatically copied into formatted errors.

The deterministic reference agent emits `agent.ready` followed by `session.complete` and makes no tool calls. Package tests use it as a reproducible protocol and cleanup baseline. There is not yet a supported public `cailab agent run` workflow.

## Tool manifest

Each explicitly registered tool declares:

- a stable name and semantic version;
- direct subprocess argv, never implicit shell text;
- a closed JSON Schema Draft 2020-12 object input (`additionalProperties: false`);
- tenant, action, and resource permissions;
- low, medium, high, or critical risk;
- a timeout from 100 ms through 300 seconds;
- expected network authority: `none`, `loopback`, or `host`;
- expected filesystem authority: `none`, `read_only`, `workspace_write`, or `host`;
- sensitive successful-output fields as non-root RFC 6901 JSON Pointers.

A valid manifest is inert data. Registration and later execution require explicit user action; scenario files cannot cause these command vectors to run.

## Decision semantics

| Effect | Meaning |
|---|---|
| `allow` | The gateway may execute the call exactly within the evaluated contract. |
| `deny` | The call must not execute. |
| `redact` | The allowed flow replaces all declared JSON Pointer values before the protected boundary. |
| `require_approval` | The call must not execute until a separately correlated approval resolves positively and policy is re-evaluated as required. |

Every decision carries a stable reason code and policy version. `redact` requires pointers; `require_approval` requires an approval ID; `allow` and `deny` cannot carry either. Denied and approval-pending events must record `not_executed`.

Governance policies default only to deny and match exact agent, tool, action, resource, tenant, and classification values. A manifest permission is a mandatory ceiling: policy cannot add undeclared authority. Multiple matching rules are independent of document order and use fixed precedence: `deny`, then `require_approval`, then `redact`, then `allow`. Redaction pointers are merged and sorted; a missing pointer becomes a stable deny.

Policy redaction pointers apply to input arguments before execution. Input instances must satisfy the manifest's closed Draft 2020-12 schema; `$ref` and `$dynamicRef` are fragment-local and schema compilation has no external loader. Only allow and redact launch a tool. Deny and approval-required remain `not_executed`.

## Tool execution

The one-shot subprocess receives exactly one `ToolExecutionRequest` JSON line and returns exactly one correlated `ToolExecutionResponse` line. The executor requires an absolute command and working directory, a complete explicit environment, the manifest timeout, bounded output and diagnostics, and no shell. A response is either `succeeded` with JSON content or `failed` with a stable error code; the correlated agent-facing `tool.result` preserves that failure code.

Successful content is canonicalized and applies every manifest `sensitiveFields` pointer before it can be returned or hashed. Missing output pointers fail the call closed. Tool subprocess ownership is lifecycle management, not isolation.

## Reproducibility and evidence

An agent run records the exact scenario digest and seed, agent/provider/model version, policy digest, prompt hash, tool digests, trial index/count, status, and UTC timestamps. A decision event adds a monotonic sequence, correlation ID, actor and tenant, tool, action, resource classification, decision, outcome, and canonical input/output hashes.

Sequence is authoritative for decision order. Wall-clock time is diagnostic context and does not determine policy or score. The gateway hashes canonical original arguments and does not persist them. It commits the immutable decision with `not_executed` before an allowed/redacted launch, then appends a separate `ToolOutcomeEvent` linked to that decision and stored record hash. Successful outcomes hash the exact protected content returned to the agent; failed outcomes carry a stable error code. Reads validate schema and hashes. This detects accidental inconsistency but is not protection from a local user who can rewrite the database. Full transcript persistence and replay remain planned.

## Security boundary

Protocol validation and owned process cleanup are not sandboxing. A declaration of `none`, `loopback`, or `read_only` becomes an isolation claim only when a runtime demonstrably enforces it. The current controller limits inherited environment and protocol resources, but the subprocess can still access the host filesystem, network, OS APIs, and independently detached descendants with the launching user's authority. It must be treated as unisolated.
