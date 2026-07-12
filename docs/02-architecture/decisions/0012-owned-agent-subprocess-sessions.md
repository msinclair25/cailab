---
title: "ADR-0012: Owned Agent Subprocess Sessions"
status: accepted
date: 2026-07-12
---

# ADR-0012: Owned agent subprocess sessions

## Context

ADR-0011 defines an untrusted JSON Lines boundary but deliberately leaves process execution to a later slice. Launching an agent introduces host authority, credential inheritance, unbounded diagnostics, hung pipes, cancellation, and process-cleanup risks before a network or filesystem sandbox exists. A lifecycle controller must fail closed without implying that ordinary operating-system process separation contains arbitrary code.

## Decision

CloudAILab will own one direct agent subprocess for the duration of each session:

1. The executable and working directory must be absolute paths. Arguments remain an explicit vector and are never interpreted by a shell.
2. The child receives a complete, explicit environment. It does not inherit the controller environment by default.
3. Standard input and output carry only the accepted protocol. Retained standard error defaults to 64 KiB, remains fully drained, and is capped at 1 MiB by configuration.
4. The controller derives `session.start` from the validated run record, requires the expected `agent.ready` identity first, and then accepts only agent-originated `tool.call`, `protocol.error`, or `session.complete` messages.
5. Message identifiers are unique within a session. Tool calls must name a tool declared by the run. Controller responses must be a correlated `tool.result` or `approval.required` message.
6. Handshake, whole-session, cleanup, frame-size, message-count, and retained-transcript limits are mandatory and bounded. The transcript defaults to 8 MiB and cannot exceed 64 MiB.
7. Cancellation terminates the direct child, closes its standard input, and waits exactly once. Go's `Cmd.WaitDelay` bounds waits caused by a child that does not exit or leaves communication pipes open.
8. A deterministic reference agent acknowledges a valid session and completes without tool calls. It is the harness baseline, not an agent-performance example.
9. This subprocess mode is unisolated. It does not restrict filesystem access, network access, operating-system calls, or descendant processes and must not be presented as safe execution of arbitrary code.

The controller API is initially internal. A supported user-facing agent command waits for the governed tool gateway, persistent decision evidence, and explicit execution-mode UX.

## Consequences

### Positive

- Host credentials are not accidentally inherited through the environment.
- Lifecycle order and message direction are enforced before gateway side effects exist.
- Malformed, silent, noisy, non-terminating, and wrong-identity agents fail with typed, bounded diagnostics. Raw standard error is available as an explicit result field but is not interpolated into formatted errors.
- Every started direct child is canceled or allowed to finish and then waited for.
- The deterministic reference process gives later gateway and scoring work a reproducible baseline.

### Negative

- Absolute executable and directory requirements add resolution work at registration time.
- Some agents need an explicit environment that they previously assumed would be inherited.
- Direct-child termination does not guarantee termination of independently detached descendants.
- A trusted in-process tool handler that ignores cancellation may outlive the session goroutine; the controller returns on deadline, but handler implementations must still honor context cancellation.
- No supported end-user `agent run` command exists in this slice.

## Validation

- Integration-style package tests launch the test binary as a real subprocess and exercise the reference agent and a correlated tool round trip.
- Negative tests cover malformed output, wrong lifecycle order and direction, duplicate identifiers, undeclared tools, bad response correlation, invalid launch configuration, and unexpected exit.
- Lifecycle tests cover handshake expiry, whole-session expiry, parent cancellation, a handler that ignores cancellation, and an agent that reports completion but refuses to exit.
- Diagnostic tests prove that standard error is drained, retained only to its configured cap, and marked as truncated.
- Race tests exercise the controller's concurrent decoder, writer, cancellation, and diagnostic paths.

## Sources

- [Go `os/exec` package](https://pkg.go.dev/os/exec)
- [Go `Cmd.WaitDelay` and `CommandContext` source documentation](https://go.dev/src/os/exec/exec.go)
- [ADR-0011: Versioned agent JSON Lines protocol](0011-versioned-agent-json-lines-protocol.md)
