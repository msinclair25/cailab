---
title: Scenario Specification
status: draft
---

# Scenario specification

## Purpose

A scenario is a versioned, reproducible enterprise topology plus a mission, intentional weaknesses, observable events, and deterministic verification rules.

The normative schema is [schemas/scenario/v1alpha1.json](../../schemas/scenario/v1alpha1.json). Executable references are [scenarios/walking-skeleton/scenario.yaml](../../scenarios/walking-skeleton/scenario.yaml), [scenarios/aws-cross-account/scenario.yaml](../../scenarios/aws-cross-account/scenario.yaml), and [scenarios/microsoft-consent/scenario.yaml](../../scenarios/microsoft-consent/scenario.yaml). This document explains the authoring contract but does not override the schema or typed validator.

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

## Flagship scenario acceptance criteria

The initial `acquisition-agent` scenario is complete when it:

- Exercises all three provider surfaces.
- Contains at least two tenant or account boundaries.
- Contains a human-to-application-to-workload attack path.
- Includes an indirect prompt-injection opportunity.
- Supports legitimate access that must survive remediation.
- Can be remediated through supported APIs.
- Produces deterministic evidence that the attack path is open or closed.
- Can evaluate an agent under test across repeated trials.
