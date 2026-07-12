---
title: Local Development OIDC Lab
status: m2-complete
last_reviewed: 2026-07-12
---

# Local development OIDC lab

## Outcome

This lab exercises issuer discovery, JWKS, a synthetic Authorization Code flow, signed ID and access tokens, local validation, replay prevention, and signing-key rollover. The same issuer participates in the completed `acquisition-agent` Google → Microsoft → AWS federation scenario.

This is a [documented local development profile](../07-compatibility/local-oidc-profile.md), not production authentication. The issuer is loopback HTTP, and `cailab_subject` selects a declared synthetic subject without proving a human logged in.

## Prerequisites

- Go 1.25.12 or newer
- `curl`
- `jq` for extracting tokens from the JSON response
- No Docker, cloud identity provider, proxy, or certificate installation

```bash
go build -o ./bin/cailab ./cmd/cailab
./bin/cailab doctor local-oidc
./bin/cailab up local-oidc
./bin/cailab status
```

Export the random `OIDC endpoint` printed by `up` or `status`:

```bash
export CAILAB_ISSUER=http://127.0.0.1:PORT
export CAILAB_CLIENT_ID=cailab-automation-client
export CAILAB_CLIENT_SECRET=cailab-synthetic-local-oidc-secret
export CAILAB_REDIRECT_URI=http://127.0.0.1:7777/callback
```

All values are synthetic and scoped to this local lab.

## 1. Discover the issuer and keys

```bash
curl --fail --silent --show-error \
  "$CAILAB_ISSUER/.well-known/openid-configuration" | jq

curl --fail --silent --show-error \
  "$CAILAB_ISSUER/jwks" | jq
```

Confirm that `issuer` exactly equals the printed endpoint, `jwks_uri` is the same origin plus `/jwks`, and the single key advertises `RSA`, `sig`, and `RS256`.

## 2. Request a synthetic authorization code

```bash
curl --silent --show-error --output /dev/null --dump-header - \
  --get "$CAILAB_ISSUER/authorize" \
  --data-urlencode "response_type=code" \
  --data-urlencode "client_id=$CAILAB_CLIENT_ID" \
  --data-urlencode "redirect_uri=$CAILAB_REDIRECT_URI" \
  --data-urlencode "scope=openid profile email" \
  --data-urlencode "state=local-lab" \
  --data-urlencode "nonce=local-nonce" \
  --data-urlencode "cailab_subject=identity-engineer"
```

The issuer returns HTTP 302. Copy the `code` value from the `Location` header and export it:

```bash
export CAILAB_CODE=PASTE_CODE_HERE
```

The redirect URI is compared exactly. CloudAILab sends the redirect response but does not run a listener on port `7777`.

## 3. Exchange the one-time code

Use an owner-readable temporary directory because the response contains synthetic credentials:

```bash
umask 077
mkdir -p .cloudailab/lab-output

curl --fail --silent --show-error \
  --request POST \
  --user "$CAILAB_CLIENT_ID:$CAILAB_CLIENT_SECRET" \
  --header "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CAILAB_CODE" \
  --data-urlencode "redirect_uri=$CAILAB_REDIRECT_URI" \
  "$CAILAB_ISSUER/token" \
  > .cloudailab/lab-output/tokens.json

jq -r .id_token .cloudailab/lab-output/tokens.json \
  > .cloudailab/lab-output/id.jwt
jq -r .access_token .cloudailab/lab-output/tokens.json \
  > .cloudailab/lab-output/access.jwt
```

Repeating the exchange with the same code returns `invalid_grant`; codes are one-time and short-lived.

## 4. Validate both tokens locally

```bash
./bin/cailab identity validate \
  --type id \
  --audience "$CAILAB_CLIENT_ID" \
  --token-file .cloudailab/lab-output/id.jwt

./bin/cailab identity validate \
  --type access \
  --audience https://identity-api.cailab.local \
  --token-file .cloudailab/lab-output/access.jwt
```

Validation retrieves discovery and JWKS only from the active loopback issuer, then checks algorithm, type, signature, exact issuer, audience, issue/expiry times, subject, and token ID. It prints claims only after those checks pass.

## 5. Rotate the signing key

```bash
./bin/cailab identity rotate
curl --fail --silent --show-error "$CAILAB_ISSUER/jwks" | jq
```

Two verification keys remain published: the retired key for existing unexpired tokens and the new active key. The access token from step 3 still validates. Repeat steps 2–4 to observe that new tokens use the new key.

## 6. Verify the canonical identity path

```bash
./bin/cailab graph path principal:identity-engineer resource:identity-api
./bin/cailab graph path principal:unregistered-user resource:identity-api
./bin/cailab verify
```

The registered subject has the intended client-mediated API path, while an undeclared subject has no path. Signed tokens do not override the canonical graph or deterministic verification result.

## External agents

A local agent can complete the same HTTP flow. The [acquisition-agent lab](acquisition-agent-lab.md) documents the scoped Microsoft/AWS federation consumer. The agent remains an independently launched process with whatever host authority you grant it; this lab does not sandbox it.

## Reset and cleanup

`reset` creates a fresh issuer URL and signing key and restores the compiled scenario. Tokens from the old issuer are intentionally outside the new active run.

```bash
./bin/cailab reset
./bin/cailab down
```

Remove `.cloudailab/lab-output` when you no longer need the synthetic tokens.
