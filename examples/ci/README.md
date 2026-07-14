# Least-privilege CI example

The [GitHub Actions example](github-actions.yml) validates, starts, verifies, and stops the synthetic data-only scenario, then uploads the deterministic JUnit report. It requires only repository read permission. It uses no cloud account, hosted model, production credential, secret, provider listener, or Docker runtime.

Copy the workflow into `.github/workflows/cailab-verify.yml` in a repository containing CloudAILab source and change `SCENARIO` to an explicit reviewed manifest. Keep action revisions immutable, retain `contents: read`, and do not add cloud/model credentials to this no-runtime job.

`cailab verify` returns `0` when every invariant passes, `3` when verification completes with findings, and `1` for usage or operational failure. The JUnit file contains one testcase per deterministic invariant and one failure per failed invariant. Cleanup uses the normal owned lifecycle even when an earlier step fails; the ephemeral runner discards the remaining synthetic state directory after the job.

Agent replay and campaign reports remain evidence metrics rather than a single pass/fail gate. Consume their JSON projection separately instead of converting completion or one rate into an invented JUnit verdict.
