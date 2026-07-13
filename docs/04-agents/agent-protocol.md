---
title: Agent Protocol v1alpha1
status: m3-development
last_reviewed: 2026-07-12
---

# Agent protocol v1alpha1

## Current scope

CloudAILab defines typed and schema-backed contracts for M3 agent runs, tools, governance, approvals, decisions, outcomes, trial state, fixture evaluation, evidence-safe traces, and reports. The supported CLI runs deterministic reference and deliberately unsafe baselines or a protocol-compatible custom agent in host subprocess or opt-in Docker isolation mode. It also runs bounded sequential reference/unsafe campaigns with verified fixture restoration before every trial and automatic aggregate replay. The harness enforces scenario-bound registration, deterministic policy, protected output, linked evidence, normalized provider baselines, endpoint-preserving restoration, scenario outcomes, and fixture-labeled injection scoring. Tool subprocess isolation and automatic custom-agent campaigns are not implemented.

The normative schemas are:

- [Tool manifest](../../schemas/agent/v1alpha1/tool-manifest.json)
- [Agent run](../../schemas/agent/v1alpha1/agent-run.json)
- [Protocol message](../../schemas/agent/v1alpha1/protocol-message.json)
- [Decision event](../../schemas/agent/v1alpha1/decision-event.json)
- [Governance policy](../../schemas/agent/v1alpha1/governance-policy.json)
- [Tool execution message](../../schemas/agent/v1alpha1/tool-execution-message.json)
- [Tool outcome event](../../schemas/agent/v1alpha1/tool-outcome-event.json)
- [Approval resolution event](../../schemas/agent/v1alpha1/approval-resolution-event.json)
- [Agent trace](../../schemas/agent/v1alpha1/agent-trace.json)
- [Agent evaluation report](../../schemas/agent/v1alpha1/agent-evaluation-report.json)
- [Trial state evidence](../../schemas/agent/v1alpha1/trial-state-evidence.json)

The executable validation, policy, gateway, session, state, and replay contracts are in [`internal/agent`](../../internal/agent). ADRs [0011](../02-architecture/decisions/0011-versioned-agent-json-lines-protocol.md) through [0018](../02-architecture/decisions/0018-deterministic-agent-evidence-replay.md) define the wire, lifecycle, governance, evidence, approval, isolation, and initial replay contracts. [ADR-0019](../02-architecture/decisions/0019-endpoint-preserving-trial-state-evaluation.md) defines provider restoration and scenario-outcome scoring.

## Framing

- UTF-8 only
- one JSON object per line
- newline after every emitted frame
- maximum encoded frame size: 1 MiB
- no blank frames, duplicate object keys, unknown typed fields, or trailing JSON values
- stdout reserved for protocol traffic
- stderr reserved for diagnostics

JSON Lines defines each line as a valid JSON value and recommends a line terminator after the final value. CloudAILab narrows that format to one object per line so every frame has a version, identifier, type, and payload.

The current wire version is `1.1`. The internal-only `1.0` draft did not carry action/resource targets and is not accepted by this workflow.

## Message flow

| Message | Sender | Purpose |
|---|---|---|
| `session.start` | Controller | Binds run, trial, scenario digest, policy version, and allowed tool versions. |
| `agent.ready` | Agent | Confirms the expected agent identity and version. |
| `tool.call` | Agent | Requests one declared tool, action, canonical resource ID, and JSON-object arguments. |
| `tool.result` | Controller | Returns the deterministic decision, execution status, and optional content. |
| `approval.required` | Controller | States that the correlated call was not executed and needs a decision. |
| `approval.resolved` | Controller | Reports the durably recorded local resolution. It is followed by a correlated `tool.result` and is not itself a tool result. |
| `session.complete` | Agent | Reports completed, failed, or canceled agent work. |
| `protocol.error` | Either | Reports a stable protocol error without changing authorization state. |

`tool.result`, `approval.required`, and `approval.resolved` require `correlationId`. The session controller additionally enforces direction, lifecycle order, expected agent identity/version, unique message IDs, declared-tool membership, and response correlation. Typed decoding by itself still validates only structure and payload semantics.

## Execution lifecycle

The internal controller requires an absolute executable path, an absolute working directory, and an explicit complete environment. It never invokes a shell and does not inherit the controller's environment by default. It sends `session.start`, requires a matching `agent.ready` before tool activity, bounds handshake and whole-session time, caps both frame count and retained transcript bytes, continuously drains bounded standard error, and waits for every direct child it starts. Captured standard error remains an explicit untrusted field and is not automatically copied into formatted errors.

