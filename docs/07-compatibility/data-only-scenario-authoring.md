---
title: Data-only Scenario Authoring Compatibility
status: active
last_reviewed: 2026-07-13
---

# Data-only scenario authoring compatibility

## Supported profile

The M4.1 authoring profile is the release-packaged [scenario starter](../../examples/scenario-starter/README.md) using `cloudailab.dev/v1alpha1`. The public `cailab scenario validate <file>` command strictly decodes, semantically validates, and deterministically compiles an explicit YAML or JSON file without creating state or starting a runtime.

The tested starter supports:

- stable metadata, objective, tenant, principal, resource, relationship, and invariant IDs;
- provider-neutral canonical graph nodes and typed edges;
- `path_exists` and `path_absent` deterministic invariants;
- explicit custom-file validation and no-runtime `up`, `mission`, `graph path`, `verify`, and `down` lifecycle;
- release packaging and native archive validation on Linux, macOS, and Windows.

## Enforced boundary

The starter selects no `runtimes`, `providers`, or `evaluation` fixture. Strict decoding rejects unknown fields such as `hooks` and `command`; typed validation rejects runtime engines outside the code-owned allowlist. Scenarios are data, not executable plugins, and validation does not download artifacts, open listeners, create state, or run provider code.

The schema still contains reviewed provider-specific sections used by built-in scenarios. Adding one to a custom scenario is outside this starter profile until the author supplies the provider operation contract, authorization negatives, lifecycle/cleanup tests, security review, and compatibility documentation required by the [scenario specification](../04-scenarios/scenario-specification.md).

## Limitations

- Canonical tenants are logical graph boundaries, not operating-system, process, network, or cloud-account isolation.
- The local file and embedded executable are visible to the launching OS account; protected invariants are omitted from normal mission output but are not secret.
- The starter does not emulate AWS, Microsoft, Google, Azure, or any other provider.
- It does not support arbitrary hooks, subprocesses, plugins, dynamic provider loading, agent campaigns, provider mutation, or state restoration.
- Passing the starter invariants proves only the declared graph properties for that compiled fixture.

## Evidence

- `internal/scenario/scenario_test.go` loads and compiles the starter and rejects hook/runtime expansion.
- `internal/cli/cli_test.go` exercises public validation, startup, deterministic verification, shutdown, and inactive-state cleanup.
- `.github/workflows/ci.yml` validates the repository starter on Linux, macOS, and Windows.
- `.github/workflows/release.yml` verifies and validates the exact starter included in every native smoke archive.
