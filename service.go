package process

import (
	// Note: AX-6 intrinsic — bufio.Scanner frames process pipe output into lines.
	"bufio"
	"context"
	"slices"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	// Note: AX-6 intrinsic — syscall signal constants, process-group setup, and wait status fields are structural to process lifecycle management.
	"syscall"
	"time"

	"dappco.re/go"
	coreerr "dappco.re/go/log"
	// Note: AX-6 intrinsic — Reader/Writer interfaces are structural process-pipe contracts; core types do not replace stdlib stream boundaries.
	goio "io"
)

// DefaultBufferSize is the default buffer size for process output (1MB).
const DefaultBufferSize = 1024 * 1024

// Errors
var (
	ErrProcessNotFound   = coreerr.E("", "process not found", nil)
	ErrProcessNotRunning = coreerr.E("", "process is not running", nil)
	ErrStdinNotAvailable = coreerr.E("", "stdin not available", nil)
	ErrContextRequired   = coreerr.E("", "context is required", nil)
	ErrUncatchableSignal = coreerr.E("", "signal cannot be caught", nil)
)

// Service manages process execution with Core IPC integration.
type Service struct {
	*core.ServiceRuntime[Options]

	processes     map[string]*Process
	mu            sync.RWMutex
	bufSize       int
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
func NewService(opts Options) func(*core.Core) core.Result {
	return func(c *core.Core) core.Result {
		if opts.BufferSize == 0 {
			opts.BufferSize = DefaultBufferSize
		}
		svc := &Service{
			ServiceRuntime: core.NewServiceRuntime(c, opts),
			processes:      make(map[string]*Process),
			bufSize:        opts.BufferSize,
		}
		return core.Ok(svc)
	}
}

// OnStartup implements core.Startable.
//
// Example:
//
//	_ = svc.OnStartup(ctx)
func (s *Service) OnStartup(context.Context) core.Result {
	s.registrations.Do(func() {
		if c := s.coreApp(); c != nil {
			c.Action("process.run", s.handleRun)
			c.Action("process.start", s.handleStart)
			c.Action("process.kill", s.handleKill)
			c.Action("process.list", s.handleList)
			c.Action("process.get", s.handleGet)
			c.RegisterAction(s.handleTask)
		}
	})
	return core.Result{OK: true}
}

// OnShutdown implements core.Stoppable.
// Immediately kills all running processes to avoid shutdown stalls.
//
// Example:
//
//	_ = svc.OnShutdown(ctx)
func (s *Service) OnShutdown(context.Context) core.Result {
	s.mu.RLock()
	procs := make([]*Process, 0, len(s.processes))
	for _, p := range s.processes {
		if p.IsRunning() {
			procs = append(procs, p)
		}
	}
	s.mu.RUnlock()

	for _, p := range procs {
		if result := p.killTree(); !result.OK {
			core.Print(core.Stderr(), "process shutdown kill failed: %s", result.Error())
		}
	}

	return core.Ok(nil)
}

// Start spawns a new process with the given command and args.
//
// Example:
//
//	result := svc.Start(ctx, "echo", "hello")
func (s *Service) Start(ctx context.Context, command string, args ...string) core.Result {
	return s.StartWithOptions(ctx, RunOptions{
		Command: command,
		Args:    args,
	})
}

// StartWithOptions spawns a process with full configuration.
//
// Example:
//
//	result := svc.StartWithOptions(ctx, process.RunOptions{Command: "pwd", Dir: "/tmp"})
func (s *Service) StartWithOptions(ctx context.Context, opts RunOptions) core.Result {
	if opts.Command == "" {
		return ServiceError("command is required", nil)
	}
	if ctx == nil {
		return ServiceError("context is required", ErrContextRequired)
	}

	id := core.ID()
	startedAt := time.Now()

	if opts.KillGroup && !opts.Detach {
		return core.Fail(coreerr.E("Service.StartWithOptions", "KillGroup requires Detach", nil))
	}

	// Detached processes use Background context so they survive parent death
	parentCtx := ctx
	if opts.Detach {
		parentCtx = context.Background()
	}
	procCtx, cancel := context.WithCancel(parentCtx)
	cmd := commandContext(procCtx, opts.Command, opts.Args...)

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
		return core.Fail(coreerr.E("Service.StartWithOptions", "failed to create stdout pipe", err))
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return core.Fail(coreerr.E("Service.StartWithOptions", "failed to create stderr pipe", err))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return core.Fail(coreerr.E("Service.StartWithOptions", "failed to create stdin pipe", err))
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
			if result := c.ACTION(ActionProcessExited{
				ID:       id,
				ExitCode: -1,
				Duration: proc.Duration,
				Error:    startErr,
			}); !result.OK {
				return core.Fail(startErr)
			}
		}
		return core.Fail(startErr)
	}

	proc.mu.Lock()
	proc.Status = StatusRunning
	proc.mu.Unlock()

	if ctx != nil {
		go func() {
			<-procCtx.Done()
			if proc.IsRunning() && cmd.Process != nil {
				if err := cmd.Process.Kill(); err != nil {
					core.Print(core.Stderr(), "process context kill failed: %s", err)
				}
			}
		}()
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
				if result := proc.Shutdown(); !result.OK {
					core.Print(core.Stderr(), "process timeout shutdown failed: %s", result.Error())
				}
			}
		}()
	}

	// Broadcast start
	if c := s.coreApp(); c != nil {
		if result := c.ACTION(ActionProcessStarted{
			ID:      id,
			Command: opts.Command,
			Args:    opts.Args,
			Dir:     opts.Dir,
			PID:     cmd.Process.Pid,
		}); !result.OK {
			core.Print(core.Stderr(), "process start broadcast failed: %s", result.Error())
		}
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
		exit := classifyProcessExit(err)

		proc.mu.Lock()
		proc.Duration = duration
		proc.ExitCode = exit.exitCode
		proc.Status = exit.status
		proc.mu.Unlock()

		close(proc.done)

		if exit.status == StatusKilled {
			s.emitKilledAction(proc, exit.signalName)
		}

		exitAction := ActionProcessExited{
			ID:       id,
			ExitCode: exit.exitCode,
			Duration: duration,
			Error:    exit.err,
		}

		if c := s.coreApp(); c != nil {
			if result := c.ACTION(exitAction); !result.OK {
				return
			}
		}
	}()

	return core.Ok(proc)
}

