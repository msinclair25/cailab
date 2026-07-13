---
title: "ADR-0026: Apache-2.0 License and Release Notice Bundle"
status: accepted
date: 2026-07-13
---

# ADR-0026: Apache-2.0 license and release notice bundle

## Context

CloudAILab had no public license before its first planned release. Its release archives contained only the executable and README even though the statically linked executable incorporates the Go runtime and permissively licensed third-party modules. A repository license alone would not make the binary archive self-describing or preserve every dependency notice required for redistribution. An SBOM inventories detected components but is not a substitute for license terms or copyright notices.

The project also needs a reviewable trigger when the set of modules linked into any supported release target changes. Auditing `go.mod` alone overstates test/tool-only modules and can miss target-specific linked packages or the Go runtime.

## Decision

1. CloudAILab source, documentation, and project-owned release artifacts are licensed under Apache License 2.0 beginning before the first public tag. This provides an explicit permissive copyright license and patent grant. The repository owner must review this decision before approving the first tag.
2. Source and binary distributions include the complete project `LICENSE` and `NOTICE` files.
3. Release archives also include `THIRD_PARTY_NOTICES.md`, `CHANGELOG.md`, the exact `third_party/modules.txt` inventory, and the copied license/notice material under `third_party/licenses/`.
4. `third_party/modules.txt` records the sorted union of non-main Go modules linked into every declared CGO-free release target. The code-owned release tool computes that union with `go list`; normal CI and the release workflow fail if it drifts.
5. The notice bundle separately includes the Go 1.25.12 runtime/standard-library license, which is not a Go module and is therefore absent from module-only reports.
6. Dependency changes must review the new version, applicable license/notice files, SBOM/vulnerability results, and compatibility/security impact in the same change. License classification assists review but does not make a legal conclusion.
7. Floci and Docker are obtained separately and are not copied into release archives. Their reviewed license and digest evidence remains in the technical basis and compatibility records.

## Consequences

- Every archive is self-contained with project and linked-component legal material.
- Release sizes increase modestly because notice texts are intentionally duplicated across platform archives.
- Dependency pull requests fail until the linked-module inventory and applicable notices are updated.
- The inventory is specific to the supported release target matrix. New targets require both build validation and a fresh union calculation.
- Apache-2.0 does not make CloudAILab, its dependencies, or its scenarios warranty-backed, vulnerability-free, provider-certified, or suitable for production identity use.

## Validation

- Unit tests require the complete distribution document set, linked-module manifest, and sorted third-party license tree.
- Release smoke jobs assert that project license/notice, third-party notices, changelog, and the Go license exist in extracted archives.
- CI and release jobs compare `third_party/modules.txt` with the code-owned multi-target linked-module calculation.
- The release-candidate audit checks repository settings, license inventory, SBOM, reachable vulnerabilities, and compatibility limitations independently.

## Sources

- [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0.txt)
- [Apache guidance for applying License 2.0](https://www.apache.org/legal/apply-license)
- [Google go-licenses documentation](https://pkg.go.dev/github.com/google/go-licenses)
- [Go 1.25.12 license](https://github.com/golang/go/blob/go1.25.12/LICENSE)
