package process

import (
	"fmt"
	"strconv"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/log"
)

// --- ACTION messages (broadcast via Core.ACTION) ---

// TaskProcessStart requests asynchronous process execution through Core.PERFORM.
// The handler returns a snapshot of the started process immediately.
//
// Example:
//
//	c.PERFORM(process.TaskProcessStart{Command: "sleep", Args: []string{"10"}})
type TaskProcessStart struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Dir     string   `json:"dir"`
	Env     []string `json:"env"`
	// DisableCapture skips buffering process output before returning it.
	DisableCapture bool `json:"disableCapture"`
	// Detach runs the command in its own process group.
	Detach bool `json:"detach"`
	// Timeout bounds the execution duration.
	Timeout time.Duration `json:"timeout"`
	// GracePeriod controls SIGTERM-to-SIGKILL escalation.
	GracePeriod time.Duration `json:"gracePeriod"`
	// KillGroup terminates the entire process group instead of only the leader.
	KillGroup bool `json:"killGroup"`
}

// TaskProcessRun requests synchronous command execution through Core.PERFORM.
// The handler returns the combined command output on success.
//
// Example:
//
//	c.PERFORM(process.TaskProcessRun{Command: "echo", Args: []string{"hello"}})
type TaskProcessRun struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Dir     string   `json:"dir"`
	Env     []string `json:"env"`
	// DisableCapture skips buffering process output before returning it.
	DisableCapture bool `json:"disableCapture"`
	// Detach runs the command in its own process group.
	Detach bool `json:"detach"`
	// Timeout bounds the execution duration.
	Timeout time.Duration `json:"timeout"`
	// GracePeriod controls SIGTERM-to-SIGKILL escalation.
	GracePeriod time.Duration `json:"gracePeriod"`
	// KillGroup terminates the entire process group instead of only the leader.
	KillGroup bool `json:"killGroup"`
}

// TaskProcessKill requests termination of a managed process by ID or PID.
//
// Example:
//
//	c.PERFORM(process.TaskProcessKill{ID: "proc-1"})
type TaskProcessKill struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
	// PID targets a process directly when ID is not available.
	PID int `json:"pid"`
}

// TaskProcessSignal requests signalling a managed process by ID or PID through Core.PERFORM.
// Signal 0 is allowed for liveness checks.
//
// Example:
//
//	c.PERFORM(process.TaskProcessSignal{ID: "proc-1", Signal: syscall.SIGTERM})
type TaskProcessSignal struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
	// PID targets a process directly when ID is not available.
	PID int `json:"pid"`
	// Signal is delivered to the process or process group.
	Signal syscall.Signal `json:"signal"`
}

// TaskProcessGet requests a snapshot of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessGet{ID: "proc-1"})
type TaskProcessGet struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
}

// TaskProcessWait waits for a managed process to finish through Core.PERFORM.
// Successful exits return an Info snapshot. Unsuccessful exits return a
// TaskProcessWaitError value that preserves the final snapshot.
//
// Example:
//
//	c.PERFORM(process.TaskProcessWait{ID: "proc-1"})
type TaskProcessWait struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
}

// TaskProcessWaitError is returned as the task value when TaskProcessWait
// completes with a non-successful process outcome. It preserves the final
// process snapshot while still behaving like the underlying wait error.
type TaskProcessWaitError struct {
	Info Info
	Err  error
}

// Error implements error.
func (e *TaskProcessWaitError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap returns the underlying wait error.
func (e *TaskProcessWaitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// TaskProcessOutput requests the captured output of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessOutput{ID: "proc-1"})
type TaskProcessOutput struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
}

// TaskProcessInput writes data to the stdin of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessInput{ID: "proc-1", Input: "hello\n"})
type TaskProcessInput struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
	// Input is written verbatim to the process stdin pipe.
	Input string `json:"input"`
}

// TaskProcessCloseStdin closes the stdin pipe of a managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessCloseStdin{ID: "proc-1"})
type TaskProcessCloseStdin struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
}

// processActionInput models the options passed via core.Actions.
// Keys:
// command, args, dir, env, disableCapture, detach, timeout,
// gracePeriod, killGroup, id, pid.
type processActionInput struct {
	Command        string
	Args           []string
	Dir            string
	Env            []string
	DisableCapture bool
	Detach         bool
	Timeout        time.Duration
	GracePeriod    time.Duration
	KillGroup      bool
	ID             string
	PID            int
}

