---
title: CloudAILab
aliases:
  - CAL
status: m3-development
---

# CloudAILab

> A local enterprise identity and AI-agent security range.

CloudAILab is a working project name for a reproducible learning and evaluation environment spanning AWS, Microsoft Entra/Microsoft 365, and Google Workspace-compatible ecosystems. It is intended to teach enterprise IAM and cybersecurity, and to evaluate how AI agents behave when they receive identities, credentials, tools, and access to sensitive data.

The command-line name is planned as `cailab`; `cal` is intentionally avoided because it conflicts with the traditional Unix calendar utility.

## Project status

M0, the M1 AWS identity slice, and the M2 cross-provider identity milestone are complete. M3 is in development: versioned agent, policy, tool-execution, approval, and outcome contracts; supported reference and custom subprocess runs; scenario-bound tool registration; exact-match policy; interactive or fail-closed approval resolution; Draft 2020-12 input validation; protected tool output; immutable run metadata; and append-only linked evidence are implemented and test-backed. Enforced isolation, full trace replay, and repeated-trial scoring remain planned. Provider and protocol compatibility is limited to the tested matrices and schemas.

## Build and try the walking skeleton

Prerequisites: Go 1.25.12 or newer. Docker is required only for AWS/Floci scenarios.

```bash
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor
./bin/cailab scenario list
./bin/cailab scenario show walking-skeleton
./bin/cailab up walking-skeleton
./bin/cailab mission
./bin/cailab graph path google:alex aws:acquisition-data
./bin/cailab verify
./bin/cailab down
```

The default state database is `.cloudailab/cailab.db`. Override it with `--state-dir` on lifecycle commands or set `CAILAB_HOME`.

## Try the AWS vertical slice

The initial `verify` is expected to fail because the scenario starts vulnerable. Follow the [AWS cross-account lab guide](docs/07-guides/aws-cross-account-lab.md) to prove the access with the AWS CLI, narrow the role trust, and make both invariants pass.

```bash
./bin/cailab up aws-cross-account
./bin/cailab status
./bin/cailab mission
./bin/cailab graph path aws:parent-root aws:acquisition-data
./bin/cailab verify
```

## Try the Microsoft identity slice

This scenario starts with an ordinary analyst holding an excessive delegated Microsoft Graph grant. Follow the [Microsoft consent lab guide](docs/07-guides/microsoft-consent-lab.md) to inspect the local Graph-shaped endpoint, revoke only that grant, and preserve the approved administrator path. It does not require Docker, a Microsoft tenant, a global proxy, or a trusted local certificate.

```bash
./bin/cailab up microsoft-consent
./bin/cailab status
./bin/cailab graph path microsoft:analyst microsoft:directory-data
./bin/cailab verify
```

## Try the Google identity slice

This scenario starts with a contractor holding a direct permission on a restricted Drive file while an approved administrator reaches it through a group. Follow the [Google Drive sharing lab guide](docs/07-guides/google-drive-sharing-lab.md) to inspect Directory and Drive state, remove only the contractor grant, and preserve the intended path. Docker and a Google account are not required.

```bash
./bin/cailab up google-drive-sharing
./bin/cailab status
./bin/cailab graph path principal:contractor resource:retention-plan
./bin/cailab verify
```

## Try the local identity issuer

Follow the [local OIDC lab guide](docs/07-guides/local-oidc-lab.md) to retrieve discovery and JWKS, exchange a one-time synthetic code, validate RS256 ID and access tokens locally, and rotate signing keys. It requires no Docker, cloud identity provider, proxy, or certificate installation. Loopback HTTP and synthetic subject selection make this a development profile, not a production OpenID Provider.

```bash
./bin/cailab up local-oidc
./bin/cailab status
./bin/cailab identity rotate
./bin/cailab verify
```

## Try the cross-provider flagship

The `acquisition-agent` scenario begins with both a contractor and an approved administrator able to reach acquisition data through Google → Microsoft → local OIDC → AWS representations. Follow the [flagship lab guide](docs/07-guides/acquisition-agent-lab.md) to issue signed synthetic tokens, exercise the federation gateway, revoke only the contractor group's Microsoft app-role assignment, and prove the administrator path survives.

