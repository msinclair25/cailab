---
title: AWS Cross-Account Acquisition Data Lab
status: active
last_reviewed: 2026-07-12
---

# AWS cross-account acquisition data lab

## Outcome

This lab teaches a narrow but realistic IAM workflow: discover an excessive cross-account role trust, prove its S3 impact with standard AWS tooling, remove the unintended principal, and preserve the intended principal. CloudAILab rebuilds the trust edges from live IAM state before every path query and verification.

This M1 scenario is training software, not a complete AWS emulator. Read the [operation-level compatibility matrix](../07-compatibility/aws-floci-1.5.32.md) before treating behavior as AWS-compatible.

## Prerequisites

- Go 1.25.12 or newer
- Docker 20.10 or newer with a running engine
- AWS CLI v2 for the interactive commands

Build and check the host:

```bash
mkdir -p ./bin
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor aws-cross-account
```

## 1. Start the range

```bash
./bin/cailab up aws-cross-account
./bin/cailab status
./bin/cailab mission
```

Record the random loopback endpoint printed by `up` or `status`, such as `http://127.0.0.1:60654`. Replace `PORT` below with its port number.

CloudAILab pulls only the allowlisted Floci 1.5.32 image pinned in the scenario schema. It runs the container as UID `1001`, drops all Linux capabilities, enables `no-new-privileges`, limits memory and CPU, and publishes port 4566 only on IPv4 loopback. The container is not described as an agent sandbox and may still have Docker bridge-network egress.

## 2. Inspect the vulnerable path

```bash
./bin/cailab graph path aws:parent-root aws:acquisition-data
./bin/cailab verify
```

The initial verification must report one pass and one failure. A failing exit code is intentional: the parent account still reaches the restricted bucket through `AcquisitionDataReader`.

## 3. Prove access with the AWS CLI

All credentials below are synthetic and valid only against the local endpoint.

```bash
export AWS_ENDPOINT_URL=http://127.0.0.1:PORT
export AWS_DEFAULT_REGION=us-east-1
export AWS_EC2_METADATA_DISABLED=true
export AWS_ACCESS_KEY_ID=111111111111
export AWS_SECRET_ACCESS_KEY=cailab-local-only

aws sts assume-role \
  --role-arn arn:aws:iam::222222222222:role/AcquisitionDataReader \
  --role-session-name cailab-learner \
  --endpoint-url "$AWS_ENDPOINT_URL" \
  --query Credentials \
  --output json
```

Copy the returned `AccessKeyId`, `SecretAccessKey`, and `SessionToken` into the corresponding `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` environment variables. Then retrieve the synthetic object:

```bash
aws s3 cp \
  s3://cailab-acquisition-data/restricted/acquisition-summary.txt \
  - \
  --endpoint-url "$AWS_ENDPOINT_URL"
```

The output is visibly labeled synthetic training data.

## 4. Narrow the trust policy

Switch to the synthetic bootstrap identity for the acquired account and apply the provided remediation policy:

```bash
export AWS_ACCESS_KEY_ID=222222222222
export AWS_SECRET_ACCESS_KEY=cailab-local-only
unset AWS_SESSION_TOKEN

aws iam update-assume-role-policy \
  --role-name AcquisitionDataReader \
  --policy-document file://scenarios/aws-cross-account/remediation/acquired-only-trust.json \
  --endpoint-url "$AWS_ENDPOINT_URL"
```

This removes account `111111111111` while retaining account `222222222222`.

## 5. Verify the result

```bash
./bin/cailab graph path aws:parent-root aws:acquisition-data
./bin/cailab graph path aws:acquired-root aws:acquisition-data
./bin/cailab verify
```

Expected outcome:

- No parent-account path is found.
- The acquired-account reader path remains visible.
- Both deterministic invariants pass.

## Reset and cleanup

`reset` destroys and recreates the provider runtime from the compiled scenario, restoring the intentionally vulnerable trust:

```bash
./bin/cailab reset
./bin/cailab verify
```

Stop and remove the runtime when finished:

```bash
./bin/cailab down
```

CloudAILab verifies the run label before removing a container. If startup was interrupted before runtime metadata was persisted, cleanup discovers containers using CloudAILab's run and ownership labels.