// streamOutput reads from a pipe and broadcasts lines via ACTION.
func (s *Service) streamOutput(proc *Process, r goio.Reader, stream Stream) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		// Write to ring buffer
		if proc.output != nil {
			if result := proc.output.Write([]byte(line + "\n")); !result.OK {
				continue
			}
		}

		// Broadcast output
		if c := s.coreApp(); c != nil {
			if result := c.ACTION(ActionProcessOutput{
				ID:     proc.ID,
				Line:   line,
				Stream: stream,
			}); !result.OK {
				continue
			}
		}
	}
}

// Get returns a process by ID.
//
// Example:
//
//	result := svc.Get("proc-1")
func (s *Service) Get(id string) core.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proc, ok := s.processes[id]
	if !ok {
		return core.Fail(ErrProcessNotFound)
	}
	return core.Ok(proc)
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
func (s *Service) Kill(id string) core.Result {
	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)

	sent := proc.kill()
	if !sent.OK {
		return sent
	}
	if sent.Value.(bool) {
		s.emitKilledAction(proc, "SIGKILL")
	}
	return core.Ok(nil)
}

// KillPID terminates a process by operating-system PID.
//
// Example:
//
//	_ = svc.KillPID(1234)
func (s *Service) KillPID(pid int) core.Result {
	if pid <= 0 {
		return ServiceError("pid must be positive", nil)
	}

	if proc := s.findByPID(pid); proc != nil {
		sent := proc.kill()
		if !sent.OK {
			return sent
		}
		if sent.Value.(bool) {
			s.emitKilledAction(proc, "SIGKILL")
		}
		return core.Ok(nil)
	}

	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return core.Fail(coreerr.E("Service.KillPID", core.Sprintf("failed to signal pid %d", pid), err))
	}

	return core.Ok(nil)
}

