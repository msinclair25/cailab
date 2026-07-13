---
title: Release-Candidate Readiness Audit
status: active
candidate: 0.1.0-rc.1
audit_date: 2026-07-13
baseline_commit: 4fa8bb2b9b6d25a9be7db1ad7b45239e04840638
decision: conditional-candidate-approval
---

# Release-candidate readiness audit

## Decision

CloudAILab is **conditionally approved to exercise `0.1.0-rc.1`**, but it is **not approved for a public `v0.1.0` tag**.

The candidate approval becomes effective only after this release-readiness change is merged with all required CI, CodeQL, and dependency-review checks green. Any critical or high release-blocking result revokes the approval. The candidate exists to test the exact archives and release controls; it is not a stable-version promise.

Public-tag approval remains a separate owner decision after candidate artifact inspection, a published demo recording, changelog promotion, and explicit acceptance of Apache-2.0 plus the residual risks in this record.

## Scope and evidence reviewed

| Area | Evidence | Audit conclusion |
|---|---|---|
| User contract | `README.md`, requirements, installation, upgrade, troubleshooting, architecture, support, and demo guides | Current behavior and prerequisites are separated from planned behavior; no published version is claimed. |
| Deterministic core | Scenario compiler, canonical graph, policy, verification, evidence, replay, trial-state, and campaign tests | Authorization and scoring remain code-owned and deterministic; no AI coach ships in this candidate, and any future AI feature cannot override them. |
| Provider fidelity | AWS/Floci, Microsoft, Google, local OIDC, and cross-provider compatibility records | Claims remain operation-specific and test-backed; direct Floci web-identity behavior is not treated as authorization evidence. |
| Agent boundary | Agent protocol, host execution, Docker isolation, prompt-injection, replay, trial-state, and campaign records | Host agents and all tools are explicitly unisolated; the opt-in Docker claim applies only to the agent and only to the tested Linux-engine contract. |
| Security | Threat model, security policy, repository security settings, negative tests, race tests, `govulncheck`, CodeQL, and dependency review | No known critical or high release-blocking defect is accepted silently. Branch and candidate scans remain mandatory gates. |
| Distribution | Release packager, archive tests, multi-OS smoke jobs, checksums, SPDX SBOM, attestations, and verification guide | Tag publication is gated after native smoke and attestation; the release-candidate path does not publish a release. |
| Licensing | Apache-2.0 ADR, project `LICENSE`/`NOTICE`, linked-module union, Go runtime license, and copied third-party material | The audited linked set is permissive after manual resolution of the `modernc.org/mathutil` classifier gap; this is an engineering record, not legal advice. |
| Community readiness | Security, support, contribution, conduct, changelog, and upgrade policies | Reporting and contribution expectations are explicit before the first public tag. |

## Repository security controls

The following repository settings were observed or enabled on 2026-07-13:

| Control | State |
|---|---|
| Secret scanning | Enabled |
| Push protection | Enabled |
| Dependabot vulnerability alerts | Enabled |
| Dependabot automatic security updates | Enabled |
| Private vulnerability reporting | Enabled; the private reporting link is published in `SECURITY.md` |
| Dependency review | Full-SHA-pinned pull-request workflow; moderate-or-higher introduced vulnerabilities fail the check after merge |
| CodeQL | Full-SHA-pinned Go manual-build workflow on pull requests, `main`, manual dispatch, and a weekly schedule after merge |
| Workflow permissions | Read-only by default; write scopes are isolated to tag-gated attestation and publication jobs |

## Compatibility conclusions

- Docker is the only tested container runtime. Podman, remote daemons, Windows containers, and Kubernetes are not supported claims.
- Linux amd64, macOS amd64/arm64 as available on hosted runners, and Windows amd64 receive native archive smoke tests. Linux arm64 is cross-compiled only.
- Microsoft, Google, and local OIDC facades implement documented scenario slices, not provider parity or production identity services.
- Floci supplies selected AWS-shaped behavior. CloudAILab's canonical evaluator and federation gateway remain authoritative for supported security conclusions.
- The supported agent protocol can launch external agents. Host-mode agents and every tool subprocess retain the launching user's ambient authority and must be trusted.
- Docker agent mode blocks external networking and host mounts under the documented Linux contract, but Docker Engine and the host kernel remain trusted; tools remain outside that boundary.
- Pre-1.0 state compatibility has no in-place migration promise. Back up and restore the executable and full state directory as a pair.

