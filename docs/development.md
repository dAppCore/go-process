---
title: Development
description: How to build, test, and contribute to go-process.
---

# Development

## Prerequisites

- **Go 1.26+** (uses Go workspaces)
- **Core CLI** (`core` binary) for running tests and quality checks
- Access to `forge.lthn.ai` (private module registry)

Ensure `GOPRIVATE` includes `forge.lthn.ai/*`:

```bash
go env -w GOPRIVATE=forge.lthn.ai/*
```

## Go Workspace

This module is part of the workspace defined at `~/Code/go.work`. After
cloning, run:

```bash
go work sync
```

## Running Tests

```bash
# All tests
core go test

# Single test
core go test --run TestService_Start

# With verbose output
core go test -v
```

Alternatively, using `go test` directly:

```bash
go test ./...
go test -run TestRunner_RunAll ./...
go test -v -count=1 ./exec/...
```

## Quality Assurance

```bash
# Format, vet, lint, test
core go qa

# Full suite (includes race detector, vulnerability scan, security audit)
core go qa full
```

Individual commands:

```bash
core go fmt          # Format code
core go vet          # Go vet
core go lint         # Lint
core go cov          # Generate coverage report
core go cov --open   # Open coverage in browser
```

## Test Naming Convention

Tests follow the `_Good`, `_Bad`, `_Ugly` suffix pattern used across the Core
ecosystem:

- **`_Good`** — happy path, expected success.
- **`_Bad`** — expected error conditions, graceful handling.
- **`_Ugly`** — panics, edge cases, degenerate inputs.

Where this pattern does not fit naturally, descriptive sub-test names are used
instead (e.g. `TestService_Start/echo_command`, `TestService_Start/context_cancellation`).

## Project Structure

```
go-process/
    .core/
        build.yaml          # Build configuration
        release.yaml        # Release configuration
    exec/
        exec.go             # Fluent command wrapper
        exec_test.go        # exec tests
        logger.go           # Logger interface and NopLogger
    actions.go              # IPC action message types
    buffer.go               # RingBuffer implementation
    buffer_test.go          # RingBuffer tests
    daemon.go               # Daemon lifecycle manager
    daemon_test.go          # Daemon tests
    go.mod                  # Module definition
    health.go               # HTTP health check server
    health_test.go          # Health server tests
    pidfile.go              # PID file single-instance lock
    pidfile_test.go         # PID file tests
    process.go              # Process type and methods
    process_test.go         # Process tests
    registry.go             # Daemon registry (JSON file store)
    registry_test.go        # Registry tests
    runner.go               # Pipeline runner (sequential, parallel, DAG)
    runner_test.go          # Runner tests
    service.go              # Core service (DI integration, lifecycle)
    service_test.go         # Service tests
    types.go                # Shared types (Status, Stream, RunOptions, Info)
```

## Adding a New Feature

1. Write the implementation in the appropriate file (or create a new one if
   the feature is clearly distinct).
2. Add tests following the naming conventions above.
3. If the feature introduces new IPC events, add the message types to
   `actions.go`.
4. Run `core go qa` to verify formatting, linting, and tests pass.
5. Commit using conventional commits: `feat(process): add XYZ support`.

## Coding Standards

- **UK English** in documentation and comments (colour, organisation, centre).
- **`declare(strict_types=1)`-equivalent**: all functions have explicit
  parameter and return types.
- **Error handling**: return errors rather than panicking. Use sentinel errors
  (`ErrProcessNotFound`, `ErrProcessNotRunning`, `ErrStdinNotAvailable`) for
  well-known conditions.
- **Thread safety**: all public types must be safe for concurrent use. Use
  `sync.RWMutex` for read-heavy workloads, `sync.Mutex` where writes dominate.
- **Formatting**: `gofmt` / `goimports` via `core go fmt`.

## Error Types

| Error | Meaning |
|-------|---------|
| `ErrProcessNotFound` | No process with the given ID exists in the service |
| `ErrProcessNotRunning` | Operation requires a running process (e.g. SendInput, Signal) |
| `ErrStdinNotAvailable` | Stdin pipe is nil (already closed or never created) |

## Build Configuration

The `.core/build.yaml` defines cross-compilation targets:

| OS | Architecture |
|----|-------------|
| linux | amd64 |
| linux | arm64 |
| darwin | arm64 |
| windows | amd64 |

Since this is a library (no binary), the build configuration is primarily
used for CI validation. The `binary` field is empty.

## Licence

EUPL-1.2. See the repository root for the full licence text.
