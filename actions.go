package process

import "time"

// --- ACTION messages (broadcast via Core.ACTION) ---

// TaskProcessRun requests synchronous command execution through Core.PERFORM.
// The handler returns the combined command output on success.
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
type TaskProcessKill struct {
	// ID identifies a managed process started by this service.
	ID string
	// PID targets a process directly when ID is not available.
	PID int
}

// ActionProcessStarted is broadcast when a process begins execution.
type ActionProcessStarted struct {
	ID      string
	Command string
	Args    []string
	Dir     string
	PID     int
}

// ActionProcessOutput is broadcast for each line of output.
// Subscribe to this for real-time streaming.
type ActionProcessOutput struct {
	ID     string
	Line   string
	Stream Stream
}

// ActionProcessExited is broadcast when a process completes.
// Check ExitCode for success (0) or failure.
type ActionProcessExited struct {
	ID       string
	ExitCode int
	Duration time.Duration
	Error    error // Non-nil if failed to start or was killed
}

// ActionProcessKilled is broadcast when a process is terminated.
type ActionProcessKilled struct {
	ID     string
	Signal string
}
