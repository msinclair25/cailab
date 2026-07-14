---
title: CloudAILab Documentation Map
status: active
last_reviewed: 2026-07-13
---

# CloudAILab documentation map

Open the repository root—not only this directory—as the Obsidian vault so root project files, code, scenarios, and documentation remain in one navigable workspace. Obsidian is optional; every note must also render and link correctly on GitHub without community plugins.

## Start here

| Need | Authoritative document |
|---|---|
| Verified value, status, and quick start | [Repository README](../README.md) |
| Mission, audience, and boundaries | [Project charter](00-project/charter.md) |
| Delivery order, current slice, risks, and exit gates | [Master plan](00-project/master-plan.md) |
| Stable behavior requirements | [Product requirements](01-product/requirements.md) |
| System boundaries and source-of-truth ownership | [System architecture](02-architecture/system-architecture.md) |
| Durable architecture and security decisions | [ADR index](02-architecture/decisions/README.md) |
| Trust boundaries, threats, and mitigations | [Threat model](03-security/threat-model.md) |
| Scenario and agent contracts | [Scenario specification](04-scenarios/scenario-specification.md), [data-only starter](../examples/scenario-starter/README.md), and [agent protocol](04-agents/agent-protocol.md) |
| Engineering and validation policy | [Engineering standards](05-engineering/engineering-standards.md) and [quality strategy](05-engineering/quality-strategy.md) |
| First-user release gate | [First-user acceptance](05-engineering/first-user-acceptance.md) |
| Current candidate preparation | [RC2 preparation record](05-engineering/rc2-preparation.md) |
| Provider and feature fidelity | [Compatibility index](07-compatibility/README.md) |
| Installation and hands-on workflows | [Guide index](07-guides/README.md) |
| Self-guided lessons and learning metadata | [Learning index](08-learning/README.md) |

The current delivery sequence is the **Immediate next actions** section of the [master plan](00-project/master-plan.md#immediate-next-actions). The roadmap is a summary; it does not override the master plan or requirements.

## Vault conventions

The shared Obsidian settings create new notes under `docs/`, store pasted attachments under `docs/assets/`, update links after renames, and generate relative standard Markdown links. Workspace layout, appearance, Sync state, hotkeys, bookmarks, and community plugins remain local and are ignored by Git.

Follow the [documentation conventions](05-engineering/documentation-conventions.md) when creating or moving notes. Run the repository documentation check after every documentation change:

```bash
go run ./internal/tools/doccheck .
```

## Source groups

- `00-project/` — charter, glossary, master plan
- `01-product/` — requirements
- `02-architecture/` — system design and ADRs
- `03-security/` — threat model
- `04-scenarios/` and `04-agents/` — versioned author and protocol contracts
- `05-engineering/` — standards, quality, roadmap, release evidence, and documentation practice
- `06-research/` — technical basis and primary-source register
- `07-guides/` — user workflows
- `07-compatibility/` — exact tested fidelity and limitations
- `08-learning/` — data-only lesson contract, focused paths, and adaptation provenance

Do not create a second source of truth to make the graph look richer. Add links where a real ownership, dependency, evidence, or workflow relationship exists.