## Residual-risk acceptances required for a public tag

| Risk | Potential impact | Current control and release disposition |
|---|---|---|
| Untrusted host-mode agent or tool | Critical host compromise or data access | No isolation claim; documentation says to run only trusted code. Public-tag approval must accept this explicit boundary. |
| Direct discovery of permissive Floci routes | High bypass of supported gateway semantics | Direct calls are excluded from authorization evidence; supported agents use governed tools, and Docker agent mode has no provider network access. |
| Docker daemon or kernel escape | Critical host impact | Local Docker is trusted, remote/rootless engines are rejected for isolation, and the boundary is not described as a VM. |
| Emulation differs from live providers | High incorrect generalization | Operation-level matrices, canonical verification, and unsupported-operation documentation prohibit provider-parity claims. |
| Linux arm64 archive lacks native smoke | Medium portability failure | Cross-compilation is explicit in the release matrix; users must report target-specific failures. |
| Pre-1.0 state/schema change | Medium local state incompatibility | Upgrade guide requires backup and permits clean reset; no in-place guarantee is made. |
| Solo-maintainer response capacity | Medium delayed support or security response | Support and security policies are best-effort with target response times but no SLA. |

These are bounded design limitations, not permission to waive a new defect. A newly discovered critical authorization, tenant-isolation, disclosure, cleanup, artifact-integrity, or overclaim defect blocks the candidate and tag.

## Candidate validation record

The following local gates passed on 2026-07-13. The host checks used Go 1.26.5 on macOS arm64; the clean-demo build separately used the digest-pinned Go 1.25.12 image declared by the repository.

| Gate | Result |
|---|---|
| Formatting, module tidy/integrity, vet, actionlint, documentation, and linked-module equality | Passed |
| Full `go test -count=1 ./...` and `go test -race -count=1 ./...` | Passed |
| Reachable vulnerability scan with `govulncheck` 1.6.0 | No vulnerabilities found |
| Floci provider, cross-provider federation, public CLI E2E, native facade, and Docker agent-isolation integrations | Passed against local Docker Engine 29.5.3 |
| Linked-license report and source-file comparison | Permissive inventory confirmed; `modernc.org/mathutil` required manual BSD-3-Clause review because the classifier returned unknown |
| Five-target `0.1.0-rc.1` packaging dry run | Passed; complete project/legal/third-party bundle present in tar and zip layouts |
| Repeated packaging with identical metadata | Passed; archive SHA-256 manifests were byte-identical |
| Native archive binary smoke on macOS arm64 | Passed version output and embedded scenario listing |
| Digest-pinned clean-container build, image-contract inspection, hardened walking-skeleton run, filesystem inspection, and managed-container leak check | Passed |

GitHub evidence for this change must still cover:

- the normal formatting, module, vet, unit, race, documentation, and reachable-vulnerability gates on the pinned CI toolchain;
- provider, cross-provider, public CLI, native facade, Docker agent-isolation, and clean-container integrations;
- CodeQL and pull-request dependency review;
- exact linked-module inventory equality and third-party license review;
- deterministic multi-target packaging with complete legal bundle and archive inspection;
- manual `0.1.0-rc.1` workflow execution from the merged commit, including SBOM, checksums, and all native smoke jobs.

The pull request and manual workflow are the durable remote execution records. This document records the local result, decision, and required evidence rather than copying complete transient logs.

## Public-tag gates

All of the following are required before `v0.1.0`:

1. Merge this audit and its implementation with every required check green.
2. Run the manual release workflow for `0.1.0-rc.1` from the exact merged commit and inspect the downloaded archives, checksum manifest, SPDX SBOM, legal bundle, and smoke results.
3. Record the portfolio demo from that verified candidate and link the published recording from the repository.
4. Move the changelog content from `Unreleased` to a dated `0.1.0` entry.
5. Obtain explicit repository-owner approval of Apache-2.0 and the residual-risk acceptances above.
6. Confirm the proposed tag still points to the reviewed commit with no intervening behavior, dependency, workflow, notice, or compatibility change.
7. Record the final tag decision and create the signed or otherwise verified tag only after the preceding gates pass.

## Reopen triggers

Re-run this audit after a provider/runtime upgrade, Go or release-target change, new listener or agent capability, dependency/license change, material workflow change, failed candidate gate, or any change to the compatibility claims above.
