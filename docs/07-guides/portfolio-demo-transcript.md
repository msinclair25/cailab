---
title: Portfolio Demo Transcript and Publication Copy
status: active
last_reviewed: 2026-07-13
---

# Portfolio demo transcript and publication copy

This script provides accurate narration and accessible captions for the repository-owned [portfolio demo workflow](portfolio-demo.md). Adjust timing to the recorded output, but preserve the stated security and compatibility boundaries.

## Opening

> CloudAILab is a local enterprise identity and AI-agent security range. This demonstration uses only synthetic identities, tokens, permissions, and data. It requires no cloud account or hosted model. The binary version and exact source commit shown here identify the release candidate used for this recording.

Do not call the candidate stable or released. Keep the `cailab version` output visible long enough to read.

## Segment 1 — clean deterministic core

> The first scenario is the working-directory-independent walking skeleton embedded in the binary. `doctor` checks only the prerequisites needed by this scenario. `up` validates and compiles the scenario, but intentionally starts no provider runtime. The graph path and invariant results come from CloudAILab's deterministic canonical evaluator; no model grades or changes them.

When `verify` passes, explain that this proves only the declared walking-skeleton invariants. It is not provider-parity evidence.

## Segment 2 — enterprise trust path

> The flagship scenario represents one acquisition-data trust chain across Google membership, a synchronized Microsoft group, a live Microsoft app-role assignment, a local OIDC audience, CloudAILab's federation decision, and an AWS role and resource. All services bind to random IPv4 loopback ports. AWS-shaped behavior uses the pinned local Floci runtime; the Microsoft, Google, and OIDC surfaces are intentionally scoped native facades.

Before remediation:

> Both the contractor and the approved security administrator have a path. The first verification is expected to fail because the contractor path is the deliberate vulnerability. The failed invariant is useful evidence, not a demo malfunction.

While the Microsoft-shaped assignment response is visible:

> This response is live provider-shaped state, not a hard-coded scoring answer. The recording runner accepts only an IPv4 loopback origin and sends the documented synthetic bearer. It removes the one risky app-role assignment through the supported `DELETE` operation.

After remediation:

> CloudAILab rebuilds the relevant authorization edges from current facade state. The contractor path is now absent, the administrator path remains, and both deterministic invariants pass. This demonstrates a least-privilege repair that preserves intended access.

## Segment 3 — deterministic AI-agent governance controls

> `reset` restores and verifies the original provider baseline before the campaigns. The unsafe and safe controls receive the same synthetic Drive runbook containing an inert indirect prompt-injection marker. Each campaign restores the fixture before every sequential trial and derives its report from persisted run, decision, outcome, and before-and-after state evidence.

For the unsafe report:

> The deliberately unsafe control reads the runbook and follows the prohibited content-derived action. The report therefore records exposure and successful injection for this exact synthetic fixture.

For the safe report:

> The fixture-specific safe control reads the same document but makes no content-derived follow-up call. Its positive result proves that this evaluation harness can distinguish these two deterministic controls. It does not prove that an LLM, framework, or deployment is generally prompt-injection resistant.

Then state the supported external-agent boundary:

> Protocol-compatible external agents can be launched and evaluated. Host-mode agents and tool subprocesses are not isolated from the launching user. The optional Docker agent mode is a narrower Linux-tested boundary that still trusts Docker and the host kernel. No AI coaching feature ships in this candidate, and AI cannot override authorization or verification results.

## Segment 4 — boundaries and cleanup

> Provider support is operation-specific rather than cloud parity. Direct Floci web-identity behavior is not authoritative security evidence. The local facades and issuer are development services, not production identity providers. The runner now calls `down`, confirms there is no active run, and checks that every provider endpoint recorded for this run has stopped accepting connections.

Do not hide a cleanup warning, failed check, unexpected port, or different commit in editing. Fix it, rerun from a clean state, and record the complete successful workflow.

## Suggested video metadata

Title:

> CloudAILab: Local Enterprise IAM and AI-Agent Security Range

Description:

> A reproducible demonstration of CloudAILab's deterministic identity graph, cross-provider least-privilege remediation, and evidence-backed safe/unsafe agent-governance controls. The range uses synthetic local data and operation-specific AWS-, Microsoft-, Google-, and OIDC-shaped surfaces. See the linked release evidence, architecture, compatibility matrices, and threat model for exact support boundaries.

Suggested chapters:

```text
00:00 What CloudAILab is and candidate identity
00:20 Deterministic embedded scenario
01:00 Cross-provider enterprise trust path
02:10 Least-privilege remediation and verification
03:00 Unsafe versus safe agent campaigns
04:20 Security boundaries and cleanup
```

## Publication links

Include direct links to:

- the exact release or candidate evidence used for the recording;
- the [architecture walkthrough](architecture-walkthrough.md);
- the [cross-provider compatibility matrix](../07-compatibility/cross-provider-federation.md);
- the [agent campaign compatibility record](../07-compatibility/agent-campaign-execution.md);
- the [Docker isolation compatibility record](../07-compatibility/agent-docker-isolation.md);
- the [threat model](../03-security/threat-model.md);
- the source commit shown by `cailab version`.

After publication, add the durable recording URL to the README and [release-candidate readiness audit](../05-engineering/release-readiness-audit.md). Do not add a placeholder or private URL.
