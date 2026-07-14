---
title: RC2 Preparation Record
status: active
last_reviewed: 2026-07-13
base_commit: c9e359ea545747a39b31f626e1a505179ed8831e
branch: codex/m4-1-adoption-quality
decision: ready-for-review-not-a-candidate
---

# RC2 preparation record

## Decision

The M4.1 working tree is **ready for branch review and remote candidate gates**, but it is not `0.1.0-rc.2` and is not approved for a tag or public release.

The local dry-run archives were produced from the pre-commit working tree. They use development version metadata and embed the unchanged base commit even though their inputs included M4.1 changes. They prove packaging behavior only; they are not provenance-identifiable candidate artifacts. RC2 must be produced from the exact reviewed and merged commit by the repository release workflow.

## Scope prepared

- Truth/navigation and Obsidian documentation reconciliation
- Successful public CLI help and versioned JSON status/endpoint contract
- Guided no-Docker quick start
- Release-packaged governed external-agent starter
- Versioned learning schema, catalog, validator, focused path, and adaptation provenance
- Public custom-scenario validation and release-packaged no-runtime authoring starter
- Deterministic invariant JUnit reporting and least-privilege synthetic CI example
- First-user acceptance protocol, maintainer rehearsal, and resolved source/archive documentation friction
- Exact-commit release-link rewriting and expanded archive contents/smoke checks

## Local evidence

Host: macOS arm64, Go 1.26.5, Docker Engine 29.5.3. All identities, credentials, endpoints, and payloads were synthetic.

| Gate | Result |
|---|---|
| Formatting, module tidy/integrity, vet, documentation, learning semantics, workflow YAML, and diff whitespace | Passed |
| Full unit suite and full race-detector suite | Passed |
| `govulncheck` 1.6.0 reachable-code scan | No vulnerabilities found |
| Linked-module union compared with `third_party/modules.txt` | Exact match |
| Native Microsoft, Google, and OIDC lifecycle integrations | Passed |
| Pinned Floci IAM/STS/S3 integration | Passed |
| Cross-provider federation integration | Passed |
| Docker agent-isolation adversarial probe and public CLI isolation integration | Passed |
| Public cross-provider CLI E2E, including packaged-style external starter, replay, campaigns, federation, remediation, and intended-access preservation | Passed |
| Portfolio workflow against a packaged development binary | Passed finding, exact remediation, safe/unsafe controls, and endpoint/container cleanup |
| Five-target development packaging, archive validation, JUnit/source-starter smoke, and exact-commit link rewriting | Passed |
| Repeated packaging with identical metadata | All five archive SHA-256 values byte-identical |
| Digest-pinned clean-demo image contract and network-none/read-only/resource-bounded run | Passed; temporary image removed |
| Managed provider/agent container leak checks | No container remained |

The [first-user acceptance record](first-user-acceptance.md) contains the separate usability evidence and remaining human gate.

## Gates that require the exact reviewed commit

These cannot be closed by a dirty-working-tree dry run:

1. Review and merge the complete M4.1 branch with required CI green.
2. Confirm CodeQL, dependency review, secret scanning, action pinning, and branch protections on that exact lineage.
3. Run the release workflow for `0.1.0-rc.2` without publishing a stable tag.
4. Inspect the resulting five archives, checksum manifest, SPDX JSON SBOM, copied legal material, exact linked modules, link integrity, and native Linux/macOS/Windows smoke results.
5. Rebuild from identical RC2 metadata and compare binaries/archives where the supported reproducibility process applies.
6. Run the unfamiliar-participant acceptance protocol against the exact RC2 archive and resolve release-blocking friction.
7. Record and publish the portfolio demo from verified RC2.
8. Obtain explicit repository-owner approval of Apache-2.0 and the residual risks in the release audit.
9. Promote the changelog, confirm the final commit/tag lineage has not changed, and only then create verified `v0.1.0`.

## RC2 execution command

After the reviewed commit is merged, use the manual release workflow with version `0.1.0-rc.2`. Do not create a `v0.1.0-rc.2` tag merely to exercise the candidate; the workflow's manual-dispatch path packages and smoke-tests without tag-only attestation or publication. Record the workflow run and downloaded artifact hashes in the final RC2 audit.

Any behavior, dependency, workflow, legal, compatibility, or documentation change after the reviewed RC2 commit reopens the affected gates.
