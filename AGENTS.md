# CloudAILab repository guidance

CloudAILab is a local enterprise identity and AI-agent security range. M0, the scoped M1 AWS/Floci IAM, STS, and S3 slice, and the M2 flagship cross-provider identity chain are complete. M3 contains test-backed agent contracts, strict agent and one-shot tool subprocess controllers, deterministic reference processes, exact-match governed-tool policy, Draft 2020-12 input validation, protected output, and append-only decision/outcome evidence. A supported public registration/run workflow, enforced isolation, full trace replay, interactive approval resolution, and repeated-trial scoring remain planned. Present only behavior covered by implementation, tests, schemas, and compatibility records.

## Before changing the project

- Read `README.md` and the documentation relevant to the change.
- Use `docs/00-project/master-plan.md` for milestone scope, delivery order, risks, and exit gates.
- Treat `docs/01-product/requirements.md` as the requirements index.
- Treat accepted records in `docs/02-architecture/decisions/` as durable architectural constraints.
- Review `docs/03-security/threat-model.md` when a change adds a trust boundary, credential, network listener, downloaded dependency, hosted integration, or agent capability.
- Use `docs/05-engineering/quality-strategy.md` to choose validation in proportion to risk.

## Design guardrails

- Keep scenario compilation, authorization, attack-path evaluation, evidence, and scoring deterministic.
- AI features are optional and cannot override authorization or verification results.
- Use a provider-neutral canonical graph; isolate provider-specific behavior behind adapters or facades.
- Add provider operations only for a documented scenario or compatibility contract.
- Do not claim provider parity without contract tests and a documented fidelity level.
- Bind services to loopback by default and use synthetic credentials.
- Never execute scenario-provided shell text implicitly.
- Do not describe agent execution as isolated unless network and filesystem isolation are actually enforced.

## Implementation expectations

- Keep `cmd/cailab` thin and place behavior in testable internal packages.
- Prefer typed boundaries, small consumer-defined interfaces, explicit cancellation, and wrapped errors.
- Avoid package-level mutable state and hidden global configuration.
- Preserve provider-native evidence during normalization.
- Pin and verify external runtime artifacts.

## Validation

- Add unit tests for domain behavior and policy semantics.
- Add contract tests for provider-shaped requests, responses, and claimed authorization behavior.
- Add negative tests for tenant isolation and explicit-deny behavior.
- Exercise lifecycle changes through deploy, mutate, collect, normalize, verify, and cleanup as applicable.
- Security defects require regression tests.

## Documentation

- Update documentation in the same change as behavior.
- Use stable requirement identifiers and do not renumber removed requirements.
- Record costly-to-reverse architecture or security decisions in an ADR.
- Keep Markdown portable between Obsidian and GitHub; prefer standard relative Markdown links.
- Clearly label illustrative examples and proposed behavior.
- Keep the README limited to verified status, value, quick start, and navigation.

## Completion standard

A change is complete only when implementation, relevant tests, documentation, security impact, compatibility impact, diagnostics, and cleanup behavior have been addressed in proportion to risk.
