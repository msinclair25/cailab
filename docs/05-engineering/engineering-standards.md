---
title: Engineering Standards
status: active
---

# Engineering standards

## General principles

- Prefer simple, explicit designs over speculative abstraction.
- Keep the deterministic security core independent from optional AI features.
- Validate at boundaries and use typed internal representations.
- Preserve provider-specific evidence even when normalizing state.
- Treat documentation, tests, and compatibility claims as product behavior.
- Optimize only after measurement.

## Go conventions

- Follow standard Go project layout only where it adds clear ownership; avoid empty package hierarchies.
- Use `internal/` for non-public implementation packages.
- Keep `cmd/cailab` thin; business logic belongs in testable packages.
- Accept `context.Context` for blocking or externally scoped operations.
- Wrap errors with operation and resource context; do not compare error strings.
- Avoid package-level mutable state.
- Make concurrency ownership and cancellation explicit.
- Keep interfaces small and define them at the consumer boundary.
- Use structured logging and stable event fields.

## Testing strategy

### Unit tests

- Manifest validation and compilation
- Policy evaluation and deny precedence
- Graph path traversal
- Provider translation
- Evidence construction

### Contract tests

- Supported request and response shapes
- Authentication and authorization behavior
- Negative tenant-isolation cases
- Documented provider limitations

### Integration tests

- Lifecycle orchestration with real local dependencies
- Deploy → mutate → collect → normalize → verify
- Local token issuance and federation
- Agent gateway decisions and audit events

### End-to-end tests

- One flagship scenario using public CLI commands
- Offline/CI execution with no hosted model
- Cleanup and repeat execution

Use golden files only for stable structured output. Normalize timestamps and generated identifiers before comparison.

## Security engineering

- Threat-model new trust boundaries before implementation.
- Never execute scenario-provided shell text implicitly.
- Pin and verify third-party runtime artifacts.
- Bind locally by default and document every listening port.
- Keep hosted model integration opt-in and redact outbound data.
- Add regression tests for every security defect.
- Generate an SBOM for releases.
- Ship project and linked-component license/notice material with every release archive and fail CI when the linked-module inventory drifts.

## Documentation practice

- `README.md` explains value, status, quick start, and verified capabilities.
- Requirements use stable identifiers.
- Significant structural or security choices use ADRs.
- Compatibility claims link to tests and known limitations.
- Commands in documentation are exercised in CI when practical.
- Update documentation in the same change as behavior.
- Prefer standard Markdown links so notes render correctly in both Obsidian and GitHub.

## Git and review

- Keep commits focused and use imperative commit subjects.
- Do not combine broad formatting changes with behavioral changes.
- Pull requests state the problem, decision, validation, risks, and documentation impact.
- Require review for changes to schemas, policy semantics, authentication, verification, or scenario ground truth.

## Definition of done

A change is done when:

1. Behavior and acceptance criteria are clear.
2. Implementation is complete and appropriately scoped.
3. Relevant automated tests pass.
4. Security and compatibility implications are assessed.
5. User-facing and architectural documentation is updated.
6. Diagnostics and failure messages are actionable.
7. Generated artifacts and temporary state are cleaned up.