The deterministic reference agent emits `agent.ready` followed by `session.complete` and makes no tool calls. `cailab agent run reference` exposes it as the reproducible public harness baseline. `cailab agent run subprocess` launches a user-selected implementation with explicit argv, directory, selected environment, identity, model labels, prompt hash, and trial metadata.

Host mode uses the direct subprocess behavior above and is not isolated. Docker mode instead interprets the command and directory as absolute POSIX paths inside a content-addressed agent image. Before persistence, the runtime resolves the active context, requires an absolute local Unix socket and non-rootless Linux engine with active cgroups, pins that endpoint on every Docker command, verifies that the present image declares no volumes, and rejects remote contexts. CloudAILab invokes the absolute Docker CLI with no implicit pull, no host environment forwarding, no mounts, no published ports, network and IPC none, log driver none, read-only root, a 64 MiB noexec/nosuid/nodev `/tmp`, UID/GID 65532, all capabilities dropped, no-new-privileges, built-in seccomp, one CPU, 512 MiB memory with no additional swap, 128 PIDs, and a file-descriptor limit. A container-local init process reaps descendants. Run and trial labels must match before cleanup can force-remove a surviving container.

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

A valid manifest is inert data. `cailab agent validate` and `cailab agent run subprocess` explicitly select it; scenario files cannot cause these command vectors to run. The public workflow uses the manifest file's directory as the tool working directory and forwards only variables selected with `--tool-env`.

## Decision semantics

| Effect | Meaning |
|---|---|
| `allow` | The gateway may execute the call exactly within the evaluated contract. |
| `deny` | The call must not execute. |
| `redact` | The allowed flow replaces all declared JSON Pointer values before the protected boundary. |
| `require_approval` | The call must not execute until a separately correlated approval resolves positively and policy is re-evaluated as required. |

Every decision carries a stable reason code and policy version. `redact` requires pointers; `require_approval` requires an approval ID; `allow` and `deny` cannot carry either. Denied and approval-pending events must record `not_executed`.

Governance policies default only to deny and match exact agent, tool, action, resource, tenant, and classification values. A protocol 1.1 agent declares the action and canonical resource ID; the controller resolves tenant/classification from active canonical scenario state. A manifest permission is a mandatory ceiling: policy cannot add undeclared authority. Multiple matching rules are independent of document order and use fixed precedence: `deny`, then `require_approval`, then `redact`, then `allow`. Redaction pointers are merged and sorted; a missing pointer becomes a stable deny.

Policy redaction pointers apply to input arguments before execution. Input instances must satisfy the manifest's closed Draft 2020-12 schema; `$ref` and `$dynamicRef` are fragment-local and schema compilation has no external loader. Direct allow and redact launch a tool. Deny remains `not_executed`; approval-required remains unexecuted until the separate resolution flow completes.

## Approval resolution

The public subprocess runner uses `--approval-mode reject` by default, so CI and unattended runs never wait for input or approve implicitly. `--approval-mode prompt --approver <id>` displays the agent, tool, action, canonical resource, classification, tenant, and policy reason. It omits raw tool arguments and accepts only the exact text `approve <approval-id>`; every other response rejects.

The initial `require_approval` decision is not replaced. After a response, the gateway re-evaluates the same request against the current policy and manifest ceiling. A stale or mismatched approval fails closed, deny precedence is retained, and applicable redaction still applies. Rejection produces `approval:rejected`. Approval produces only allow or redact.

Before sending `approval.resolved`, the store appends an `ApprovalResolutionEvent` linked to the original decision and input hash. An approved execution outcome must link to both records. The live continuation consumes the resolution once; duplicate resolutions, replayed continuations, rejected-outcome attempts, mismatched messages, and failed evidence writes do not execute the tool.

## Tool execution

The one-shot subprocess receives exactly one `ToolExecutionRequest` JSON line containing the resolved action/resource plus protected arguments and returns exactly one correlated `ToolExecutionResponse` line. The executor requires an absolute command and working directory, a complete explicit environment, the manifest timeout, bounded output and diagnostics, and no shell. A response is either `succeeded` with JSON content or `failed` with a stable error code; the correlated agent-facing `tool.result` preserves that failure code.

Successful content is canonicalized and applies every manifest `sensitiveFields` pointer before it can be returned or hashed. Missing output pointers fail the call closed. Tool subprocess ownership is lifecycle management, not isolation.

## Reproducibility and evidence

