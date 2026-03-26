package process

import (
	"bufio"
	"context"
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

	processes map[string]*Process
	mu        sync.RWMutex
	bufSize   int
	idCounter atomic.Uint64
}

// Options configures the process service.
type Options struct {
	// BufferSize is the ring buffer size for output capture.
	// Default: 1MB (1024 * 1024 bytes).
	BufferSize int
}

// Register is the WithService factory for go-process.
// Registers the process service with Core — OnStartup registers named Actions
// (process.run, process.start, process.kill, process.list, process.get).
//
//	core.New(core.WithService(process.Register))
func Register(c *core.Core) core.Result {
	svc := &Service{
		ServiceRuntime: core.NewServiceRuntime(c, Options{BufferSize: DefaultBufferSize}),
		processes:      make(map[string]*Process),
		bufSize:        DefaultBufferSize,
	}
	return core.Result{Value: svc, OK: true}
}

// NewService creates a process service factory for Core registration.
// Deprecated: Use Register with core.WithService(process.Register) instead.
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

// OnStartup implements core.Startable — registers named Actions.
//
//	c.Process().Run(ctx, "git", "log") // → calls process.run Action
func (s *Service) OnStartup(ctx context.Context) core.Result {
	c := s.Core()
	c.Action("process.run", s.handleRun)
	c.Action("process.start", s.handleStart)
	c.Action("process.kill", s.handleKill)
	c.Action("process.list", s.handleList)
	c.Action("process.get", s.handleGet)
	return core.Result{OK: true}
}

// OnShutdown implements core.Stoppable — kills all managed processes.
//
//	c.ServiceShutdown(ctx) // calls OnShutdown on all Stoppable services
func (s *Service) OnShutdown(ctx context.Context) core.Result {
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

	return core.Result{OK: true}
}

// Start spawns a new process with the given command and args.
//
//	r := svc.Start(ctx, "echo", "hello")
//	if r.OK { proc := r.Value.(*Process) }
func (s *Service) Start(ctx context.Context, command string, args ...string) core.Result {
	return s.StartWithOptions(ctx, RunOptions{
		Command: command,
		Args:    args,
	})
}

