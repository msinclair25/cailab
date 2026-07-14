# CloudAILab repository guidance

CloudAILab is a local enterprise identity and AI-agent security range. Keep this file focused on durable repository rules; do not copy milestone snapshots here. Treat the README and compatibility records as the current public capability boundary, and present only behavior covered by implementation, tests, schemas, and compatibility evidence.

## Before changing the project

- Inspect the working tree first. Preserve user changes and keep unrelated edits out of the change.
- Read `README.md`, the [documentation map](docs/README.md), and the documentation relevant to the change.
- Use `docs/00-project/master-plan.md` for milestone scope, delivery order, risks, and exit gates.
- Treat `docs/01-product/requirements.md` as the requirements index.
- Treat accepted records in `docs/02-architecture/decisions/` as durable architectural constraints.
- Review `docs/03-security/threat-model.md` when a change adds a trust boundary, credential, network listener, downloaded dependency, hosted integration, or agent capability.
- Use `docs/05-engineering/quality-strategy.md` to choose validation in proportion to risk.
- Identify the governing requirement, milestone slice, compatibility contract, and security impact before implementation. If one does not exist, update the contract first or explicitly record the gap.

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
- Run the smallest relevant checks during iteration. Before handoff, run the complete applicable baseline from `CONTRIBUTING.md`; documentation-only changes require `go run ./internal/tools/doccheck .` and `git diff --check`.

## Documentation

- Update documentation in the same change as behavior.
- Use stable requirement identifiers and do not renumber removed requirements.
- Record costly-to-reverse architecture or security decisions in an ADR.
- Follow `docs/05-engineering/documentation-conventions.md` for frontmatter, navigation, links, diagrams, and Obsidian settings.
- Keep Markdown portable between Obsidian and GitHub. Use standard relative Markdown links with explicit `.md` extensions; do not use Wikilinks, Obsidian URIs, or local absolute paths.
- Clearly label illustrative examples and proposed behavior.
- Keep the README limited to verified status, value, quick start, and navigation.

## Completion standard

A change is complete only when implementation, relevant tests, documentation, security impact, compatibility impact, diagnostics, cleanup behavior, and the final diff have been reviewed in proportion to risk. Report which checks ran and any checks that did not run.
