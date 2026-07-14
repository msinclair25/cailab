# Changelog

All notable changes to CloudAILab will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases use [Semantic Versioning](https://semver.org/spec/v2.0.0.html). No public version has been released yet; the entries below describe the current first-release candidate scope.

## [Unreleased]

### Added

- Deterministic scenario compilation, canonical identity graph, SQLite state, lifecycle commands, path explanation, and invariant verification.
- Scenario-scoped AWS IAM/STS/S3 support through pinned Floci 1.5.32 with documented authorization gaps.
- Native loopback Microsoft Graph-shaped and Google Workspace-shaped facades for the tested scenario operations.
- Local development OIDC issuer and CloudAILab-authorized cross-provider federation gateway.
- Flagship Google → Microsoft → local OIDC → AWS acquisition scenario with supported remediation and intended-access preservation.
- Versioned agent, policy, tool, approval, outcome, evidence, and replay contracts.
- Deterministic reference, fixture-specific safe, deliberately unsafe, and custom subprocess agent workflows.
- Optional Linux CI-tested Docker isolation for custom agents, with explicit host/tool subprocess limitations.
- Repeated restored campaigns and fixture-labeled prompt-injection evaluation.
- CGO-free release archives for the declared Linux, macOS, and Windows target matrix; checksums, SPDX JSON SBOM, native smoke tests, and tag-only build/SBOM attestations.
- Embedded built-in scenario catalog and digest-pinned CI-only clean-container demo.
- Full-SHA-pinned CodeQL analysis and pull-request dependency review workflows.
- A guided no-Docker `quickstart` that deploys, explains, verifies, and cleans up the embedded walking-skeleton scenario.
- Consistent success-returning help for public CLI commands and a versioned, secret-minimized JSON status contract for automation.
- Release-package Markdown link validation and exact-source-commit rewriting for documentation that is not bundled in an archive.
- A release-packaged, dependency-free external-agent starter with a facade-backed governed tool, generated scenario-bound registrations, expected evidence, and flagship end-to-end coverage.
- A versioned, schema- and semantics-validated data-only learning catalog plus the self-guided Identity and Agent Foundations path and one-time adaptation provenance.
- A public validate-without-starting command and release-packaged data-only scenario starter with strict capability rejection, lifecycle coverage, authoring guidance, and compatibility evidence.
- Timestamp-free JUnit output for deterministic scenario invariants and a release-packaged least-privilege GitHub Actions example using only synthetic local state.
- First-user acceptance protocol plus resolved clean-source and packaged external-agent path friction found during a complete maintainer archive rehearsal.

### Security

- Loopback defaults, synthetic credentials, strict scenario decoding, runtime ownership checks, explicit-deny policy semantics, bounded subprocess lifecycle, evidence integrity chains, and release supply-chain controls.
- Private vulnerability reporting, Dependabot vulnerability alerts/security updates, secret scanning, and push protection enabled for the public repository.

### Known limitations

- Provider compatibility is operation-specific and training-focused; the project does not claim general cloud-provider parity.
- Host-mode agents and registered tool subprocesses are not isolated.
- The local OIDC profile is intentionally not a production OpenID Provider.
- Linux ARM64 release artifacts are cross-compiled but are not yet executed on a native hosted runner.
- Docker is the only tested container runtime; Podman support remains planned.
- Optional AI coaching is not implemented in this candidate.
- No public version tag has been created.
