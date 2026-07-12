---
title: Threat Model
status: draft
---

# Threat model

## Scope

This threat model covers the CloudAILab control plane, local provider services, scenario files, generated credentials, agent gateway, audit records, optional model integrations, and the developer host on which the range runs.

It does not assert containment for arbitrary agent processes unless an isolation mode enforces network and filesystem boundaries.

## Protected assets

- Host credentials and personal files
- Integrity of scenario ground truth and verification rules
- Integrity and confidentiality of audit evidence
- Isolation between simulated tenants and accounts
- Authenticity of downloaded runtime dependencies
- Availability of the local control plane
- Secrets supplied for optional hosted model providers

## Trust boundaries

1. User shell ↔ CloudAILab CLI
2. CloudAILab control plane ↔ external containers
3. Agent under test ↔ governed tool gateway
4. Provider facade ↔ canonical state
5. Local issuer ↔ token-consuming services
6. Local range ↔ optional hosted model API
7. Workspace files ↔ downloaded third-party artifacts

## Initial threats and controls

| ID | Threat | Primary controls |
|---|---|---|
| TM-001 | A generated credential is accidentally sent to a real cloud endpoint. | Visibly fake credentials, endpoint-scoped environment, network isolation option, no production credential discovery in range processes. |
| TM-002 | An agent reads host files or accesses the public internet. | Explicit isolated execution mode, minimal mounts, deny-by-default egress, accurate documentation of non-isolated mode. |
| TM-003 | Prompt-injected content causes unauthorized tool use. | Treat content as untrusted, action-level policy gateway, approvals, scoped credentials, complete audit trail. |
| TM-004 | A malicious scenario executes arbitrary host code. | Declarative schema, no implicit shell evaluation, capability allowlist, validation before apply. |
| TM-005 | Compromised dependency images or binaries execute locally. | Pinned versions, checksums/signatures where available, provenance metadata, SBOM, update policy. |
| TM-006 | Global proxy or certificate changes weaken the host. | Endpoint override by default, explicit consent for advanced proxy mode, reversible changes, diagnostics and cleanup. |
| TM-007 | Tenant state leaks across simulated boundaries. | Tenant-scoped keys, authorization tests, negative contract tests, unique identifiers, graph invariants. |
| TM-008 | Verification rules are disclosed or modified during a mission. | Separate public objectives from protected ground truth, integrity hashes, read-only mission mounts in isolated mode. |
| TM-009 | Hosted AI receives sensitive traces or secrets. | Opt-in adapters, field-level redaction, outbound preview, provider configuration, no implicit uploads. |
| TM-010 | An AI-generated explanation contradicts evidence. | Evidence citations, deterministic score precedence, explicit non-authoritative labeling. |

## Security invariants

- An action without an authenticated simulated principal is denied unless explicitly public.
- Explicit deny takes precedence over allow in the canonical evaluator.
- Tenant boundary crossings require a visible trust edge.
- Agent authority cannot exceed the credential and policy attached to its current run.
- Every governed agent tool call produces a decision record.
- Optional coaching cannot change verification state or score.

## Review triggers

Review this document when adding a provider, execution mode, downloadable dependency, hosted integration, new credential type, or new agent tool protocol.
