# Security policy

CloudAILab is security training and evaluation software. Responsible private reporting matters even though all built-in identities, credentials, tenants, and data are synthetic.

## Supported versions

No public version has been released yet, so `main` and development release candidates do not carry a stable support commitment. Reports against the current code are welcome. After the first public release, the latest published release will receive security fixes on a best-effort basis; older `0.x` versions will not be maintained unless an advisory says otherwise.

## Report a vulnerability privately

Use GitHub's [private vulnerability reporting form](https://github.com/msinclair25/cailab/security/advisories/new). Do not include vulnerability details in a public issue, discussion, or pull request.

Include, when possible:

- the affected commit or release and operating system;
- the scenario, command, runtime mode, and isolation mode;
- minimal reproduction steps using synthetic data;
- expected and observed authorization, isolation, or cleanup behavior;
- potential impact and any suggested mitigation;
- whether the report may affect users before a fix is available.

Never test a report against production cloud tenants, third-party systems, or data you do not own. Remove real credentials, tokens, personal data, and provider identifiers from logs and examples.

## Priority scope

Examples of security-sensitive behavior include:

- authorization or explicit-deny bypass;
- cross-tenant state or evidence confusion;
- unsafe credential, signing-key, or control-token handling;
- cleanup of resources not owned by the active CloudAILab run;
- escape from a boundary that CloudAILab explicitly claims to enforce;
- agent evidence tampering that changes deterministic scoring;
- release, dependency, runtime-image, or workflow supply-chain compromise;
- an undocumented network listener or external data transmission.

Documented provider-fidelity gaps, unsupported operations, and host-mode isolation limitations are not vulnerabilities by themselves. A contradiction between implementation and a documented security claim is in scope.

## Response process

The maintainer targets acknowledgment within five business days and an initial severity assessment within ten business days, but this solo-maintainer project provides no response-time guarantee. Valid reports will be coordinated privately until a fix or explicit mitigation is available. Public disclosure timing will consider user risk, fix availability, and reporter input.

Security fixes require a regression test, threat-model update when applicable, and a release/advisory decision proportionate to impact.
