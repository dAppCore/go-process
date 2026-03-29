package process

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"dappco.re/go/core"
)

// execCmd is kept for backwards-compatible test stubbing/mocking.
type execCmd = exec.Cmd

type streamReader interface {
	Read(p []byte) (n int, err error)
}

// Default buffer size for process output (1MB).
const DefaultBufferSize = 1024 * 1024

// Errors
var (
	ErrProcessNotFound   = core.E("", "process not found", nil)
	ErrProcessNotRunning = core.E("", "process is not running", nil)
	ErrStdinNotAvailable = core.E("", "stdin not available", nil)
)

// Service manages process execution with Core IPC integration.
type Service struct {
	*core.ServiceRuntime[Options]

	managed *core.Registry[*ManagedProcess]
	bufSize int
}

// Options configures the process service.
type Options struct {
	// BufferSize is the ring buffer size for output capture.
	// Default: 1MB (1024 * 1024 bytes).
	BufferSize int
}

// NewService constructs a process service factory for Core registration.
//
//	c := framework.New(core.WithName("process", process.NewService(process.Options{})))
func NewService(opts Options) func(*core.Core) (any, error) {
	return func(c *core.Core) (any, error) {
		if opts.BufferSize == 0 {
			opts.BufferSize = DefaultBufferSize
		}
		svc := &Service{
			ServiceRuntime: core.NewServiceRuntime(c, opts),
			managed:        core.NewRegistry[*ManagedProcess](),
			bufSize:        opts.BufferSize,
		}
		return svc, nil
	}
}

// Register constructs a Service bound to the provided Core instance.
//
//	c := core.New()
//	svc := process.Register(c).Value.(*process.Service)
func Register(c *core.Core) core.Result {
	r := NewService(Options{BufferSize: DefaultBufferSize})(c)
	if r == nil {
		return core.Result{Value: core.E("process.register", "factory returned nil service", nil), OK: false}
	}

	return core.Result{Value: r, OK: true}
}

// OnStartup implements core.Startable.
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
func (s *Service) OnShutdown(ctx context.Context) core.Result {
	s.managed.Each(func(_ string, proc *ManagedProcess) {
		_ = proc.Kill()
	})
	return core.Result{OK: true}
}

// Start spawns a new process with the given command and args.
//
//	proc := svc.Start(ctx, "echo", "hello")
func (s *Service) Start(ctx context.Context, command string, args ...string) (*ManagedProcess, error) {
	return s.StartWithOptions(ctx, RunOptions{
		Command: command,
		Args:    args,
	})
}

// StartWithOptions spawns a process with full configuration.
//
//	proc := svc.StartWithOptions(ctx, process.RunOptions{Command: "go", Args: []string{"test", "./..."}})
func (s *Service) StartWithOptions(ctx context.Context, opts RunOptions) (*ManagedProcess, error) {
	return startResultToProcess(s.startWithOptions(ctx, opts), "process.start")
}

