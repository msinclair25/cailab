---
title: "ADR-0003: Native Provider Facades by Default"
status: accepted
date: 2026-07-11
---

# ADR-0003: Native provider facades by default

## Context

CloudAILab needs mutable Microsoft- and Google-shaped state that participates in a canonical cross-provider graph and deterministic authorization model. Transparent HTTPS interception can improve compatibility for clients that cannot override service endpoints, but it introduces certificate trust, proxy configuration, runtime-specific behavior, and cleanup risk.

Microsoft Dev Proxy supports static mocks, dynamic CRUD APIs, failures, throttling, and Graph guidance, but these capabilities do not constitute Entra directory or authorization semantics. Google publishes Discovery contracts that can guide selected local routes.

## Decision

CloudAILab will implement native, deliberately scoped Microsoft and Google provider facades backed by canonical state. Documented endpoint or client customization is the default integration path.

Microsoft Dev Proxy may be added later as an optional compatibility and chaos-testing mode. It will not be the authoritative identity or policy store. Global certificate or proxy changes will require explicit consent and reversible cleanup.

## Consequences

### Positive

- Default deployment avoids global certificate and proxy changes.
- Canonical state and authorization remain under CloudAILab control.
- Supported behavior is testable without another runtime dependency.
- Provider operations can be added strictly for scenario needs.

### Negative

- Clients with hard-coded production endpoints need configuration or later proxy mode.
- CloudAILab must implement routing, pagination, errors, and selected side effects.
- Compatibility remains limited and must be documented operation by operation.

## Validation

- The flagship scenario runs without host-wide proxy or certificate changes.
- Documented SDK or HTTP examples can target both facades.
- Each supported operation has a compatibility record and contract tests.
- Optional proxy mode, if added, passes installation, trust, and cleanup tests on each supported host platform.

## Sources

- [Microsoft Dev Proxy overview](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/overview)
- [Microsoft Dev Proxy CRUD simulation](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/simulate-crud-api)
- [Microsoft Dev Proxy certificate troubleshooting](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/troubleshooting)
- [Google Admin SDK Directory API](https://developers.google.com/workspace/admin/directory/reference/rest)