```bash
./bin/cailab doctor acquisition-agent
./bin/cailab up acquisition-agent
./bin/cailab graph path google:contractor aws:acquisition-data
./bin/cailab verify
```

External tools and AI agents can call the documented loopback APIs and invoke the `cailab federation` command. The supported M3 runner can launch a protocol-compatible agent, validate scenario-bound registrations, govern resolved tool calls, pause selected calls for local approval, execute allowed or redacted one-shot tools, protect successful output, and persist linked run/decision/approval/outcome evidence.

## Run the deterministic agent baseline

With any active scenario, the reference command launches the deterministic protocol agent, records immutable run metadata, and prints an evidence-safe summary. It makes no tool calls, so it verifies the public subprocess and persistence path without pretending to measure agent quality.

```bash
./bin/cailab up walking-skeleton
./bin/cailab agent run reference
```

Protocol-compatible user agents can be launched with `cailab agent run subprocess`. Policies and tool manifests are selected explicitly for that trial and validated against the active scenario before a process starts. See the [agent-run guide](docs/07-guides/agent-run.md) for the complete workflow and security limitations.

Agent and tool subprocesses are owned and bounded but **not isolated** from the launching user's filesystem, network, syscalls, or detached descendants. Do not run untrusted code in this mode.

## Development checks

```bash
gofmt -w cmd internal
go mod tidy -diff
go mod verify
go vet ./...
go test ./...
go test -race ./...
go run ./internal/tools/doccheck .
```

## Principles

- Deterministic security decisions; optional AI explanations.
- One deep, end-to-end scenario before broad API coverage.
- Explicit compatibility claims instead of implied cloud parity.
- Local-first operation with no required cloud account or hosted model.
- Safe defaults, fake credentials, loopback binding, and complete cleanup.
- Documentation and tests evolve with the implementation.

## Documentation

- [Project charter](docs/00-project/charter.md)
- [Master plan](docs/00-project/master-plan.md)
- [Glossary](docs/00-project/glossary.md)
- [Product requirements](docs/01-product/requirements.md)
- [System architecture](docs/02-architecture/system-architecture.md)
- [Architecture decisions](docs/02-architecture/decisions/README.md)
- [Threat model](docs/03-security/threat-model.md)
- [Scenario specification](docs/04-scenarios/scenario-specification.md)
- [Engineering standards](docs/05-engineering/engineering-standards.md)
- [Quality strategy](docs/05-engineering/quality-strategy.md)
- [Delivery roadmap](docs/05-engineering/roadmap.md)
- [Technical basis and source register](docs/06-research/technical-basis.md)
- [AWS cross-account lab](docs/07-guides/aws-cross-account-lab.md)
- [AWS/Floci compatibility matrix](docs/07-compatibility/aws-floci-1.5.32.md)
- [Microsoft consent lab](docs/07-guides/microsoft-consent-lab.md)
- [Microsoft Graph facade compatibility matrix](docs/07-compatibility/microsoft-graph-facade.md)
- [Google Drive sharing lab](docs/07-guides/google-drive-sharing-lab.md)
- [Google Workspace facade compatibility matrix](docs/07-compatibility/google-workspace-facade.md)
- [Local development OIDC lab](docs/07-guides/local-oidc-lab.md)
- [Local development OIDC compatibility matrix](docs/07-compatibility/local-oidc-profile.md)
- [Cross-provider acquisition-agent lab](docs/07-guides/acquisition-agent-lab.md)
- [Cross-provider federation compatibility matrix](docs/07-compatibility/cross-provider-federation.md)
- [Agent protocol v1alpha1](docs/04-agents/agent-protocol.md)
- [Agent-run guide](docs/07-guides/agent-run.md)

## Working vocabulary

- **Tenant:** an isolated organizational identity boundary.
- **Provider facade:** a deliberately scoped API surface shaped like a cloud provider.
- **Canonical graph:** CloudAILab's provider-neutral model of identities, resources, permissions, and trust.
- **Agent under test:** an external or embedded agent evaluated inside a scenario.
- **Invariant:** a deterministic security requirement used for verification.

## Documentation policy

GitHub-readable Markdown is the portable source format. Obsidian is the authoring and navigation experience. Architectural decisions that are costly to reverse require an ADR, and behavior changes require corresponding requirements, documentation, and tests.
