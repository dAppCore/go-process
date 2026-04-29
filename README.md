# go-process

`go-process` is the process-management service for Core applications. It wraps
external command execution, lifecycle tracking, output capture, daemon PID files,
health endpoints, and REST/WebSocket provider integration around the `dappco.re/go`
runtime conventions.

The package is intended for applications that need to start and observe local
commands without scattering process handling across the codebase. A `Service`
owns the process table, streams output through Core actions, exposes snapshots
through `Info`, and provides helpers for stdin, waiting, killing, signalling, and
cleanup. Higher-level packages build on the same primitives: `Runner` executes
dependent command pipelines, `Daemon` manages PID files and health probes, and
`pkg/api` mounts the service into Gin routes.

## Quick Start

```go
coreApp := core.New()
raw, err := process.NewService(process.Options{})(coreApp)
if err != nil {
    return err
}

svc := raw.(*process.Service)
proc, err := svc.Start(context.Background(), "echo", "hello")
if err != nil {
    return err
}
<-proc.Done()
core.Println(proc.Output())
```

For command pipelines, construct a `Runner` from the same service and submit
`RunSpec` values. Dependencies are declared with `After`, and failed dependencies
skip downstream work unless the dependency allows failure.

## Development

This repository follows the Core v0.9 compliance shape. Public symbols have
file-aware tests and examples in sibling files, Core wrappers replace banned
stdlib convenience packages, and Result-returning operations must be checked.

Before handing off changes, run:

```sh
GOWORK=off go mod tidy
GOWORK=off go vet ./...
GOWORK=off go test -count=1 ./...
gofmt -l .
bash /Users/snider/Code/core/go/tests/cli/v090-upgrade/audit.sh .
```
