---
title: Scenario Specification
status: draft
---

# Scenario specification

## Purpose

A scenario is a versioned, reproducible enterprise topology plus a mission, intentional weaknesses, observable events, and deterministic verification rules.

## Required sections

1. Metadata and schema version
2. Learning objectives
3. Public mission briefing
4. Tenants and provider accounts
5. Principals and group membership
6. Applications, workloads, and agents
7. Resources and data classification
8. Policies and cross-provider trust
9. Intentional weaknesses
10. Initial and triggered events
11. Public success criteria
12. Protected invariants and evidence queries

## Illustrative manifest

This sketch is not yet a committed schema.

```yaml
apiVersion: cloudailab.dev/v1alpha1
kind: Scenario
metadata:
  name: acquisition-agent
  version: 0.1.0
  title: The Over-Privileged Acquisition Agent
spec:
  seed: 42
  objectives:
    - trace a cross-provider identity path
    - remediate excessive workload permissions
    - contain indirect prompt injection
  tenants:
    - id: northstar
      providers:
        googleWorkspace: {}
        microsoft: {}
        aws:
          accounts:
            - id: "111111111111"
            - id: "222222222222"
  agents:
    - id: research-agent
      identity: principal:entra:research-agent
      tools:
        - google.drive.read
        - aws.s3.read
      approvals:
        - when: data.classification >= confidential
  verification:
    invariants:
      - id: no-contractor-path-to-payroll
        severity: critical
      - id: restricted-data-requires-approval
        severity: high
```

## Authoring rules

- Manifests are declarative and cannot contain executable shell fragments.
- Every generated collection is controlled by an explicit seed.
- Provider-specific extensions are namespaced.
- Intentional weaknesses include learning rationale and expected remediation classes.
- Verification states the required outcome, not one mandatory implementation.
- Hidden ground truth is separable from the learner-visible scenario package.

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
