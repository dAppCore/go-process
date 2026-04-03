package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// Default buffer size for process output (1MB).
const DefaultBufferSize = 1024 * 1024

// Errors
var (
	ErrProcessNotFound   = coreerr.E("", "process not found", nil)
	ErrProcessNotRunning = coreerr.E("", "process is not running", nil)
	ErrStdinNotAvailable = coreerr.E("", "stdin not available", nil)
)

// Service manages process execution with Core IPC integration.
type Service struct {
	*core.ServiceRuntime[Options]

	processes     map[string]*Process
	mu            sync.RWMutex
	bufSize       int
	idCounter     atomic.Uint64
	registrations sync.Once
}

// Options configures the process service.
type Options struct {
	// BufferSize is the ring buffer size for output capture.
	// Default: 1MB (1024 * 1024 bytes).
	BufferSize int
}

// NewService creates a process service factory for Core registration.
//
//	core, _ := core.New(
//	    core.WithName("process", process.NewService(process.Options{})),
//	)
func NewService(opts Options) func(*core.Core) (any, error) {
	return func(c *core.Core) (any, error) {
		if opts.BufferSize == 0 {
			opts.BufferSize = DefaultBufferSize
		}
		svc := &Service{
			ServiceRuntime: core.NewServiceRuntime(c, opts),
			processes:      make(map[string]*Process),
			bufSize:        opts.BufferSize,
		}
		return svc, nil
	}
}

// OnStartup implements core.Startable.
func (s *Service) OnStartup(ctx context.Context) error {
	s.registrations.Do(func() {
		if s.Core() != nil {
			s.Core().RegisterTask(s.handleTask)
		}
	})
	return nil
}

// OnShutdown implements core.Stoppable.
// Gracefully shuts down all running processes (SIGTERM → SIGKILL).
func (s *Service) OnShutdown(ctx context.Context) error {
	s.mu.RLock()
	procs := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		if p.IsRunning() {
			procs = append(procs, p)
		}
	}
	s.mu.RUnlock()

	for _, p := range procs {
		_ = p.Shutdown()
	}

	return nil
}

// Start spawns a new process with the given command and args.
func (s *Service) Start(ctx context.Context, command string, args ...string) (*Process, error) {
	return s.StartWithOptions(ctx, RunOptions{
		Command: command,
		Args:    args,
	})
}

// StartWithOptions spawns a process with full configuration.
func (s *Service) StartWithOptions(ctx context.Context, opts RunOptions) (*Process, error) {
	id := fmt.Sprintf("proc-%d", s.idCounter.Add(1))

	if opts.KillGroup && !opts.Detach {
		return nil, coreerr.E("Service.StartWithOptions", "KillGroup requires Detach", nil)
	}

	// Detached processes use Background context so they survive parent death
	parentCtx := ctx
	if opts.Detach {
		parentCtx = context.Background()
	}
	procCtx, cancel := context.WithCancel(parentCtx)
	cmd := exec.CommandContext(procCtx, opts.Command, opts.Args...)

	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), opts.Env...)
	}

	// Detached processes get their own process group
	if opts.Detach {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, coreerr.E("Service.StartWithOptions", "failed to create stdout pipe", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, coreerr.E("Service.StartWithOptions", "failed to create stderr pipe", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, coreerr.E("Service.StartWithOptions", "failed to create stdin pipe", err)
	}

	// Create output buffer (enabled by default)
	var output *RingBuffer
	if !opts.DisableCapture {
		output = NewRingBuffer(s.bufSize)
	}

	proc := &Process{
		ID:          id,
		Command:     opts.Command,
		Args:        opts.Args,
		Dir:         opts.Dir,
		Env:         opts.Env,
		StartedAt:   time.Now(),
		Status:      StatusRunning,
		cmd:         cmd,
		ctx:         procCtx,
		cancel:      cancel,
		output:      output,
		stdin:       stdin,
		done:        make(chan struct{}),
		gracePeriod: opts.GracePeriod,
		killGroup:   opts.KillGroup && opts.Detach,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, coreerr.E("Service.StartWithOptions", "failed to start process", err)
	}

	// Store process
	s.mu.Lock()
	s.processes[id] = proc
	s.mu.Unlock()

	// Start timeout watchdog if configured
	if opts.Timeout > 0 {
		go func() {
			select {
			case <-proc.done:
				// Process exited before timeout
			case <-time.After(opts.Timeout):
				proc.Shutdown()
			}
		}()
	}

	// Broadcast start
	_ = s.Core().ACTION(ActionProcessStarted{
		ID:      id,
		Command: opts.Command,
		Args:    opts.Args,
		Dir:     opts.Dir,
		PID:     cmd.Process.Pid,
	})

	// Stream output in goroutines
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s.streamOutput(proc, stdout, StreamStdout)
	}()
	go func() {
		defer wg.Done()
		s.streamOutput(proc, stderr, StreamStderr)
	}()

	// Wait for process completion
	go func() {
		// Wait for output streaming to complete
		wg.Wait()

		// Wait for process exit
		err := cmd.Wait()

		duration := time.Since(proc.StartedAt)
		status, exitCode, exitErr, signalName := classifyProcessExit(err)

		proc.mu.Lock()
		proc.Duration = duration
		proc.ExitCode = exitCode
		proc.Status = status
		proc.mu.Unlock()

		close(proc.done)

		// Broadcast lifecycle completion.
		switch status {
		case StatusKilled:
			_ = s.Core().ACTION(ActionProcessKilled{
				ID:     id,
				Signal: signalName,
			})
		default:
			_ = s.Core().ACTION(ActionProcessExited{
				ID:       id,
				ExitCode: exitCode,
				Duration: duration,
				Error:    exitErr,
			})
		}
	}()

	return proc, nil
}