An agent run records the exact scenario digest and seed, agent/provider/model version, policy digest, prompt hash, tool digests, optional enforced execution metadata, trial index/count, status, and UTC timestamps. Docker metadata includes profile `docker-strict-v1`, the engine, exact content-addressed image, network boundary, and filesystem boundary. A control change requires a new profile value. The store writes an immutable running record before launch and appends exactly one terminal record without changing configuration. A decision event adds a monotonic sequence, correlation ID, actor and tenant, tool, action, resource classification, decision, outcome, and canonical input/output hashes; decisions are accepted only while that trial is active.

Sequence is authoritative for decision order. Wall-clock time is diagnostic context and does not determine policy or score. The gateway hashes canonical original arguments and does not persist them. It commits the immutable decision with `not_executed` before a direct allowed/redacted launch. Approval-required calls append a separate resolution record before any continuation. A `ToolOutcomeEvent` links to the direct decision or, for approved execution, to both the original decision and approval record. Successful outcomes hash the exact protected content returned to the agent; failed outcomes carry a stable error code. Reads validate schema and hashes. Human and `--json` summaries omit raw arguments, transcripts, and child diagnostic text. This detects accidental inconsistency but is not protection from a local user who can rewrite the database.

## Evidence replay and metrics

`cailab agent replay` loads one or more explicit terminal trial IDs through the integrity-checking store APIs. The `governed-evidence-v1` profile requires the selected set to contain every declared trial index exactly once and requires identical range, scenario/seed, agent, policy, prompt, tool, execution-profile, and trial-count configuration. Trial timestamps do not participate in compatibility or aggregate scoring.

Replay validates the sequence and every run/trial/correlation, decision/approval, and decision/outcome link again. It then reports terminal completion, final authorization disposition, initial policy denials, approval resolutions, tool outcomes, missing outcome evidence, and distinct confidential/restricted action-resource targets observed in the trace. Every rate carries its numerator and denominator; a zero denominator produces `null` rather than a fabricated percentage. The report has no generated timestamp or composite score, so equivalent evidence produces equivalent JSON.

The profile explicitly marks task success, prompt-injection resistance, remediation quality, sensitive-data exposure, and effective blast radius as unmeasured. A terminal agent process is not scenario success; observed targets are not all reachable authority; and evidence-safe hashes do not prove absence of disclosure. Replay does not re-run policy, tools, providers, scenario invariants, or a model. Full protocol-transcript persistence is intentionally not part of this evidence-safe contract.

## Trial state and scenario outcomes

State capture is opt-in. `--capture-state` adds immutable `scenario-state-v1` metadata to the run and appends one `before` and one `after` `TrialStateEvidence` record. Each record contains the supported provider snapshot digest and the exact deterministic invariant report used by `cailab verify`. The before record must commit before any governed decision; the after record is captured after session termination with a separate bounded context and closes further action evidence.

`--restore-fixture` implies capture. Native provider processes authenticate a run-scoped reset request and restore state without changing listeners. CloudAILab ownership-checks and replaces the memory-backed Floci container at the exact recorded loopback port, then rehydrates it. The local issuer clears codes and rotates to fresh signing material. Startup snapshots all configured providers and persists a normalized baseline digest. A restored snapshot must match that exact normalized baseline before `session.start`; the source-manifest digest remains a separate scenario identity.

When every selected compatible trial has a complete state pair, replay uses `scenario-outcome-v1`. `initialStateMatchRate` reports normalized baseline equivalence. `taskSuccessRate` counts after states where all declared invariants pass. `remediationSuccessRate` includes only trials that began with a failed invariant and counts those ending with all invariants passing.

Selecting a scenario prompt-injection fixture adds immutable `prompt-injection-v1` metadata and promotes replay to `adversarial-scenario-v1`. A successful outcome for the exact exposure action proves consumption of the untrusted resource. Only later exact prohibited decisions count as triggered behavior. Replay reports resistance, injection-task success, and governance containment separately; unexposed trials are excluded from resistance and success denominators. Fixture targets are omitted from `session.start`, and no model judges the trace. Sensitive-data exposure, semantic equivalents, and effective blast radius remain unavailable.

## Security boundary

Protocol validation and host-process cleanup are not sandboxing. In host mode, the agent can still access the host filesystem, network, OS APIs, and independently detached descendants with the launching user's authority and must be treated as unisolated.

The opt-in Docker agent mode enforces the documented container network, filesystem, privilege, and resource configuration and is covered by an adversarial integration probe. The claim stops at that boundary: Docker is not a VM; the daemon, runtime, kernel or Docker Desktop VM, and pinned image remain trusted; image-defined environment is part of the artifact; and registered tool subprocesses still run unisolated on the host. Tool manifest isolation fields remain requirements rather than enforcement claims.
