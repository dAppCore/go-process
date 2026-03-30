// Package process provides process management with Core IPC integration.
//
// The process package enables spawning, monitoring, and controlling external
// processes with output streaming via the Core ACTION system.
//
// # Getting Started
//
//	c := core.New(core.WithService(process.Register))
//	_ = c.ServiceStartup(ctx, nil)
//
//	r := c.Process().Run(ctx, "go", "test", "./...")
//	output := r.Value.(string)
//
// # Listening for Events
//
// Process events are broadcast via Core.ACTION:
//
//	c.RegisterAction(func(c *core.Core, msg core.Message) core.Result {
//	    switch m := msg.(type) {
//	    case process.ActionProcessOutput:
//	        fmt.Print(m.Line)
//	    case process.ActionProcessExited:
//	        fmt.Printf("Exit code: %d\n", m.ExitCode)
//	    }
//	    return core.Result{OK: true}
//	})
package process

import "time"

// Status represents the process lifecycle state.
type Status string

const (
	// StatusPending indicates the process is queued but not yet started.
	StatusPending Status = "pending"
	// StatusRunning indicates the process is actively executing.
	StatusRunning Status = "running"
	// StatusExited indicates the process completed (check ExitCode).
	StatusExited Status = "exited"
	// StatusFailed indicates the process could not be started.
	StatusFailed Status = "failed"
	// StatusKilled indicates the process was terminated by signal.
	StatusKilled Status = "killed"
)

// Stream identifies the output source.
type Stream string

const (
	// StreamStdout is standard output.
	StreamStdout Stream = "stdout"
	// StreamStderr is standard error.
	StreamStderr Stream = "stderr"
)

// RunOptions configures process execution.
type RunOptions struct {
	// Command is the executable to run.
	Command string
	// Args are the command arguments.
	Args []string
	// Dir is the working directory (empty = current).
	Dir string
	// Env are additional environment variables (KEY=VALUE format).
	Env []string
	// DisableCapture disables output buffering.
	// By default, output is captured to a ring buffer.
	DisableCapture bool
	// Detach creates the process in its own process group (Setpgid).
	// Detached processes survive parent death and context cancellation.
	// The context is replaced with context.Background() when Detach is true.
	Detach bool
	// Timeout is the maximum duration the process may run.
	// After this duration, the process receives SIGTERM (or SIGKILL if
	// GracePeriod is zero). Zero means no timeout.
	Timeout time.Duration
	// GracePeriod is the time between SIGTERM and SIGKILL when stopping
	// a process (via timeout or Shutdown). Zero means immediate SIGKILL.
	// Default: 0 (immediate kill for backwards compatibility).
	GracePeriod time.Duration
	// KillGroup kills the entire process group instead of just the leader.
	// Requires Detach to be true (process must be its own group leader).
	// This ensures child processes spawned by the command are also killed.
	KillGroup bool
}

// ProcessInfo provides a snapshot of process state without internal fields.
type ProcessInfo struct {
	ID        string        `json:"id"`
	Command   string        `json:"command"`
	Args      []string      `json:"args"`
	Dir       string        `json:"dir"`
	StartedAt time.Time     `json:"startedAt"`
	Running   bool          `json:"running"`
	Status    Status        `json:"status"`
	ExitCode  int           `json:"exitCode"`
	Duration  time.Duration `json:"duration"`
	PID       int           `json:"pid"`
}

// Info is kept as a compatibility alias for ProcessInfo.
type Info = ProcessInfo