// startWithOptions is the Result-form internal implementation for StartWithOptions.
func (s *Service) startWithOptions(ctx context.Context, opts RunOptions) core.Result {
	if opts.Command == "" {
		return core.Result{Value: core.E("process.start", "command is required", nil), OK: false}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	id := core.ID()

	// Detached processes use Background context so they survive parent death
	parentCtx := ctx
	if opts.Detach {
		parentCtx = context.Background()
	}
	procCtx, cancel := context.WithCancel(parentCtx)
	cmd := execCommandContext(procCtx, opts.Command, opts.Args...)

	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), opts.Env...)
	}

	// Detached processes get their own process group.
	if opts.Detach {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Set up pipes.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return core.Result{Value: core.E("process.start", core.Concat("stdout pipe failed: ", opts.Command), err), OK: false}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return core.Result{Value: core.E("process.start", core.Concat("stderr pipe failed: ", opts.Command), err), OK: false}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return core.Result{Value: core.E("process.start", core.Concat("stdin pipe failed: ", opts.Command), err), OK: false}
	}

	// Create output buffer (enabled by default).
	var output *RingBuffer
	if !opts.DisableCapture {
		output = NewRingBuffer(s.bufSize)
	}

	proc := &ManagedProcess{
		ID:          id,
		Command:     opts.Command,
		Args:        append([]string(nil), opts.Args...),
		Dir:         opts.Dir,
		Env:         append([]string(nil), opts.Env...),
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

	// Start the process.
	if err := cmd.Start(); err != nil {
		cancel()
		return core.Result{Value: core.E("process.start", core.Concat("command failed: ", opts.Command), err), OK: false}
	}
	proc.PID = cmd.Process.Pid

	// Store process.
	if r := s.managed.Set(id, proc); !r.OK {
		cancel()
		_ = cmd.Process.Kill()
		return r
	}

	// Start timeout watchdog if configured.
	if opts.Timeout > 0 {
		go func() {
			select {
			case <-proc.done:
			case <-time.After(opts.Timeout):
				_ = proc.Shutdown()
			}
		}()
	}

	// Broadcast start.
	_ = s.Core().ACTION(ActionProcessStarted{
		ID:      id,
		Command: opts.Command,
		Args:    opts.Args,
		Dir:     opts.Dir,
		PID:     cmd.Process.Pid,
	})

	// Stream output in goroutines.
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

	// Wait for process completion.
	go func() {
		wg.Wait()
		waitErr := cmd.Wait()

		duration := time.Since(proc.StartedAt)
		status, exitCode, actionErr, killedSignal := classifyProcessExit(proc, waitErr)

		proc.mu.Lock()
		proc.Duration = duration
		proc.ExitCode = exitCode
		proc.Status = status
		proc.mu.Unlock()

		close(proc.done)

		if status == StatusKilled {
			_ = s.Core().ACTION(ActionProcessKilled{
				ID:     id,
				Signal: killedSignal,
			})
		}
		_ = s.Core().ACTION(ActionProcessExited{
			ID:       id,
			ExitCode: exitCode,
			Duration: duration,
			Error:    actionErr,
		})
	}()

	return core.Result{Value: proc, OK: true}
}

// streamOutput reads from a pipe and broadcasts lines via ACTION.
func (s *Service) streamOutput(proc *ManagedProcess, r streamReader, stream Stream) {
	scanner := bufio.NewScanner(r)
	// Increase buffer for long lines.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Write to ring buffer.
		if proc.output != nil {
			_, _ = proc.output.Write([]byte(line + "\n"))
		}

		// Broadcast output.
		_ = s.Core().ACTION(ActionProcessOutput{
			ID:     proc.ID,
			Line:   line,
			Stream: stream,
		})
	}
}

// Get returns a process by ID.
func (s *Service) Get(id string) (*ManagedProcess, error) {
	r := s.managed.Get(id)
	if !r.OK {
		return nil, ErrProcessNotFound
	}
	return r.Value, nil
}

// List returns all processes.
func (s *Service) List() []*ManagedProcess {
	result := make([]*ManagedProcess, 0, s.managed.Len())
	s.managed.Each(func(_ string, proc *ManagedProcess) {
		result = append(result, proc)
	})
	return result
}

// Running returns all currently running processes.
func (s *Service) Running() []*ManagedProcess {
	result := make([]*ManagedProcess, 0, s.managed.Len())
	s.managed.Each(func(_ string, proc *ManagedProcess) {
		if proc.IsRunning() {
			result = append(result, proc)
		}
	})
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
	return nil
}

// Remove removes a completed process from the list.
func (s *Service) Remove(id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}
	if proc.IsRunning() {
		return core.E("process.remove", core.Concat("cannot remove running process: ", id), nil)
	}
	r := s.managed.Delete(id)
	if !r.OK {
		return ErrProcessNotFound
	}
	return nil
}

