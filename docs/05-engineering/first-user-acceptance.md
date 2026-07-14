---
title: First-User Acceptance
status: active
last_reviewed: 2026-07-13
release_gate: pending-unfamiliar-participant
---

# First-user acceptance

## Decision boundary

Automated tests prove repeatability; they do not prove that an unfamiliar person understands the product. M4.1 first-user acceptance therefore has two distinct parts:

1. a maintainer clean-environment rehearsal that detects packaging, command, reporting, and cleanup defects; and
2. one observed walkthrough by a participant who has not contributed to CloudAILab and has not been coached on repository internals.

Only the second part can close the unfamiliar-user and comprehension gate. Neither part is a credential, assessment of the participant, or employment claim.

## Candidate and observer record

Record the following before an observed session:

| Field | Value |
|---|---|
| Candidate version, commit, and archive SHA-256 | Required |
| Host OS and architecture | Required |
| Docker version, when the flagship is attempted | Required |
| Participant prior CloudAILab exposure | Must be none beyond the invitation and public README |
| Participant background, in broad non-identifying terms | Optional |
| Observer | Required |
| Start, first-success, and end times | Required |
| Hints given and reason | Required; record verbatim where practical |
| Errors, abandoned steps, and cleanup result | Required |

Use an owner-safe temporary directory and synthetic fixtures. Do not collect the participant's credentials, shell history, personal files, screen recording, or identifying information unless they explicitly consent outside this engineering protocol.

## Observed workflow

The observer provides only the verified archive, [release verification](../07-guides/release-verification.md), and root README. The observer does not explain commands until the participant asks or is blocked.

### A. Install and first success

1. Select and verify the matching archive.
2. Extract it without a repository checkout or Go toolchain.
3. Run `cailab version`, `cailab --help`, and `cailab quickstart`.
4. Stop the timer when the participant can state that verification passed and the owned run stopped.

Acceptance: first success takes ten minutes or less; Docker, a cloud account, and a hosted model are not used; no active run remains.

### B. Interpret the result

Ask the participant, without supplying terms from the answer:

- What did this lab prove?
- Did it contact or reproduce AWS, Microsoft, or Google services?
- What does a passing invariant mean?
- What was cleaned up, and what evidence remained?
- Would you run an unknown agent in host mode? Why?

Acceptance: the participant distinguishes the canonical local graph from provider parity, deterministic verification from AI judgment, stopped runtime resources from retained local evidence, and trusted host execution from isolation.

### C. Authoring and automation

Ask the participant to locate the bundled scenario starter, validate it without starting a run, execute its lifecycle, and export JUnit. Ask where compatibility evidence and contribution tests are documented.

Acceptance: the participant does not add executable hooks, can identify the two invariants, uses machine-readable output without parsing the human status table, and stops the run.

### D. Flagship and external agent

When Docker is available, ask the participant to follow the flagship guide through initial finding, minimal Microsoft-assignment remediation, preserved administrator access, deterministic verification, packaged external-agent configuration/validation/run/replay, and cleanup.

Acceptance: the participant recognizes the intentional exit code `3`, changes only the risky assignment, does not treat the synthetic token as production authentication, understands that the starter does not remediate the scenario, and can state that the host agent and tool are unisolated.

## Friction classification

| Severity | Meaning | Release action |
|---|---|---|
| Blocker | Cannot install, achieve first success, verify, or clean up using public material | Fix and repeat the affected journey before RC2 |
| High | Public guidance causes a security/compatibility misunderstanding or requires repository-internal knowledge | Fix and repeat before RC2 |
| Medium | Recoverable ambiguity, unnecessary manual transcription, or platform-specific confusion | Fix when bounded; otherwise document with an explicit release decision |
| Low | Wording or presentation preference that does not change outcome or boundary comprehension | Record for post-release refinement |

## Maintainer rehearsal — development package

This rehearsal used a local working-tree package named `0.0.0-reporting`, not RC2. Its embedded base commit does not identify the uncommitted working-tree changes, so it is product feedback only and cannot satisfy candidate provenance or release approval.

Environment: macOS arm64, Docker Engine 29.5.3, five-target repository packager output.

| Journey | Result |
|---|---|
| Empty-environment archive quick start without Go or Docker | Passed in 0.07 seconds; one invariant passed; no active run remained |
| Bundled data-only starter and JUnit | Passed validation, two-invariant lifecycle, timestamp-free XML export, and cleanup |
| Flagship remediation | Passed initial expected finding, exact risky-assignment deletion, preserved administrator path, two passing final invariants, endpoint closure, and container cleanup |
| Safe/unsafe deterministic campaigns | Two trials each passed restoration/evidence checks and produced the expected opposite injection outcomes |
| Packaged external-agent starter | Passed configuration, registration, one governed provider-backed read, protected output, replay, two state snapshots, and cleanup |
| Managed-container leak check | No provider or agent container remained |

### Friction found and resolved

| ID | Severity | Observation | Resolution |
|---|---|---|---|
| UAT-001 | Blocker | Clean source instructions wrote to `./bin/cailab`, but a clean clone has no `bin` directory. | Added `mkdir -p ./bin` before every documented source build that targets that directory. |
| UAT-002 | High | The bundled external-agent guide described adjacent release binaries but executed source-only `./bin/...` paths. | Added explicit absolute executable variables for source and archive layouts, a dedicated state directory, and an accurate Windows test boundary. |
| UAT-003 | Low | An artificial environment with a replacement `HOME` hid Docker Desktop client context during fixture restoration. | The acceptance protocol preserves the participant's normal Docker configuration while omitting cloud/model credentials; this was not classified as a product failure. |

## Remaining gate

An unfamiliar-participant walkthrough against the exact proposed RC2 archive remains required. Record its archive hash, timing, comprehension answers, hints, friction, and cleanup here or in a linked immutable issue/record. Do not mark M4.1 acceptance complete based solely on the maintainer rehearsal above.
