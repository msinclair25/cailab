---
title: "ADR-0015: Scenario-Bound Public Agent Runs"
status: accepted
date: 2026-07-12
---

# ADR-0015: Scenario-bound public agent runs

## Context

The internal M3 harness could supervise an agent and tool subprocess, authorize calls, and persist decision/outcome evidence, but users had no supported command that composed those pieces. The original `tool.call` also omitted the intended action and resource, so test resolvers supplied authorization targets out of band. Exposing that design would make registration ambiguous and could let provider metadata become caller-controlled.

Public execution additionally needs an exact run record, bounded configuration files, explicit credential forwarding, useful output that does not repeat raw tool arguments, and an honest unisolated-mode warning.

## Decision

1. Protocol `1.1` requires every `tool.call` to declare its tool, action, canonical resource identifier, and arguments. This is a breaking change from the internal-only `1.0` draft and is versioned explicitly.
2. The controller resolves the resource tenant and classification from the active canonical scenario. The agent cannot supply those policy attributes. The resolved action and resource are included in the one-shot tool request.
3. `cailab agent validate` loads bounded strict JSON policy and tool files, verifies tool launch configuration and permission ceilings, and binds relevant rules and resources to the active scenario. Validation never launches a process.
4. `cailab agent run reference` uses a built-in default-deny policy, inert registered tool, and deterministic no-tool-call agent as the public harness baseline.
5. `cailab agent run subprocess` requires explicit agent identity/version/provider/model, command argv, working directory, actor tenant, prompt file, policy, and tools. Prompt content is hashed into metadata but is not delivered to the agent by CloudAILab.
6. Tool working directories resolve to their manifest directories. Agent and tool environments contain only variables selected by repeated CLI flags; selected values are neither persisted nor printed.
7. Each trial appends a canonical running record before launch and exactly one canonical terminal record after success, failure, or cancellation. Immutable configuration cannot change between those records. Decision and tool-outcome events are accepted only while the matching trial is active.
8. Human and JSON summaries include run metadata plus protected decision/outcome evidence. They omit raw protocol transcripts and raw child diagnostics.
9. Direct subprocess ownership remains unisolated. Public commands warn that filesystem, network, syscalls, and independently detached descendants retain the launching user's authority.

Approval resolution, transcript replay, repeated trials, scoring, and enforced isolation remain separate work.

## Consequences

### Positive

- A clean built binary can execute the deterministic baseline through a supported CLI command.
- Protocol-compatible user agents and tools can exercise the same deterministic gateway tested internally.
- Authorization attributes are tied to active canonical state rather than caller-provided tenant/classification strings.
- Start, terminal, decision, and tool-outcome evidence remain distinguishable and integrity-checked.
- Explicit environment selection avoids accidental wholesale credential inheritance.

### Negative

- Protocol `1.0` agents must add action and resource fields and emit `protocolVersion: "1.1"`.
- The CLI surface is intentionally verbose because command boundaries, identities, prompt provenance, and credential selection are explicit.
- A prompt file contributes provenance only; CloudAILab does not define how an external agent consumes its prompt.
- Tool registrations are selected per invocation rather than installed in a long-lived registry.
- Unisolated subprocesses can still bypass the governed tool boundary using ambient host authority.

## Validation

- A packaged-binary smoke test runs the public deterministic reference baseline.
- CLI integration launches a real protocol agent and a real registered tool subprocess, persists allow/outcome evidence, and proves raw arguments are absent from JSON output.
- App tests cover canonical target resolution, successful execution, invalid scenario resources before persistence, and terminal failure persistence.
- State tests cover start/terminal persistence, duplicate start/completion, immutable-field rejection, reopen, record mutation, and decision rejection after completion.
- Existing malformed-frame, timeout, cancellation, redaction, explicit-environment, and race tests continue to cover the underlying subprocess boundaries.

## Sources

- [JSON Lines](https://jsonlines.org/)
- [Go `os/exec`](https://pkg.go.dev/os/exec)
- [SQLite transactional guarantees](https://www.sqlite.org/transactional.html)
- [ADR-0011: Versioned agent JSON Lines protocol](0011-versioned-agent-json-lines-protocol.md)
- [ADR-0014: Strict one-shot tool execution](0014-strict-one-shot-tool-execution.md)