// Signal sends a signal to a process by ID.
//
// Example:
//
//	_ = svc.Signal("proc-1", syscall.SIGTERM)
func (s *Service) Signal(id string, sig syscall.Signal) core.Result {
	if r := validateCatchableSignals(sig); !r.OK {
		return r
	}

	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)
	return proc.Signal(sig)
}

// SignalPID sends a signal to a process by operating-system PID.
//
// Example:
//
//	_ = svc.SignalPID(1234, syscall.SIGTERM)
func (s *Service) SignalPID(pid int, sig syscall.Signal) core.Result {
	if r := validateCatchableSignals(sig); !r.OK {
		return r
	}

	if pid <= 0 {
		return ServiceError("pid must be positive", nil)
	}

	if proc := s.findByPID(pid); proc != nil {
		return proc.Signal(sig)
	}

	if err := syscall.Kill(pid, sig); err != nil {
		return core.Fail(coreerr.E("Service.SignalPID", core.Sprintf("failed to signal pid %d", pid), err))
	}

	return core.Ok(nil)
}

func validateCatchableSignals(sig syscall.Signal) core.Result {
	switch sig {
	case syscall.SIGKILL, syscall.SIGSTOP:
		return core.Fail(coreerr.E(
			"Service.validateCatchableSignals",
			core.Sprintf("signal %d cannot be caught", int(sig)),
			ErrUncatchableSignal,
		))
	}

	return core.Ok(nil)
}

// Remove removes a completed process from the list.
//
// Example:
//
//	_ = svc.Remove("proc-1")
func (s *Service) Remove(id string) core.Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	proc, ok := s.processes[id]
	if !ok {
		return core.Fail(ErrProcessNotFound)
	}

	if proc.IsRunning() {
		return core.Fail(coreerr.E("Service.Remove", "cannot remove running process", nil))
	}

	delete(s.processes, id)
	return core.Ok(nil)
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
//	result := svc.Output("proc-1")
func (s *Service) Output(id string) core.Result {
	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)
	return core.Ok(proc.Output())
}

// Input writes data to the stdin of a managed process.
//
// Example:
//
//	_ = svc.Input("proc-1", "hello\n")
func (s *Service) Input(id string, input string) core.Result {
	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)
	return proc.SendInput(input)
}

// CloseStdin closes the stdin pipe of a managed process.
//
// Example:
//
//	_ = svc.CloseStdin("proc-1")
func (s *Service) CloseStdin(id string) core.Result {
	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)
	return proc.CloseStdin()
}

// Wait blocks until a managed process exits and returns its final snapshot.
//
// Example:
//
//	result := svc.Wait("proc-1")
func (s *Service) Wait(id string) core.Result {
	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)

	if r := proc.Wait(); !r.OK {
		return core.Fail(&TaskProcessWaitError{
			Info: proc.Info(),
			Err:  r.Value.(error),
		})
	}

	return core.Ok(proc.Info())
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
//	result := svc.Run(ctx, "echo", "hello")
func (s *Service) Run(ctx context.Context, command string, args ...string) core.Result {
	result := s.Start(ctx, command, args...)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)

	<-proc.Done()

	output := proc.Output()
	if proc.Status == StatusKilled {
		return core.Fail(coreerr.E("Service.Run", "process was killed", nil))
	}
	if proc.ExitCode != 0 {
		return core.Fail(coreerr.E("Service.Run", core.Sprintf("process exited with code %d", proc.ExitCode), nil))
	}
	return core.Ok(output)
}

// RunWithOptions executes a command with options and waits for completion.
//
// Example:
//
//	result := svc.RunWithOptions(ctx, process.RunOptions{Command: "echo", Args: []string{"hello"}})
func (s *Service) RunWithOptions(ctx context.Context, opts RunOptions) core.Result {
	result := s.StartWithOptions(ctx, opts)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)

	<-proc.Done()

	output := proc.Output()
	if proc.Status == StatusKilled {
		return core.Fail(coreerr.E("Service.RunWithOptions", "process was killed", nil))
	}
	if proc.ExitCode != 0 {
		return core.Fail(coreerr.E("Service.RunWithOptions", core.Sprintf("process exited with code %d", proc.ExitCode), nil))
	}
	return core.Ok(output)
}

