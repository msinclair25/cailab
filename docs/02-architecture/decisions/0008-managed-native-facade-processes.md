---
title: "ADR-0008: Managed Native Facade Processes"
status: accepted
date: 2026-07-12
---

# ADR-0008: Managed native facade processes

## Context

Microsoft and Google training surfaces must remain reachable after `cailab up` exits so learners, SDKs, and external agents can interact with them. A server embedded only in the invoking CLI process would disappear immediately. A global HTTPS interception proxy would add certificate trust, proxy configuration, and cleanup requirements to the default path. A provider-specific container would add another downloaded executable artifact for logic already owned by CloudAILab.

The runtime also needs stronger ownership evidence than a persisted PID. Operating systems can reuse PIDs, so killing a process by PID after a crash could affect an unrelated process.

## Decision

CloudAILab starts native provider facades as private child commands of the same compiled `cailab` binary. The process is detached from the launching session using OS-specific process settings and binds to a random IPv4 loopback port.

Each native runtime receives a run-specific directory under the configured CloudAILab state directory. The directory is owner-only and contains:

- an owner-only control document with the run ID, random control token, initial provider state, and file paths;
- an owner-only readiness document containing the run ID, endpoint, and diagnostic PID;
- an owner-only mutable facade-state document;
- an owner-only runtime log.

Readiness requires both a matching run ID and a successful health response. Normal shutdown uses an HTTP control endpoint that requires the random control token and matching run ID. A PID is diagnostic data, not sufficient cleanup authority. Startup may kill the exact process it just created if readiness fails, but later cleanup does not kill an unverified PID.

The Microsoft API bearer used by the first M2 scenario is deliberately separate from the control token. It is synthetic training access, not a caller identity. Local OIDC will replace that limitation for supported federation flows later in M2.

## Consequences

### Positive

- One compiled artifact owns the control plane and native facade implementation.
- External tools can use a persistent endpoint across independent CLI commands.
- Default operation needs no global proxy, trusted certificate, Microsoft tenant, or additional runtime download.
- Run ownership and cleanup are stronger than PID-only lifecycle management.
- The same lifecycle contract can support the future Google facade and local issuer.

### Negative

- Cross-platform detached-process behavior requires dedicated integration tests.
- The facade runs with the same OS account authority as the CLI and is not an agent sandbox.
- Abrupt host termination can leave stale owner-only runtime files.
- HTTP is used for the loopback-only synthetic surface until a tested local TLS requirement exists.

## Validation

- Unit tests cover authorization, Graph-shaped errors, pagination, mutation persistence, and live normalization.
- A native integration test starts the detached process, snapshots its state, deletes a grant, verifies path changes, performs authenticated shutdown, and checks directory cleanup.
- The native integration test runs on Linux, macOS, and Windows in CI.
- A compiled-binary manual test covers `up`, separate `status`/`graph`/`verify` commands, remediation, reset, and down.

## Sources

- [Customize the Microsoft Graph SDK client](https://learn.microsoft.com/en-us/graph/sdks/customize-client)
- [Dev Proxy overview](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/overview)
- [Dev Proxy certificate troubleshooting](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/troubleshooting)
