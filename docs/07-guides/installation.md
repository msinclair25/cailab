---
title: Installation
status: active
last_reviewed: 2026-07-13
---

# Installation

CloudAILab has no public version tag yet. Until the first release is published, build from a reviewed commit or use a short-lived release-candidate artifact from GitHub Actions. Do not present an untagged development build as a supported release.

## Requirements

- A supported Linux, macOS, or Windows host from the [release matrix](release-verification.md)
- Go 1.25.12 or newer when building from source
- Docker 20.10 or newer for AWS/Floci scenarios, Docker-isolated custom agents, and the clean-container demo
- No cloud account, hosted model, global proxy, or trusted local certificate for built-in scenarios

Run `cailab doctor <scenario>` before starting a lab. Docker is not required for `walking-skeleton`, `microsoft-consent`, `google-drive-sharing`, or `local-oidc`.

After building or extracting the binary, the shortest first run is:

```bash
cailab quickstart
```

This exercises and cleans up the normal no-Docker lifecycle. See the [no-Docker quick-start guide](no-docker-quickstart.md) for the exact manual equivalent and evidence boundary.

## Build from source

```bash
git clone https://github.com/msinclair25/cailab.git
cd cailab
go mod verify
mkdir -p ./bin
go build -trimpath -o ./bin/cailab ./cmd/cailab
./bin/cailab version
./bin/cailab doctor walking-skeleton
```

Source builds use development version metadata. Tagged archives are the release contract because their checksums, SBOM, smoke tests, and provenance are produced together.

## Install a release archive

When a release exists, select the archive matching the [declared platform matrix](release-verification.md), then verify its checksum and provenance before extraction.

On Linux or macOS:

```bash
tar -xzf cailab_VERSION_OS_ARCH.tar.gz
cd cailab_VERSION_OS_ARCH
./cailab version
./cailab scenario list
```

On Windows PowerShell:

```powershell
Expand-Archive cailab_VERSION_windows_amd64.zip
Set-Location cailab_VERSION_windows_amd64
.\cailab.exe version
.\cailab.exe scenario list
```

Each archive contains the `cailab` executable, the native `cailab-agent-starter` executable and adaptable agent files, the validated data-only scenario starter, least-privilege CI example, validated learning catalog/schema and initial learning-path documents, README, changelog, project license/notice, third-party notice index, exact linked-module inventory, and copied third-party license material. The built-in scenario catalog is compiled into the executable. Relative README links to bundled files remain local; links to documentation or community files that are not bundled are rewritten during packaging to the exact source commit.

Move the executable into a user-owned directory already on `PATH` only after verification. Administrator/root installation is not required and is discouraged for normal use.

## State location

CloudAILab writes local state beneath `.cloudailab` by default. Set a dedicated location before running commands from changing working directories:

```bash
export CAILAB_HOME="$HOME/.local/state/cailab"
```

PowerShell:

```powershell
$env:CAILAB_HOME = "$HOME\AppData\Local\cailab"
```

State contains synthetic scenario data, run metadata, local signing material, and evidence. File permissions remain security-relevant even though the fixtures are synthetic.

For scripts and external tools, use the versioned JSON status contract instead of parsing the human table. See [CLI automation](automation.md).

## Upgrade or remove

Read the [upgrade guide](upgrading.md) before replacing a binary. To uninstall, run `cailab down` for any active range, remove the executable, and delete the selected state directory only after confirming it contains no run you need to preserve. CloudAILab does not delete arbitrary Docker resources or user directories as part of binary removal.