// streamOutput reads from a pipe and broadcasts lines via ACTION.
func (s *Service) streamOutput(proc *Process, r io.Reader, stream Stream) {
	scanner := bufio.NewScanner(r)
	// Increase buffer for long lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Write to ring buffer
		if proc.output != nil {
			_, _ = proc.output.Write([]byte(line + "\n"))
		}

		// Broadcast output
		_ = s.Core().ACTION(ActionProcessOutput{
			ID:     proc.ID,
			Line:   line,
			Stream: stream,
		})
	}
}

// Get returns a process by ID.
func (s *Service) Get(id string) (*Process, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proc, ok := s.processes[id]
	if !ok {
		return nil, ErrProcessNotFound
	}
	return proc, nil
}

// List returns all processes.
func (s *Service) List() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		result = append(result, p)
	}
	return result
}

// Running returns all currently running processes.
func (s *Service) Running() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Process
	for _, p := range s.processes {
		if p.IsRunning() {
			result = append(result, p)
		}
	}
	return result
}

// Kill terminates a process by ID.
func (s *Service) Kill(id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}

	return proc.Kill()
}

// Remove removes a completed process from the list.
func (s *Service) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	proc, ok := s.processes[id]
	if !ok {
		return ErrProcessNotFound
	}

	if proc.IsRunning() {
		return coreerr.E("Service.Remove", "cannot remove running process", nil)
	}

	delete(s.processes, id)
	return nil
}

// Clear removes all completed processes.
func (s *Service) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, p := range s.processes {
		if !p.IsRunning() {
			delete(s.processes, id)
		}
	}
}

// Output returns the captured output of a process.
func (s *Service) Output(id string) (string, error) {
	proc, err := s.Get(id)
	if err != nil {
		return "", err
	}
	return proc.Output(), nil
}

// Run executes a command and waits for completion.
// Returns the combined output and any error.
func (s *Service) Run(ctx context.Context, command string, args ...string) (string, error) {
	proc, err := s.Start(ctx, command, args...)
	if err != nil {
		return "", err
	}

	<-proc.Done()

	output := proc.Output()
	if proc.Status == StatusKilled {
		return output, coreerr.E("Service.Run", "process was killed", nil)
	}
	if proc.ExitCode != 0 {
		return output, coreerr.E("Service.Run", fmt.Sprintf("process exited with code %d", proc.ExitCode), nil)
	}
	return output, nil
}

// RunWithOptions executes a command with options and waits for completion.
func (s *Service) RunWithOptions(ctx context.Context, opts RunOptions) (string, error) {
	proc, err := s.StartWithOptions(ctx, opts)
	if err != nil {
		return "", err
	}

	<-proc.Done()

	output := proc.Output()
	if proc.Status == StatusKilled {
		return output, coreerr.E("Service.RunWithOptions", "process was killed", nil)
	}
	if proc.ExitCode != 0 {
		return output, coreerr.E("Service.RunWithOptions", fmt.Sprintf("process exited with code %d", proc.ExitCode), nil)
	}
	return output, nil
}

// handleTask dispatches Core.PERFORM messages for the process service.
func (s *Service) handleTask(c *core.Core, task core.Task) core.Result {
	switch m := task.(type) {
	case TaskProcessRun:
		output, err := s.RunWithOptions(c.Context(), RunOptions{
			Command: m.Command,
			Args:    m.Args,
			Dir:     m.Dir,
			Env:     m.Env,
		})
		if err != nil {
			return core.Result{Value: err, OK: false}
		}
		return core.Result{Value: output, OK: true}
	default:
		return core.Result{}
	}
}

// classifyProcessExit maps a command completion error to lifecycle state.
func classifyProcessExit(err error) (Status, int, error, string) {
	if err == nil {
		return StatusExited, 0, nil, ""
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			signalName := ws.Signal().String()
			if signalName == "" {
				signalName = "signal"
			}
			return StatusKilled, -1, nil, signalName
		}
		return StatusExited, exitErr.ExitCode(), nil, ""
	}

	return StatusFailed, 0, err, ""
}
