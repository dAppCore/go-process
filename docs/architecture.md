---
title: Architecture
description: Internals of go-process — key types, data flow, and design decisions.
---

# Architecture

This document explains how `go-process` is structured, how data flows through
the system, and the role of each major component.

## Overview

The package is organised into four layers:

1. **Service** — The Core-integrated service that owns processes and broadcasts events.
2. **Process** — An individual managed process with output capture and lifecycle state.
3. **Runner** — A pipeline orchestrator that runs multiple processes with dependency resolution.
4. **Daemon** — A higher-level abstraction for long-running services with PID files, health checks, and registry integration.

A separate `exec/` sub-package provides a thin, fluent wrapper around `os/exec`
for simple one-shot commands.

## Key Types

### Status

Process lifecycle is tracked as a state machine:

```
pending -> running -> exited
                   -> failed
                   -> killed
```

```go
type Status string

const (
    StatusPending Status = "pending"
    StatusRunning Status = "running"
    StatusExited  Status = "exited"
    StatusFailed  Status = "failed"
    StatusKilled  Status = "killed"
)
```

- **pending** — queued but not yet started (currently unused by the service,
  reserved for future scheduling).
- **running** — actively executing.
- **exited** — completed; check `ExitCode` for success (0) or failure.
- **failed** — could not be started (e.g. binary not found).
- **killed** — terminated by signal or context cancellation.

### Service

`Service` is the central type. It embeds `core.ServiceRuntime[Options]` to
participate in the Core DI container and implements both `Startable` and
`Stoppable` lifecycle interfaces.

```go
type Service struct {
    *core.ServiceRuntime[Options]
    processes map[string]*Process
    mu        sync.RWMutex
    bufSize   int
    idCounter atomic.Uint64
}
```

Key behaviours:

- **OnStartup** — currently a no-op; reserved for future initialisation.
- **OnShutdown** — iterates all running processes and calls `Kill()` on each,
  ensuring no orphaned child processes when the application exits.
- Process IDs are generated as `proc-N` using an atomic counter, guaranteeing
  uniqueness without locks.

#### Registration

The service is registered with Core via a factory function:

```go
process.NewService(process.Options{BufferSize: 2 * 1024 * 1024})
```

`NewService` returns a `func(*core.Core) (any, error)` closure — the standard
Core service factory signature. The `Options` struct is captured by the closure
and applied when Core instantiates the service.

### Process

`Process` wraps an `os/exec.Cmd` with:

- Thread-safe state (`sync.RWMutex` guards all mutable fields).
- A `RingBuffer` for output capture (configurable size, default 1 MB).
- A `done` channel that closes when the process exits, enabling `select`-based
  coordination.
- Stdin pipe access via `SendInput()` and `CloseStdin()`.
- Context-based cancellation — cancelling the context kills the process.

#### Info Snapshot

`Process.Info()` returns an `Info` struct — a serialisable snapshot of the
process state, suitable for JSON APIs or UI display:

```go
type Info struct {
    ID        string        `json:"id"`
    Command   string        `json:"command"`
    Args      []string      `json:"args"`
    Dir       string        `json:"dir"`
    StartedAt time.Time     `json:"startedAt"`
    Status    Status        `json:"status"`
    ExitCode  int           `json:"exitCode"`
    Duration  time.Duration `json:"duration"`
    PID       int           `json:"pid"`
}
```

### RingBuffer

A fixed-size circular buffer that overwrites the oldest data when full.
Thread-safe for concurrent reads and writes.

```go
rb := process.NewRingBuffer(64 * 1024) // 64 KB
rb.Write([]byte("data"))
fmt.Println(rb.String()) // "data"
fmt.Println(rb.Len())    // 4
fmt.Println(rb.Cap())    // 65536
rb.Reset()
```

The ring buffer is used internally to capture process stdout and stderr. When
a process produces more output than the buffer capacity, the oldest data is
silently overwritten. This prevents unbounded memory growth for long-running
or verbose processes.

### ACTION Messages

Four IPC message types are broadcast through `Core.ACTION()`:

| Type | When | Key Fields |
|------|------|------------|
| `ActionProcessStarted` | Process begins execution | `ID`, `Command`, `Args`, `Dir`, `PID` |
| `ActionProcessOutput` | Each line of stdout/stderr | `ID`, `Line`, `Stream` |
| `ActionProcessExited` | Process completes | `ID`, `ExitCode`, `Duration`, `Error` |
| `ActionProcessKilled` | Process is terminated | `ID`, `Signal` |

The `Stream` type distinguishes stdout from stderr:

```go
type Stream string

const (
    StreamStdout Stream = "stdout"
    StreamStderr Stream = "stderr"
)
```

## Data Flow

When `Service.StartWithOptions()` is called:

```
1. Generate unique ID (atomic counter)
2. Create context with cancel
3. Build os/exec.Cmd with dir, env, pipes
4. Create RingBuffer (unless DisableCapture is set)
5. cmd.Start()
6. Store process in map
7. Broadcast ActionProcessStarted via Core.ACTION
8. Spawn 2 goroutines to stream stdout and stderr
   - Each line is written to the RingBuffer
   - Each line is broadcast as ActionProcessOutput
9. Spawn 1 goroutine to wait for process exit
   - Waits for output goroutines to finish first
   - Calls cmd.Wait()
   - Updates process status and exit code
   - Closes the done channel
   - Broadcasts ActionProcessExited
```