// Clear removes all completed processes.
func (s *Service) Clear() {
	ids := make([]string, 0)
	s.managed.Each(func(id string, proc *ManagedProcess) {
		if !proc.IsRunning() {
			ids = append(ids, id)
		}
	})
	for _, id := range ids {
		s.managed.Delete(id)
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
func (s *Service) Run(ctx context.Context, command string, args ...string) (string, error) {
	return s.RunWithOptions(ctx, RunOptions{
		Command: command,
		Args:    args,
	})
}

// RunWithOptions executes a command with options and waits for completion.
func (s *Service) RunWithOptions(ctx context.Context, opts RunOptions) (string, error) {
	return runResultToString(s.runCommand(ctx, opts), "process.run")
}

// --- Internal request helpers. ---

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

	result, err := s.runCommand(ctx, runOpts)
	if err != nil {
		return core.Result{Value: err, OK: false}
	}
	return core.Result{Value: result, OK: true}
}

func (s *Service) handleStart(ctx context.Context, opts core.Options) core.Result {
	command := opts.String("command")
	if command == "" {
		return core.Result{Value: core.E("process.start", "command is required", nil), OK: false}
	}

	startOpts := RunOptions{
		Command: command,
		Dir:     opts.String("dir"),
		Detach:  opts.Bool("detach"),
	}
	if r := opts.Get("args"); r.OK {
		startOpts.Args = optionStrings(r.Value)
	}
	if r := opts.Get("env"); r.OK {
		startOpts.Env = optionStrings(r.Value)
	}

	proc, err := s.StartWithOptions(ctx, startOpts)
	if err != nil {
		return core.Result{Value: err, OK: false}
	}
	return core.Result{Value: proc.ID, OK: true}
}

func (s *Service) handleKill(ctx context.Context, opts core.Options) core.Result {
	id := opts.String("id")
	if id != "" {
		if err := s.Kill(id); err != nil {
			if errors.Is(err, ErrProcessNotFound) {
				return core.Result{Value: core.E("process.kill", core.Concat("not found: ", id), nil), OK: false}
			}
			return core.Result{Value: err, OK: false}
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

func (s *Service) handleList(ctx context.Context, opts core.Options) core.Result {
	return core.Result{Value: s.managed.Names(), OK: true}
}

func (s *Service) handleGet(ctx context.Context, opts core.Options) core.Result {
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

func (s *Service) runCommand(ctx context.Context, opts RunOptions) (string, error) {
	if opts.Command == "" {
		return "", core.E("process.run", "command is required", nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := execCommandContext(ctx, opts.Command, opts.Args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), opts.Env...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", core.E("process.run", core.Concat("command failed: ", opts.Command), err)
	}
	return string(output), nil
}

func classifyProcessExit(proc *ManagedProcess, err error) (Status, int, error, string) {
	if err == nil {
		return StatusExited, 0, nil, ""
	}

	if sig, ok := processExitSignal(err); ok {
		return StatusKilled, -1, err, normalizeSignalName(sig)
	}

	if ctxErr := proc.ctx.Err(); ctxErr != nil {
		signal := proc.requestedSignal()
		if signal == "" {
			signal = "SIGKILL"
		}
		return StatusKilled, -1, ctxErr, signal
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return StatusExited, exitErr.ExitCode(), err, ""
	}

	return StatusFailed, -1, err, ""
}

func processExitSignal(err error) (syscall.Signal, bool) {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ProcessState == nil {
		return 0, false
	}

	waitStatus, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !waitStatus.Signaled() {
		return 0, false
	}
	return waitStatus.Signal(), true
}

func startResultToProcess(r core.Result, operation string) (*ManagedProcess, error) {
	if r.OK {
		proc, ok := r.Value.(*ManagedProcess)
		if !ok {
			return nil, core.E(operation, "invalid process result type", nil)
		}
		return proc, nil
	}
	if err, ok := r.Value.(error); ok {
		return nil, err
	}
	return nil, core.E(operation, "process start failed", nil)
}

func runResultToString(r core.Result, operation string) (string, error) {
	if r.OK {
		output, ok := r.Value.(string)
		if !ok {
			return "", core.E(operation, "invalid run result type", nil)
		}
		return output, nil
	}
	if err, ok := r.Value.(error); ok {
		return "", err
	}
	return "", core.E(operation, "process run failed", nil)
}

func normalizeSignalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGTERM:
		return "SIGTERM"
	default:
		return sig.String()
	}
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

func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func execLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func currentPID() int {
	return os.Getpid()
}

func processHandle(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

func userHomeDir() (string, error) {
	return os.UserHomeDir()
}

func tempDir() string {
	return os.TempDir()
}

func isNotExist(err error) bool {
	return os.IsNotExist(err)
}