// StartWithOptions spawns a process with full configuration.
//
//	r := svc.StartWithOptions(ctx, process.RunOptions{Command: "go", Args: []string{"test", "./..."}})
//	if r.OK { proc := r.Value.(*Process) }
func (s *Service) StartWithOptions(ctx context.Context, opts RunOptions) core.Result {
	id := core.ID()

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
		return core.Result{OK: false}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return core.Result{OK: false}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return core.Result{OK: false}
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
		return core.Result{OK: false}
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
			case <-time.After(opts.Timeout):
				proc.Shutdown()
			}
		}()
	}

	// Broadcast start
	s.Core().ACTION(ActionProcessStarted{
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
		wg.Wait()
		err := cmd.Wait()

		duration := time.Since(proc.StartedAt)

		proc.mu.Lock()
		proc.Duration = duration
		if err != nil {
			var exitErr *exec.ExitError
			if core.As(err, &exitErr) {
				proc.ExitCode = exitErr.ExitCode()
				proc.Status = StatusExited
			} else {
				proc.Status = StatusFailed
			}
		} else {
			proc.ExitCode = 0
			proc.Status = StatusExited
		}
		status := proc.Status
		exitCode := proc.ExitCode
		proc.mu.Unlock()

		close(proc.done)

		s.Core().ACTION(ActionProcessExited{
			ID:       id,
			ExitCode: exitCode,
			Duration: duration,
		})
		_ = status
	}()

	return core.Result{Value: proc, OK: true}
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

	if err := proc.Kill(); err != nil {
		return err
	}

	_ = s.Core().ACTION(ActionProcessKilled{
		ID:     id,
		Signal: "SIGKILL",
	})

	return nil
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
// Value is always the output string. OK is true if exit code is 0.
//
//	r := svc.Run(ctx, "go", "test", "./...")
//	output := r.Value.(string)
func (s *Service) Run(ctx context.Context, command string, args ...string) core.Result {
	r := s.Start(ctx, command, args...)
	if !r.OK {
		return core.Result{Value: "", OK: false}
	}

	proc := r.Value.(*Process)
	<-proc.Done()

	return core.Result{Value: proc.Output(), OK: proc.ExitCode == 0}
}

// RunWithOptions executes a command with options and waits for completion.
// Value is always the output string. OK is true if exit code is 0.
//
//	r := svc.RunWithOptions(ctx, process.RunOptions{Command: "go", Args: []string{"test"}})
//	output := r.Value.(string)
func (s *Service) RunWithOptions(ctx context.Context, opts RunOptions) core.Result {
	r := s.StartWithOptions(ctx, opts)
	if !r.OK {
		return core.Result{Value: "", OK: false}
	}

	proc := r.Value.(*Process)
	<-proc.Done()

	return core.Result{Value: proc.Output(), OK: proc.ExitCode == 0}
}

// --- Named Action Handlers ---
// These are registered during OnStartup and called via c.Process() sugar.
// c.Process().Run(ctx, "git", "log") → c.Action("process.run").Run(ctx, opts)

// handleRun executes a command synchronously and returns the output.
//
//	r := c.Action("process.run").Run(ctx, core.NewOptions(
//	    core.Option{Key: "command", Value: "git"},
//	    core.Option{Key: "args", Value: []string{"log"}},
//	    core.Option{Key: "dir", Value: "/repo"},
//	))
func (s *Service) handleRun(ctx context.Context, opts core.Options) core.Result {
	command := opts.String("command")
	if command == "" {
		return core.Result{Value: coreerr.E("process.run", "command is required", nil), OK: false}
	}

	runOpts := RunOptions{
		Command: command,
		Dir:     opts.String("dir"),
	}
	if r := opts.Get("args"); r.OK {
		if args, ok := r.Value.([]string); ok {
			runOpts.Args = args
		}
	}
	if r := opts.Get("env"); r.OK {
		if env, ok := r.Value.([]string); ok {
			runOpts.Env = env
		}
	}

	return s.RunWithOptions(ctx, runOpts)
}

// handleStart spawns a detached/background process and returns the process ID.
//
//	r := c.Action("process.start").Run(ctx, core.NewOptions(
//	    core.Option{Key: "command", Value: "docker"},
//	    core.Option{Key: "args", Value: []string{"run", "nginx"}},
//	))
//	id := r.Value.(string)
func (s *Service) handleStart(ctx context.Context, opts core.Options) core.Result {
	command := opts.String("command")
	if command == "" {
		return core.Result{Value: coreerr.E("process.start", "command is required", nil), OK: false}
	}

	runOpts := RunOptions{
		Command: command,
		Dir:     opts.String("dir"),
	}
	if r := opts.Get("args"); r.OK {
		if args, ok := r.Value.([]string); ok {
			runOpts.Args = args
		}
	}

	r := s.StartWithOptions(ctx, runOpts)
	if !r.OK {
		return r
	}
	return core.Result{Value: r.Value.(*Process).ID, OK: true}
}

// handleKill terminates a process by ID.
//
//	r := c.Action("process.kill").Run(ctx, core.NewOptions(
//	    core.Option{Key: "id", Value: "id-42-a3f2b1"},
//	))
func (s *Service) handleKill(ctx context.Context, opts core.Options) core.Result {
	id := opts.String("id")
	if id != "" {
		if err := s.Kill(id); err != nil {
			return core.Result{Value: err, OK: false}
		}
		return core.Result{OK: true}
	}
	return core.Result{Value: coreerr.E("process.kill", "id is required", nil), OK: false}
}

// handleList returns the IDs of all managed processes.
//
//	r := c.Action("process.list").Run(ctx, core.NewOptions())
//	ids := r.Value.([]string)
func (s *Service) handleList(ctx context.Context, opts core.Options) core.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.processes))
	for id := range s.processes {
		ids = append(ids, id)
	}
	return core.Result{Value: ids, OK: true}
}

// handleGet returns process info by ID.
//
//	r := c.Action("process.get").Run(ctx, core.NewOptions(
//	    core.Option{Key: "id", Value: "id-42-a3f2b1"},
//	))
//	info := r.Value.(process.Info)
func (s *Service) handleGet(ctx context.Context, opts core.Options) core.Result {
	id := opts.String("id")
	proc, err := s.Get(id)
	if err != nil {
		return core.Result{Value: err, OK: false}
	}
	return core.Result{Value: proc.Info(), OK: true}
}
