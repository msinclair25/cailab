---
title: Quality Strategy
status: draft
last_reviewed: 2026-07-12
---

# Quality strategy

## Objective

CloudAILab must be trustworthy enough to teach authorization and evaluate agents. Quality therefore means more than successful compilation: security decisions are correct and explainable, provider claims are bounded, scenarios are reproducible, failure cleanup is reliable, and documentation matches observed behavior.

## Risk-based test model

| Risk class | Examples | Required evidence |
|---|---|---|
| Critical | Authorization, tenant isolation, agent approvals, credential redaction, protected ground truth | Positive, negative, deny-precedence, regression, and end-to-end tests |
| High | Scenario compilation, graph paths, state migrations, provider normalization, cleanup | Unit, property/fuzz where applicable, integration, and failure-path tests |
| Medium | CLI rendering, reports, configuration diagnostics | Unit or golden tests plus smoke tests |
| Low | Static prose and non-behavioral examples | Formatting and link validation |

## Test layers

### Unit tests

Fast and hermetic. Cover typed domain rules, validation, deterministic compilation, policy decisions, graph traversal, evidence creation, redaction, and report models.

### Fuzz and property tests

Use Go's native fuzzing for untrusted or combinatorial inputs:

- Scenario YAML/JSON parsing
- Reference resolution
- Policy condition parsing
- Provider identifier normalization
- JWT claim parsing and validation
- Event and report decoding
- Agent JSON Lines framing, typed payloads, duplicate keys, size bounds, and redaction pointers

Fuzz targets must be deterministic, avoid persistent global state, and retain minimized failures as regression corpus entries. [Go fuzzing](https://go.dev/doc/security/fuzz/)

### Contract tests

Each claimed provider operation has fixtures that test:

- Method, route, query, headers, and body
- Success response and supported error forms
- Pagination where claimed
- Authorization behavior where claimed
- Side effects and audit events where claimed
- Explicit unsupported behavior

The compatibility matrix links to the test package or test identifier.

### Integration tests

Exercise real local dependencies and lifecycle behavior:

```text
plan → apply → health → mutate → collect → normalize → verify → reset → stop
```

Failure cases include occupied ports, unavailable container runtime, corrupt state, partial startup, expired tokens, invalid signatures, and interrupted cleanup.

### End-to-end tests

Use only public CLI commands and documented workflows. The flagship scenario is the primary end-to-end test. It verifies both the vulnerable initial path and at least one valid remediation while preserving required access.

### Agent evaluations

- A deterministic reference agent establishes harness correctness.
- Deliberately unsafe fixtures establish that findings trigger.
- Nondeterministic models run multiple trials.
- Reports include denominators, configuration, errors, and incomplete trials.
- Optional model judging cannot change deterministic findings.

## Code-quality policy

- `gofmt` is mandatory.
- `go vet`, `go test`, `go test -race`, `go mod verify`, and `govulncheck` are baseline checks.
- `go.mod` and `go.sum` are committed; dependency changes explain why they are needed.
- Errors include operation and resource context and remain machine-testable through wrapping or typed errors.
- Logging is structured; credentials and classified payloads are redacted before logging.
- Package APIs are minimal, documented, and owned by consumers.
- Concurrency has explicit cancellation and lifecycle ownership.
- No package-level mutable state unless justified and synchronized.
- Generated code is reproducible and carries its generator/source metadata.

The Go project recommends govulncheck, fuzzing, and the race detector as complementary security practices. [Security Best Practices for Go Developers](https://go.dev/doc/security/best-practices)

## CI policy

### Pull requests

- Formatting and generated-file consistency
- Module tidy and verification
- Vet, unit tests, race tests
- Vulnerability scan
- Scenario and JSON Schema validation
- Markdown formatting and link checks
- Secret scanning and dependency review
- Targeted integration tests when affected paths change

### Default branch and scheduled

- Full provider integration suite
- Bounded fuzzing
- Cross-platform compile matrix
- Container image build without publication
- CodeQL and dependency/security scans
- Cleanup leak detection

### Release

- All prior gates
- Reproducible build configuration
- Checksums
- SBOM
- Build provenance attestation
- Clean-install smoke tests
- Documentation and compatibility review

GitHub Actions receive minimum permissions and third-party actions are pinned to full commit SHAs. Release jobs are separate from pull-request jobs and use protected environments when credentials are introduced.

## Coverage policy

Coverage is diagnostic evidence, not the definition of correctness. M0 established the initial baseline; numeric thresholds remain deferred until provider and agent packages make a repository-wide percentage meaningful. Regardless of percentage:

- Every security invariant has positive and negative tests.
- Every fixed security defect has a regression test.
- Every claimed provider operation has a contract test.
- Every migration has forward and rollback/failure-behavior tests as applicable.
- Every lifecycle component has cleanup tests.

## Documentation-quality policy

- Standard Markdown must render in GitHub and Obsidian.
- Relative links are preferred for repository documents.
- Diagrams use Mermaid source committed beside the prose.
- Proposed and implemented behavior are visibly distinguished.
- Examples are executable in CI when practical.
- External claims cite primary sources near the claim.
- Version-sensitive sources appear in the source register with a review trigger.
- README capability statements must be supported by tests on the default branch.

## Review checklist

Every implementation review asks:

1. Which requirement and milestone does this satisfy?
2. What trust boundary or failure mode changed?
3. Which compatibility claim changed?
4. Are allow and deny cases both tested?
5. Are errors actionable without leaking sensitive data?
6. Is cleanup correct after success, failure, cancellation, and repeat execution?
7. Did documentation, ADRs, threat model, or source register need an update?
8. Can a clean environment reproduce the result?

## Release-blocking defects

- Critical authorization or tenant-isolation defect
- Credential or protected-ground-truth disclosure
- Unbounded host/network access described as isolated
- Non-reproducible deterministic score
- Missing compatibility limitation for known semantic divergence
- Release artifact without checksum, SBOM, or provenance after M4
- README claim unsupported by the released implementation
