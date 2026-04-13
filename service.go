package process

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	goio "io"
)

// Default buffer size for process output (1MB).
const DefaultBufferSize = 1024 * 1024

// Errors
var (
	ErrProcessNotFound   = coreerr.E("", "process not found", nil)
	ErrProcessNotRunning = coreerr.E("", "process is not running", nil)
	ErrStdinNotAvailable = coreerr.E("", "stdin not available", nil)
	ErrContextRequired   = coreerr.E("", "context is required", nil)
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

// coreApp returns the attached Core runtime, if one exists.
func (s *Service) coreApp() *core.Core {
	if s == nil || s.ServiceRuntime == nil {
		return nil
	}
	return s.ServiceRuntime.Core()
}

// Options configures the process service.
//
// Example:
//
//	svc := process.NewService(process.Options{BufferSize: 2 * 1024 * 1024})
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
//
// Example:
//
//	factory := process.NewService(process.Options{})
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
//
// Example:
//
//	_ = svc.OnStartup(ctx)
func (s *Service) OnStartup(ctx context.Context) error {
	s.registrations.Do(func() {
		if c := s.coreApp(); c != nil {
			c.RegisterAction(s.handleTask)
		}
	})
	return nil
}

// OnShutdown implements core.Stoppable.
// Immediately kills all running processes to avoid shutdown stalls.
//
// Example:
//
//	_ = svc.OnShutdown(ctx)
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
		_, _ = p.killTree()
	}

	return nil
}

// Start spawns a new process with the given command and args.
//
// Example:
//
//	proc, err := svc.Start(ctx, "echo", "hello")
func (s *Service) Start(ctx context.Context, command string, args ...string) (*Process, error) {
	return s.StartWithOptions(ctx, RunOptions{
		Command: command,
		Args:    args,
	})
}

// StartWithOptions spawns a process with full configuration.
//
// Example:
//
//	proc, err := svc.StartWithOptions(ctx, process.RunOptions{Command: "pwd", Dir: "/tmp"})
func (s *Service) StartWithOptions(ctx context.Context, opts RunOptions) (*Process, error) {
	if opts.Command == "" {
		return nil, ServiceError("command is required", nil)
	}
	if ctx == nil {
		return nil, ServiceError("context is required", ErrContextRequired)
	}

	id := core.Sprintf("proc-%d", s.idCounter.Add(1))
	startedAt := time.Now()

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

	// Put every subprocess in its own process group so shutdown can terminate
	// the full tree without affecting the parent process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		Args:        append([]string(nil), opts.Args...),
		Dir:         opts.Dir,
		Env:         append([]string(nil), opts.Env...),
		StartedAt:   startedAt,
		Status:      StatusPending,
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
		startErr := coreerr.E("Service.StartWithOptions", "failed to start process", err)
		proc.mu.Lock()
		proc.Status = StatusFailed
		proc.ExitCode = -1
		proc.Duration = time.Since(startedAt)
		proc.mu.Unlock()

		s.mu.Lock()
		s.processes[id] = proc
		s.mu.Unlock()

		close(proc.done)
		cancel()
		if c := s.coreApp(); c != nil {
			_ = c.ACTION(ActionProcessExited{
				ID:       id,
				ExitCode: -1,
				Duration: proc.Duration,
				Error:    startErr,
			})
		}
		return proc, startErr
	}

	proc.mu.Lock()
	proc.Status = StatusRunning
	proc.mu.Unlock()

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
	if c := s.coreApp(); c != nil {
		_ = c.ACTION(ActionProcessStarted{
			ID:      id,
			Command: opts.Command,
			Args:    opts.Args,
			Dir:     opts.Dir,
			PID:     cmd.Process.Pid,
		})
	}

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

		if status == StatusKilled {
			s.emitKilledAction(proc, signalName)
		}

		exitAction := ActionProcessExited{
			ID:       id,
			ExitCode: exitCode,
			Duration: duration,
			Error:    exitErr,
		}

		if c := s.coreApp(); c != nil {
			_ = c.ACTION(exitAction)
		}
	}()

	return proc, nil
}

