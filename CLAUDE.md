# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`forge.lthn.ai/core/go-process` is the process management framework for CoreGO. It handles process execution (spawn, monitor, stream, kill), daemon lifecycle (PID files, health checks, graceful shutdown, registry), and pipeline orchestration (parallel, sequential, or DAG-ordered multi-process runs). All process events broadcast via Core IPC actions.

## Commands

```bash
core go test              # Run all tests
core go test --run Name   # Single test
core go fmt               # Format
core go lint              # Lint
core go vet               # Vet
```

## Architecture

The package has three layers, all in the root `process` package (plus a `exec` subpackage):

### Layer 1: Process Execution (service.go, process.go, process_global.go)

`Service` is a Core service (`*core.ServiceRuntime[Options]`) that manages all `Process` instances. It spawns subprocesses, pipes stdout/stderr through goroutines, captures output to a `RingBuffer`, and broadcasts IPC actions (`ActionProcessStarted`, `ActionProcessOutput`, `ActionProcessExited`, `ActionProcessKilled` — defined in actions.go).

`process_global.go` provides package-level convenience functions (`Start`, `Run`, `Kill`, `List`) that delegate to a global `Service` singleton initialized via `Init(core)`. Follows the same pattern as Go's `i18n` package.

### Layer 2: Daemon Lifecycle (daemon.go, pidfile.go, health.go, registry.go)

`Daemon` composes three independent components into a lifecycle manager:
- **PIDFile** (pidfile.go) — single-instance enforcement via PID file with stale-PID detection (`Signal(0)`)
- **HealthServer** (health.go) — HTTP `/health` (liveness) and `/ready` (readiness) endpoints
- **Registry** (registry.go) — tracks running daemons as JSON files in `~/.core/daemons/`, auto-prunes dead PIDs on `List()`

`Daemon.Start()` acquires PID → starts health → registers. `Daemon.Run(ctx)` blocks until context cancellation, then `Stop()` reverses in order (unregister → release PID → stop health).

### Layer 3: Pipeline Orchestration (runner.go)

`Runner` executes multiple `RunSpec`s with three modes: `RunAll` (DAG-aware, parallelizes independent specs in waves), `RunSequential` (stops on first failure), `RunParallel` (ignores dependencies). Dependency failures cascade via `Skipped` status unless `AllowFailure` is set.

### exec Subpackage (exec/)

Builder-pattern wrapper around `os/exec` with structured logging via a pluggable `Logger` interface. `NopLogger` is the default. Separate from the main package — no Core/IPC integration, just a convenience wrapper.

## Key Patterns

- **Core integration**: `Service` embeds `*core.ServiceRuntime[Options]` and uses `s.Core().ACTION(...)` to broadcast typed action messages. Tests create a Core instance via `framework.New(framework.WithName("process", NewService(...)))`.
- **Output capture**: All process output goes through a fixed-size `RingBuffer` (default 1MB). Oldest data is silently overwritten when full. Set `RunOptions.DisableCapture` to skip buffering for long-running processes where output is only streamed via IPC.
- **Process lifecycle**: Status transitions are `StatusPending → StatusRunning → StatusExited|StatusFailed|StatusKilled`. The `done` channel closes on exit; use `<-proc.Done()` or `proc.Wait()`.
- **Detach / process group isolation**: Set `RunOptions.Detach = true` to run the subprocess in its own process group (`Setpgid`). Detached processes use `context.Background()` so they survive parent context cancellation and parent death.
- **Graceful shutdown**: `Service.OnShutdown` kills all running processes. `Daemon.Stop()` performs ordered teardown: sets health to not-ready → shuts down health server → releases PID file → unregisters from registry. `DaemonOptions.ShutdownTimeout` (default 30 s) bounds the shutdown context.
- **Auto-registration**: Pass a `Registry` and `RegistryEntry` in `DaemonOptions` to automatically register the daemon on `Start()` and unregister on `Stop()`.
- **PID liveness checks**: Both `PIDFile` and `Registry` use `proc.Signal(syscall.Signal(0))` to check if a PID is alive before trusting stored state.
- **Error handling**: All errors MUST use `coreerr.E()` from `go-log` (imported as `coreerr`), never `fmt.Errorf` or `errors.New`. Sentinel errors are package-level vars created with `coreerr.E("", "message", nil)`.

## Dependencies

- `forge.lthn.ai/core/go/pkg/core` — Core DI framework, IPC actions, `ServiceRuntime`
- `forge.lthn.ai/core/go-log` — Structured error constructor (`coreerr.E()`)
- `forge.lthn.ai/core/go-io` — Filesystem abstraction (`coreio.Local`) used by PIDFile and Registry
- `github.com/stretchr/testify` — test assertions (require/assert)

## Testing

- Tests spawn real subprocesses (typically `echo`, `sleep`, `cat`). Use short timeouts to avoid hanging.
- Test helper `newTestService(t)` in service_test.go creates a Core instance with a small buffer (1024 bytes) and returns both the `Service` and `Core`.
- Uses `github.com/stretchr/testify` with `require` for setup and `assert` for checks.