func (s *Service) handleRun(ctx context.Context, opts core.Options) core.Result {
	parsedResult := parseProcessActionInput(opts, true)
	if !parsedResult.OK {
		return parsedResult
	}
	parsed := parsedResult.Value.(processActionInput)

	outputResult := s.RunWithOptions(ctx, runOptionsFromAction(parsed))
	if !outputResult.OK {
		return outputResult
	}
	return core.Ok(outputResult.Value)
}

func (s *Service) handleStart(ctx context.Context, opts core.Options) core.Result {
	parsedResult := parseProcessActionInput(opts, true)
	if !parsedResult.OK {
		return parsedResult
	}
	parsed := parsedResult.Value.(processActionInput)

	startResult := s.StartWithOptions(ctx, startRunOptionsFromAction(parsed))
	if !startResult.OK {
		return startResult
	}
	proc := startResult.Value.(*Process)
	return core.Ok(proc.ID)
}

func (s *Service) handleKill(ctx context.Context, opts core.Options) core.Result {
	_ = ctx
	targetResult := parseProcessActionTarget(opts)
	if !targetResult.OK {
		return targetResult
	}
	target := targetResult.Value.(processActionInput)

	switch {
	case target.ID != "":
		if r := s.Kill(target.ID); !r.OK {
			return r
		}
	case target.PID > 0:
		if r := s.KillPID(target.PID); !r.OK {
			return r
		}
	}
	return core.Ok(nil)
}

func (s *Service) handleList(ctx context.Context, opts core.Options) core.Result {
	_ = ctx
	runningOnly := opts.Bool("runningOnly")

	procs := s.List()
	if runningOnly {
		procs = s.Running()
	}

	ids := make([]string, 0, len(procs))
	for _, proc := range procs {
		ids = append(ids, proc.ID)
	}

	return core.Ok(ids)
}

func (s *Service) handleGet(ctx context.Context, opts core.Options) core.Result {
	_ = ctx
	id := core.Trim(opts.String("id"))
	if id == "" {
		return core.Fail(coreerr.E("Service.handleGet", "id is required", nil))
	}

	result := s.Get(id)
	if !result.OK {
		return result
	}
	proc := result.Value.(*Process)

	return core.Ok(proc.Info())
}

func runOptionsFromAction(input processActionInput) RunOptions {
	return RunOptions{
		Command:        input.Command,
		Args:           append([]string(nil), input.Args...),
		Dir:            input.Dir,
		Env:            append([]string(nil), input.Env...),
		DisableCapture: input.DisableCapture,
		Detach:         input.Detach,
		Timeout:        input.Timeout,
		GracePeriod:    input.GracePeriod,
		KillGroup:      input.KillGroup,
	}
}

func startRunOptionsFromAction(input processActionInput) RunOptions {
	opts := runOptionsFromAction(input)
	// RFC semantics: process.start is background execution and must not be
	// coupled to the caller context unless the caller bypasses the action layer.
	opts.Detach = true
	return opts
}

