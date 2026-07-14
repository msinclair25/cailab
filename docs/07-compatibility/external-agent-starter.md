---
title: External Agent Starter Compatibility
status: active
last_reviewed: 2026-07-13
---

# External agent starter compatibility

## Supported contract

The repository and release packager provide `cailab-agent-starter`, a dependency-free Go reference implementation of the external protocol boundary. It is tested against the `acquisition-agent` scenario through the public CLI workflow.

| Surface | Supported behavior |
|---|---|
| Agent protocol | Protocol `1.1`; one `session.start`, one `agent.ready`, one fixed `tool.call`, one correlated `tool.result`, and one `session.complete` |
| Tool protocol | One UTF-8, duplicate-key-free, strict JSON request and one correlated JSON response |
| Provider operation | `GET /drive/v3/files/drive_file_agent_runbook?alt=media` on the exact active IPv4 loopback Google origin |
| Registration | Generated closed tool schema, fixed scenario tenant/action/resource/classification permission, and default-deny exact-match policy |
| Secret handling | Google endpoint and synthetic token selected explicitly through `--tool-env`; values absent from manifest and evidence output |
| Protected output | `/content` declared sensitive and redacted before agent return and outcome hashing |
| Evidence | One completion, authorization, execution outcome, and optional restored before/after state pair |
| Distribution | Native starter executable and adaptable source, policy, prompt, guide, and expected-evidence document in every declared release archive |

## Tested boundary

- Unit tests cover configuration refusal on overwrite, fixed registration content, UTF-8 and duplicate-key-free protocol framing, correlation, the exact provider route, loopback-only endpoint validation, and exact tenant/action/resource/classification mismatch refusal.
- The Linux flagship public CLI integration builds and configures the starter, validates registrations, performs the real facade-backed tool read, replays evidence, and checks the expected completion, authorization, execution, and task-result numerators. The packaged workflow is also acceptance-rehearsed on macOS.
- Release tests and Linux/macOS/Windows smoke jobs require the starter binary and supporting files and exercise help plus configuration generation.

## Limitations

- The starter is deterministic and framework-neutral; it is not an LLM, model SDK, MCP server, or general agent benchmark.
- Its fixed read does not remediate the flagship scenario, so deterministic task success is zero.
- Host mode and the registered tool subprocess are unisolated. The manifest's `loopback` and `none` fields declare needed authority; they do not enforce it.
- Docker mode can isolate a self-contained agent but cannot run this provider-reading tool inside that boundary. Tools remain on the host.
- The fixed local Google token is synthetic and does not model Google OAuth or user identity.
- The content field is intentionally redacted, so this starter validates governed execution and evidence handling rather than prompt-injection exposure scoring.
- Windows release smoke covers executable help and configuration generation, not the complete provider-backed host-subprocess lifecycle.
