---
title: Release-Candidate Readiness Audit
status: active
candidate: 0.1.0-rc.1
audit_date: 2026-07-13
baseline_commit: 4fa8bb2b9b6d25a9be7db1ad7b45239e04840638
candidate_commit: 9190af11a6188fd17a614d2b9d9833d08f164188
candidate_workflow_run: 29241014494
decision: candidate-validated-public-tag-pending
---

# Release-candidate readiness audit

## Decision

CloudAILab **successfully exercised and validated `0.1.0-rc.1`**, but it is **not approved for a public `v0.1.0` tag**.

The release-readiness change was merged from the exact reviewed head after all required CI, CodeQL, and dependency-review checks passed. The manual candidate workflow then packaged and smoke-tested the exact merge commit without creating a tag or GitHub release. The candidate exists to test the exact archives and release controls; it is not a stable-version promise.

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

## Remote candidate execution record

Pull request [#22](https://github.com/msinclair25/cailab/pull/22) merged the exact reviewed head `bf658e9772521f832ca4a4c0da8a48366176b7ae` as commit `9190af11a6188fd17a614d2b9d9833d08f164188` on 2026-07-13. Its CI, clean-container demo, CodeQL, dependency-review, release packaging, and Linux/macOS/Windows archive smoke checks all passed.

Manual release workflow [run 29241014494](https://github.com/msinclair25/cailab/actions/runs/29241014494) then exercised `0.1.0-rc.1` from that merge commit. Packaging and all three native operating-system smoke jobs passed. The tag-only attestation and publication jobs were skipped as designed, so this run created neither a public tag nor a GitHub release.

The downloaded workflow artifact was independently inspected with the following results:

| Candidate check | Result |
|---|---|
| Asset set | Five declared platform archives, one SPDX JSON SBOM, and one checksum manifest were present; no unexpected asset was present. |
| Checksum manifest | All five archives and the SBOM passed the downloaded SHA-256 manifest. |
| Archive structure | Every archive contained 32 regular files under one target-specific root, with no symlink or non-regular entry. |
| Project and legal material | `README.md`, `CHANGELOG.md`, `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, and the complete `third_party/` tree matched the merged source byte-for-byte. |
| Linked-module inventory | The native candidate binary contained exactly the 27 versioned modules in `third_party/modules.txt`, with no missing or extra entry. |
| Native candidate smoke | The macOS arm64 binary reported `0.1.0-rc.1`, the exact merge commit, and the commit timestamp, then listed all six embedded scenarios without a checkout. |
| SBOM structure and linkage | The artifact is valid SPDX 2.3 generated by Syft 1.42.3, with 142 package records, five binary file records, and 283 relationships. Every recorded binary SHA-256 matched the corresponding archive payload. |
| Cross-host reproducibility | A clean local rebuild with the pinned Go 1.25.12 toolchain and identical version, commit, and timestamp produced five binaries with the recorded SBOM hashes and five archives byte-for-byte identical to the Ubuntu workflow artifacts. |

Syft identifies the linked components and binary files, but reports the main CloudAILab module's package version and license fields as `UNKNOWN`/`NOASSERTION` when cataloging these Go binaries. The embedded CLI version and commit, project legal bundle, checksum manifest, and tag-gated provenance attestations remain the authoritative complementary records. This limitation does not remove a detected component, but must not be represented as license analysis performed by the SBOM.

The pull request and manual workflow are the durable remote execution records. This document records the decision and independently checked artifact properties rather than copying complete transient logs. The workflow artifact itself has the configured seven-day retention period.

## Public-tag gates

All of the following are required before `v0.1.0`:

| Gate | Status |
|---|---|
| Merge the release-readiness implementation with every required check green. | Complete: PR #22 merged the exact reviewed head. |
| Exercise `0.1.0-rc.1` from the exact merge commit and inspect its archives, checksums, SBOM, legal bundle, reproducibility, and native smoke evidence. | Complete: workflow run 29241014494 and the independent inspection above passed. |
| Record the portfolio demo from the verified candidate and link the published recording from the repository. | Pending. |
| Move the changelog content from `Unreleased` to a dated `0.1.0` entry. | Pending. |
| Obtain explicit repository-owner approval of Apache-2.0 and the residual-risk acceptances above. | Pending. |
| Confirm the proposed tag still points to the reviewed commit with no intervening behavior, dependency, workflow, notice, or compatibility change. | Pending until tag time. |
| Record the final tag decision and create the signed or otherwise verified tag only after the preceding gates pass. | Pending. |

## Reopen triggers

Re-run this audit after a provider/runtime upgrade, Go or release-target change, new listener or agent capability, dependency/license change, material workflow change, failed candidate gate, or any change to the compatibility claims above.
