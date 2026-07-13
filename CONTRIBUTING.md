# Contributing to CloudAILab

Thank you for helping improve CloudAILab. Small, evidence-backed changes are easiest to review and maintain.

By submitting a contribution, you represent that you have the right to submit it and agree that it is provided under the repository's [Apache License 2.0](LICENSE), consistent with section 5 of that license. No separate contributor license agreement is currently required.

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). Report suspected vulnerabilities through [SECURITY.md](SECURITY.md), not a public issue.

## Development setup

Prerequisites are Go 1.25.12 or newer and Git. Docker is required for Floci integration tests, Docker-isolated agent tests, and the clean-container demo.

```bash
git clone https://github.com/msinclair25/cailab.git
cd cailab
go mod download
go test ./...
```

## Before coding

- Read the [master plan](docs/00-project/master-plan.md), [requirements](docs/01-product/requirements.md), applicable [ADRs](docs/02-architecture/decisions/README.md), and [threat model](docs/03-security/threat-model.md).
- Open an issue before a broad provider surface, schema change, new trust boundary, new executable dependency, or costly-to-reverse design.
- Add provider operations only for a documented scenario or compatibility contract.
- Never place real credentials, tenant data, personal data, or proprietary prompts in fixtures, tests, issues, or pull requests.

## Implementation expectations

- Keep authorization, attack paths, evidence, and scoring deterministic.
- Keep `cmd/cailab` thin and behavior in testable internal packages.
- Bind services to loopback by default and use synthetic credentials.
- Do not execute scenario-provided shell text.
- Do not claim parity or isolation beyond test-backed compatibility records.
- Update implementation, tests, requirements, security impact, compatibility limits, diagnostics, cleanup behavior, and documentation together.

## Validation

Run the checks relevant to your change. The full local baseline is:

```bash
gofmt -w cmd internal scenarios
go mod tidy -diff
go mod verify
go vet ./...
go test ./...
go test -race ./...
go run ./internal/tools/doccheck .
```

Container and provider changes also require the targeted integration tests documented in [quality strategy](docs/05-engineering/quality-strategy.md).

When a dependency changes, update `third_party/modules.txt`, `THIRD_PARTY_NOTICES.md`, and applicable files under `third_party/licenses/`. CI compares the versioned linked-module inventory with the packages that actually build `cmd/cailab`.

## Pull requests

- Use a focused branch and imperative commit subject.
- Explain the problem, decision, user impact, security/compatibility impact, and validation.
- Keep unrelated formatting or generated changes out of the pull request.
- Resolve or explicitly record every critical/high security finding before requesting a release.
