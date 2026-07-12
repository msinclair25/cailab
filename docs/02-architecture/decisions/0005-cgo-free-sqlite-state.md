---
title: "ADR-0005: CGO-Free SQLite State"
status: accepted
date: 2026-07-11
---

# ADR-0005: CGO-free SQLite state

## Context

CloudAILab needs transactional local state, forward-only migrations, snapshots, and a single-file database without requiring users to operate a server. The target release is a cross-platform Go binary. A CGO-based SQLite driver complicates cross-compilation and requires a platform C toolchain.

## Decision

CloudAILab will use `database/sql` with the CGO-free `modernc.org/sqlite` driver. The initial accepted version is `v1.53.0`, with transitive versions recorded in `go.mod` and `go.sum`.

Database schema changes use numbered, forward-only migrations executed transactionally. The application rejects state created by a newer unsupported schema version.

## Consequences

### Positive

- No C compiler or platform SQLite library is required to build or run CAL.
- The same database interface is available across supported operating systems.
- State remains a portable local file with SQLite transaction semantics.

### Negative

- The pure-Go driver has a larger dependency and compile-time footprint.
- The driver's generated platform code and `modernc.org/libc` dependency require careful version upgrades.
- Cross-platform behavior must be verified in CI rather than assumed.

## Validation

- State migration and lifecycle tests run against a real temporary SQLite database.
- CI builds on Linux, macOS, and Windows.
- `go mod verify`, `govulncheck`, and dependency review cover the driver and transitive modules.
- Driver upgrades require migration, lifecycle, race, and cross-platform build checks.

## Source

- [`modernc.org/sqlite` package documentation](https://pkg.go.dev/modernc.org/sqlite)
