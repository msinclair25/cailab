---
title: Docker Agent Isolation Compatibility
status: m3-complete
last_reviewed: 2026-07-12
---

# Docker agent isolation compatibility

## Claim boundary

CloudAILab supports an opt-in Docker boundary for the **agent process**. The Linux CI integration is authoritative for this development slice. Host subprocess mode and registered tool subprocesses are not isolated.

The immutable execution profile is `docker-strict-v1`. Any future change to its fixed controls requires a new profile identifier.

| Capability | Status | Evidence and limitation |
|---|---|---|
| Content-addressed image | Supported | Only `sha256:<image-id>` and `repository@sha256:<digest>` are accepted. Mutable tags are rejected and implicit pulls are disabled. |
| Image-declared volumes | Rejected | The runtime inspects `.Config.Volumes` before persistence because Dockerfile `VOLUME` paths would remain writable under `--read-only`. |
| Local engine selection | Supported | The active context must expose an absolute local Unix socket and a non-rootless Linux server with an active cgroup driver. The endpoint is pinned with `--host`; TCP, SSH, named-pipe, remote, rootless, no-cgroup, and non-Linux contexts are rejected. |
| Host filesystem exclusion | Supported | No bind mounts, named volumes, devices, or Docker socket are passed. An in-container probe cannot read a host sentinel path. Image contents remain visible by design. |
| External network exclusion | Supported | `--network none` is mandatory; the probe cannot establish an external TCP connection. Container-private loopback still exists. |
| Docker log persistence | Disabled | `--log-driver none` prevents protocol stdout/stderr from being retained for `docker logs`; the controller still receives attached streams. |
| Root filesystem writes | Blocked | `--read-only` is mandatory and the probe cannot create a root-level file. |
| Temporary writes | Supported, bounded | IPC none prevents `/dev/shm`; `/tmp` is a 64 MiB tmpfs with `noexec`, `nosuid`, `nodev`, and UID/GID 65532. The probe also verifies that an ordinary `/dev` file cannot be created. |
| Privilege reduction | Supported | UID/GID 65532, all capabilities dropped, `no-new-privileges`, and built-in seccomp are explicit run flags. Docker and kernel vulnerabilities remain outside the claim. |
| Resource limits | Supported by contract | One CPU, 512 MiB memory/no additional swap, 128 PIDs, and 1024 file descriptors are requested. Rootless and no-cgroup engines are rejected; unsupported run flags fail startup. |
| Descendant ownership | Supported within container | Docker `--init` reaps child processes and container removal owns the remaining process tree. Host-mode detached descendants remain uncontained. |
| Cleanup | Supported | Deterministic name plus run/trial labels; cleanup inspects both labels before forced removal and leak checks cover completion and cancellation. |
| Direct provider/API access | Intentionally unsupported | Network none prevents direct provider, hosted-model, and public-internet calls. The agent uses governed tool calls over standard I/O. |
| Tool subprocess isolation | Unsupported | Registered tools remain trusted bounded host subprocesses. Their manifest isolation declarations are not enforcement claims. |
| VM-grade isolation | Unsupported | Docker daemon/runtime and the host kernel or Docker Desktop VM are trusted. |

## Tested platform

- GitHub-hosted Ubuntu runner with its available Docker Engine: required CI integration.
- Docker Desktop on macOS: development smoke-tested, not yet a release compatibility commitment.
- Windows containers, Podman, containerd CLI, Kubernetes, remote Docker daemons, custom runtimes, GPUs, host networking, and host mounts: unsupported in this slice.

## Contract test

[`TestDockerAgentIsolationIntegration`](../../internal/agent/isolation_integration_test.go) builds a temporary scratch image and validates the boundary from inside the container, then checks that no owned container remains after normal completion or cancellation.
