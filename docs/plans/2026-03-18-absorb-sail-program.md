# Plan: Absorb Sail Program

**Date:** 2026-03-18
**Branch:** agent/implement-the-plan-at-docs-plans-2026-03

## Overview

Add a `Program` struct to the `process` package that locates a named binary on PATH and provides lightweight run helpers. This absorbs the "sail program" pattern — a simple way to find and invoke a known CLI tool without wiring the full Core IPC machinery.

## API

```go
// Program represents a named executable located on the system PATH.
type Program struct {
    Name string // binary name, e.g. "go", "node"
    Path string // absolute path resolved by Find()
}

// Find resolves the program's absolute path via exec.LookPath.
func (p *Program) Find() error

// Run executes the program with args in the current working directory.
// Returns combined stdout+stderr output and any error.
func (p *Program) Run(ctx context.Context, args ...string) (string, error)

// RunDir executes the program with args in dir.
// Returns combined stdout+stderr output and any error.
func (p *Program) RunDir(ctx context.Context, dir string, args ...string) (string, error)
```

## Tasks

### Task 1: Implement Program in program.go

- Create `program.go` in the root `process` package
- Add `ErrProgramNotFound` sentinel error using `coreerr.E`
- Add `Program` struct with exported `Name` and `Path` fields
- Implement `Find() error` using `exec.LookPath`; if `Name` is empty return error
- Implement `RunDir(ctx, dir, args...) (string, error)` using `exec.CommandContext`
  - Capture combined stdout+stderr into a `bytes.Buffer`
  - Set `cmd.Dir` if `dir` is non-empty
  - Wrap run errors with `coreerr.E`
  - Trim trailing whitespace from output
- Implement `Run(ctx, args...) (string, error)` as `p.RunDir(ctx, "", args...)`
- Commit: `feat(process): add Program struct`

### Task 2: Write tests in program_test.go

- Create `program_test.go` in the root `process` package
- Test `Find()` succeeds for a binary that exists on PATH (`echo`)
- Test `Find()` fails for a binary that does not exist
- Test `Run()` executes and returns output
- Test `RunDir()` runs in the specified directory (verify via `pwd` or `ls`)
- Test `Run()` before `Find()` still works (falls back to `Name`)
- Commit: `test(process): add Program tests`

## Acceptance Criteria

- `go test ./...` passes with zero failures
- No `fmt.Errorf` or `errors.New` — only `coreerr.E`
- `Program` is in the root `process` package (not exec subpackage)
- `Run` delegates to `RunDir` — no duplication
