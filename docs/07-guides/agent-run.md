---
title: Agent Run Guide
status: m3-development
last_reviewed: 2026-07-12
---

# Agent run guide

## Safety boundary

Host mode owns and bounds direct agent and tool subprocesses, but it does **not** isolate their filesystem, network, syscalls, or independently detached descendants. Run only code you trust under your current OS account. `--agent-env` and `--tool-env` forward real values from your current environment to those unisolated processes.

The opt-in Docker mode isolates the agent process with no host mounts or forwarded host environment, no external network, a read-only root filesystem, bounded temporary storage and compute resources, and reduced privileges. It does not isolate registered tool subprocesses and is not a virtual-machine boundary.

## Verify the harness baseline

Build CloudAILab, start any scenario, and run the deterministic reference agent:

```bash
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab up walking-skeleton
./bin/cailab agent run reference
```

The reference agent completes without tool calls. CloudAILab persists its scenario digest/seed, agent and policy identity, prompt hash, tool digest, lifecycle timestamps, and terminal status. Reusing the default `trial:1` in the same active range is rejected; select a new value with `--trial-id` when intentionally recording another trial.

## Register a custom tool and policy

Tool and policy documents use the normative [v1alpha1 schemas](../../schemas/agent/v1alpha1). A tool command must identify an absolute executable. Its working directory is the directory containing the manifest file.

Before execution, bind the registrations to the active scenario:

```bash
./bin/cailab agent validate \
  --policy /absolute/path/policy.json \
  --tool /absolute/path/tool.json \
  --agent-id agent:my-agent \
  --actor-tenant tenant:northstar
```

Repeat `--tool` for multiple manifests. Repeat `--tool-env NAME` only for environment variables each tool genuinely requires. Validation checks bounded strict JSON, schema compilation, unique tool names, absolute tool launch configuration, active-scenario resources and tenants, policy metadata, and manifest permission ceilings. It does not start the agent or tools.

## Implement protocol 1.1

The agent reads one `session.start` JSON line from standard input, writes `agent.ready`, and then may write `tool.call` messages. Standard output is protocol-only; diagnostics belong on standard error.

An illustrative call is:

```json
{
  "protocolVersion": "1.1",
  "id": "call:1",
  "type": "tool.call",
  "payload": {
    "tool": "google.drive.read",
    "action": "drive.files.get",
    "resource": "google:agent-runbook",
    "arguments": {
      "fileId": "google:agent-runbook"
    }
  }
}
```

The resource value is a canonical scenario identifier. CloudAILab resolves its tenant and classification from active canonical state, evaluates the policy and manifest ceiling, records the decision, and only then may invoke the registered tool. The tool receives the resolved action/resource plus protected arguments in its one-shot request.

## Run your agent

CloudAILab records a hash of `--prompt-file` for provenance. It does not send that prompt to your process; configure your agent to consume its own prompt using explicit argv or an explicitly selected environment variable.

```bash
./bin/cailab agent run subprocess \
  --policy /absolute/path/policy.json \
  --tool /absolute/path/tool.json \
  --prompt-file /absolute/path/prompt.txt \
  --agent-id agent:my-agent \
  --agent-version 0.1.0 \
  --provider local \
  --model my-agent-build \
  --actor-tenant tenant:northstar \
  --command /absolute/path/my-agent \
  --directory /absolute/path/agent-workdir \
  --timeout 60s
```

Use repeated `--arg VALUE` flags to preserve argv boundaries. Use repeated `--agent-env NAME` and `--tool-env NAME` flags for explicitly selected variables; the rest of the parent environment is not inherited. `--json` emits run, completion, decision, approval, and outcome records without raw tool arguments, raw protocol transcripts, or child diagnostic text.

## Run an agent with Docker isolation

Build the agent into a Linux container image. Its protocol executable and working directory must already exist inside the image. Resolve the immutable local image ID:

```bash
docker build --tag my-cailab-agent:local /absolute/path/to/agent-image
docker image inspect --format '{{.Id}}' my-cailab-agent:local
```

Then use that `sha256:...` value:

```bash
./bin/cailab agent run subprocess \
  --policy /absolute/path/policy.json \
  --tool /absolute/path/tool.json \
  --prompt-file /absolute/path/prompt.txt \
  --agent-id agent:my-agent \
  --agent-version 0.1.0 \
  --provider local \
  --model my-agent-build \
  --actor-tenant tenant:northstar \
  --isolation docker \
  --image sha256:REPLACE_WITH_64_HEX_CHARACTERS \
  --command /app/my-agent \
  --directory /workspace \
  --timeout 60s
```

A repository reference is accepted only as `repository@sha256:<digest>` and must be pulled before the run; CloudAILab uses `--pull=never`. Mutable tags and images with Dockerfile `VOLUME` declarations are rejected. `--agent-env` is rejected in Docker mode. The active Docker context must use a local Unix socket and report a non-rootless Linux engine with an active cgroup driver; remote TCP/SSH, rootless, no-cgroup, and Windows-container contexts are rejected before the trial starts. The agent cannot directly reach provider loopback endpoints, hosted model APIs, or the public internet; package prompts and local model assets into the image and use protocol tool calls to cross the governed boundary.

The run summary and immutable run record include the exact image and enforced network/filesystem profile. The Linux Docker integration is the compatibility gate; Docker Desktop may work but is not yet a release-tested platform claim. See the [compatibility record](../07-compatibility/agent-docker-isolation.md).

## Resolve approval-required calls

Approval-required calls reject safely by default. The agent receives `approval.required`, then a rejected `approval.resolved` and correlated `tool.result`; the tool is not launched.

To make a local reviewer part of the run, add:

```bash
--approval-mode prompt --approver user:alice
```

CloudAILab displays canonical target metadata and the approval ID on standard error. It does not display raw tool arguments. Type the exact prompted `approve <approval-id>` value to approve; blank, malformed, or different input rejects. The gateway then re-evaluates current policy, records the resolution, and only continues if the resulting decision is allow or redact. The whole interaction remains subject to the agent session timeout.
