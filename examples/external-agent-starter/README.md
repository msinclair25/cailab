---
title: External Agent Starter
status: active
last_reviewed: 2026-07-13
---

# External agent starter

This dependency-free Go program is the supported minimal example for CloudAILab protocol `1.1`. One executable has two subprocess modes:

- `agent` reads `session.start`, makes exactly one governed read request, checks the correlated result, and completes without deriving another call from returned content.
- `tool` accepts exactly one CloudAILab tool request and reads the fixed synthetic acquisition runbook from the active Google facade.

`configure` writes a scenario-bound default-deny policy, closed tool manifest, and prompt. The manifest fixes the canonical tenant, action, and resource, declares loopback network authority, declares no filesystem need, and redacts `/content` at CloudAILab's protected output boundary.

## Safety boundary

The default launch below runs both processes on the host. CloudAILab bounds and owns their direct lifecycles, but it does not isolate their filesystem, network, syscalls, or independently detached descendants. Run only code you trust. The tool is also unisolated even if an agent later uses Docker isolation.

The starter tool accepts only an exact `http://127.0.0.1:PORT` origin and a fixed provider path. `CAILAB_GOOGLE_TOKEN` is synthetic, but it is still forwarded explicitly and is not persisted in the manifest or evidence output.

## Select the executables

Run the workflow from the repository or extracted-archive root. The commands below use Bash syntax on Linux or macOS.

From a source checkout:

```bash
mkdir -p ./bin
go build -o ./bin/cailab ./cmd/cailab
go build -o ./bin/cailab-agent-starter ./examples/external-agent-starter
export CAILAB="$PWD/bin/cailab"
export CAILAB_AGENT_STARTER="$PWD/bin/cailab-agent-starter"
```

A verified Linux or macOS release archive places `cailab-agent-starter` beside `cailab`:

```bash
export CAILAB="$PWD/cailab"
export CAILAB_AGENT_STARTER="$PWD/cailab-agent-starter"
```

Windows archives use `.exe`. Configuration generation is smoke-tested on Windows, but the complete provider-backed starter lifecycle is currently integration-tested on Linux and acceptance-rehearsed on macOS; do not infer a complete Windows host-subprocess compatibility claim.

Use a dedicated state directory so this workflow cannot collide with another active lab:

```bash
export CAILAB_STATE="$PWD/.cloudailab/external-starter"
```

Create a new configuration directory. The command refuses to overwrite an existing directory:

```bash
"$CAILAB_AGENT_STARTER" configure --output ./starter-config
```

## Run against the flagship scenario

Start the scenario and copy the Google endpoint from `cailab status --format json`:

```bash
"$CAILAB" up --state-dir "$CAILAB_STATE" acquisition-agent
"$CAILAB" status --state-dir "$CAILAB_STATE" --format json
export CAILAB_GOOGLE_ENDPOINT=http://127.0.0.1:PORT
export CAILAB_GOOGLE_TOKEN=cailab-google-local
```

Validate without launching either process:

```bash
"$CAILAB" agent validate \
  --state-dir "$CAILAB_STATE" \
  --policy ./starter-config/policy.json \
  --tool ./starter-config/tool.json \
  --agent-id agent:external-starter \
  --actor-tenant tenant:northstar \
  --tool-env CAILAB_GOOGLE_ENDPOINT \
  --tool-env CAILAB_GOOGLE_TOKEN
```

Launch one restored, state-captured trial. Use absolute paths for the executable and working directory as required by the protocol boundary:

```bash
"$CAILAB" agent run subprocess \
  --state-dir "$CAILAB_STATE" \
  --policy "$PWD/starter-config/policy.json" \
  --tool "$PWD/starter-config/tool.json" \
  --prompt-file "$PWD/starter-config/prompt.txt" \
  --agent-id agent:external-starter \
  --agent-version 0.1.0 \
  --provider local \
  --model deterministic-starter \
  --actor-tenant tenant:northstar \
  --command "$CAILAB_AGENT_STARTER" \
  --arg agent \
  --directory "$PWD" \
  --tool-env CAILAB_GOOGLE_ENDPOINT \
  --tool-env CAILAB_GOOGLE_TOKEN \
  --restore-fixture \
  --trial-id trial:external-starter
```

Replay the persisted evidence:

```bash
"$CAILAB" agent replay --state-dir "$CAILAB_STATE" --trial-id trial:external-starter --format markdown
```

The expected evidence boundary is recorded in [expected-report.md](expected-report.md). Stop the range with `"$CAILAB" down --state-dir "$CAILAB_STATE"`; the configuration directory contains no runtime token and can be removed normally.

## Adaptation points

Copy the program into your own repository and change one concern at a time:

1. Preserve strict JSON Lines framing and correlation while replacing deterministic agent logic.
2. Add a tool permission only after adding its exact scenario target and default-deny policy rule.
3. Keep model credentials out of manifests and pass only explicitly selected environment variables in trusted host mode.
4. Use Docker isolation only for a self-contained agent that needs no hosted model or provider endpoint; registered tools remain on the host.

This starter demonstrates protocol and governance integration. It does not demonstrate general model quality, prompt-injection resistance, remediation quality, or safe execution of arbitrary code.
