---
title: go-process
description: Process management with Core IPC integration for Go applications.
---

# go-process

`forge.lthn.ai/core/go-process` is a process management library that provides
spawning, monitoring, and controlling external processes with real-time output
streaming via the Core ACTION (IPC) system. It integrates directly with the
[Core DI framework](https://forge.lthn.ai/core/go) as a first-class service.

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
    framework "forge.lthn.ai/core/go/pkg/core"
    "forge.lthn.ai/core/go-process"
)

// Create a Core instance with the process service
c, err := framework.New(
    framework.WithName("process", process.NewService(process.Options{})),
)
if err != nil {
    log.Fatal(err)
}

// Retrieve the typed service
svc, err := framework.ServiceFor[*process.Service](c, "process")
if err != nil {
    log.Fatal(err)
}
```

### Run a Command

```go
// Fire-and-forget (async)
proc, err := svc.Start(ctx, "go", "test", "./...")
if err != nil {
    return err
}
<-proc.Done()
fmt.Println(proc.Output())

// Synchronous convenience
output, err := svc.Run(ctx, "echo", "hello world")
```

### Listen for Events

Process lifecycle events are broadcast through Core's ACTION system:

```go
c.RegisterAction(func(c *framework.Core, msg framework.Message) error {
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
    return nil
})
```

### Global Convenience API

For applications that only need a single process service, a global singleton
is available:

```go
// Initialise once at startup
process.Init(coreInstance)

// Then use package-level functions anywhere
proc, _ := process.Start(ctx, "ls", "-la")
output, _ := process.Run(ctx, "date")
procs := process.List()
running := process.Running()
```

## Package Layout

| Path | Description |
|------|-------------|
| `*.go` (root) | Core process service, types, actions, runner, daemon, health, PID file, registry |
| `exec/` | Lightweight command wrapper with fluent API and structured logging |

## Module Information

| Field | Value |
|-------|-------|
| Module path | `forge.lthn.ai/core/go-process` |
| Go version | 1.26.0 |
| Licence | EUPL-1.2 |

## Dependencies

| Module | Purpose |
|--------|---------|
| `forge.lthn.ai/core/go` | Core DI framework (`ServiceRuntime`, `Core.ACTION`, lifecycle interfaces) |
| `github.com/stretchr/testify` | Test assertions (test-only) |

The package has no other runtime dependencies beyond the Go standard library
and the Core framework.
