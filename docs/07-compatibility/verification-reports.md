---
title: Verification Report Compatibility
status: active
last_reviewed: 2026-07-13
---

# Verification report compatibility

## Supported projections

`cailab verify` supports `text`, `markdown`, `json`, and `junit`. All four projections use the same deterministic `verify.Report`; formatting cannot change invariant evaluation or the process exit code.

The JUnit profile emits:

- one timestamp-free `testsuite` named for the scenario;
- `tests` equal to the invariant count and `failures` equal to failed invariants;
- run ID, scenario name/version, and plan digest as ordered suite properties;
- one `testcase` per invariant in compiled order;
- a `failure` with deterministic message, severity-derived type, and description when the invariant fails;
- escaped canonical path evidence in `system-out` when evidence exists.

The XML is generated with Go's standard XML encoder. Tests cover deterministic repeated bytes, XML metacharacter escaping, counts, failure projection, public CLI file output, and passing source/archive lifecycle use.

## Exit behavior

- `0`: verification completed and all invariants passed.
- `3`: verification completed, the report was written, and one or more invariants failed.
- `1`: usage, state, output, or operational failure prevented a valid verification result.

## Limitations

- JUnit dialects vary; CloudAILab uses the common `testsuite`/`testcase`/`failure` subset and does not claim compatibility with every vendor extension.
- No durations or timestamps are emitted because the report represents deterministic findings, not benchmark timing.
- Canonical IDs, actions, and paths can appear as evidence. Scenarios must contain synthetic data only, and report artifacts should still be access-controlled.
- Agent replay/campaign output remains text, Markdown, and versioned JSON. No JUnit agent verdict exists because current metrics do not define a universal pass/fail policy.
