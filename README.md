---
title: CloudAILab
aliases:
  - CAL
status: m1-development
---

# CloudAILab

> A local enterprise identity and AI-agent security range.

CloudAILab is a working project name for a reproducible learning and evaluation environment spanning AWS, Microsoft Entra/Microsoft 365, and Google Workspace-compatible ecosystems. It is intended to teach enterprise IAM and cybersecurity, and to evaluate how AI agents behave when they receive identities, credentials, tools, and access to sensitive data.

The command-line name is planned as `cailab`; `cal` is intentionally avoided because it conflicts with the traditional Unix calendar utility.

## Project status

M0, the executable walking skeleton, is complete. M1 now includes a development AWS vertical slice backed by a digest-pinned Floci runtime: two accounts, IAM role trust, STS role assumption, S3 data, live trust-path normalization, remediation-aware verification, reset, and cleanup. Microsoft, Google, and agent runtimes are not implemented yet. Provider compatibility is limited to the tested operations in the compatibility matrix.

## Build and try M0

Prerequisites: Go 1.25.8 or newer. Docker is required for provider-backed scenarios.

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

## Working vocabulary

- **Tenant:** an isolated organizational identity boundary.
- **Provider facade:** a deliberately scoped API surface shaped like a cloud provider.
- **Canonical graph:** CloudAILab's provider-neutral model of identities, resources, permissions, and trust.
- **Agent under test:** an external or embedded agent evaluated inside a scenario.
- **Invariant:** a deterministic security requirement used for verification.

## Documentation policy

GitHub-readable Markdown is the portable source format. Obsidian is the authoring and navigation experience. Architectural decisions that are costly to reverse require an ADR, and behavior changes require corresponding requirements, documentation, and tests.
