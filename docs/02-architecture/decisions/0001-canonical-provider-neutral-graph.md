---
title: "ADR-0001: Canonical Provider-Neutral Graph"
status: proposed
date: 2026-07-11
---

# ADR-0001: Canonical provider-neutral graph

## Context

AWS, Microsoft, and Google use different resource schemas and authorization semantics. Cross-provider attack paths cannot be reliably evaluated if each emulator is treated as an unrelated source of truth. The project also needs stable scenario manifests that do not expose backend implementation details everywhere.

## Decision

CloudAILab will define a provider-neutral canonical graph of principals, tenants, resources, policies, trust edges, data classifications, agents, tools, and audit events.

Provider adapters will translate between canonical types and supported provider representations. Current provider state will be collected and normalized before cross-provider verification.

Provider-native details may be retained as typed extensions, but they must not bypass the canonical identity and relationship model.

## Consequences

### Positive

- Cross-provider path analysis has one input model.
- Scenario authoring is not coupled to runtime implementation.
- Provider adapters can be tested independently.
- Reports can use consistent language across providers.

### Negative

- The canonical model may lose provider-specific nuance.
- Translation and reconciliation add implementation work.
- Extension points and versioning require discipline.

## Validation

The flagship scenario must compile into the graph, deploy through all three adapters, normalize back into an equivalent graph, and expose the intended attack path.
