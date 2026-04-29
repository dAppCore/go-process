# Agent Guide

This repository implements the Core process service. Treat the audit script in
`/Users/snider/Code/core/go/tests/cli/v090-upgrade/audit.sh` as the active work
provider: a change is complete only when every counter is zero and the verdict is
`COMPLIANT`.

## Repository Shape

The root package `dappco.re/go/process` contains the service, process model,
daemon support, PID-file handling, ring buffer, program helper, and pipeline
runner. The `exec` subpackage provides a small command wrapper with logging.
`pkg/api` exposes process and daemon operations through Gin routes and WebSocket
channels.

Tests and examples are file-aware. If a public symbol lives in `service.go`, its
triplet tests belong in `service_test.go` and its examples belong in
`service_example_test.go`. Do not create versioned, aggregate, or AX7-style dump
files. Example output should be real, stable stdout printed with Core helpers.

## Local Rules

Use `dappco.re/go` wrappers for banned stdlib conveniences such as formatting,
JSON, path, filesystem, strings, bytes, and environment operations. Result values
must be inspected with `OK` or converted deliberately at API boundaries. Avoid
discarding process, filesystem, and shutdown results in production code.

Native process behavior is tested with real local commands such as `true`,
`echo`, `cat`, and `sleep`. Keep new tests fast, use temporary directories for
filesystem state, and clean up long-running processes explicitly.
