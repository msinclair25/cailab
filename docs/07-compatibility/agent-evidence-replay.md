---
title: Agent Evidence Replay Compatibility
status: implemented
profile: governed-evidence-v1
last_reviewed: 2026-07-12
---

# Agent evidence replay compatibility

## Supported contract

`cailab agent replay` deterministically projects one complete, explicitly selected trial set from persisted CloudAILab evidence. The supported input is the current `cloudailab.dev/agent/v1alpha1` run, decision, approval, and outcome records read through SQLite integrity verification.

Replay supports:

- terminal `completed`, `failed`, and `canceled` trials;
- one trial or a declared repeated set with contiguous one-based indices;
- exact configuration compatibility across the range run, scenario/seed, agent, policy, prompt, tools, execution profile, and declared count;
- evidence linkage validation across decisions, approvals, and outcomes;
- deterministic text, JSON, and Markdown output;
- trace/configuration digests, primitive counts, numerator/denominator/rate values, and explicit unavailable metrics;
- replay of a stopped range when `--run-id` identifies it.
- promotion to `scenario-outcome-v1` when every compatible trace contains a valid before/after state pair.
- promotion to `adversarial-scenario-v1` when compatible restored traces bind the same prompt-injection fixture.

## Measured by `governed-evidence-v1`

| Metric | Evidence basis |
|---|---|
| Completed trials | Terminal `AgentRun.status` equals `completed`. |
| Final authorization rate | Direct allow/redact or approved approval resolution divided by all governed decisions. |
| Policy-denied actions | Initial immutable decision effect is `deny`. |
| Approval-rejected actions | An approval-required decision has a recorded negative resolution. |
| Approval resolution rate | Recorded positive or negative resolutions divided by approval-required decisions. |
| Tool execution success rate | Successful linked tool outcomes divided by all linked tool outcomes. |
| Missing outcome evidence | A final authorized action has no linked outcome record. |
| Observed protected targets | Distinct confidential/restricted action-resource pairs attempted in selected traces. |

`scenario-outcome-v1` additionally measures initial baseline matches, task success from after-state invariants, and remediation success for trials that begin with at least one failed invariant. See [Agent trial state compatibility](agent-trial-state.md).

`adversarial-scenario-v1` additionally requires one compatible restored prompt-injection fixture and measures exposure, resistance, injection-task success, and governance containment with separate denominators. See [Agent prompt-injection evaluation compatibility](agent-prompt-injection.md).

## Explicitly unsupported claims

- Terminal completion is not task or mission success; only `scenario-outcome-v1` makes that claim from captured invariants.
- Observed protected targets are not effective or reachable blast radius.
- Hashes and redaction do not prove that sensitive data was never exposed.
- `governed-evidence-v1` does not label prompt-injection fixtures; only `adversarial-scenario-v1` reports the exact fixture-scoped construct.
- Replay does not re-evaluate policy, re-execute a tool, restore provider state, or run scenario invariants.
- Compatible metadata alone does not prove equivalent mutable provider state; restored state-captured trials additionally prove their normalized baseline digest.
- The current CLI does not automatically run or reset a repeated trial set.
- Replay hashes are not signatures or tamper-proof provenance.

## Failure behavior

Replay fails without a report when a selected trial is non-terminal; an index or trial ID is duplicated; the declared set is incomplete; configuration differs; sequences are not contiguous; records refer to another run/trial/correlation; approvals do not match an approval-required decision; outcomes lack an executable decision or approved resolution; or any underlying store integrity check fails.

## Tests

- Domain replay contract: `internal/agent/replay_test.go`
- Persisted repeated-set integration: `internal/app/agent_run_test.go`
- CLI and rendering behavior: `internal/cli/cli_test.go`
- Existing record-chain integrity: `internal/state/agent_events_test.go`, `internal/state/approval_resolutions_test.go`, and `internal/state/tool_outcomes_test.go`
