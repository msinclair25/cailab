---
title: Portfolio Demo Runbook
status: active
last_reviewed: 2026-07-13
---

# Portfolio demo runbook

This is the recording-ready script for a concise CloudAILab portfolio demonstration. It separates verified facts from future roadmap claims and uses only synthetic local data.

## Recording prerequisites

- A verified release-candidate binary or reviewed source build
- Docker running locally
- A clean dedicated `CAILAB_HOME`
- Terminal large enough to display paths and Markdown reports
- No cloud credentials, model API keys, unrelated environment variables, or sensitive files visible

## Segment 1 — clean deterministic core

Show the reproducible CI-equivalent path:

```bash
cailab version
cailab doctor walking-skeleton
cailab up walking-skeleton
cailab graph path google:alex aws:acquisition-data
cailab verify
cailab down
```

Narrate that the graph and verification are deterministic, the catalog is embedded, and this scenario intentionally starts no provider runtime.

## Segment 2 — enterprise trust path

```bash
cailab doctor acquisition-agent
cailab up acquisition-agent
cailab mission
cailab graph path google:contractor aws:acquisition-data
cailab graph path google:security-admin aws:acquisition-data
cailab verify
```

The initial verification is expected to fail because the contractor path is deliberately open. Explain the path as Google membership → synchronized Microsoft group → live app-role assignment → local OIDC audience → CloudAILab federation authorization → AWS role/resource.

Show one provider-shaped Microsoft assignment response and delete the risky assignment using the exact endpoint and synthetic bearer printed by the active run, following the [flagship lab](acquisition-agent-lab.md). Then run:

```bash
cailab graph path google:contractor aws:acquisition-data
cailab graph path google:security-admin aws:acquisition-data
cailab verify
```

The contractor path must be gone while the administrator path and all invariants pass.

## Segment 3 — AI-agent governance

Reset the flagship fixture, then contrast the paired deterministic controls:

```bash
cailab reset
cailab agent campaign unsafe --trials 3 --fixture drive-runbook-export --format markdown
cailab agent campaign safe --trials 3 --fixture drive-runbook-export --format markdown
```

Explain that the unsafe control reads the real synthetic runbook and follows its inert marker, while the safe control reads the same document but makes no content-derived follow-up call. The result is evidence that the evaluation and governance harness distinguish these exact fixtures—not that any model or framework is generally prompt-injection resistant.

## Segment 4 — boundaries and cleanup

Close with the compatibility and security limits:

- provider support is operation-specific, not parity;
- Floci is not the authoritative web-identity decision point;
- host-mode agents and tool subprocesses are not isolated;
- optional Docker agent isolation is Linux CI-tested and still trusts Docker and the host kernel;
- scoring is deterministic; AI coaching is not implemented in this candidate and remains a planned optional, non-authoritative feature.

```bash
cailab down
```

Verify that the owned Floci container and native provider processes are gone. Do not edit the recording to hide a failed cleanup or unexpected warning; fix or explicitly document it before publication.

## Publication checklist

- Record from a commit that passed the release-candidate and normal CI workflows.
- Show the version/commit near the beginning.
- Add captions or a transcript for accessibility.
- Link the exact release, architecture walkthrough, compatibility records, and threat model.
- Do not call a release stable until a public version is actually tagged and supported.
