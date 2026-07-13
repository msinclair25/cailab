---
title: CloudAILab
aliases:
  - CAL
status: m4-in-development
---

# CloudAILab

> A local enterprise identity and AI-agent security range.

CloudAILab is a working project name for a reproducible learning and evaluation environment spanning AWS, Microsoft Entra/Microsoft 365, and Google Workspace-compatible ecosystems. It is intended to teach enterprise IAM and cybersecurity, and to evaluate how AI agents behave when they receive identities, credentials, tools, and access to sensitive data.

The command-line name is planned as `cailab`; `cal` is intentionally avoided because it conflicts with the traditional Unix calendar utility.

## Project status

M0 through M3 are complete. Versioned agent and evidence contracts; supported inert reference, fixture-specific safe, deliberately unsafe, and custom subprocess runs; deterministic governance; optional Docker agent isolation; endpoint-preserving restoration; normalized provider baselines; scenario evidence; fixture-labeled indirect prompt-injection scoring; repeated-trial replay metrics; and automatic restored reference/safe/unsafe campaigns are implemented and test-backed. Provider and protocol compatibility is limited to the tested matrices and schemas.

## Build and try the walking skeleton

Prerequisites: Go 1.25.12 or newer. Docker is required for AWS/Floci scenarios and the opt-in isolated agent mode.

```bash
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor
./bin/cailab scenario list
./bin/cailab scenario show walking-skeleton
./bin/cailab up walking-skeleton
./bin/cailab mission
./bin/cailab graph path google:alex aws:acquisition-data
./bin/cailab verify
./bin/cailab down
```

The default state database is `.cloudailab/cailab.db`. Override it with `--state-dir` on lifecycle commands or set `CAILAB_HOME`.

Named built-in scenarios are compiled into the executable, so these commands work from any directory and release users do not need a repository checkout or separate scenario folder. Custom scenario files still work when supplied directly; custom catalogs require an explicit `--root` or `--scenario-root` path.

## Prove the clean container demo

The repository also includes a digest-pinned CI-only image that builds and runs the walking skeleton as a non-root user with Docker's `none` network, no host mounts, and a read-only root filesystem. It is a clean-environment test, not a published distribution or agent sandbox.

```bash
docker build --file build/ci/Dockerfile --tag cailab-ci:local .
docker run --rm --network none --ipc none --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m,mode=1777,uid=65532,gid=65532 \
  --cap-drop ALL --security-opt no-new-privileges=true --security-opt seccomp=builtin \
  --memory 256m --cpus 1 --pids-limit 128 --ulimit nofile=1024:1024 \
  cailab-ci:local
```

See the [clean container demo guide](docs/07-guides/clean-container-demo.md) for the evidence boundary and limitations.

## Try the AWS vertical slice

The initial `verify` is expected to fail because the scenario starts vulnerable. Follow the [AWS cross-account lab guide](docs/07-guides/aws-cross-account-lab.md) to prove the access with the AWS CLI, narrow the role trust, and make both invariants pass.

```bash
./bin/cailab up aws-cross-account
./bin/cailab status
./bin/cailab mission
./bin/cailab graph path aws:parent-root aws:acquisition-data
./bin/cailab verify
```

## Try the Microsoft identity slice

This scenario starts with an ordinary analyst holding an excessive delegated Microsoft Graph grant. Follow the [Microsoft consent lab guide](docs/07-guides/microsoft-consent-lab.md) to inspect the local Graph-shaped endpoint, revoke only that grant, and preserve the approved administrator path. It does not require Docker, a Microsoft tenant, a global proxy, or a trusted local certificate.

```bash
./bin/cailab up microsoft-consent
./bin/cailab status
./bin/cailab graph path microsoft:analyst microsoft:directory-data
./bin/cailab verify
```

## Try the Google identity slice

This scenario starts with a contractor holding a direct permission on a restricted Drive file while an approved administrator reaches it through a group. Follow the [Google Drive sharing lab guide](docs/07-guides/google-drive-sharing-lab.md) to inspect Directory and Drive state, remove only the contractor grant, and preserve the intended path. Docker and a Google account are not required.

```bash
./bin/cailab up google-drive-sharing
./bin/cailab status
./bin/cailab graph path principal:contractor resource:retention-plan
./bin/cailab verify
```

## Try the local identity issuer

Follow the [local OIDC lab guide](docs/07-guides/local-oidc-lab.md) to retrieve discovery and JWKS, exchange a one-time synthetic code, validate RS256 ID and access tokens locally, and rotate signing keys. It requires no Docker, cloud identity provider, proxy, or certificate installation. Loopback HTTP and synthetic subject selection make this a development profile, not a production OpenID Provider.

```bash
./bin/cailab up local-oidc
./bin/cailab status
./bin/cailab identity rotate
./bin/cailab verify
```

