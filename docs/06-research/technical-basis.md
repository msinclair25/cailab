---
title: Technical Basis and Source Register
status: active
last_reviewed: 2026-07-11
---

# Technical basis and source register

## Purpose

This register records external evidence that materially affects architecture, scope, security, compatibility, or quality gates. It is not a general bibliography. Primary sources are preferred, and version-sensitive findings are reviewed before the affected milestone ships.

## Provider and protocol evidence

| Area | Primary source | Finding | Plan implication | Review trigger |
|---|---|---|---|---|
| Floci STS | [Floci STS documentation](https://floci.io/floci/services/sts/) | IAM enforcement is opt-in; STS trust evaluation omits `Condition` blocks and caller-side `sts:AssumeRole` authorization. | CloudAILab owns authoritative scenario policy semantics and documents gaps. | Before M1 and on Floci upgrade |
| Floci accounts | [Floci multi-account isolation](https://floci.io/floci/configuration/multi-account/) | Twelve-digit access-key IDs select account namespaces; temporary credentials route to the assumed account; signature validation is off by default. | Use deliberate test credentials and explicitly configure signature behavior. | Before M1 and on Floci upgrade |
| AWS policy semantics | [AWS IAM policy evaluation logic](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic.html) | AWS combines multiple policy types, uses implicit deny by default, and gives applicable explicit denies precedence. | Model only declared IAM semantics, preserve deny precedence, and document every omitted policy type. | Before M1 policy claims |
| AWS cross-account access | [AWS cross-account evaluation](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic-cross-account.html) | Cross-account access commonly requires permission in both the trusted principal's account and the trusting resource's account. | Test both sides of supported cross-account paths instead of relying on Floci's trust-policy-only behavior. | Before M1 exit |
| Microsoft simulation | [Dev Proxy overview](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/overview) | Dev Proxy focuses on API simulation, resilience, failures, throttling, and Graph guidance. | Use as optional compatibility/chaos tooling, not Entra source of truth. | Before any Dev Proxy integration |
| Microsoft CRUD | [Dev Proxy CRUD simulation](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/simulate-crud-api) | CrudApiPlugin supplies generic in-memory CRUD state. | Native facade remains preferable for canonical identity and authorization semantics. | Before M2 |
| Microsoft TLS | [Dev Proxy troubleshooting](https://learn.microsoft.com/en-us/microsoft-cloud/dev/dev-proxy/how-to/troubleshooting) | HTTPS interception requires certificate trust, with runtime-specific handling. | Default mode avoids host-wide interception and global trust changes. | Before advanced proxy mode |
| Graph clients | [Customize Microsoft Graph SDK client](https://learn.microsoft.com/en-us/graph/sdks/customize-client) | SDK middleware and client behavior can be customized for testing. | Publish supported endpoint/client examples rather than requiring transparent interception. | Before M2 SDK examples |
| Graph app roles | [Grant an app role assignment to a group](https://learn.microsoft.com/en-us/graph/api/group-post-approleassignments?view=graph-rest-1.0) | Graph models group app-role assignments using principal, resource service principal, and app-role identifiers. | Preserve those identifiers and relationships in the Microsoft facade and canonical graph. | Before M2 application contracts |
| Google Directory | [Admin SDK Directory API](https://developers.google.com/workspace/admin/directory/reference/rest) | Google publishes a Discovery document and service endpoint for Directory API resources. | Select and generate supported contracts from Discovery metadata. | Before M2 and on contract refresh |
| Google Drive files | [Drive API files resource](https://developers.google.com/workspace/drive/api/reference/rest/v3/files) | Drive exposes file metadata, ownership, hierarchy, content-related fields, and permission references. | Implement only fields and methods required by the prompt-injection scenario and document omissions. | Before M2 Drive contracts |
| Google Drive permissions | [Drive API permissions resource](https://developers.google.com/workspace/drive/api/reference/rest/v3/permissions) | Permissions can grant users, groups, domains, or anyone roles on files and drives. | Normalize selected grants into trust edges and test inherited/direct limitations explicitly. | Before M2 Drive authorization claims |
| OpenID discovery | [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html) | Issuer metadata is discovered from a well-known location, and issuer values must match configuration and tokens. | Local issuer implements exact issuer consistency, discovery, and JWKS behavior required by the scenario. | Before M2 issuer release |

## AI security and governance evidence

| Area | Primary source | Finding | Plan implication | Review trigger |
|---|---|---|---|---|
| AI risk lifecycle | [NIST AI RMF](https://www.nist.gov/itl/ai-risk-management-framework) | AI RMF structures risk work around Govern, Map, Measure, and Manage; version 1.0 is under revision. | CAL uses the framework for control alignment, not certification, and tracks revisions. | Before M3 and framework revision |
| AI evaluation | [NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/) | TEVV should be objective, repeatable/scalable, documented, and relevant to deployment conditions; uncertainty and limitations matter. | Agent reports include metrics, run context, repeated trials, errors, and limitations. | Before M3 metric freeze |
| Agent threats | [OWASP Top 10 for Agentic Applications 2026](https://genai.owasp.org/resource/owasp-top-10-for-agentic-applications-for-2026/) | Current agentic risks include goal hijacking, tool misuse, identity/privilege abuse, supply chain, and unexpected code execution. | Flagship scenario must cover the first three; threat model tracks the broader set. | Before M3 and annual OWASP update |
| AI control catalog | [CSA AI Controls Matrix v1.1](https://cloudsecurityalliance.org/artifacts/ai-controls-matrix-v1-1) | AICM provides vendor-neutral cloud AI control objectives and mappings to other standards. | Scenario metadata may map measured controls to AICM identifiers without claiming compliance. | Before control mapping release |

## Engineering and supply-chain evidence

| Area | Primary source | Finding | Plan implication | Review trigger |
|---|---|---|---|---|
| Go security | [Go security best practices](https://go.dev/doc/security/best-practices) | Go recommends vulnerability analysis, fuzzing, and race detection as complementary practices. | Add govulncheck, fuzz targets, and race tests at defined CI cadences. | On Go toolchain upgrade |
| Go modules | [Go Modules reference](https://go.dev/ref/mod) | `go mod tidy -diff` reports required module-file changes without modifying files, and `go mod verify` checks downloaded module content. | Commit module metadata and require tidy/integrity checks in CI. | On Go toolchain or dependency policy change |
| Go fuzzing | [Go fuzzing](https://go.dev/doc/security/fuzz/) | Fuzz targets should be fast, deterministic, and free of persistent cross-call state. | Apply fuzzing to parsers, policies, identifiers, tokens, and event decoding. | When adding untrusted input surface |
| YAML parser | [`go.yaml.in/yaml/v3`](https://pkg.go.dev/go.yaml.in/yaml/v3) | The package is maintained by the YAML organization and supports strict known-field decoding. | Pin the parser and reject unknown scenario fields. | On parser upgrade |
| SQLite driver | [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) | The driver provides a CGO-free `database/sql` implementation across the target desktop platforms. | Use it for local transactional state and verify every target in CI. | On driver or Go upgrade |
| Workflow pinning | [GitHub Actions settings](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository) | Repositories can require actions to be pinned to full-length commit SHAs. | Pin actions and minimize workflow permissions from the first workflow. | On workflow addition/update |
| Build provenance | [GitHub artifact attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations) | GitHub supports provenance attestations for binaries, images, and SBOMs. | M4 release artifacts include checksums, SBOM, and attestations. | Before M4 release |
| Provenance model | [SLSA provenance](https://slsa.dev/spec/v1.2/provenance) | Provenance links artifacts to where, when, and how they were produced. | Release documentation explains artifact verification and build origin. | On SLSA spec change |
| Policy testing | [OPA policy testing](https://www.openpolicyagent.org/docs/policy-testing) | OPA provides a declarative policy language and test framework. | OPA is evaluated in M0 but not assumed until an ADR accepts it. | During M0 policy spike |
| Telemetry | [OpenTelemetry semantic conventions](https://opentelemetry.io/docs/concepts/semantic-conventions/) | Common conventions provide consistent names across traces, metrics, logs, and resources. | Reuse stable names where applicable while keeping CAL's event schema portable. | Before event schema stability |

## Source quality rules

- Prefer vendor documentation, standards bodies, and project documentation over blogs or search summaries.
- Record the exact limitation, not a generalized product judgment.
- Do not copy marketing performance claims into requirements without independent measurement.
- Cite version-sensitive facts near the corresponding plan or compatibility claim.
- If a source conflicts with observed behavior, preserve the observed test evidence and document the discrepancy.
- Review this register at each milestone exit and before dependency upgrades that affect recorded behavior.
