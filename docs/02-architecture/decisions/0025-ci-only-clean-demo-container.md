---
title: "ADR-0025: CI-Only Clean Demo Container"
status: accepted
date: 2026-07-12
---

# ADR-0025: CI-only clean demo container

## Context

M4 requires evidence that a new user can reproduce the documented CloudAILab demo from a clean supported environment. Running the demo directly in a repository checkout can accidentally depend on the host Go installation, local files, ambient network, credentials, or writable paths. A container is useful evidence only if its inputs and claim boundary are explicit; an unpinned base image, source-filled final image, root default, host mount, or unrestricted runtime would replace one ambiguous environment with another.

This artifact has a narrower purpose than a release archive. It validates the built CLI and embedded catalog inside Linux CI. It is not a supported installation channel, a provider runtime, an agent sandbox, or a published image.

## Decision

1. `build/ci/Dockerfile` defines a multi-stage CI image. The builder is the Docker Official `golang:1.25.12-bookworm` image pinned to its multi-platform digest. The final stage is `scratch` and therefore introduces no second base-image filesystem.
2. The build context is allowlisted through `.dockerignore`. The final stage receives only the CGO-free `cailab` binary and a small CGO-free code-owned demo runner; it contains neither the repository source tree, a shell, an operating-system package set, nor the Go toolchain.
3. The final image declares numeric UID/GID `65532:65532`, no port, no volume, and no provider or Docker socket access. Its default entrypoint is the deterministic walking-skeleton demo.
4. CI validates the image configuration and runs it without host mounts, external networking, or IPC. The root filesystem is read-only; `/tmp` is a bounded `noexec`, `nosuid`, `nodev` tmpfs; capabilities are dropped; `no-new-privileges` and Docker's built-in seccomp profile are explicit; CPU, memory, PID, and file-descriptor limits are bounded; and the container is removed after execution.
5. The demo exercises version metadata, prerequisites, the embedded catalog, scenario briefing, startup, status, mission, graph traversal, deterministic verification, and shutdown. It requires no cloud account, hosted model, provider container, or interactive input.
6. The demo runner launches the fixed `cailab` command sequence directly without a shell and supplies only `HOME`, `PATH`, and `CAILAB_HOME` to each child. CI output remains in GitHub Actions logs so a failure is diagnosable. This differs deliberately from untrusted agent isolation, where Docker log persistence is disabled.
7. The image is built on pull requests and `main` but is not pushed, signed, attested, or described as a release artifact. The release archives remain the supported distribution contract.

## Consequences

- The clean demo is reproducible with Docker from a small, reviewable command sequence.
- Pull requests gain a network-dependent image build and a short container run. Registry availability can fail this independent environment gate.
- Digest pinning makes base-image updates explicit and reviewable, but also requires deliberate refreshes to receive upstream fixes.
- The final image has no OS package manager, shell, or CA bundle. Expanding the demo to require those facilities would require a new decision rather than an implicit package installation.
- Docker Engine, its daemon privileges, the container runtime, and the host kernel or Docker Desktop VM remain trusted. The controls demonstrate a clean bounded demo environment, not VM-grade containment.

## Validation

- Actionlint validates the workflow structure.
- Unit tests cover the fixed demo sequence plus cleanup before and after successful startup.
- CI builds the image from the allowlisted context and rejects unexpected user, port, volume, entrypoint, source-tree, or Go-toolchain state.
- CI executes the complete demo with no external network route, no mounts, a read-only root, bounded temporary storage, reduced privileges, and resource limits.
- The normal Go, race, provider, agent-isolation, documentation, vulnerability, and cross-platform build checks remain independent gates.

## Sources

- [Docker build best practices](https://docs.docker.com/build/building/best-practices/)
- [Docker multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
- [Dockerfile `USER` and `WORKDIR` reference](https://docs.docker.com/reference/dockerfile)
- [Docker Official Go image metadata](https://github.com/docker-library/repo-info/blob/master/repos/golang/remote/1.25.12-bookworm.md)
