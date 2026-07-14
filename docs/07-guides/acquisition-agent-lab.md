---
title: Cross-Provider Acquisition Agent Lab
status: active
last_reviewed: 2026-07-13
---

# Cross-provider acquisition agent lab

## Outcome

This flagship lab follows one synthetic identity path across four representations:

```text
Google user → Google group → synchronized Microsoft group
→ live Microsoft app-role assignment → local OIDC client/audience
→ CloudAILab federation decision → AWS role → restricted S3 object
```

You will prove the initial contractor access, remove only the risky Microsoft app-role assignment, then prove both that the contractor is denied and the approved security administrator still has a path. The exact compatibility boundary is in the [cross-provider matrix](../07-compatibility/cross-provider-federation.md).

The Drive runbook contains an illustrative indirect prompt-injection string. The identity-remediation workflow in this guide treats it as inert data and never executes document text. Separate supported agent workflows can deliberately expose code-owned safe or unsafe controls to that fixture and evaluate their governed actions after the run.

## Prerequisites

- Go 1.25.12 or newer
- Docker
- `curl`, `jq`, and optionally the AWS CLI
- No cloud account, hosted model, global proxy, or certificate installation

```bash
mkdir -p ./bin
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor acquisition-agent
./bin/cailab up acquisition-agent
./bin/cailab status
```

Record the random loopback endpoints printed by `up` or `status`:

```bash
export CAILAB_GOOGLE_ENDPOINT=http://127.0.0.1:PORT
export CAILAB_GRAPH_ENDPOINT=http://127.0.0.1:PORT
export CAILAB_ISSUER=http://127.0.0.1:PORT
export CAILAB_AWS_ENDPOINT=http://127.0.0.1:PORT
export CAILAB_GRAPH_TOKEN=cailab-local
export CAILAB_GOOGLE_TOKEN=cailab-google-local
```

Every identity, bearer, token, credential, and data object in this guide is synthetic and local to the run.

## 1. Inspect the initial path

```bash
./bin/cailab mission
./bin/cailab graph path google:contractor aws:acquisition-data
./bin/cailab graph path google:security-admin aws:acquisition-data
./bin/cailab verify
```

Initial verification intentionally exits with code `3`: the contractor path must eventually be absent, but it starts open. The approved administrator path starts open and must remain open.

Inspect provider-shaped state directly:

```bash
curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GOOGLE_TOKEN" \
  "$CAILAB_GOOGLE_ENDPOINT/admin/directory/v1/groups"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/groups"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/servicePrincipals/77777777-7777-4777-8777-777777777777/appRoleAssignedTo"
```

The Google-to-Microsoft synchronization edges are declared scenario contracts. Google membership and Microsoft app-role assignment edges are rebuilt from live facade state before each path query or verification.

## 2. Issue a signed contractor access token

Request a one-time code. The issuer returns a `302` response; copy the `code` query value from the `Location` header.

```bash
curl --silent --show-error --output /dev/null --dump-header - \
  --get "$CAILAB_ISSUER/authorize" \
  --data-urlencode "response_type=code" \
  --data-urlencode "client_id=cailab-acquisition-automation" \
  --data-urlencode "redirect_uri=http://127.0.0.1:7777/callback" \
  --data-urlencode "scope=openid profile email" \
  --data-urlencode "state=contractor-lab" \
  --data-urlencode "cailab_subject=northstar-contractor"

export CAILAB_CODE=PASTE_CODE_HERE
umask 077
mkdir -p .cloudailab/lab-output

curl --fail --silent --show-error \
  --request POST \
  --user "cailab-acquisition-automation:cailab-synthetic-acquisition-secret" \
  --header "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CAILAB_CODE" \
  --data-urlencode "redirect_uri=http://127.0.0.1:7777/callback" \
  "$CAILAB_ISSUER/token" \
  > .cloudailab/lab-output/contractor-tokens.json

jq -r .access_token .cloudailab/lab-output/contractor-tokens.json \
  > .cloudailab/lab-output/contractor.jwt
```

