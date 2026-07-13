---
title: "ADR-0023: Release Artifact and Provenance Pipeline"
status: accepted
date: 2026-07-12
---

# ADR-0023: Release artifact and provenance pipeline

## Context

M4 requires installable cross-platform artifacts whose bytes, dependency inventory, and build origin can be checked independently. A tag-only build that uploads binaries directly would not exercise packaging on pull requests, would make archive layout an untested shell convention, and would leave users without an integrity or provenance verification path.

The release workflow is also a privileged trust boundary. Mutable action tags, broad default permissions, implicit release uploads, unvalidated tag-derived versions, or stale files in a shared output directory can turn a normal packaging step into a supply-chain risk.

## Decision

1. A repository-owned Go tool builds the release matrix and writes the archives. Linux amd64/arm64 and macOS amd64/arm64 use `tar.gz`; Windows amd64 uses `zip`.
2. Builds set `CGO_ENABLED=0`, use the committed module graph in read-only mode, remove local paths and implicit VCS metadata, and inject an explicit semantic version, full source commit, and source-commit timestamp. Archive timestamps use that same timestamp.
3. The tool cleans only its fixed `dist/binaries` and `dist/packages` children, stages binaries separately from release assets, rejects unsafe output roots, and produces sorted SHA-256 entries only for the declared archive and SPDX JSON asset types.
4. The release workflow runs for relevant pull requests and manual dispatches. It packages the complete matrix, generates an SPDX JSON SBOM with a pinned Syft release, generates checksums, and executes the native archive on Linux, macOS, and Windows before publication is possible.
5. Only a `v*` tag can reach attestation and publication jobs. The package tool and workflow both reject a non-semantic version after removing the leading `v`.
6. GitHub Actions are pinned to full commit SHAs. The default permission is `contents: read`; attestation and publication use separate jobs with only their required write permissions.
7. A tag build uses GitHub's consolidated attestation action to create both build provenance for every release asset and an SPDX SBOM attestation relating the archives to the generated SBOM. Publication waits for native smoke tests and attestations.
8. The checksum file, SPDX JSON document, and archives are normal GitHub release assets. Attestations are stored by GitHub/Sigstore and are verified with `gh attestation verify`.
9. Provenance proves the recorded build identity and process, not that the source, dependencies, workflow, or resulting binary are free of vulnerabilities. Checksums detect byte changes only when the manifest itself is obtained from a trusted release and/or verified attestation.

## Consequences

- Archive construction and checksum selection are reviewable Go code with deterministic unit tests.
- Pull requests exercise the same package and native-install path used by tags without minting attestations or creating releases.
- The workflow introduces two pinned downloaded components: the SBOM action and its explicitly selected Syft version. Their updates require dependency review.
- One SBOM covers all staged target binaries. Consumers can inspect the machine-readable target paths; a later packaging change may split it only through a new compatibility decision.
- Linux arm64 is cross-compiled and archived but is not yet executed on a native hosted runner. The compatibility claim remains limited accordingly.
- Source-date inputs make equivalent builds more reproducible, but independent bit-for-bit reproduction across different Go toolchain or dependency environments is not yet claimed.

## Validation

- Unit tests cover metadata validation, unsafe output rejection, deterministic tar/zip bytes, archive paths, modes, timestamps, checksum ordering, and asset selection.
- The release workflow builds all declared targets and verifies every checksum on each native smoke runner.
- Linux, macOS, and Windows smoke jobs extract the matching archive, execute `cailab version`, and enumerate the embedded scenario catalog.
- Tag-only jobs attest the verified asset set before a GitHub release can be created.

## Sources

- [GitHub: Using artifact attestations to establish provenance](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
- [GitHub: Artifact attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations)
- [Go command build flags](https://pkg.go.dev/cmd/go)
- [Anchore SBOM Action](https://github.com/anchore/sbom-action)
- [SLSA provenance model](https://slsa.dev/spec/v1.2/provenance)