// streamOutput reads from a pipe and broadcasts lines via ACTION.
func (s *Service) streamOutput(proc *Process, r goio.Reader, stream Stream) {
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
		if c := s.coreApp(); c != nil {
			_ = c.ACTION(ActionProcessOutput{
				ID:     proc.ID,
				Line:   line,
				Stream: stream,
			})
		}
	}
}

// Get returns a process by ID.
//
// Example:
//
//	proc, err := svc.Get("proc-1")
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
//
// Example:
//
//	for _, proc := range svc.List() { _ = proc }
func (s *Service) List() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		result = append(result, p)
	}
	sortProcesses(result)
	return result
}

// Running returns all currently running processes.
//
// Example:
//
//	running := svc.Running()
func (s *Service) Running() []*Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Process
	for _, p := range s.processes {
		if p.IsRunning() {
			result = append(result, p)
		}
	}
	sortProcesses(result)
	return result
}

// Kill terminates a process by ID.
//
// Example:
//
//	_ = svc.Kill("proc-1")
func (s *Service) Kill(id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}

	sent, err := proc.kill()
	if err != nil {
		return err
	}
	if sent {
		s.emitKilledAction(proc, "SIGKILL")
	}
	return nil
}

// KillPID terminates a process by operating-system PID.
//
// Example:
//
//	_ = svc.KillPID(1234)
func (s *Service) KillPID(pid int) error {
	if pid <= 0 {
		return ServiceError("pid must be positive", nil)
	}

	if proc := s.findByPID(pid); proc != nil {
		sent, err := proc.kill()
		if err != nil {
			return err
		}
		if sent {
			s.emitKilledAction(proc, "SIGKILL")
		}
		return nil
	}

	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return coreerr.E("Service.KillPID", core.Sprintf("failed to signal pid %d", pid), err)
	}

	return nil
}

// Signal sends a signal to a process by ID.
//
// Example:
//
//	_ = svc.Signal("proc-1", syscall.SIGTERM)
func (s *Service) Signal(id string, sig os.Signal) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

// SignalPID sends a signal to a process by operating-system PID.
//
// Example:
//
//	_ = svc.SignalPID(1234, syscall.SIGTERM)
func (s *Service) SignalPID(pid int, sig os.Signal) error {
	if pid <= 0 {
		return ServiceError("pid must be positive", nil)
	}

	if proc := s.findByPID(pid); proc != nil {
		return proc.Signal(sig)
	}

	target, err := os.FindProcess(pid)
	if err != nil {
		return coreerr.E("Service.SignalPID", core.Sprintf("failed to find pid %d", pid), err)
	}

	if err := target.Signal(sig); err != nil {
		return coreerr.E("Service.SignalPID", core.Sprintf("failed to signal pid %d", pid), err)
	}

	return nil
}

// Remove removes a completed process from the list.
//
// Example:
//
//	_ = svc.Remove("proc-1")
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
//
// Example:
//
//	svc.Clear()
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
//
// Example:
//
//	out, err := svc.Output("proc-1")
func (s *Service) Output(id string) (string, error) {
	proc, err := s.Get(id)
	if err != nil {
		return "", err
	}
	return proc.Output(), nil
}

// Input writes data to the stdin of a managed process.
//
// Example:
//
//	_ = svc.Input("proc-1", "hello\n")
func (s *Service) Input(id string, input string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}
	return proc.SendInput(input)
}

// CloseStdin closes the stdin pipe of a managed process.
//
// Example:
//
//	_ = svc.CloseStdin("proc-1")
func (s *Service) CloseStdin(id string) error {
	proc, err := s.Get(id)
	if err != nil {
		return err
	}
	return proc.CloseStdin()
}

