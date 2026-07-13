---
title: Agent Campaign Execution Compatibility
status: implemented
last_reviewed: 2026-07-13
---

# Agent campaign execution compatibility

## Supported contract

CloudAILab automatically executes bounded repeated sets for its code-owned deterministic inert reference, fixture-specific safe control, and deliberately unsafe fixtures:

```bash
cailab agent campaign reference --trials 3 --format markdown
cailab agent campaign safe --trials 3 --fixture drive-runbook-export --format json
cailab agent campaign unsafe --trials 3 --fixture drive-runbook-export --format json
```

The supported count is 2 through 100. A campaign derives one-based immutable trial IDs from `--trial-prefix`, rejects invalid or existing IDs before the first launch, restores the active fixture at the recorded endpoints before every trial, captures before/after scenario evidence, and automatically replays the complete set. Reports use the existing deterministic text, JSON, or Markdown `AgentEvaluationReport` contract.

Trials run sequentially. A terminal agent failure remains in the declared denominator when complete state evidence exists. Restoration, persistence, cancellation, incomplete state evidence, or replay failure stops the campaign and no aggregate report is emitted from the partial set.

## Explicit limitations

- Automatic campaigns support only `reference`, `safe`, and `unsafe`; custom subprocess agents still use explicit `agent run subprocess` trial index/count flags followed by `agent replay`.
- Campaigns do not run trials concurrently.
- A partial campaign cannot resume or reuse its derived IDs. Recover the range if necessary and choose a new prefix.
- Restoration is not atomic across providers, and host-mode process limitations still apply.
- Campaign success means a complete evaluation report was produced. Individual trial status and task, remediation, injection, and governance metrics remain separate facts in that report.
- The 100-trial limit bounds one CLI invocation; it is not a statistical adequacy claim.

## Tests

- Orchestration, restoration, failure, bounds, and preflight: `internal/app/agent_campaign_test.go`
- Public CLI and report output: `internal/cli/cli_test.go`
- Replay compatibility and deterministic aggregation: `internal/agent/replay_test.go`, `internal/app/agent_run_test.go`
- Provider restoration and lifecycle cleanup: provider unit and integration suites listed in [Agent trial state compatibility](agent-trial-state.md)

## Architecture

- [ADR-0021: Sequential restored agent campaigns](../02-architecture/decisions/0021-sequential-restored-agent-campaigns.md)
