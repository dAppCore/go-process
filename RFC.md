# RFC — Windows support for go-process

**Status:** draft · **Author:** Cladius · **Date:** 2026-05-12
**Triggered by:** Lethean Desktop (`lthn/desktop`) Windows cross-compile blocked at `dappco.re/go/process` syscall references. See parent: [`lthn/desktop` Windows build audit](https://github.com/LetheanNetwork/desktop/issues).

## Problem

`GOOS=windows go build` fails because four call sites in `go/process.go` and `go/service.go` use Unix-only `syscall.*` primitives that don't exist in `syscall` on Windows:

```
process.go:173,191,241,269,272,289,299   syscall.Kill(-pid, SIGKILL)        [process-group kill]
process.go:249,272,280-299               syscall.Signal               (used with -pid form)
service.go:176                           SysProcAttr{Setpgid: true}        [process-group create]
service.go:470,500,513                   syscall.Kill(pid, sig)            [direct PID signal]
service.go:978                           WaitStatus.Signaled()             [exit-cause discrimination]
```

The repo **already has the platform-split pattern** for one helper:

```
pidfile_unix.go     //go:build !windows
pidfile_windows.go  //go:build windows
```

…which defines `processSignal(pid int, sig syscall.Signal) core.Result` — Unix uses `syscall.Kill`, Windows returns `Ok(nil)` for positive PIDs (best-effort stub). This RFC extends that same shape to the remaining call sites.

## Scope

In: `go/process.go`, `go/service.go`, plus two new files.

Out of scope:
- Full Job Object orchestration (Phase 2 — see §6)
- Cross-platform signal-name parsing (consumers pass `syscall.Signal` directly today)
- Renaming any public API

## Constraints

- Maintain the existing `processSignal()` helper signature — it's already exported in spirit (lowercase but used across files).
- Keep all `core.Result` return types.
- No test-suite signature changes — existing `process_test.go` / `service_test.go` should run on both platforms (with `//go:build !windows` on currently Unix-only test cases where needed).

## Design

Three new lowercase package helpers, each with a `_unix.go` and `_windows.go` pair:

| Helper | Unix impl | Windows impl (Phase 1 — stub) |
|---|---|---|
| `processSignal(pid, sig) core.Result` | `syscall.Kill(pid, sig)` — already exists | already stubbed |
| `processKillGroup(pid int) core.Result` | `syscall.Kill(-pid, syscall.SIGKILL)` | `os.FindProcess(pid).Kill()` on the leader only — best-effort |
| `processSignalGroup(pid int, sig syscall.Signal) core.Result` | `syscall.Kill(-pid, sig)` | `Ok(nil)` stub — Windows has no process-group signal primitive |
| `applyProcessGroup(cmd *exec.Cmd)` | `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` | no-op |
| `exitWasSignaled(state *os.ProcessState) bool` | `state.Sys().(syscall.WaitStatus).Signaled()` | `false` (Windows has no signals) |

All five helpers live in the existing `process` package — no new sub-package.

### File layout

```
go/
├── process.go              # body unchanged; switch direct syscall.* sites to helpers
├── service.go              # body unchanged; switch direct syscall.* sites to helpers
├── platform.go             # NEW — public helper signatures + doc comments
├── platform_unix.go        # //go:build !windows  (current Unix implementations)
└── platform_windows.go     # //go:build windows   (stub implementations)
```

`platform.go` documents the contract (signatures + behaviour notes); `_unix.go` and `_windows.go` provide the bodies. Same pattern as the existing `pidfile_*.go` triplet but with `pidfile.go` being the shared types file.

### Per-site rewrites

`process.go`:

| Line(s) | Before | After |
|---|---|---|
| 173, 191 | `syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)` | `processKillGroup(p.cmd.Process.Pid)` |
| 241 | `syscall.Kill(pid, syscall.SIGTERM)` | `processSignal(pid, syscall.SIGTERM)` |
| 269 | `syscall.Kill(-cmd.Process.Pid, 0)` (liveness probe) | `processSignalGroup(cmd.Process.Pid, 0)` |
| 272, 289 | `syscall.Kill(-pid, sig)` | `processSignalGroup(pid, sig)` |
| 299 | `syscall.Kill(-pid, syscall.SIGKILL)` | `processKillGroup(pid)` |

`service.go`:

| Line | Before | After |
|---|---|---|
| 176 | `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` | `applyProcessGroup(cmd)` |
| 470, 513 | `syscall.Kill(pid, sig)` | `processSignal(pid, sig)` |
| 978 | `exitErr.Sys().(syscall.WaitStatus).Signaled()` | `exitWasSignaled(exitErr.ProcessState)` (and refactor the surrounding cast since the `os.ProcessState` is the cleaner handle) |

`validateCatchableSignals` (service.go:520) — leave untouched. Signal constants `SIGKILL` / `SIGSTOP` exist in `syscall` on Windows (just not the `Kill` function), and that validator is purely about reading sig values.

### Tests

Existing tests should continue to pass on Unix. For Windows:
- Mark Unix-semantic test cases with `//go:build !windows` (e.g. tests that assert SIGTERM delivers and the child catches it).
- Add a Windows-only smoke test in `platform_windows_test.go` confirming the stubs return `Ok(nil)` for valid PIDs and don't panic.

`go test ./...` should pass on both `GOOS=linux` and `GOOS=windows` after this lands.

## Acceptance criteria

1. `GOOS=windows GOARCH=amd64 go build ./...` succeeds with `CGO_ENABLED=0` (no `syscall.Kill` / `Setpgid` errors).
2. `GOOS=windows go vet ./...` clean.
3. `go test ./...` passes on Linux and Darwin (existing behaviour preserved).
4. Lethean Desktop's `wails3 build GOOS=windows` reaches the next blocker (which is `dappco.re/go/io` — separate RFC) without choking on this package.
5. README gains a "Windows status" section: Phase 1 stubs ship; Job Object lifecycle is Phase 2 (see §6).

## §6 Phase 2 — real Job Object lifecycle

Out-of-scope for this RFC. Outline only:

- Windows process-group equivalent = a **Job Object** (`CreateJobObjectW` → `AssignProcessToJobObject` → `TerminateJobObject`). Owns the entire descendant tree, gets reaped together.
- `applyProcessGroup` on Windows would call `CreateJobObjectW` + set `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` so process death cascades.
- `processKillGroup` calls `TerminateJobObject`.
- `processSignalGroup` for `sig == 0` (liveness probe) maps to `IsProcessInJob`. For real signals — no Windows primitive; either stub OK or surface "unsupported" error.

Real Windows-supervisor semantics need this. The Lethean Desktop tray ships with stubs in Phase 1; the `lthn-mlx` subsystem supervisor that Snider's roadmap calls for (see memory `design_subsystem_binary_split.md`) needs Phase 2 before it runs on Windows.

## Estimated effort

- Phase 1 (this RFC): **~1.5h** — file split, mechanical call-site rewrites, test build-tag annotations, Windows smoke test.
- Phase 2 (Job Object): **~6-8h** — `golang.org/x/sys/windows` bindings, lifecycle wiring, integration tests on a real Windows runner.

## Cross-references

- Existing pattern: `go/pidfile_unix.go` + `go/pidfile_windows.go` (`processSignal` shim)
- Triggered from: Lethean Desktop Windows cross-compile audit
- Companion RFC: `~/Code/core/go-io/RFC.md` (the other Windows source-level blocker)