The output streaming goroutines use `bufio.Scanner` with a 1 MB line buffer
to handle long lines without truncation.

## Runner

The `Runner` orchestrates multiple processes, defined as `RunSpec` values:

```go
type RunSpec struct {
    Name         string
    Command      string
    Args         []string
    Dir          string
    Env          []string
    After        []string   // dependency names
    AllowFailure bool
}
```

Three execution strategies are available:

### RunAll (dependency graph)

Processes dependencies in waves. In each wave, all specs whose dependencies
are satisfied run in parallel. If a dependency fails (and `AllowFailure` is
false), its dependents are skipped. Circular dependencies are detected and
reported as skipped with an error.

```
Wave 1: [lint, vet]        (no dependencies)
Wave 2: [test]             (depends on lint, vet)
Wave 3: [build]            (depends on test)
```

### RunSequential

Executes specs one after another. Stops on the first failure unless
`AllowFailure` is set. Remaining specs are marked as skipped.

### RunParallel

Runs all specs concurrently, ignoring the `After` field entirely. Failures
do not affect other specs.

All three strategies return a `RunAllResult` with aggregate counts:

```go
type RunAllResult struct {
    Results  []RunResult
    Duration time.Duration
    Passed   int
    Failed   int
    Skipped  int
}
```

## Daemon

The `Daemon` type manages the full lifecycle of a long-running service:

```
NewDaemon(opts) -> Start() -> Run(ctx) -> Stop()
```

### PID File

`PIDFile` provides single-instance enforcement. `Acquire()` writes the current
process PID to a file; if the file already exists and the recorded PID is still
alive (verified via `syscall.Signal(0)`), it returns an error. Stale PID files
from dead processes are automatically cleaned up.

```go
pid := process.NewPIDFile("/var/run/myapp.pid")
err := pid.Acquire()  // writes current PID, fails if another instance is live
defer pid.Release()   // removes the file
```

### Health Server

`HealthServer` exposes two HTTP endpoints:

- **`/health`** — runs all registered `HealthCheck` functions. Returns 200 if
  all pass, 503 if any fail.
- **`/ready`** — returns 200 or 503 based on the readiness flag, toggled via
  `SetReady(bool)`.

The server binds to a configurable address (use port `0` for ephemeral port
allocation in tests). `WaitForHealth()` is a polling utility that waits for
`/health` to return 200 within a timeout.

### Registry

`Registry` tracks running daemons via JSON files in a directory (default:
`~/.core/daemons/`). Each daemon is a `DaemonEntry`:

```go
type DaemonEntry struct {
    Code    string    `json:"code"`
    Daemon  string    `json:"daemon"`
    PID     int       `json:"pid"`
    Health  string    `json:"health,omitempty"`
    Project string    `json:"project,omitempty"`
    Binary  string    `json:"binary,omitempty"`
    Started time.Time `json:"started"`
}
```

The registry automatically prunes entries with dead PIDs on `List()` and
`Get()`. When a `Daemon` is configured with a `Registry`, it auto-registers
on `Start()` and auto-unregisters on `Stop()`.

File naming convention: `{code}-{daemon}.json` (slashes replaced with dashes).

## exec Sub-Package

The `exec` package (`forge.lthn.ai/core/go-process/exec`) provides a fluent
wrapper around `os/exec` for simple, one-shot commands that do not need Core
integration:

```go
import "forge.lthn.ai/core/go-process/exec"

// Fluent API
err := exec.Command(ctx, "go", "build", "./...").
    WithDir("/path/to/project").
    WithEnv([]string{"CGO_ENABLED=0"}).
    WithLogger(myLogger).
    Run()

// Get output
out, err := exec.Command(ctx, "git", "status").Output()

// Combined stdout + stderr
out, err := exec.Command(ctx, "make").CombinedOutput()

// Quiet mode (suppresses stdout, includes stderr in error)
err := exec.RunQuiet(ctx, "go", "vet", "./...")
```

### Logging

Commands are automatically logged at debug level before execution and at error
level on failure. The logger interface is minimal:

```go
type Logger interface {
    Debug(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
}
```

A `NopLogger` (the default) discards all messages. Use `SetDefaultLogger()` to
set a package-wide logger, or `WithLogger()` for per-command overrides.

## Thread Safety

All public types are safe for concurrent use:

- `Service` — `sync.RWMutex` protects the process map; atomic counter for IDs.
- `Process` — `sync.RWMutex` protects mutable state.
- `RingBuffer` — `sync.RWMutex` on all read/write operations.
- `PIDFile` — `sync.Mutex` on acquire/release.
- `HealthServer` — `sync.Mutex` on check list and readiness flag.
- `Registry` — filesystem-level isolation (one file per daemon).
- Global singleton — `atomic.Pointer` for lock-free reads.
