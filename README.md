<!-- SPDX-License-Identifier: EUPL-1.2 -->

# go-process

> Process orchestration — daemons, runners, registry, pidfiles, IPC bus

[![CI](https://github.com/dappcore/go-process/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/dappcore/go-process/actions/workflows/ci.yml)
[![Quality Gate](https://sonarcloud.io/api/project_badges/measure?project=dappcore_go-process&metric=alert_status)](https://sonarcloud.io/dashboard?id=dappcore_go-process)
[![Coverage](https://codecov.io/gh/dappcore/go-process/branch/dev/graph/badge.svg)](https://codecov.io/gh/dappcore/go-process)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=dappcore_go-process&metric=security_rating)](https://sonarcloud.io/dashboard?id=dappcore_go-process)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=dappcore_go-process&metric=sqale_rating)](https://sonarcloud.io/dashboard?id=dappcore_go-process)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=dappcore_go-process&metric=reliability_rating)](https://sonarcloud.io/dashboard?id=dappcore_go-process)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=dappcore_go-process&metric=code_smells)](https://sonarcloud.io/dashboard?id=dappcore_go-process)
[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=dappcore_go-process&metric=ncloc)](https://sonarcloud.io/dashboard?id=dappcore_go-process)
[![Go Reference](https://pkg.go.dev/badge/dappco.re/go/go-process.svg)](https://pkg.go.dev/dappco.re/go/go-process)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](https://eupl.eu/1.2/en/)


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