func parseProcessActionInput(opts core.Options, requireCommand bool) (processActionInput, error) {
	parsed := processActionInput{
		Command:        core.Trim(opts.String("command")),
		Dir:            opts.String("dir"),
		DisableCapture: opts.Bool("disableCapture"),
		Detach:         opts.Bool("detach"),
		KillGroup:      opts.Bool("killGroup"),
		Timeout:        parseDurationOption(opts, "timeout"),
		GracePeriod:    parseDurationOption(opts, "gracePeriod"),
	}

	var err error

	parsed.Args, err = parseStringSliceOption(opts, "args")
	if err != nil {
		return processActionInput{}, err
	}

	parsed.Env, err = parseStringSliceOption(opts, "env")
	if err != nil {
		return processActionInput{}, err
	}

	parsed.ID = core.Trim(opts.String("id"))
	parsed.PID = parseIntOption(opts, "pid")

	if requireCommand && parsed.Command == "" {
		return processActionInput{}, coreerr.E("process action", "command is required", nil)
	}

	return parsed, nil
}

func parseProcessActionTarget(opts core.Options) (string, int, error) {
	id := core.Trim(opts.String("id"))
	pid := parseIntOption(opts, "pid")
	if id == "" && pid <= 0 {
		return "", 0, coreerr.E("process action", "id or pid is required", nil)
	}
	return id, pid, nil
}

func parseDurationOption(opts core.Options, key string) time.Duration {
	r := opts.Get(key)
	if !r.OK {
		return 0
	}

	switch value := r.Value.(type) {
	case time.Duration:
		return value
	case int:
		return time.Duration(value)
	case int8:
		return time.Duration(value)
	case int16:
		return time.Duration(value)
	case int32:
		return time.Duration(value)
	case int64:
		return time.Duration(value)
	case uint:
		return time.Duration(value)
	case uint8:
		return time.Duration(value)
	case uint16:
		return time.Duration(value)
	case uint32:
		return time.Duration(value)
	case uint64:
		return time.Duration(value)
	case float32:
		return time.Duration(value)
	case float64:
		return time.Duration(value)
	case string:
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
		if n, parseErr := strconv.ParseInt(value, 10, 64); parseErr == nil {
			return time.Duration(n)
		}
	}

	return 0
}

func parseIntOption(opts core.Options, key string) int {
	r := opts.Get(key)
	if !r.OK {
		return 0
	}

	switch value := r.Value.(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}

	return 0
}

func parseStringSliceOption(opts core.Options, key string) ([]string, error) {
	r := opts.Get(key)
	if !r.OK {
		return nil, nil
	}

	raw, ok := r.Value.([]string)
	if ok {
		return raw, nil
	}

	anyList, ok := r.Value.([]any)
	if !ok {
		if alt, ok := r.Value.([]interface{}); ok {
			anyList = alt
		} else {
			return nil, coreerr.E("process action", fmt.Sprintf("%s must be an array", key), nil)
		}
	}

	items := make([]string, 0, len(anyList))
	for _, item := range anyList {
		value, ok := item.(string)
		if !ok {
			return nil, coreerr.E("process action", fmt.Sprintf("%s entries must be strings", key), nil)
		}
		items = append(items, value)
	}

	return items, nil
}

// TaskProcessList requests a snapshot of managed processes through Core.PERFORM.
// If RunningOnly is true, only active processes are returned.
//
// Example:
//
//	c.PERFORM(process.TaskProcessList{RunningOnly: true})
type TaskProcessList struct {
	RunningOnly bool `json:"runningOnly"`
}

// TaskProcessRemove removes a completed managed process through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessRemove{ID: "proc-1"})
type TaskProcessRemove struct {
	// ID identifies a managed process started by this service.
	ID string `json:"id"`
}

// TaskProcessClear removes all completed managed processes through Core.PERFORM.
//
// Example:
//
//	c.PERFORM(process.TaskProcessClear{})
type TaskProcessClear struct{}

// ActionProcessStarted is broadcast when a process begins execution.
//
// Example:
//
//	case process.ActionProcessStarted: core.Println("started", msg.ID)
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
//	case process.ActionProcessOutput: core.Println(msg.Line)
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
//	case process.ActionProcessExited: core.Println(msg.ExitCode)
type ActionProcessExited struct {
	ID       string
	ExitCode int
	Duration time.Duration
	Error    error // Set for failed starts, non-zero exits, or killed processes.
}

// ActionProcessKilled is broadcast when a process is terminated.
//
// Example:
//
//	case process.ActionProcessKilled: core.Println(msg.Signal)
type ActionProcessKilled struct {
	ID     string
	Signal string
}
