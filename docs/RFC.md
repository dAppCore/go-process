# go-process API Contract — RFC Specification

> `dappco.re/go/core/process` — Managed process execution for the Core ecosystem.
> This package is the ONLY package that imports `os/exec`. Everything else uses
> `c.Process()` which delegates to Actions registered by this package.

**Status:** v0.8.0
**Module:** `dappco.re/go/core/process`
**Depends on:** core/go v0.8.0

---

## 1. Purpose

go-process provides the implementation behind `c.Process()`. Core defines the primitive (Section 17). go-process registers the Action handlers that make it work.

```
core/go defines:     c.Process().Run(ctx, "git", "log")
                     → calls c.Action("process.run").Run(ctx, opts)

go-process provides:  c.Action("process.run", s.handleRun)
                     → actually executes the command via os/exec
```

Without go-process registered, `c.Process().Run()` returns `Result{OK: false}`. Permission-by-registration.

---

## 2. Registration

```go
// Register is the WithService factory.
//
//   core.New(core.WithService(process.Register))
func Register(c *core.Core) core.Result {
    svc := &Service{
        ServiceRuntime: core.NewServiceRuntime(c, Options{}),
        managed:        core.NewRegistry[*ManagedProcess](),
    }
    return core.Result{Value: svc, OK: true}
}
```

### OnStartup — Register Actions

```go
func (s *Service) OnStartup(ctx context.Context) core.Result {
    c := s.Core()
    c.Action("process.run", s.handleRun)
    c.Action("process.start", s.handleStart)
    c.Action("process.kill", s.handleKill)
    c.Action("process.list", s.handleList)
    c.Action("process.get", s.handleGet)
    return core.Result{OK: true}
}
```

### OnShutdown — Kill Managed Processes

```go
func (s *Service) OnShutdown(ctx context.Context) core.Result {
    s.managed.Each(func(id string, p *ManagedProcess) {
        p.Kill()
    })
    return core.Result{OK: true}
}
```

---

## 3. Action Handlers

### process.run — Synchronous Execution

```go
func (s *Service) handleRun(ctx context.Context, opts core.Options) core.Result {
    command := opts.String("command")
    args, _ := opts.Get("args").Value.([]string)
    dir := opts.String("dir")
    env, _ := opts.Get("env").Value.([]string)

    cmd := exec.CommandContext(ctx, command, args...)
    if dir != "" { cmd.Dir = dir }
    if len(env) > 0 { cmd.Env = append(os.Environ(), env...) }

    output, err := cmd.CombinedOutput()
    if err != nil {
        return core.Result{Value: err, OK: false}
    }
    return core.Result{Value: string(output), OK: true}
}
```

> Note: go-process is the ONLY package allowed to import `os` and `os/exec`.

### process.start — Detached/Background

```go
func (s *Service) handleStart(ctx context.Context, opts core.Options) core.Result {
    command := opts.String("command")
    args, _ := opts.Get("args").Value.([]string)

    cmd := exec.Command(command, args...)
    cmd.Dir = opts.String("dir")

    if err := cmd.Start(); err != nil {
        return core.Result{Value: err, OK: false}
    }

    id := core.ID()
    managed := &ManagedProcess{
        ID: id, PID: cmd.Process.Pid, Command: command,
        cmd: cmd, done: make(chan struct{}),
    }
    s.managed.Set(id, managed)

    go func() {
        cmd.Wait()
        close(managed.done)
        managed.ExitCode = cmd.ProcessState.ExitCode()
    }()

    return core.Result{Value: id, OK: true}
}
```

### process.kill — Terminate by ID or PID