// handleTask dispatches Core.PERFORM messages for the process service.
func (s *Service) handleTask(c *core.Core, task core.Message) core.Result {
	switch m := task.(type) {
	case TaskProcessStart:
		result := s.StartWithOptions(c.Context(), startRunOptionsFromAction(processActionInput{
			Command:        m.Command,
			Args:           m.Args,
			Dir:            m.Dir,
			Env:            m.Env,
			DisableCapture: m.DisableCapture,
			Detach:         m.Detach,
			Timeout:        m.Timeout,
			GracePeriod:    m.GracePeriod,
			KillGroup:      m.KillGroup,
		}))
		if !result.OK {
			return result
		}
		proc := result.Value.(*Process)
		return core.Ok(proc.Info())
	case TaskProcessRun:
		result := s.RunWithOptions(c.Context(), RunOptions{
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
		if !result.OK {
			return result
		}
		return core.Ok(result.Value)
	case TaskProcessKill:
		switch {
		case m.ID != "":
			if r := s.Kill(m.ID); !r.OK {
				return r
			}
			return core.Ok(nil)
		case m.PID > 0:
			if r := s.KillPID(m.PID); !r.OK {
				return r
			}
			return core.Ok(nil)
		default:
			return core.Fail(coreerr.E("Service.handleTask", "task process kill requires an id or pid", nil))
		}
	case TaskProcessSignal:
		switch {
		case m.ID != "":
			if r := s.Signal(m.ID, m.Signal); !r.OK {
				return r
			}
			return core.Ok(nil)
		case m.PID > 0:
			if r := s.SignalPID(m.PID, m.Signal); !r.OK {
				return r
			}
			return core.Ok(nil)
		default:
			return core.Fail(coreerr.E("Service.handleTask", "task process signal requires an id or pid", nil))
		}
	case TaskProcessGet:
		if m.ID == "" {
			return core.Fail(coreerr.E("Service.handleTask", "task process get requires an id", nil))
		}

		result := s.Get(m.ID)
		if !result.OK {
			return result
		}
		proc := result.Value.(*Process)

		return core.Ok(proc.Info())
	case TaskProcessWait:
		if m.ID == "" {
			return core.Fail(coreerr.E("Service.handleTask", "task process wait requires an id", nil))
		}

		result := s.Wait(m.ID)
		if !result.OK {
			if waitErr, ok := result.Value.(*TaskProcessWaitError); ok {
				return core.Ok(waitErr)
			}
			return result
		}

		return core.Ok(result.Value)
	case TaskProcessOutput:
		if m.ID == "" {
			return core.Fail(coreerr.E("Service.handleTask", "task process output requires an id", nil))
		}

		result := s.Output(m.ID)
		if !result.OK {
			return result
		}

		return core.Ok(result.Value)
	case TaskProcessInput:
		if m.ID == "" {
			return core.Fail(coreerr.E("Service.handleTask", "task process input requires an id", nil))
		}

		result := s.Get(m.ID)
		if !result.OK {
			return result
		}
		proc := result.Value.(*Process)

		if r := proc.SendInput(m.Input); !r.OK {
			return r
		}

		return core.Ok(nil)
	case TaskProcessCloseStdin:
		if m.ID == "" {
			return core.Fail(coreerr.E("Service.handleTask", "task process close stdin requires an id", nil))
		}

		result := s.Get(m.ID)
		if !result.OK {
			return result
		}
		proc := result.Value.(*Process)

		if r := proc.CloseStdin(); !r.OK {
			return r
		}

		return core.Ok(nil)
	case TaskProcessList:
		procs := s.List()
		if m.RunningOnly {
			procs = s.Running()
		}

		infos := make([]Info, 0, len(procs))
		for _, proc := range procs {
			infos = append(infos, proc.Info())
		}

		return core.Ok(infos)
	case TaskProcessRemove:
		if m.ID == "" {
			return core.Fail(coreerr.E("Service.handleTask", "task process remove requires an id", nil))
		}

		if r := s.Remove(m.ID); !r.OK {
			return r
		}

		return core.Ok(nil)
	case TaskProcessClear:
		s.Clear()
		return core.Ok(nil)
	default:
		return core.Result{}
	}
}

type processExit struct {
	status     Status
	exitCode   int
	signalName string
	err        error
}

// classifyProcessExit maps a command completion error to lifecycle state.
func classifyProcessExit(err error) processExit {
	if err == nil {
		return processExit{status: StatusExited}
	}

	var exitErr interface {
		ExitCode() int
		Sys() any
	}
	if core.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			signalName := ws.Signal().String()
			if signalName == "" {
				signalName = "signal"
			}
			return processExit{
				status:     StatusKilled,
				exitCode:   -1,
				signalName: signalName,
				err:        coreerr.E("Service.StartWithOptions", "process was killed", nil),
			}
		}
		exitCode := exitErr.ExitCode()
		return processExit{
			status:   StatusExited,
			exitCode: exitCode,
			err:      coreerr.E("Service.StartWithOptions", core.Sprintf("process exited with code %d", exitCode), nil),
		}
	}

	return processExit{status: StatusFailed, err: err}
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
		if result := c.ACTION(ActionProcessKilled{
			ID:     proc.ID,
			Signal: signal,
		}); !result.OK {
			return
		}
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
