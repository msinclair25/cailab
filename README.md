---
title: CloudAILab
aliases:
  - CAL
status: discovery
---

# CloudAILab

> A local enterprise identity and AI-agent security range.

CloudAILab is a working project name for a reproducible learning and evaluation environment spanning AWS, Microsoft Entra/Microsoft 365, and Google Workspace-compatible ecosystems. It is intended to teach enterprise IAM and cybersecurity, and to evaluate how AI agents behave when they receive identities, credentials, tools, and access to sensitive data.

The command-line name is planned as `cailab`; `cal` is intentionally avoided because it conflicts with the traditional Unix calendar utility.

## Project status

The project is in discovery and architecture definition. No compatibility or fidelity claims should be treated as implemented until they appear in a tested compatibility matrix.

## Principles

- Deterministic security decisions; optional AI explanations.
- One deep, end-to-end scenario before broad API coverage.
- Explicit compatibility claims instead of implied cloud parity.
- Local-first operation with no required cloud account or hosted model.
- Safe defaults, fake credentials, loopback binding, and complete cleanup.
- Documentation and tests evolve with the implementation.

## Documentation

- [Project charter](docs/00-project/charter.md)
- [Glossary](docs/00-project/glossary.md)
- [Product requirements](docs/01-product/requirements.md)
- [System architecture](docs/02-architecture/system-architecture.md)
- [Architecture decisions](docs/02-architecture/decisions/README.md)
- [Threat model](docs/03-security/threat-model.md)
- [Scenario specification](docs/04-scenarios/scenario-specification.md)
- [Engineering standards](docs/05-engineering/engineering-standards.md)
- [Delivery roadmap](docs/05-engineering/roadmap.md)

## Working vocabulary

- **Tenant:** an isolated organizational identity boundary.
- **Provider facade:** a deliberately scoped API surface shaped like a cloud provider.
- **Canonical graph:** CloudAILab's provider-neutral model of identities, resources, permissions, and trust.
- **Agent under test:** an external or embedded agent evaluated inside a scenario.
- **Invariant:** a deterministic security requirement used for verification.

## Documentation policy

GitHub-readable Markdown is the portable source format. Obsidian is the authoring and navigation experience. Architectural decisions that are costly to reverse require an ADR, and behavior changes require corresponding requirements, documentation, and tests.
