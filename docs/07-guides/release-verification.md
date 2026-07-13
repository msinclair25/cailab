---
title: Release Verification
status: active
last_reviewed: 2026-07-13
---

# Release verification

CloudAILab tag releases are designed to provide three different facts:

1. `checksums.txt` detects whether downloaded asset bytes differ from the release manifest.
2. The SPDX JSON SBOM lists components Syft detected in the staged release binaries.
3. GitHub artifact attestations bind the assets and SBOM relationship to the repository, commit, and release workflow that produced them.

None of these facts proves the software is vulnerability-free. Review the source, workflow, SBOM, vulnerability results, and known limitations as part of your trust decision.

## Select the archive

| Host | Archive suffix | Native release smoke test |
|---|---|---|
| Linux x86-64 | `linux_amd64.tar.gz` | Yes |
| Linux ARM64 | `linux_arm64.tar.gz` | Cross-compiled only |
| macOS Intel | `darwin_amd64.tar.gz` | Yes when the hosted macOS runner is Intel |
| macOS Apple silicon | `darwin_arm64.tar.gz` | Yes when the hosted macOS runner is ARM64 |
| Windows x86-64 | `windows_amd64.zip` | Yes |

The archives contain one versioned directory with the `cailab` executable, README, changelog, project `LICENSE`/`NOTICE`, `THIRD_PARTY_NOTICES.md`, exact `third_party/modules.txt` linked-module inventory, and copied license material under `third_party/licenses/`. Built-in scenario manifests are compiled into the executable, so no checkout or adjacent scenario directory is required. Docker remains required for AWS/Floci scenarios and optional Docker-isolated agent runs; the Microsoft, Google, local OIDC, and walking-skeleton paths do not require it.

## Verify SHA-256

Download the archive, `checksums.txt`, and the release SBOM from the same GitHub release. On Linux:

```bash
sha256sum --check checksums.txt
```

On macOS:

```bash
shasum --algorithm 256 --check checksums.txt
```

On Windows PowerShell, replace `ARCHIVE_NAME` with the downloaded archive:

```powershell
$expected = (Get-Content checksums.txt | Where-Object { $_ -match '  ARCHIVE_NAME$' }) -split '  ', 2
$actual = (Get-FileHash -Algorithm SHA256 ARCHIVE_NAME).Hash.ToLowerInvariant()
if ($actual -ne $expected[0]) { throw 'checksum mismatch' }
```

## Verify provenance

With a current GitHub CLI, verify the selected archive against this repository:

```bash
gh attestation verify ./ARCHIVE_NAME --repo msinclair25/cailab
```

The command verifies the signed attestation and shows the workflow identity and source repository. Confirm that the digest, repository, commit, tag context, and workflow match the release you intended to install. GitHub documents that attestations establish build origin and integrity; they do not guarantee that an artifact is secure.

## Inspect the SBOM

The `cailab_VERSION_sbom.spdx.json` release asset is standard JSON. Search its `packages`, `relationships`, and file paths with any SPDX-compatible viewer or a JSON tool. The document covers the staged binaries for every packaged target, so repeated modules across targets are expected.

The SBOM and third-party notice bundle answer different questions. The SBOM records components Syft detected; the notice bundle preserves reviewed license/copyright material for the Go runtime and version-locked modules linked into the declared target matrix. Neither is a legal opinion or a security guarantee.

## Execute the archive

After extraction:

```bash
./cailab_VERSION_OS_ARCH/cailab version
./cailab_VERSION_OS_ARCH/cailab scenario list
```

On Windows, use `cailab.exe`. The version output should identify the release version, source commit, and source-commit build timestamp recorded by the release workflow.

`scenario list` reads the immutable catalog compiled into that executable, independent of the current working directory. To use custom content, supply an existing scenario file or explicitly select a catalog with `--root` or `--scenario-root`; no ambient `./scenarios` directory overrides the built-ins. Embedded manifests are public data, not a confidentiality boundary.
