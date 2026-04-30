package process

import (
	"context"
	// Note: AX-6 — internal concurrency primitives; structural per RFC §2
	"sync"
	"sync/atomic"
	"syscall"

	"dappco.re/go"
)

// Global default service used by package-level helpers.
var (
	defaultService atomic.Pointer[Service]
	defaultOnce    sync.Once
	defaultResult  core.Result
)

// Default returns the global process service.
// Returns nil if not initialised.
//
// Example:
//
//	svc := process.Default()
func Default() *Service {
	return defaultService.Load()
}

// SetDefault sets the global process service.
// Thread-safe: can be called concurrently with Default().
//
// Example:
//
//	_ = process.SetDefault(svc)
func SetDefault(s *Service) core.Result {
	if s == nil {
		return core.Fail(ErrSetDefaultNil)
	}
	defaultService.Store(s)
	return core.Ok(nil)
}

// Init initializes the default global service with a Core instance.
// This is typically called during application startup.
//
// Example:
//
//	_ = process.Init(coreInstance)
func Init(c *core.Core) core.Result {
	defaultOnce.Do(func() {
		factory := NewService(Options{})
		result := factory(c)
		if !result.OK {
			defaultResult = result
			return
		}
		defaultService.Store(result.Value.(*Service))
		defaultResult = core.Ok(nil)
	})
	return defaultResult
}

// Register creates a process service for Core registration.
//
// Example:
//
//	result := process.Register(coreInstance)
func Register(c *core.Core) core.Result {
	factory := NewService(Options{})
	result := factory(c)
	if !result.OK {
		return result
	}
	return core.Ok(result.Value)
}

// --- Global convenience functions ---

// Start spawns a new process using the default service.
//
// Example:
//
//	result := process.Start(ctx, "echo", "hello")
func Start(ctx context.Context, command string, args ...string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Start(ctx, command, args...)
}

// Run executes a command and waits for completion using the default service.
//
// Example:
//
//	result := process.Run(ctx, "echo", "hello")
func Run(ctx context.Context, command string, args ...string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Run(ctx, command, args...)
}

// Get returns a process by ID from the default service.
//
// Example:
//
//	result := process.Get("proc-1")
func Get(id string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Get(id)
}

// Output returns the captured output for a process from the default service.
//
// Example:
//
//	result := process.Output("proc-1")
func Output(id string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Output(id)
}

// Input writes data to the stdin of a managed process using the default service.
//
// Example:
//
//	_ = process.Input("proc-1", "hello\n")
func Input(id string, input string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Input(id, input)
}

// CloseStdin closes the stdin pipe of a managed process using the default service.
//
// Example:
//
//	_ = process.CloseStdin("proc-1")
func CloseStdin(id string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.CloseStdin(id)
}

// Wait blocks until a managed process exits and returns its final snapshot.
//
// Example:
//
//	result := process.Wait("proc-1")
func Wait(id string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Wait(id)
}

// List returns all processes from the default service.
//
// Example:
//
//	procs := process.List()
func List() []*Process {
	svc := Default()
	if svc == nil {
		return nil
	}
	return svc.List()
}

// Kill terminates a process by ID using the default service.
//
// Example:
//
//	_ = process.Kill("proc-1")
func Kill(id string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Kill(id)
}

// KillPID terminates a process by operating-system PID using the default service.
//
// Example:
//
//	_ = process.KillPID(1234)
func KillPID(pid int) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.KillPID(pid)
}

// Signal sends a signal to a process by ID using the default service.
//
// Example:
//
//	_ = process.Signal("proc-1", syscall.SIGTERM)
func Signal(id string, sig syscall.Signal) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Signal(id, sig)
}

// SignalPID sends a signal to a process by operating-system PID using the default service.
//
// Example:
//
//	_ = process.SignalPID(1234, syscall.SIGTERM)
func SignalPID(pid int, sig syscall.Signal) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.SignalPID(pid, sig)
}

// StartWithOptions spawns a process with full configuration using the default service.
//
// Example:
//
//	result := process.StartWithOptions(ctx, process.RunOptions{Command: "pwd", Dir: "/tmp"})
func StartWithOptions(ctx context.Context, opts RunOptions) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.StartWithOptions(ctx, opts)
}

// RunWithOptions executes a command with options and waits using the default service.
//
// Example:
//
//	result := process.RunWithOptions(ctx, process.RunOptions{Command: "echo", Args: []string{"hello"}})
func RunWithOptions(ctx context.Context, opts RunOptions) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.RunWithOptions(ctx, opts)
}

// Running returns all currently running processes from the default service.
//
// Example:
//
//	running := process.Running()
func Running() []*Process {
	svc := Default()
	if svc == nil {
		return nil
	}
	return svc.Running()
}

// Remove removes a completed process from the default service.
//
// Example:
//
//	_ = process.Remove("proc-1")
func Remove(id string) core.Result {
	svc := Default()
	if svc == nil {
		return core.Fail(ErrServiceNotInitialized)
	}
	return svc.Remove(id)
}

// Clear removes all completed processes from the default service.
//
// Example:
//
//	process.Clear()
func Clear() {
	svc := Default()
	if svc == nil {
		return
	}
	svc.Clear()
}

// Errors
var (
	// ErrServiceNotInitialized is returned when the service is not initialised.
	ErrServiceNotInitialized = core.E("", "process: service not initialized; call process.Init(core) first", nil)
	// ErrSetDefaultNil is returned when SetDefault is called with nil.
	ErrSetDefaultNil = core.E("", "process: SetDefault called with nil service", nil)
)