## Try the cross-provider flagship

The `acquisition-agent` scenario begins with both a contractor and an approved administrator able to reach acquisition data through Google → Microsoft → local OIDC → AWS representations. Follow the [flagship lab guide](docs/07-guides/acquisition-agent-lab.md) to issue signed synthetic tokens, exercise the federation gateway, revoke only the contractor group's Microsoft app-role assignment, and prove the administrator path survives.

```bash
./bin/cailab doctor acquisition-agent
./bin/cailab up acquisition-agent
./bin/cailab graph path google:contractor aws:acquisition-data
./bin/cailab verify
```

External tools and AI agents can call the documented loopback APIs and invoke the `cailab federation` command. The supported M3 runner can launch a protocol-compatible agent, validate scenario-bound registrations, govern resolved tool calls, pause selected calls for local approval, execute allowed or redacted one-shot tools, protect successful output, and persist linked run/decision/approval/outcome evidence.

## Run the deterministic agent baseline

With any active scenario, the reference command launches the deterministic protocol agent, records immutable run metadata, and prints an evidence-safe summary. It makes no tool calls, so it verifies the public subprocess and persistence path without pretending to measure agent quality.

```bash
./bin/cailab up walking-skeleton
./bin/cailab agent run reference
```

Protocol-compatible user agents can be launched with `cailab agent run subprocess`. Policies and tool manifests are selected explicitly for that trial and validated against the active scenario before a process starts. See the [agent-run guide](docs/07-guides/agent-run.md) for the complete workflow and security limitations.

Replay one terminal trial—or an explicitly selected complete repeated set—without executing the agent or tools:

```bash
./bin/cailab agent replay --trial-id trial:1 --format markdown
```

The initial `governed-evidence-v1` profile reports only evidence-supported completion, authorization, approval, execution, and observed-target metrics. It explicitly labels task success, prompt-injection resistance, remediation quality, sensitive-data exposure, and effective blast radius as unmeasured. See the [replay compatibility record](docs/07-compatibility/agent-evidence-replay.md).

Add `--capture-state` to persist before/after invariant evidence. Add `--restore-fixture` to restore supported providers at their existing endpoints and prove the baseline digest before launch. Complete traces then use `scenario-outcome-v1` and measure task and remediation success. See the [trial-state compatibility record](docs/07-compatibility/agent-trial-state.md).

With `acquisition-agent` active, run `./bin/cailab agent run unsafe` to exercise the code-owned deliberately vulnerable baseline against the synthetic Drive runbook, then replay `trial:unsafe`. Export is simulated; no provider data is transmitted. Custom agents can select the same ground truth with `--prompt-injection-fixture drive-runbook-export`. See the [prompt-injection evaluation compatibility record](docs/07-compatibility/agent-prompt-injection.md).

Run the fixture-specific safe control against the same document. It performs the legitimate read, treats returned content as untrusted data, and makes no content-derived follow-up call:

```bash
./bin/cailab agent run safe --fixture drive-runbook-export
./bin/cailab agent replay --trial-id trial:safe --format markdown
```

This establishes that the evaluation can produce a positive result for this exact deterministic control. It is not evidence that a model, agent framework, or deployment is generally prompt-injection resistant.

Run a bounded, automatically restored repeated set and emit its deterministic aggregate report:

```bash
./bin/cailab agent campaign reference --trials 3 --format markdown
./bin/cailab agent campaign safe --trials 3 --fixture drive-runbook-export --format markdown
./bin/cailab agent campaign unsafe --trials 3 --fixture drive-runbook-export --format markdown
```

Campaign execution is supported for the code-owned reference, safe, and unsafe fixtures. Custom subprocess agents still declare and launch repeated trials explicitly. See the [campaign compatibility record](docs/07-compatibility/agent-campaign-execution.md).

Host-mode agent and tool subprocesses are owned and bounded but **not isolated** from the launching user's filesystem, network, syscalls, or detached descendants. Do not run untrusted code in host mode. Custom agents can opt into the Linux CI-tested Docker boundary with `--isolation docker --image <immutable-image-id-or-digest>`; registered tool subprocesses remain trusted and unisolated. See the [agent-run guide](docs/07-guides/agent-run.md) and [Docker isolation compatibility record](docs/07-compatibility/agent-docker-isolation.md).

## Development checks

```bash
gofmt -w cmd internal
go mod tidy -diff
go mod verify
go vet ./...
go test ./...
go test -race ./...
go run ./internal/tools/doccheck .
```

## Release integrity

