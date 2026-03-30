---
title: go-process
description: Process management with Core IPC integration for Go applications.
---

# go-process

`dappco.re/go/core/process` is a process management library that provides
spawning, monitoring, and controlling external processes with real-time output
streaming via the Core ACTION (IPC) system. It integrates directly with the
[Core DI framework](https://dappco.re/go/core) as a first-class service.

## Features

- Spawn and manage external processes with full lifecycle tracking
- Real-time stdout/stderr streaming via Core IPC actions
- Ring buffer output capture (default 1 MB, configurable)
- Process pipeline runner with dependency graphs, sequential, and parallel modes
- Daemon mode with PID file locking, health check HTTP server, and graceful shutdown
- Daemon registry for tracking running instances across the system
- Lightweight `exec` sub-package for one-shot command execution with logging
- Thread-safe throughout; designed for concurrent use

## Quick Start

### Register with Core

```go
import (
    "context"
    "dappco.re/go/core"
    "dappco.re/go/core/process"
)

// Create a Core instance with the process service registered.
c := core.New(core.WithService(process.Register))

// Retrieve the typed service
svc, ok := core.ServiceFor[*process.Service](c, "process")
if !ok {
    panic("process service not registered")
}
```

### Run a Command

```go
// Fire-and-forget (async)
start := svc.Start(ctx, "go", "test", "./...")
if !start.OK {
    return start.Value.(error)
}
proc := start.Value.(*process.Process)
<-proc.Done()
fmt.Println(proc.Output())

// Synchronous convenience
run := svc.Run(ctx, "echo", "hello world")
if run.OK {
    fmt.Println(run.Value.(string))
}
```

### Listen for Events

Process lifecycle events are broadcast through Core's ACTION system:

```go
c.RegisterAction(func(c *core.Core, msg core.Message) core.Result {
    switch m := msg.(type) {
    case process.ActionProcessStarted:
        fmt.Printf("Started: %s (PID %d)\n", m.Command, m.PID)
    case process.ActionProcessOutput:
        fmt.Print(m.Line)
    case process.ActionProcessExited:
        fmt.Printf("Exit code: %d (%s)\n", m.ExitCode, m.Duration)
    case process.ActionProcessKilled:
        fmt.Printf("Killed with %s\n", m.Signal)
    }
    return core.Result{OK: true}
})
```

### Permission Model

Core's process primitive delegates to named actions registered by this module.
Without `process.Register`, `c.Process().Run(...)` fails with `OK: false`.

```go
c := core.New()
r := c.Process().Run(ctx, "echo", "blocked")
fmt.Println(r.OK) // false

c = core.New(core.WithService(process.Register))
_ = c.ServiceStartup(ctx, nil)
r = c.Process().Run(ctx, "echo", "allowed")
fmt.Println(r.OK) // true
```

## Package Layout

| Path | Description |
|------|-------------|
| `*.go` (root) | Core process service, types, actions, runner, daemon, health, PID file, registry |
| `exec/` | Lightweight command wrapper with fluent API and structured logging |

## Module Information

| Field | Value |
|-------|-------|
| Module path | `dappco.re/go/core/process` |
| Go version | 1.26.0 |
| Licence | EUPL-1.2 |

## Dependencies

| Module | Purpose |
|--------|---------|
| `dappco.re/go/core` | Core DI framework (`ServiceRuntime`, `Core.ACTION`, lifecycle interfaces) |
| `github.com/stretchr/testify` | Test assertions (test-only) |

The package has no other runtime dependencies beyond the Go standard library
and the Core framework.
