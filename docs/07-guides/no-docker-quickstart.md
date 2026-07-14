---
title: No-Docker Quick Start
status: active
last_reviewed: 2026-07-13
---

# No-Docker quick start

The guided quick start proves that the installed CloudAILab binary can load its embedded catalog, create isolated local state, compile a provider-neutral identity graph, explain a trust path, verify a deterministic invariant, and cleanly stop the run. It requires no Docker, cloud account, hosted model, network proxy, or certificate installation.

## Run it

```bash
cailab quickstart
```

Use a dedicated state location when desired:

```bash
cailab quickstart --state-dir .cloudailab/first-run
```

The command refuses to replace an active run in the selected state directory. Stop that run normally or choose a different directory.

## Visible lifecycle

The output identifies each operation instead of hiding orchestration:

1. Check the embedded `walking-skeleton` prerequisites and confirm Docker is not required.
2. Validate, compile, and start the scenario through the normal application service.
3. show the mission and explain `google:alex → aws:acquisition-data` through the canonical graph.
4. Run the normal deterministic verifier and require a passing invariant.
5. Stop the owned run through the normal cleanup contract.

The equivalent manual workflow is:

```bash
cailab doctor walking-skeleton
cailab up walking-skeleton
cailab mission
cailab graph path google:alex aws:acquisition-data
cailab verify
cailab down
```

The guided command attempts bounded cleanup if a step fails after startup. A cleanup failure is returned together with the original failure rather than being hidden. The stopped run and verification evidence remain in the selected SQLite state directory; CloudAILab does not silently delete that user-owned directory.

## Claim boundary

This first success proves the binary, embedded scenario, canonical graph, state store, verifier, and cleanup path. It does not start or emulate AWS, Microsoft, Google, or OIDC provider services and does not evaluate an AI agent. Continue with the provider labs only after this local control-plane path succeeds.

The command is exercised by unit tests and the Linux, macOS, and Windows CI build matrix. A clean maintainer archive rehearsal completed in 0.07 seconds without Go or Docker; that is machine evidence, not usability observation. The ten-minute unfamiliar-user target remains pending the [documented RC2 walkthrough](../05-engineering/first-user-acceptance.md).
