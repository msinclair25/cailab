---
title: CloudAILab
aliases:
  - CAL
status: m0-complete
---

# CloudAILab

> A local enterprise identity and AI-agent security range.

CloudAILab is a working project name for a reproducible learning and evaluation environment spanning AWS, Microsoft Entra/Microsoft 365, and Google Workspace-compatible ecosystems. It is intended to teach enterprise IAM and cybersecurity, and to evaluate how AI agents behave when they receive identities, credentials, tools, and access to sensitive data.

The command-line name is planned as `cailab`; `cal` is intentionally avoided because it conflicts with the traditional Unix calendar utility.

## Project status

M0, the executable walking skeleton, is complete. The repository implements strict scenario loading, deterministic compilation, canonical path analysis, SQLite-backed run lifecycle, deterministic invariant verification, and the initial CLI. AWS, Microsoft, Google, and agent runtimes are not implemented yet. No provider compatibility claim should be treated as implemented until it appears in a tested compatibility matrix.

## Build and try M0

Prerequisites: Go 1.25 or newer. Docker is checked by `doctor` because provider runtimes will require it beginning in M1.

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

## Working vocabulary

- **Tenant:** an isolated organizational identity boundary.
- **Provider facade:** a deliberately scoped API surface shaped like a cloud provider.
- **Canonical graph:** CloudAILab's provider-neutral model of identities, resources, permissions, and trust.
- **Agent under test:** an external or embedded agent evaluated inside a scenario.
- **Invariant:** a deterministic security requirement used for verification.

## Documentation policy

GitHub-readable Markdown is the portable source format. Obsidian is the authoring and navigation experience. Architectural decisions that are costly to reverse require an ADR, and behavior changes require corresponding requirements, documentation, and tests.