// Wait blocks until a managed process exits and returns its final snapshot.
//
// Example:
//
//	info, err := svc.Wait("proc-1")
func (s *Service) Wait(id string) (Info, error) {
	proc, err := s.Get(id)
	if err != nil {
		return Info{}, err
	}

	if err := proc.Wait(); err != nil {
		return proc.Info(), err
	}

	return proc.Info(), nil
}

// findByPID locates a managed process by operating-system PID.
func (s *Service) findByPID(pid int) *Process {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, proc := range s.processes {
		proc.mu.RLock()
		matches := proc.cmd != nil && proc.cmd.Process != nil && proc.cmd.Process.Pid == pid
		proc.mu.RUnlock()
		if matches {
			return proc
		}
	}
	return nil
}

// Run executes a command and waits for completion.
// Returns the combined output and any error.
//
// Example:
//
//	out, err := svc.Run(ctx, "echo", "hello")
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
		return output, coreerr.E("Service.Run", core.Sprintf("process exited with code %d", proc.ExitCode), nil)
	}
	return output, nil
}

// RunWithOptions executes a command with options and waits for completion.
//
// Example:
//
//	out, err := svc.RunWithOptions(ctx, process.RunOptions{Command: "echo", Args: []string{"hello"}})
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
		return output, coreerr.E("Service.RunWithOptions", core.Sprintf("process exited with code %d", proc.ExitCode), nil)
	}
	return output, nil
}

