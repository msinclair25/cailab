# Data-only scenario starter

This directory is the supported starting point for a custom CloudAILab scenario. The manifest is inert declarative data: it has no provider runtime, provider facade, evaluation fixture, executable hook, shell text, plugin, credential, listener, or download.

It models two local tenants, one approved cross-tenant trust path, one denied principal, and two deterministic invariants. It does not claim compatibility with AWS, Microsoft, Google, OIDC, agent campaigns, provider mutation, or state restoration.

## Author safely

1. Copy this directory to a new working directory outside `scenarios/`. Repository-owned scenarios become part of the embedded built-in catalog; custom work should remain explicit until reviewed.
2. Change `metadata.name`, `metadata.version`, `metadata.title`, and `spec.seed`. Keep IDs stable after sharing the scenario; use descriptive namespaces such as `principal:`, `group:`, `workload:`, `resource:`, and relationship-purpose prefixes.
3. Write `spec.briefing` and `spec.objectives` for the learner. Do not disclose the expected answer there.
4. Declare tenants, principals, resources, and typed relationships. Every reference must resolve, and cross-tenant authority must be represented by visible edges.
5. Define verification invariants separately from the learner briefing. Verification is protected from normal mission output, not confidential from someone who can read the local manifest or executable.
6. Validate before starting a run:

   ```bash
   cailab scenario validate ./scenario.yaml
   ```

7. Exercise the complete no-runtime lifecycle from the copied directory:

   ```bash
   cailab up --state-dir ./.cailab-state ./scenario.yaml
   cailab mission --state-dir ./.cailab-state
   cailab graph path --state-dir ./.cailab-state principal:analyst resource:case-file
   cailab verify --state-dir ./.cailab-state
   cailab down --state-dir ./.cailab-state
   ```

   The supplied starter reports two passing invariants. `down` must remove the active run; delete the local `.cailab-state` directory only after `down` succeeds if you no longer need its evidence.

## Safety boundary

Unknown fields are rejected. In particular, fields such as `hooks`, `command`, and arbitrary runtime engines are not part of the scenario contract. Do not add `runtimes` or `providers` by copying a provider scenario unless the contribution targets a documented compatibility contract and includes its provider contract tests, negative authorization tests, lifecycle coverage, diagnostics, and cleanup evidence. Dynamic provider loading, arbitrary plugins, and scenario-selected subprocesses are not supported.

All identities, credentials, endpoints, and data must be synthetic. Never place real secrets, personal data, production URLs, or cloud credentials in a scenario.

## Contribution checklist

- Preserve the versioned schema and stable requirement/lesson identifiers.
- Keep the learner briefing distinct from deterministic ground truth.
- Document intended weaknesses, remediation, supported operations, fidelity limits, prerequisites, diagnostics, and cleanup in a focused guide.
- Add unit tests for graph and invariant semantics; add explicit-deny and tenant-boundary negatives when authorization behavior changes.
- Add provider-shaped contract and lifecycle tests only for operations the scenario actually uses.
- Update the scenario specification, compatibility evidence, threat model, learning catalog, and release bundle when those contracts change.
- Run the checks in [CONTRIBUTING.md](../../CONTRIBUTING.md) before opening a pull request.

The normative authoring rules and schema are in the [scenario specification](../../docs/04-scenarios/scenario-specification.md) and [scenario schema](../../schemas/scenario/v1alpha1.json). The exact tested surface and exclusions are in the [data-only authoring compatibility record](../../docs/07-compatibility/data-only-scenario-authoring.md).
