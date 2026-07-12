---
title: AWS Compatibility Matrix — Floci 1.5.32
status: m1-development
last_reviewed: 2026-07-12
runtime_digest: sha256:4f69631e560120d79ad82d2af9f7dda8c6ef7ecbbae0c43ddcffa109c6588a15
---

# AWS compatibility matrix — Floci 1.5.32

## Claim boundary

CloudAILab supports only the operations below for the `aws-cross-account` workflow. A listed operation is not a claim of complete service compatibility. The authoritative automated exercise is [`TestFlociIntegration`](../../internal/provider/docker_integration_test.go), with focused policy and runtime tests in [`internal/provider`](../../internal/provider).

Fidelity labels follow [ADR-0004](../02-architecture/decisions/0004-scenario-driven-compatibility.md):

- **API-compatible:** the exercised request and response shape works with the official AWS SDK or AWS CLI.
- **Authorization-compatible:** only the explicitly described decision semantics are claimed.
- **Behavior-compatible:** only the tested side effect is claimed.
- **Training-only:** useful local behavior with no AWS-parity claim.

## Runtime

| Capability | Fidelity | Evidence | Limits |
|---|---|---|---|
| Start Floci 1.5.32 | Behavior-compatible with CAL lifecycle | Docker integration test and `cailab up` end-to-end run | Docker only; Podman is not yet tested by CAL. |
| Health readiness | Training-only | `GET /_floci/init` returns HTTP 200 before hydration | Floci-specific endpoint, not AWS. |
| Loopback endpoint | CAL security contract | Runtime unit and integration tests | IPv4 loopback only; the container may retain bridge-network egress. |
| Reset | CAL behavior contract | Integration and manual lifecycle tests | Recreates initial in-memory state; it is not an AWS API. |
| Cleanup | CAL behavior contract | Ownership-label tests and leak check | Removes only a container whose run label matches. |

## AWS operations

| Service | Operation | Fidelity | Exercised behavior | Known limits |
|---|---|---|---|---|
| IAM | `CreateRole` | API- and behavior-compatible for the scenario | Creates a named role with an AWS JSON trust document in the selected account namespace. | No general IAM role-field parity claim. |
| IAM | `PutRolePolicy` | API- and behavior-compatible for the scenario | Stores an inline role policy granting `s3:GetObject`. | Only simple `Allow`/`Deny`, action, and resource statements are authored by CAL. |
| IAM | `GetRole` | API-compatible for the scenario | Reads the current trust document for live graph normalization. | CAL normalizes only known account-root AWS principals. |
| IAM | `UpdateAssumeRolePolicy` | API- and behavior-compatible for the scenario | AWS CLI mutation changes subsequent STS decisions and CAL trust edges. | `NotPrincipal` is unsupported by Floci. Trust-policy conditions are not evaluated by Floci STS. |
| STS | `AssumeRole` | API-compatible; limited authorization compatibility | Trusted account root succeeds, untrusted root fails, and temporary credentials route to the role account. | Floci checks the target trust policy but does not fully enforce the caller-side `sts:AssumeRole` identity policy. `Condition` blocks such as `ExternalId` are ignored. |
| S3 | `CreateBucket` | API- and behavior-compatible for the scenario | Creates the scenario bucket in account B. | Only path-style addressing is used. No bucket-policy claim. |
| S3 | `PutObject` | API- and behavior-compatible for the scenario | Seeds one synthetic object. | No encryption, versioning, ACL, or metadata fidelity claim. |
| S3 | `GetObject` | API-compatible; limited authorization compatibility | Assumed-role credentials with an inline role policy retrieve the seeded object. | Floci IAM enforcement does not support S3 resource-based bucket policies. |

## Deliberate emulator semantics

- Twelve-digit access-key IDs select the simulated account. This is Floci/LocalStack-compatible behavior, not an AWS credential format.
- The secret `cailab-local-only` is synthetic. CAL leaves Floci signature validation disabled in M1, so the secret is not a cryptographic trust boundary.
- Unknown access keys are permissive in Floci. CAL uses only declared synthetic account IDs and generated STS credentials in the supported workflow.
- IAM enforcement is enabled, but Floci permits unauthenticated health routes and operations it cannot map to an IAM action.
- Resource-based policies are outside the M1 claim. Cross-provider and cross-account grading remains deterministic in CAL's normalized graph.

## Primary sources

- [Floci 1.5.32 release](https://github.com/floci-io/floci/releases/tag/1.5.32)
- [Floci installation and image tags](https://floci.io/floci/getting-started/installation/)
- [Floci IAM and enforcement](https://floci.io/floci/services/iam/)
- [Floci STS limitations](https://floci.io/floci/services/sts/)
- [Floci S3 operations](https://floci.io/floci/services/s3/)
- [Floci multi-account isolation](https://floci.io/floci/configuration/multi-account/)
- [AWS IAM policy evaluation logic](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic.html)
