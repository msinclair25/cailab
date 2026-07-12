---
title: Scenario Specification
status: draft
---

# Scenario specification

## Purpose

A scenario is a versioned, reproducible enterprise topology plus a mission, intentional weaknesses, observable events, and deterministic verification rules.

The normative schema is [schemas/scenario/v1alpha1.json](../../schemas/scenario/v1alpha1.json). Executable references are [scenarios/walking-skeleton/scenario.yaml](../../scenarios/walking-skeleton/scenario.yaml), [scenarios/aws-cross-account/scenario.yaml](../../scenarios/aws-cross-account/scenario.yaml), [scenarios/microsoft-consent/scenario.yaml](../../scenarios/microsoft-consent/scenario.yaml), [scenarios/google-drive-sharing/scenario.yaml](../../scenarios/google-drive-sharing/scenario.yaml), [scenarios/local-oidc/scenario.yaml](../../scenarios/local-oidc/scenario.yaml), and the [acquisition-agent flagship](../../scenarios/acquisition-agent/scenario.yaml). This document explains the authoring contract but does not override the schema or typed validator.

## Required sections

1. Metadata and schema version
2. Learning objectives
3. Public mission briefing
4. Tenants and provider accounts
5. Principals and group membership
6. Applications and workloads represented as principals
7. Resources and data classification
8. Policies and cross-provider trust
9. Intentional weaknesses
10. Initial and triggered events
11. Public success criteria
12. Protected invariants and evidence queries

## Minimal manifest

This abbreviated example follows the committed schema. See the reference scenarios for complete files.

```yaml
apiVersion: cloudailab.dev/v1alpha1
kind: Scenario
metadata:
  name: path-example
  version: 0.1.0
  title: Path Example
spec:
  seed: 42
  briefing: Trace the expected learning path.
  objectives:
    - id: trace-path
      description: Trace a cross-provider identity path.
  tenants:
    - id: northstar
      name: Northstar Research
      providers: [google, microsoft, aws]
  principals:
    - id: google:alex
      tenant: northstar
      type: human
      displayName: Alex Contractor
  resources:
    - id: aws:acquisition-data
      tenant: northstar
      type: s3_bucket
      displayName: Acquisition Data
      classification: restricted
  relationships:
    - id: access:alex-acquisition-data
      from: google:alex
      to: aws:acquisition-data
      type: can_access
      actions: [s3:GetObject]
  verification:
    invariants:
      - id: learning-path-visible
        type: path_exists
        from: google:alex
        to: aws:acquisition-data
        severity: medium
        description: The expected learning path is visible.
```

## Authoring rules

- Manifests are declarative and cannot contain executable shell fragments.
- Every generated collection is controlled by an explicit seed.
- Provider-specific extensions are namespaced.
- Intentional weaknesses include learning rationale and expected remediation classes.
- Verification states the required outcome, not one mandatory implementation.
- Hidden ground truth is separable from the learner-visible scenario package.
- Unknown fields are rejected by the decoder.
- IDs are unique across tenants, principals, and resources.
- Provider runtimes must be code-allowlisted and pinned to the exact digest accepted by the schema.
- AWS provider topology uses typed accounts, roles, inline policies, buckets, and synthetic objects; scenario-provided shell hooks are not supported.
- AWS account principal references and role/bucket nodes must resolve to canonical principals and resources.
- Microsoft directory object IDs use GUID syntax and map explicitly to canonical principal or resource nodes.
- Microsoft delegated grants must reference a declared user, client service principal, and resource service principal. The current schema accepts only principal-specific consent.
- Microsoft app-role assignments must reference a declared group, a node-backed service principal, and an app role declared on that service principal.
- Google user, group, member, file, and permission IDs use the provider's scoped identifier syntax and map explicitly to canonical nodes.
- Google member email addresses must reference declared users; Drive user/group permissions must reference the corresponding declared principal and file.
- The current Google slice accepts direct `USER` group members and direct `user` or `group` Drive permissions with `reader`, `commenter`, or `writer` roles.
- Local issuer clients map to canonical application, workload, or agent principals; subjects map to declared human, workload, or agent principals; group claims reference canonical groups.
- Local issuer redirect URIs are IPv4 loopback HTTP URLs with explicit ports. Each client has one canonical audience resource in the current profile.
- Code and token lifetimes are bounded by the schema. Scenarios cannot select algorithms, keys, listeners, arbitrary claims, or external redirect origins.
- AWS web-identity trust binds one canonical local client, audience resource, and exact audience value to a declared role. It is an authoritative CloudAILab contract because the pinned emulator does not validate that trust.

## Flagship scenario acceptance criteria

The initial `acquisition-agent` scenario is complete when it:

- Exercises all three provider surfaces.
- Contains at least two tenant or account boundaries.
- Contains a human-to-application-to-workload attack path.
- Includes an indirect prompt-injection opportunity.
- Supports legitimate access that must survive remediation.
- Can be remediated through supported APIs.
- Produces deterministic evidence that the attack path is open or closed.
- Provides the provider and identity ecosystem that the M3 governed agent harness will evaluate across repeated trials.
