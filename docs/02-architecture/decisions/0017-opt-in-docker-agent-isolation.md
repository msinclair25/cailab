---
title: "ADR-0017: Opt-In Docker Agent Isolation"
status: accepted
date: 2026-07-12
---

# ADR-0017: Opt-in Docker agent isolation

## Context

The direct subprocess adapter deliberately provides lifecycle ownership without containment. That mode is useful for trusted local agents, but it cannot safely evaluate adversarial or prompt-injected code that retains the launching user's filesystem and network authority. CloudAILab needs an enforceable local boundary before describing any agent run as isolated.

The default project remains local-first and must work without a hosted sandbox account. A hosted sandbox service would add deployment, credentials, cost, and an outbound trust boundary. Native per-OS sandboxes would require three different security models and still leave uneven network, filesystem, descendant, and cleanup behavior. Docker is already an accepted optional local dependency for AWS scenarios and exposes the required namespace, filesystem, privilege, and resource controls through one testable command contract.

## Decision

1. Keep the existing host subprocess mode as the default and continue to label it unisolated.
2. Add an opt-in Docker agent mode. It runs the protocol-compatible agent inside a user-selected image referenced only by a local content-addressed image ID or a repository digest. Tags and mutable names are rejected, and CloudAILab does not pull implicitly. The active Docker context must resolve to an absolute local Unix socket backed by a Linux engine; CloudAILab pins that endpoint with `--host` and rejects TCP, SSH, named-pipe, remote, and non-Linux contexts. The selected image must already exist and must not declare Dockerfile `VOLUME` paths, which would otherwise create writable storage outside the read-only root.
3. CloudAILab forwards no host environment variables, bind mounts, devices, sockets, secrets, or provider endpoints. Image-defined environment remains part of the pinned agent artifact. The only supported controller path is attached protocol JSON Lines over standard input and output, and the container logging driver is `none` so Docker does not retain that stream for `docker logs`.
4. The container runs with `--network none`, so only its private loopback device exists. It cannot directly reach the host, provider facades, Floci, a hosted model, or the public internet.
5. The root filesystem is read-only, image-declared volumes are prohibited, and IPC mode `none` prevents `/dev/shm` from being mounted. A 64 MiB `/tmp` tmpfs is the only application-writable path and is mounted `noexec`, `nosuid`, and `nodev` for UID/GID 65532.
6. The process runs as UID/GID 65532 with every Linux capability dropped, `no-new-privileges`, and Docker's built-in seccomp profile selected explicitly. The container is not privileged and receives no additional groups.
7. The container is limited to 512 MiB memory with no additional swap, one CPU, and 128 PIDs. Rootless engines and engines without an active cgroup driver are rejected because Docker documents configurations in which those limits can be ignored. Failure to apply a requested control fails startup rather than silently downgrading to host execution.
8. Every container has deterministic CloudAILab run/trial labels and a deterministic name. Cleanup inspects both labels before forced removal; a mismatch is an error. Cleanup runs after success, failure, timeout, or cancellation with a separate bounded context.
9. Immutable agent-run metadata records profile `docker-strict-v1`, the Docker engine, exact image identifier, network boundary, and filesystem boundary. A future control change requires a new profile value. Host-mode historical records remain valid without that optional field.
10. This mode makes a bounded claim only: the agent has a Docker-enforced private process/filesystem/network boundary under the configured local Docker engine. It is not a VM, does not defend against Docker daemon or kernel compromise, and does not isolate user-registered tool subprocesses. Docker daemon access remains host-equivalent authority for CloudAILab itself.

## Consequences

### Positive

- An agent can interact with the deterministic gateway without ambient access to host files, local provider ports, or the public internet.
- Digest/image-ID pinning and immutable run metadata identify the exact agent artifact evaluated.
- One container contract is testable on Linux CI and usable through Docker Desktop on supported desktop hosts.
- Resource and descendant-process ownership move under a container boundary with explicit cleanup.

### Negative

- The isolated image must contain the agent and its prompt/model assets; there are no host mounts or forwarded API keys.
- Hosted-model agents cannot operate with `--network none`; they must remain in accurately labeled host mode until a narrower egress design is accepted.
- User-provided tool adapters still execute as trusted, bounded host subprocesses.
- Docker availability and the selected image's platform determine whether a particular isolated run can start.
- Digest pinning identifies bytes but does not establish that an image is trustworthy or vulnerability-free.

## Validation

- Unit tests verify every Docker flag, strict image references, absent mounts/environment, immutable run metadata, and label-checked cleanup behavior.
- Session tests preserve protocol, cancellation, timeout, diagnostics, and response behavior through the container command.
- A Linux Docker integration builds a local scratch image containing an adversarial probe and proves: host sentinel files are absent, external networking fails, root writes fail, bounded `/tmp` writes succeed, the process is non-root, and no labeled container remains after completion or cancellation.
- Public CLI tests cover invalid/mutable image rejection, forbidden environment forwarding, evidence-safe rendering, and isolated-run metadata.

## Sources

- [Docker: Running containers](https://docs.docker.com/engine/containers/run/)
- [Docker: None network driver](https://docs.docker.com/engine/network/drivers/none/)
- [Docker: Resource constraints](https://docs.docker.com/engine/containers/resource_constraints/)
- [Docker: tmpfs mounts](https://docs.docker.com/engine/storage/tmpfs/)
- [Docker: Seccomp security profiles](https://docs.docker.com/engine/security/seccomp/)
- [Docker: Pull an image by digest](https://docs.docker.com/reference/cli/docker/image/pull/)
- [Docker Engine security](https://docs.docker.com/engine/security/)
- [ADR-0012: Owned agent subprocess sessions](0012-owned-agent-subprocess-sessions.md)
- [ADR-0015: Scenario-bound public agent runs](0015-scenario-bound-public-agent-runs.md)
