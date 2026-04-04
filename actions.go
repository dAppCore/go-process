package process

import (
	"syscall"
	"time"
)

// --- ACTION messages (broadcast via Core.ACTION) ---

// TaskProcessStart requests asynchronous process execution through Core.PERFORM.
// The handler returns a snapshot of the started process immediately.
//
// Example:
//
//	c.PERFORM(process.TaskProcessStart{Command: "sleep", Args: []string{"10"}})
type TaskProcessStart struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	// DisableCapture skips buffering process output before returning it.
	DisableCapture bool
	// Detach runs the command in its own process group.
	Detach bool
	// Timeout bounds the execution duration.
	Timeout time.Duration
	// GracePeriod controls SIGTERM-to-SIGKILL escalation.
	GracePeriod time.Duration
	// KillGroup terminates the entire process group instead of only the leader.
	KillGroup bool
}

// TaskProcessRun requests synchronous command execution through Core.PERFORM.
// The handler returns the combined command output on success.
//
// Example:
//
//	c.PERFORM(process.TaskProcessRun{Command: "echo", Args: []string{"hello"}})
type TaskProcessRun struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	// DisableCapture skips buffering process output before returning it.
	DisableCapture bool
	// Detach runs the command in its own process group.
	Detach bool
	// Timeout bounds the execution duration.
	Timeout time.Duration
	// GracePeriod controls SIGTERM-to-SIGKILL escalation.
	GracePeriod time.Duration
	// KillGroup terminates the entire process group instead of only the leader.
	KillGroup bool
}

// TaskProcessKill requests termination of a managed process by ID or PID.
//
// Example:
//
//	c.PERFORM(process.TaskProcessKill{ID: "proc-1"})
type TaskProcessKill struct {
	// ID identifies a managed process started by this service.
	ID string
	// PID targets a process directly when ID is not available.
	PID int
}

// TaskProcessSignal requests signalling a managed process by ID or PID through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessSignal{ID: "proc-1", Signal: syscall.SIGTERM})
type TaskProcessSignal struct {
	// ID identifies a managed process started by this service.
	ID string
	// PID targets a process directly when ID is not available.
	PID int
	// Signal is delivered to the process or process group.
	Signal syscall.Signal
}

// TaskProcessGet requests a snapshot of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessGet{ID: "proc-1"})
type TaskProcessGet struct {
	// ID identifies a managed process started by this service.
	ID string
}

// TaskProcessWait waits for a managed process to finish through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessWait{ID: "proc-1"})
type TaskProcessWait struct {
	// ID identifies a managed process started by this service.
	ID string
}

// TaskProcessOutput requests the captured output of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessOutput{ID: "proc-1"})
type TaskProcessOutput struct {
	// ID identifies a managed process started by this service.
	ID string
}

// TaskProcessInput writes data to the stdin of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessInput{ID: "proc-1", Input: "hello\n"})
type TaskProcessInput struct {
	// ID identifies a managed process started by this service.
	ID string
	// Input is written verbatim to the process stdin pipe.
	Input string
}

// TaskProcessCloseStdin closes the stdin pipe of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessCloseStdin{ID: "proc-1"})
type TaskProcessCloseStdin struct {
	// ID identifies a managed process started by this service.
	ID string
}

// TaskProcessList requests a snapshot of managed processes through Core.PERFORM.
// If RunningOnly is true, only active processes are returned.
//
// Example:
//
//	c.PERFORM(process.TaskProcessList{RunningOnly: true})
type TaskProcessList struct {
	RunningOnly bool
}

// ActionProcessStarted is broadcast when a process begins execution.
//
// Example:
//
//	case process.ActionProcessStarted: fmt.Println("started", msg.ID)
type ActionProcessStarted struct {
	ID      string
	Command string
	Args    []string
	Dir     string
	PID     int
}

// ActionProcessOutput is broadcast for each line of output.
// Subscribe to this for real-time streaming.
//
// Example:
//
//	case process.ActionProcessOutput: fmt.Println(msg.Line)
type ActionProcessOutput struct {
	ID     string
	Line   string
	Stream Stream
}

// ActionProcessExited is broadcast when a process completes.
// Check ExitCode for success (0) or failure.
//
// Example:
//
//	case process.ActionProcessExited: fmt.Println(msg.ExitCode)
type ActionProcessExited struct {
	ID       string
	ExitCode int
	Duration time.Duration
	Error    error // Reserved for future exit metadata; currently left unset by the service
}

// ActionProcessKilled is broadcast when a process is terminated.
//
// Example:
//
//	case process.ActionProcessKilled: fmt.Println(msg.Signal)
type ActionProcessKilled struct {
	ID     string
	Signal string
}
