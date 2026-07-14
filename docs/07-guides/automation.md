---
title: CLI Automation
status: active
last_reviewed: 2026-07-13
---

# CLI automation

CloudAILab separates human-readable terminal output from versioned machine-readable contracts. Scripts must not parse the human `status` table or startup messages.

## Help contract

`-h` and `--help` exit successfully for the root, every public command, and every public command group. Help is written to standard output and is never prefixed as an operational error.

```bash
cailab --help
cailab scenario --help
cailab status --help
cailab agent run subprocess --help
```

Unknown commands, invalid flags, missing required inputs, and runtime failures still exit nonzero and write diagnostics to standard error.

## Status and endpoint contract

Use:

```bash
cailab status --format json
```

The current schema identifier is `cloudailab.dev/status/v1alpha1`, with kind `RangeStatus`. The projection contains stable run identity, scenario version, lifecycle state, seed, deterministic plan and baseline digests, and provider runtime summaries. Runtimes are sorted by provider, engine, and endpoint.

Illustrative shape:

```json
{
  "apiVersion": "cloudailab.dev/status/v1alpha1",
  "kind": "RangeStatus",
  "run": {
    "id": "run:example",
    "scenario": "google-drive-sharing",
    "scenarioVersion": "0.1.0",
    "status": "active",
    "seed": 42,
    "planDigest": "sha256-like-hex",
    "baselineDigest": "sha256-like-hex"
  },
  "runtimes": [
    {
      "provider": "google",
      "engine": "native",
      "endpoint": "http://127.0.0.1:12345",
      "status": "ready"
    }
  ]
}
```

The status projection deliberately excludes native control paths, control tokens, process IDs, container IDs, image references, temporary credentials, provider bearer tokens, signing material, and provider payloads. An endpoint is routing information, not authorization.

## Bash and PowerShell consumption

Bash with `jq`:

```bash
google_endpoint="$(cailab status --format json | jq -er '.runtimes[] | select(.provider == "google") | .endpoint')"
```

PowerShell:

```powershell
$status = cailab status --format json | ConvertFrom-Json
$googleEndpoint = ($status.runtimes | Where-Object provider -eq 'google').endpoint
```

CloudAILab does not currently emit shell assignment text. JSON avoids shell-specific quoting, accidental `eval`, ambiguous secret handling, and separate Bash/PowerShell contracts. If environment rendering is added later, it requires its own versioned escaping and secret-exclusion tests.

## JUnit verification contract

Export deterministic scenario invariants for CI:

```bash
cailab verify --format junit --output cailab-verification.xml
```

The timestamp-free XML contains one `testsuite`, one `testcase` per invariant, and one `failure` per failed invariant. Suite properties identify the run, scenario, scenario version, and deterministic plan digest; canonical path evidence is escaped and placed in `system-out`. The command still returns `3` when invariant findings exist, even when it successfully writes the report.

Use the release-packaged [least-privilege GitHub Actions example](../../examples/ci/README.md) for a synthetic no-runtime workflow with owned cleanup and no cloud/model credentials. Exact compatibility and limitations are recorded in [verification reports](../07-compatibility/verification-reports.md).

Agent replay and campaign reports support text, Markdown, and their versioned JSON contract. They intentionally do not emit JUnit: completion, authorization rate, tool execution, task success, remediation, and adversarial metrics are separate facts, and CloudAILab will not invent one universal pass/fail verdict. A future scenario-specific agent JUnit profile requires declared criteria and a separate compatibility contract.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Command or help completed successfully. |
| `1` | Usage, validation, prerequisite, state, or operational error. |
| `3` | Verification ran successfully and one or more deterministic invariants failed. |

An intentionally vulnerable initial scenario commonly returns `3`; automation should distinguish that verified finding from an execution error.