## 3. Exchange through the authoritative federation gateway

```bash
./bin/cailab federation assume-aws \
  --token-file .cloudailab/lab-output/contractor.jwt \
  --role-node aws:acquisition-reader \
  --output .cloudailab/lab-output/contractor-aws.json
```

The command writes credentials to an owner-only file and does not print them. It validates the active issuer and JWKS, RS256 signature, access-token type, exact audience, time claims, declared subject and groups, live Microsoft app-role assignment, and typed AWS web-identity trust before asking Floci for a temporary response.

If the AWS CLI is installed, prove access without using host AWS credentials:

```bash
export AWS_ACCESS_KEY_ID="$(jq -r .accessKeyId .cloudailab/lab-output/contractor-aws.json)"
export AWS_SECRET_ACCESS_KEY="$(jq -r .secretAccessKey .cloudailab/lab-output/contractor-aws.json)"
export AWS_SESSION_TOKEN="$(jq -r .sessionToken .cloudailab/lab-output/contractor-aws.json)"
export AWS_DEFAULT_REGION=us-east-1

aws --endpoint-url "$CAILAB_AWS_ENDPOINT" s3api get-object \
  --bucket cailab-acquisition-data \
  --key restricted/acquisition-summary.txt \
  .cloudailab/lab-output/acquisition-summary.txt
```

Always pass the printed local endpoint. These credentials must never be sent to AWS or another network service.

## 4. Remove only the risky assignment

```bash
curl --fail --silent --show-error \
  --request DELETE \
  -H "Authorization: Bearer $CAILAB_GRAPH_TOKEN" \
  "$CAILAB_GRAPH_ENDPOINT/v1.0/servicePrincipals/77777777-7777-4777-8777-777777777777/appRoleAssignedTo/99999999-9999-4999-8999-999999999999"
```

The supported Microsoft-shaped operation returns `204 No Content` and persists immediately.

## 5. Prove denial and preserved access

```bash
./bin/cailab graph path google:contractor aws:acquisition-data
./bin/cailab graph path google:security-admin aws:acquisition-data
./bin/cailab verify
```

Repeating `federation assume-aws` with the still-unexpired contractor token now fails before Floci is called. To exercise the preserved path, repeat the code and token steps with `cailab_subject=northstar-security-admin`; the administrator token remains authorized.

## Governed agents and tools

With the scenario active, CloudAILab can launch its deterministic reference, fixture-specific safe, deliberately unsafe, or protocol-compatible custom subprocess workflows through the governed tool boundary. The runner validates scenario-bound policy and tool registration, resolves canonical targets, records immutable linked decision/approval/outcome evidence, and can capture and restore supported provider state for deterministic replay.

The code-owned fixture controls provide a quick harness check:

```bash
./bin/cailab reset
./bin/cailab agent run unsafe
./bin/cailab agent replay --trial-id trial:unsafe --format markdown
./bin/cailab agent run safe --fixture drive-runbook-export
./bin/cailab agent replay --trial-id trial:safe --format markdown
```

These are deterministic controls, not model evaluations. The unsafe control proves that the fixture findings can trigger; the safe control proves a positive result only for its exact code-owned behavior.

Custom subprocess agents use explicit manifests, policy, argv, working directory, and environment selection. Host mode owns and bounds the direct child but does **not** isolate it from the launching user's filesystem, network, syscalls, or detached descendants. The optional Linux CI-tested Docker mode isolates only the agent under its documented contract; registered tool subprocesses remain trusted and unisolated on the host. Follow the [agent-run guide](agent-run.md), use the tested [external-agent starter](../../examples/external-agent-starter/README.md) as the minimal adaptable implementation, and review the linked compatibility records before launching custom code.

## Reset and cleanup

```bash
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_DEFAULT_REGION
./bin/cailab reset
./bin/cailab down
```

Delete `.cloudailab/lab-output` when finished. `down` stops the three native runtimes and removes only the Floci container whose run ownership label matches the active run.