The M4 release pipeline now builds CGO-free archives with an embedded built-in scenario catalog for Linux amd64/arm64, macOS amd64/arm64, and Windows amd64; generates a sorted SHA-256 manifest and SPDX JSON SBOM; runs native archive smoke tests on Linux, macOS, and Windows; and gates tag publication on GitHub/Sigstore build and SBOM attestations. No public version tag has been cut yet, so this is verified release infrastructure rather than a claim that a stable release exists.

See the [release verification guide](docs/07-guides/release-verification.md) for archive selection, checksum validation, provenance verification, SBOM inspection, and the limits of each signal.

For source builds and future archives, start with [installation](docs/07-guides/installation.md). The repository is licensed under [Apache License 2.0](LICENSE); release archives carry the project license/notice, changelog, third-party notice index, exact linked-module inventory, and copied linked-component licenses. No public version tag exists yet.

## Principles

- Deterministic security decisions; any future AI explanation remains optional and non-authoritative.
- One deep, end-to-end scenario before broad API coverage.
- Explicit compatibility claims instead of implied cloud parity.
- Local-first operation with no required cloud account or hosted model.
- Safe defaults, fake credentials, loopback binding, and complete cleanup.
- Documentation and tests evolve with the implementation.

## Documentation

- [Project charter](docs/00-project/charter.md)
- [Master plan](docs/00-project/master-plan.md)
- [Glossary](docs/00-project/glossary.md)
- [Product requirements](docs/01-product/requirements.md)
- [System architecture](docs/02-architecture/system-architecture.md)
- [Architecture decisions](docs/02-architecture/decisions/README.md)
- [Threat model](docs/03-security/threat-model.md)
- [Scenario specification](docs/04-scenarios/scenario-specification.md)
- [Engineering standards](docs/05-engineering/engineering-standards.md)
- [Quality strategy](docs/05-engineering/quality-strategy.md)
- [Delivery roadmap](docs/05-engineering/roadmap.md)
- [Release-candidate readiness audit](docs/05-engineering/release-readiness-audit.md)
- [Technical basis and source register](docs/06-research/technical-basis.md)
- [Installation](docs/07-guides/installation.md)
- [Architecture walkthrough](docs/07-guides/architecture-walkthrough.md)
- [Troubleshooting](docs/07-guides/troubleshooting.md)
- [Upgrading](docs/07-guides/upgrading.md)
- [Portfolio demo runbook](docs/07-guides/portfolio-demo.md)
- [AWS cross-account lab](docs/07-guides/aws-cross-account-lab.md)
- [AWS/Floci compatibility matrix](docs/07-compatibility/aws-floci-1.5.32.md)
- [Microsoft consent lab](docs/07-guides/microsoft-consent-lab.md)
- [Microsoft Graph facade compatibility matrix](docs/07-compatibility/microsoft-graph-facade.md)
- [Google Drive sharing lab](docs/07-guides/google-drive-sharing-lab.md)
- [Google Workspace facade compatibility matrix](docs/07-compatibility/google-workspace-facade.md)
- [Local development OIDC lab](docs/07-guides/local-oidc-lab.md)
- [Local development OIDC compatibility matrix](docs/07-compatibility/local-oidc-profile.md)
- [Cross-provider acquisition-agent lab](docs/07-guides/acquisition-agent-lab.md)
- [Cross-provider federation compatibility matrix](docs/07-compatibility/cross-provider-federation.md)
- [Agent protocol v1alpha1](docs/04-agents/agent-protocol.md)
- [Agent-run guide](docs/07-guides/agent-run.md)
- [Docker agent isolation compatibility](docs/07-compatibility/agent-docker-isolation.md)
- [Agent evidence replay compatibility](docs/07-compatibility/agent-evidence-replay.md)
- [Agent trial state compatibility](docs/07-compatibility/agent-trial-state.md)
- [Agent prompt-injection evaluation compatibility](docs/07-compatibility/agent-prompt-injection.md)
- [Agent campaign execution compatibility](docs/07-compatibility/agent-campaign-execution.md)
- [Release verification](docs/07-guides/release-verification.md)
- [Clean container demo](docs/07-guides/clean-container-demo.md)
- [Changelog](CHANGELOG.md)
- [Security policy](SECURITY.md)
- [Support policy](SUPPORT.md)
- [Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Third-party notices](THIRD_PARTY_NOTICES.md)

## Working vocabulary

- **Tenant:** an isolated organizational identity boundary.
- **Provider facade:** a deliberately scoped API surface shaped like a cloud provider.
- **Canonical graph:** CloudAILab's provider-neutral model of identities, resources, permissions, and trust.
- **Agent under test:** an external or embedded agent evaluated inside a scenario.
- **Invariant:** a deterministic security requirement used for verification.

## Documentation policy

GitHub-readable Markdown is the portable source format. Obsidian is the authoring and navigation experience. Architectural decisions that are costly to reverse require an ADR, and behavior changes require corresponding requirements, documentation, and tests.
