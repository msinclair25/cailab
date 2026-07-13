---
title: Clean Container Demo
status: active
last_reviewed: 2026-07-12
---

# Clean container demo

This workflow proves the documented walking-skeleton path in a fresh Linux container using only Docker. It builds `cailab`, uses the embedded built-in scenario catalog, starts no provider runtime, needs no cloud account or hosted model, and has no external network route while the demo runs.

The image is a CI validation artifact, not a supported CloudAILab distribution or agent sandbox. Release archives remain the installable artifacts. The image is not published, signed, attested, or included in the release SBOM.

## Prerequisites

- A local Linux Docker Engine or Docker Desktop running Linux containers
- Network access while building, to fetch the digest-pinned Docker Official Go builder and Go modules
- No cloud credentials or model API keys

## Build

From the repository root:

```bash
docker build \
  --file build/ci/Dockerfile \
  --tag cailab-ci:local \
  .
```

The multi-stage build uses the repository's pinned Go version, verifies downloaded module content, and creates two CGO-free binaries: the CLI and a small code-owned demo runner. Only those binaries enter the final `scratch` image. It contains no shell, OS package set, repository source, or Go toolchain, and `.dockerignore` allowlists the build inputs.

## Run the bounded demo

```bash
docker run --rm \
  --network none \
  --ipc none \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m,mode=1777,uid=65532,gid=65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges=true \
  --security-opt seccomp=builtin \
  --memory 256m \
  --cpus 1 \
  --pids-limit 128 \
  --ulimit nofile=1024:1024 \
  cailab-ci:local
```

The image runs as UID/GID `65532:65532`, declares no port or volume, receives no host mount, and stores its temporary SQLite state only in the bounded `/tmp` filesystem. The runner invokes the fixed CLI sequence without a shell, uses an independent bounded cleanup context after post-start failure, and `--rm` removes the container after success or failure.

## What success proves

The code-owned runner executes:

```text
version → doctor → scenario list/show → up → status → mission → graph path → verify → down
```

A successful run proves that the compiled Linux binary and embedded walking-skeleton scenario complete the deterministic no-provider workflow in this declared container configuration. It does not prove AWS/Floci, Microsoft, Google, OIDC, custom agents, or Docker agent isolation compatibility.

## Security boundary

The digest pin establishes the exact selected builder-image content, not that the builder or its packages are vulnerability-free. The final `scratch` stage removes that package filesystem, but Docker Engine, the daemon, container runtime, host kernel or Docker Desktop VM, build inputs, and final image content remain trusted. CI retains stdout/stderr for diagnostics. Do not use this image to execute an untrusted agent or to infer VM-grade isolation.
