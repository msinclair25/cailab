# Support

CloudAILab is a portfolio and learning project maintained on a best-effort basis. It currently has no public stable release, paid support, service-level agreement, or production-operation commitment.

## Before requesting help

1. Run `cailab version` and `cailab doctor <scenario>`.
2. Check the relevant [compatibility record](docs/07-compatibility) and [troubleshooting guide](docs/07-guides/troubleshooting.md).
3. Reproduce with a built-in scenario and synthetic data when possible.
4. Confirm that the behavior is part of a documented operation or scenario contract.

## Requesting help

Use a [GitHub issue](https://github.com/msinclair25/cailab/issues) for reproducible bugs, documentation defects, and scoped feature proposals. Include:

- CloudAILab version or commit;
- operating system and architecture;
- Docker Engine version when relevant;
- scenario and exact command;
- expected and observed behavior;
- sanitized diagnostics and minimal reproduction steps.

Do not post credentials, control documents, signing keys, provider data, unredacted agent transcripts, or security vulnerability details. Use the private process in [SECURITY.md](SECURITY.md) for suspected vulnerabilities.

## Compatibility boundary

Only operations listed in the repository's compatibility records are supported. CloudAILab does not promise general AWS, Microsoft Graph, Google Workspace, OIDC, Docker, Podman, agent-framework, or model parity. Linux, macOS, and Windows release coverage is bounded by the [release verification matrix](docs/07-guides/release-verification.md); container-dependent workflows require a documented Docker configuration.