// handleTask dispatches Core.PERFORM messages for the process service.
func (s *Service) handleTask(c *core.Core, task core.Message) core.Result {
	switch m := task.(type) {
	case TaskProcessStart:
		proc, err := s.StartWithOptions(c.Context(), RunOptions{
			Command:        m.Command,
			Args:           m.Args,
			Dir:            m.Dir,
			Env:            m.Env,
			DisableCapture: m.DisableCapture,
			Detach:         m.Detach,
			Timeout:        m.Timeout,
			GracePeriod:    m.GracePeriod,
			KillGroup:      m.KillGroup,
		})
		if err != nil {
			return core.Result{Value: err, OK: false}
		}
		return core.Result{Value: proc.Info(), OK: true}
	case TaskProcessRun:
		output, err := s.RunWithOptions(c.Context(), RunOptions{
			Command:        m.Command,
			Args:           m.Args,
			Dir:            m.Dir,
			Env:            m.Env,
			DisableCapture: m.DisableCapture,
			Detach:         m.Detach,
			Timeout:        m.Timeout,
			GracePeriod:    m.GracePeriod,
			KillGroup:      m.KillGroup,
		})
		if err != nil {
			return core.Result{Value: err, OK: false}
		}
		return core.Result{Value: output, OK: true}
	case TaskProcessKill:
		switch {
		case m.ID != "":
			if err := s.Kill(m.ID); err != nil {
				return core.Result{Value: err, OK: false}
			}
			return core.Result{OK: true}
		case m.PID > 0:
			if err := s.KillPID(m.PID); err != nil {
				return core.Result{Value: err, OK: false}
			}
			return core.Result{OK: true}
		default:
			return core.Result{Value: coreerr.E("Service.handleTask", "task process kill requires an id or pid", nil), OK: false}
		}
	case TaskProcessSignal:
		switch {
		case m.ID != "":
			if err := s.Signal(m.ID, m.Signal); err != nil {
				return core.Result{Value: err, OK: false}
			}
			return core.Result{OK: true}
		case m.PID > 0:
			if err := s.SignalPID(m.PID, m.Signal); err != nil {
				return core.Result{Value: err, OK: false}
			}
			return core.Result{OK: true}
		default:
			return core.Result{Value: coreerr.E("Service.handleTask", "task process signal requires an id or pid", nil), OK: false}
		}
	case TaskProcessGet:
		if m.ID == "" {
			return core.Result{Value: coreerr.E("Service.handleTask", "task process get requires an id", nil), OK: false}
		}

		proc, err := s.Get(m.ID)
		if err != nil {
			return core.Result{Value: err, OK: false}
		}

		return core.Result{Value: proc.Info(), OK: true}
	case TaskProcessWait:
		if m.ID == "" {
			return core.Result{Value: coreerr.E("Service.handleTask", "task process wait requires an id", nil), OK: false}
		}

		info, err := s.Wait(m.ID)
		if err != nil {
			return core.Result{
				Value: &TaskProcessWaitError{
					Info: info,
					Err:  err,
				},
				OK: true,
			}
		}

		return core.Result{Value: info, OK: true}
	case TaskProcessOutput:
		if m.ID == "" {
			return core.Result{Value: coreerr.E("Service.handleTask", "task process output requires an id", nil), OK: false}
		}

		output, err := s.Output(m.ID)
		if err != nil {
			return core.Result{Value: err, OK: false}
		}

		return core.Result{Value: output, OK: true}
	case TaskProcessInput:
		if m.ID == "" {
			return core.Result{Value: coreerr.E("Service.handleTask", "task process input requires an id", nil), OK: false}
		}

		proc, err := s.Get(m.ID)
		if err != nil {
			return core.Result{Value: err, OK: false}
		}

		if err := proc.SendInput(m.Input); err != nil {
			return core.Result{Value: err, OK: false}
		}

		return core.Result{OK: true}
	case TaskProcessCloseStdin:
		if m.ID == "" {
			return core.Result{Value: coreerr.E("Service.handleTask", "task process close stdin requires an id", nil), OK: false}
		}

		proc, err := s.Get(m.ID)
		if err != nil {
			return core.Result{Value: err, OK: false}
		}

		if err := proc.CloseStdin(); err != nil {
			return core.Result{Value: err, OK: false}
		}

		return core.Result{OK: true}
	case TaskProcessList:
		procs := s.List()
		if m.RunningOnly {
			procs = s.Running()
		}

		infos := make([]Info, 0, len(procs))
		for _, proc := range procs {
			infos = append(infos, proc.Info())
		}

		return core.Result{Value: infos, OK: true}
	case TaskProcessRemove:
		if m.ID == "" {
			return core.Result{Value: coreerr.E("Service.handleTask", "task process remove requires an id", nil), OK: false}
		}

		if err := s.Remove(m.ID); err != nil {
			return core.Result{Value: err, OK: false}
		}

		return core.Result{OK: true}
	case TaskProcessClear:
		s.Clear()
		return core.Result{OK: true}
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
	if core.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			signalName := ws.Signal().String()
			if signalName == "" {
				signalName = "signal"
			}
			return StatusKilled, -1, coreerr.E("Service.StartWithOptions", "process was killed", nil), signalName
		}
		exitCode := exitErr.ExitCode()
		return StatusExited, exitCode, coreerr.E("Service.StartWithOptions", core.Sprintf("process exited with code %d", exitCode), nil), ""
	}

	return StatusFailed, 0, err, ""
}

// emitKilledAction broadcasts a kill event once for the given process.
func (s *Service) emitKilledAction(proc *Process, signalName string) {
	if proc == nil {
		return
	}

	proc.mu.Lock()
	if proc.killNotified {
		proc.mu.Unlock()
		return
	}
	proc.killNotified = true
	if signalName != "" {
		proc.killSignal = signalName
	} else if proc.killSignal == "" {
		proc.killSignal = "SIGKILL"
	}
	signal := proc.killSignal
	proc.mu.Unlock()

	if c := s.coreApp(); c != nil {
		_ = c.ACTION(ActionProcessKilled{
			ID:     proc.ID,
			Signal: signal,
		})
	}
}

// sortProcesses orders processes by start time, then ID for stable output.
func sortProcesses(procs []*Process) {
	slices.SortFunc(procs, func(a, b *Process) int {
		if a.StartedAt.Equal(b.StartedAt) {
			if a.ID < b.ID {
				return -1
			}
			if a.ID > b.ID {
				return 1
			}
			return 0
		}
		if a.StartedAt.Before(b.StartedAt) {
			return -1
		}
		return 1
	})
}
