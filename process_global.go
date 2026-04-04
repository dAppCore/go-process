package process

import (
	"context"
	"sync"
	"sync/atomic"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// Global default service used by package-level helpers.
var (
	defaultService atomic.Pointer[Service]
	defaultOnce    sync.Once
	defaultErr     error
)

// Default returns the global process service.
// Returns nil if not initialized.
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
func SetDefault(s *Service) error {
	if s == nil {
		return ErrSetDefaultNil
	}
	defaultService.Store(s)
	return nil
}

// Init initializes the default global service with a Core instance.
// This is typically called during application startup.
//
// Example:
//
//	_ = process.Init(coreInstance)
func Init(c *core.Core) error {
	defaultOnce.Do(func() {
		factory := NewService(Options{})
		svc, err := factory(c)
		if err != nil {
			defaultErr = err
			return
		}
		defaultService.Store(svc.(*Service))
	})
	return defaultErr
}

// --- Global convenience functions ---

// Start spawns a new process using the default service.
//
// Example:
//
//	proc, err := process.Start(ctx, "echo", "hello")
func Start(ctx context.Context, command string, args ...string) (*Process, error) {
	svc := Default()
	if svc == nil {
		return nil, ErrServiceNotInitialized
	}
	return svc.Start(ctx, command, args...)
}

// Run executes a command and waits for completion using the default service.
//
// Example:
//
//	out, err := process.Run(ctx, "echo", "hello")
func Run(ctx context.Context, command string, args ...string) (string, error) {
	svc := Default()
	if svc == nil {
		return "", ErrServiceNotInitialized
	}
	return svc.Run(ctx, command, args...)
}

// Get returns a process by ID from the default service.
//
// Example:
//
//	proc, err := process.Get("proc-1")
func Get(id string) (*Process, error) {
	svc := Default()
	if svc == nil {
		return nil, ErrServiceNotInitialized
	}
	return svc.Get(id)
}

// Output returns the captured output for a process from the default service.
//
// Example:
//
//	out, err := process.Output("proc-1")
func Output(id string) (string, error) {
	svc := Default()
	if svc == nil {
		return "", ErrServiceNotInitialized
	}
	return svc.Output(id)
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
func Kill(id string) error {
	svc := Default()
	if svc == nil {
		return ErrServiceNotInitialized
	}
	return svc.Kill(id)
}

// KillPID terminates a process by operating-system PID using the default service.
//
// Example:
//
//	_ = process.KillPID(1234)
func KillPID(pid int) error {
	svc := Default()
	if svc == nil {
		return ErrServiceNotInitialized
	}
	return svc.KillPID(pid)
}

// StartWithOptions spawns a process with full configuration using the default service.
//
// Example:
//
//	proc, err := process.StartWithOptions(ctx, process.RunOptions{Command: "pwd", Dir: "/tmp"})
func StartWithOptions(ctx context.Context, opts RunOptions) (*Process, error) {
	svc := Default()
	if svc == nil {
		return nil, ErrServiceNotInitialized
	}
	return svc.StartWithOptions(ctx, opts)
}

// RunWithOptions executes a command with options and waits using the default service.
//
// Example:
//
//	out, err := process.RunWithOptions(ctx, process.RunOptions{Command: "echo", Args: []string{"hello"}})
func RunWithOptions(ctx context.Context, opts RunOptions) (string, error) {
	svc := Default()
	if svc == nil {
		return "", ErrServiceNotInitialized
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

// Errors
var (
	// ErrServiceNotInitialized is returned when the service is not initialized.
	ErrServiceNotInitialized = coreerr.E("", "process: service not initialized; call process.Init(core) first", nil)
	// ErrSetDefaultNil is returned when SetDefault is called with nil.
	ErrSetDefaultNil = coreerr.E("", "process: SetDefault called with nil service", nil)
)
