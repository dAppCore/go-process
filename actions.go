package process

import (
	"context"
	"syscall"
	"time"

	"dappco.re/go/core"
)

// --- ACTION messages (broadcast via Core.ACTION) ---

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

// --- Core Action Handlers ---------------------------------------------------

func (s *Service) handleRun(ctx context.Context, opts core.Options) core.Result {
	command := opts.String("command")
	if command == "" {
		return core.Result{Value: core.E("process.run", "command is required", nil), OK: false}
	}

	runOpts := RunOptions{
		Command: command,
		Dir:     opts.String("dir"),
	}
	if r := opts.Get("args"); r.OK {
		runOpts.Args = optionStrings(r.Value)
	}
	if r := opts.Get("env"); r.OK {
		runOpts.Env = optionStrings(r.Value)
	}

	return s.runCommand(ctx, runOpts)
}

func (s *Service) handleStart(ctx context.Context, opts core.Options) core.Result {
	command := opts.String("command")
	if command == "" {
		return core.Result{Value: core.E("process.start", "command is required", nil), OK: false}
	}

	detach := true
	if opts.Has("detach") {
		detach = opts.Bool("detach")
	}

	runOpts := RunOptions{
		Command: command,
		Dir:     opts.String("dir"),
		Detach:  detach,
	}
	if r := opts.Get("args"); r.OK {
		runOpts.Args = optionStrings(r.Value)
	}
	if r := opts.Get("env"); r.OK {
		runOpts.Env = optionStrings(r.Value)
	}

	r := s.StartWithOptions(ctx, runOpts)
	if !r.OK {
		return r
	}
	return core.Result{Value: r.Value.(*ManagedProcess).ID, OK: true}
}

func (s *Service) handleKill(_ context.Context, opts core.Options) core.Result {
	id := opts.String("id")
	if id != "" {
		if err := s.Kill(id); err != nil {
			if core.Is(err, ErrProcessNotFound) {
				return core.Result{Value: core.E("process.kill", core.Concat("not found: ", id), nil), OK: false}
			}
			return core.Result{Value: core.E("process.kill", core.Concat("kill failed: ", id), err), OK: false}
		}
		return core.Result{OK: true}
	}

	pid := opts.Int("pid")
	if pid > 0 {
		proc, err := processHandle(pid)
		if err != nil {
			return core.Result{Value: core.E("process.kill", core.Concat("find pid failed: ", core.Sprintf("%d", pid)), err), OK: false}
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			return core.Result{Value: core.E("process.kill", core.Concat("signal failed: ", core.Sprintf("%d", pid)), err), OK: false}
		}
		return core.Result{OK: true}
	}

	return core.Result{Value: core.E("process.kill", "need id or pid", nil), OK: false}
}

func (s *Service) handleList(_ context.Context, _ core.Options) core.Result {
	return core.Result{Value: s.managed.Names(), OK: true}
}

func (s *Service) handleGet(_ context.Context, opts core.Options) core.Result {
	id := opts.String("id")
	if id == "" {
		return core.Result{Value: core.E("process.get", "id is required", nil), OK: false}
	}
	proc, err := s.Get(id)
	if err != nil {
		return core.Result{Value: core.E("process.get", core.Concat("not found: ", id), err), OK: false}
	}
	return core.Result{Value: proc.Info(), OK: true}
}

func optionStrings(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil
			}
			result = append(result, text)
		}
		return result
	default:
		return nil
	}
}