```go
func (s *Service) handleKill(ctx context.Context, opts core.Options) core.Result {
    id := opts.String("id")
    if id != "" {
        r := s.managed.Get(id)
        if !r.OK {
            return core.Result{Value: core.E("process.kill", core.Concat("not found: ", id), nil), OK: false}
        }
        r.Value.(*ManagedProcess).Kill()
        return core.Result{OK: true}
    }

    pid := opts.Int("pid")
    if pid > 0 {
        proc, err := os.FindProcess(pid)
        if err != nil { return core.Result{Value: err, OK: false} }
        proc.Signal(syscall.SIGTERM)
        return core.Result{OK: true}
    }

    return core.Result{Value: core.E("process.kill", "need id or pid", nil), OK: false}
}
```

### process.list / process.get

```go
func (s *Service) handleList(ctx context.Context, opts core.Options) core.Result {
    return core.Result{Value: s.managed.Names(), OK: true}
}

func (s *Service) handleGet(ctx context.Context, opts core.Options) core.Result {
    id := opts.String("id")
    r := s.managed.Get(id)
    if !r.OK { return r }
    return core.Result{Value: r.Value.(*ManagedProcess).Info(), OK: true}
}
```

---

## 4. ManagedProcess

```go
type ManagedProcess struct {
    ID        string
    PID       int
    Command   string
    ExitCode  int
    StartedAt time.Time
    cmd       *exec.Cmd
    done      chan struct{}
}

func (p *ManagedProcess) IsRunning() bool {
    select {
    case <-p.done: return false
    default: return true
    }
}

func (p *ManagedProcess) Kill() {
    if p.cmd != nil && p.cmd.Process != nil {
        p.cmd.Process.Signal(syscall.SIGTERM)
    }
}

func (p *ManagedProcess) Done() <-chan struct{} { return p.done }

func (p *ManagedProcess) Info() ProcessInfo {
    return ProcessInfo{
        ID: p.ID, PID: p.PID, Command: p.Command,
        Running: p.IsRunning(), ExitCode: p.ExitCode, StartedAt: p.StartedAt,
    }
}
```

---

## 5. Daemon Registry

Higher-level abstraction over `process.start`:

```
process.start  → low level: start a command, get a handle
daemon.Start   → high level: PID file, health endpoint, restart policy, signals
```

Daemon registry uses `core.Registry[*DaemonEntry]`.

---

## 6. Error Handling

All errors via `core.E()`. String building via `core.Concat()`.

```go
return core.Result{Value: core.E("process.run", core.Concat("command failed: ", command), err), OK: false}
```

---

## 7. Test Strategy

AX-7: `TestFile_Function_{Good,Bad,Ugly}`

```
TestService_Register_Good           — factory returns Result
TestService_OnStartup_Good          — registers 5 Actions
TestService_HandleRun_Good          — runs command, returns output
TestService_HandleRun_Bad           — command not found
TestService_HandleRun_Ugly          — timeout via context
TestService_HandleStart_Good        — starts detached, returns ID
TestService_HandleStart_Bad         — invalid command
TestService_HandleKill_Good         — kills by ID
TestService_HandleKill_Bad          — unknown ID
TestService_HandleList_Good         — returns managed process IDs
TestService_OnShutdown_Good         — kills all managed processes
TestService_Ugly_PermissionModel    — no go-process = c.Process().Run() fails
```

---

## 8. Quality Gates

go-process is the ONE exception — it imports `os` and `os/exec` because it IS the process primitive. All other disallowed imports still apply:

```bash
# Should only find os/exec in service.go, os in service.go
grep -rn '"os"\|"os/exec"' *.go | grep -v _test.go

# No other disallowed imports
grep -rn '"io"\|"fmt"\|"errors"\|"log"\|"encoding/json"\|"path/filepath"\|"unsafe"\|"strings"' *.go \
  | grep -v _test.go
```

---

## Consumer RFCs

| Package | RFC | Role |
|---------|-----|------|
| core/go | `core/go/docs/RFC.md` | Primitives — Process primitive (Section 17) |
| core/agent | `core/agent/docs/RFC.md` | Consumer — `c.Process().RunIn()` for git/build ops |

---

## Changelog

- 2026-03-25: v0.8.0 spec — written with full core/go domain context.
