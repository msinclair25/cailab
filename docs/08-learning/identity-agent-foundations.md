---
title: Identity and Agent Foundations Learning Path
status: active
last_reviewed: 2026-07-13
---

# Identity and agent foundations

This is the initial self-guided CloudAILab path. It progresses from a no-Docker trust graph to provider-specific IAM, cross-provider remediation, release evidence, governed agents, and scenario-authoring orientation. It uses only implemented scenarios and workflows.

Estimated total: 8-10 focused hours. Complete it over several sessions; do not treat the sequence as a timed exam.

## Operating rhythm

For every lesson:

1. Read the outcome and safety boundary before running commands.
2. Predict the trust path or verification result.
3. Inspect current state before mutating it.
4. Make the smallest documented change.
5. Run deterministic verification.
6. Confirm intended access and cleanup.
7. Record the production connection and one limitation.

When stuck, use hints in order. Do not jump directly to protected scenario ground truth or edit the state database.

## Stage 1 — See a trust path

Lesson ID: `lesson:first-success`

- Safety: local, no Docker, no cloud account, no hosted model.
- Outcome: explain how a principal reaches a resource through typed edges and distinguish graph evidence from real-provider parity.
- Run: `cailab quickstart`.
- Verify: the guided workflow reports one passing invariant and stops its owned run.
- Production connection: identity investigations begin by making implicit authority visible.

- Hint 1: read the path from left to right as principal, group, workload identity, and resource.
- Hint 2: edge labels explain why traversal is allowed.
- Hint 3: rerun the manual commands in the [no-Docker guide](../07-guides/no-docker-quickstart.md).

## Stage 2 — Compare provider-shaped authorization

Complete these provider labs in order:

1. `lesson:google-sharing` — [Google Drive sharing](../07-guides/google-drive-sharing-lab.md)
2. `lesson:microsoft-consent` — [Microsoft delegated consent](../07-guides/microsoft-consent-lab.md)
3. `lesson:local-oidc` — [Local OIDC](../07-guides/local-oidc-lab.md)
4. `lesson:aws-cross-account` — [AWS cross-account IAM](../07-guides/aws-cross-account-lab.md)

The Google, Microsoft, and OIDC labs need no Docker. AWS uses the pinned local Floci container. These are separate products and planes: Google Workspace is not Google Cloud, Microsoft 365/Entra directory behavior is not Azure resource management, and Floci compatibility is operation-specific.

For each lab, record:

- the principal and tenant;
- the resource and classification;
- the policy, grant, membership, or token that creates authority;
- the initial failed invariant;
- the smallest remediation;
- the intended path that still passes;
- one emulator/facade limitation.

Failure drill: if verification does not change as expected, stop editing. Re-inspect live provider-shaped state, confirm the exact ID you changed, compare it with the graph path, and use `cailab down` before restarting from a clean fixture.

## Stage 3 — Remediate the enterprise chain

Lesson ID: `lesson:cross-provider-remediation`

- Safety: local Docker for Floci plus loopback native facades; all identities and data are synthetic.
- Outcome: trace Google group membership through Microsoft application assignment and local OIDC into an AWS role, then close only the contractor path.
- Guide: [acquisition-agent flagship](../07-guides/acquisition-agent-lab.md).
- Verify: `cailab verify` reports both the risky path removed and the approved administrator path preserved.
- Production connection: cross-provider incidents require current state from every authority plane, not one vendor console.

Progressive hints:

1. Identify the first live provider edge shared by the risky path but not the approved path.
2. Compare the two Microsoft app-role assignments rather than changing the AWS role.
3. Remove only assignment `99999999-9999-4999-8999-999999999999`, then verify both invariants.

## Stage 4 — Verify delivery evidence

Lesson ID: `lesson:release-integrity`

- Safety: repository-local source checks; no publication or credentials.
- Outcome: distinguish checksum integrity, provenance, SBOM inventory, native smoke evidence, and human review.
- Guide: [release verification](../07-guides/release-verification.md).
- Verify from source: `go test ./internal/tools/release` and compare `go run ./internal/tools/release modules` with `third_party/modules.txt`.
- CI connection: inspect the [least-privilege workflow](../../examples/ci/README.md) and explain why invariant JUnit is a valid gate while multidimensional agent evidence has no universal pass/fail verdict.
- Production connection: an artifact is not trustworthy merely because a vulnerability scan passed.

Reflection: Which signal proves bytes were unchanged? Which signal records build origin? What do neither of them prove?

## Stage 5 — Run a governed external agent

Lesson ID: `lesson:agent-governance`

- Safety: the agent and registered tool run as trusted, unisolated host subprocesses; Docker isolation covers only a self-contained agent and not its registered tools.
- Outcome: configure, validate, launch, and replay the packaged [external-agent starter](../../examples/external-agent-starter/README.md).
- Verify: replay shows one completed trial, one allowed governed action, one successful tool execution, restored initial state, and zero scenario task success.
- Production connection: process completion is not task success, and a tool schema is not authorization.

Failure drill: remove the exact allow rule or change the canonical resource in a temporary copy of the policy. Confirm the call is denied and the provider-backed tool does not execute. Restore the original generated configuration rather than weakening the manifest ceiling.

## Stage 6 — Author a safe data-only scenario

Lesson ID: `lesson:scenario-authoring-orientation`

- Safety: local no-Docker state with synthetic data; scenario data cannot select executable hooks.
- Outcome: adapt the [data-only starter](../../examples/scenario-starter/README.md), identify learner briefing, canonical graph, protected evaluation rules, cleanup expectations, and compatibility claims, then exercise its complete lifecycle.
- Read: [scenario starter](../../examples/scenario-starter/README.md) and [scenario specification](../04-scenarios/scenario-specification.md).
- Verify from source: `cailab scenario validate examples/scenario-starter/scenario.yaml`.
- Production connection: extensibility starts with constrained data contracts; executable integrations require a separate trust decision.

## Completion reflection

Write a short evidence-based summary:

- Which trust boundary was hardest to reason about?
- Which remediation closed risky access without breaking intended access?
- Which provider behaviors were simulated rather than parity claims?
- What did deterministic verification prove?
- What remained unmeasured in the agent report?
- Which cleanup evidence did you observe?
- How would you explain one failure and prevention control to another engineer?

This reflection is a learning artifact, not proof of identity, a credential, a score, or an employment recommendation.
