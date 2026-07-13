---
title: "ADR-0024: Embedded Built-In Scenario Catalog"
status: accepted
date: 2026-07-12
---

# ADR-0024: Embedded built-in scenario catalog

## Context

CloudAILab's supported deployment is one `cailab` executable, plus Docker only for container-backed scenarios. The first M4 release-candidate smoke test verified every archive and ran `cailab version`, but `cailab scenario list` failed because the default catalog was the relative filesystem directory `scenarios`. A release archive contains the executable and README, not a repository checkout, so scenario discovery accidentally depended on the caller's working directory.

An ambient relative catalog is also a source-selection problem. A same-named directory in the working directory could replace the repository-owned scenarios without the user explicitly choosing it. Copying scenarios beside every archive would fix the immediate missing-file error but would make installation layout part of runtime correctness and weaken the single-binary deployment contract.

## Decision

1. The repository-owned `*/scenario.yaml` files under `scenarios/` are compiled into the executable with Go's standard `embed.FS` support.
2. An empty catalog root means the immutable built-in catalog. `doctor`, `scenario list`, `scenario show`, and `up` use that catalog by default and do not search an ambient `./scenarios` directory.
3. A user can deliberately select custom content by supplying an existing scenario file or an explicit `--root` or `--scenario-root` directory. Explicit filesystem content is never merged with or allowed to shadow the built-in catalog implicitly.
4. Embedded and external manifests pass through the same strict decoder, typed validation, deterministic compiler, and provider-runtime allowlist.
5. Embedding is a distribution and source-selection boundary, not a confidentiality control. Built-in manifests and their verification rules remain inspectable in the public repository and can be extracted from the executable by the launching OS account. Agent-facing ground-truth disclosure and isolation controls remain separate concerns.
6. Scenario manifests remain declarative data. Embedding does not add hooks, scripts, templates, or implicit execution.

## Consequences

- A release executable can list, show, validate, and start its built-in scenarios from any working directory.
- Built-in scenario changes require rebuilding the executable and become part of the binary covered by archive checksums and provenance.
- Custom scenario development remains supported but requires an explicit source selection, which is visible in the invocation.
- The executable grows by approximately the size of the scenario manifests. This is small relative to the binary and avoids a platform-specific installation layout.
- Users must not treat embedded ground truth as secret from the local account running CloudAILab or a host-mode agent with that account's filesystem authority.

## Validation

- Scenario tests enumerate and load the built-in catalog without a filesystem root and separately cover explicit custom catalogs and files.
- Public CLI tests list, show, start, and stop a built-in scenario from a package working directory that has no `scenarios` child.
- Release smoke jobs extract only the archive, verify its checksums, and enumerate the built-in catalog without a repository checkout or external scenario directory.

## Sources

- [Go `embed` package](https://pkg.go.dev/embed)
- [Go 1.16 file-embedding release note](https://go.dev/doc/go1.16#library-embed)
