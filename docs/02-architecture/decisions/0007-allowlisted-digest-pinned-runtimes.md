---
title: "ADR-0007: Allowlisted and Digest-Pinned Provider Runtimes"
status: accepted
date: 2026-07-12
---

# ADR-0007: Allowlisted and digest-pinned provider runtimes

## Context

Provider-backed scenarios require CloudAILab to pull and execute containers. Accepting an arbitrary image reference from a scenario would turn a declarative training file into an implicit host-code execution mechanism. A mutable tag alone also cannot establish which bytes were evaluated or later reproduced.

## Decision

The scenario schema accepts only provider images explicitly allowlisted by the current CloudAILab release and pinned to an OCI digest. M1 accepts exactly Floci `1.5.32` at multi-architecture digest `sha256:4f69631e560120d79ad82d2af9f7dda8c6ef7ecbbae0c43ddcffa109c6588a15`.

The Docker adapter additionally:

- publishes the provider endpoint on a random IPv4 loopback port;
- runs Floci directly as its unprivileged UID `1001`;
- drops all Linux capabilities and enables `no-new-privileges`;
- applies CPU and memory limits;
- does not mount the Docker socket for the IAM/STS/S3 scenario;
- labels ownership and run identity before startup;
- verifies the run label before removal.

Adding or upgrading an image requires a code and schema change, primary-source and license review, a digest update, integration tests, compatibility review, and threat-model review. Arbitrary custom images require a separate explicit-consent design and are not supported by M1.

## Consequences

### Positive

- A scenario cannot silently select a different or attacker-controlled image.
- Runtime bytes are reproducible across supported architectures.
- Default host exposure and container privileges are reduced.
- Cleanup can distinguish CAL resources from unrelated containers.

### Negative

- Runtime upgrades require a CloudAILab release rather than a manifest-only edit.
- Users cannot substitute a private fork in the default workflow.
- Digest availability still depends on the upstream registry unless the image is already cached.
- Digest pinning does not replace provenance verification or vulnerability review.

## Validation

- Scenario tests reject mutable or non-allowlisted references.
- Runtime unit tests assert loopback publishing, non-root execution, dropped capabilities, labels, and ownership checks.
- The Docker integration test pulls the accepted digest, hydrates IAM/S3, exercises STS, applies remediation, and removes the container.
- A post-run leak check finds no container carrying CloudAILab's managed label.

## Sources

- [Floci 1.5.32 release](https://github.com/floci-io/floci/releases/tag/1.5.32)
- [Floci installation](https://floci.io/floci/getting-started/installation/)
- [Floci MIT license](https://github.com/floci-io/floci/blob/main/LICENSE)
