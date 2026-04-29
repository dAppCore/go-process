package process

import (
	"context"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"syscall"
	"time"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"
	goio "io"
)

// ManagedProcess represents a managed external process.
//
// Example:
//
//	proc, err := svc.Start(ctx, "echo", "hello")
type ManagedProcess struct {
	ID        string
	Command   string
	Args      []string
	Dir       string
	Env       []string
	StartedAt time.Time
	Status    Status
	ExitCode  int
	Duration  time.Duration

	cmd          *core.Cmd
	ctx          context.Context
	cancel       context.CancelFunc
	output       *RingBuffer
	stdin        goio.WriteCloser
	done         chan struct{}
	mu           sync.RWMutex
	gracePeriod  time.Duration
	killGroup    bool
	killNotified bool
	killSignal   string
}

// Process is kept as an alias for ManagedProcess for compatibility.
type Process = ManagedProcess

// Info returns a snapshot of process state.
//
// Example:
//
//	info := proc.Info()
func (p *ManagedProcess) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pid := 0
	if p.cmd != nil && p.cmd.Process != nil {
		pid = p.cmd.Process.Pid
	}

	duration := p.Duration
	if p.Status == StatusRunning {
		duration = time.Since(p.StartedAt)
	}

	return Info{
		ID:        p.ID,
		Command:   p.Command,
		Args:      append([]string(nil), p.Args...),
		Dir:       p.Dir,
		StartedAt: p.StartedAt,
		Running:   p.Status == StatusRunning,
		Status:    p.Status,
		ExitCode:  p.ExitCode,
		Duration:  duration,
		PID:       pid,
	}
}

// Output returns the captured output as a string.
//
// Example:
//
//	core.Println(proc.Output())
func (p *ManagedProcess) Output() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.output == nil {
		return ""
	}
	return p.output.String()
}

// OutputBytes returns the captured output as bytes.
//
// Example:
//
//	data := proc.OutputBytes()
func (p *ManagedProcess) OutputBytes() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.output == nil {
		return nil
	}
	return p.output.Bytes()
}

// IsRunning returns true if the process is still executing.
func (p *ManagedProcess) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status == StatusRunning
}

// Wait blocks until the process exits.
//
// Example:
//
//	if r := proc.Wait(); !r.OK { return r }
func (p *ManagedProcess) Wait() core.Result {
	<-p.done
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Status == StatusFailed {
		return core.Fail(coreerr.E("Process.Wait", core.Sprintf("process failed to start: %s", p.ID), nil))
	}
	if p.Status == StatusKilled {
		return core.Fail(coreerr.E("Process.Wait", core.Sprintf("process was killed: %s", p.ID), nil))
	}
	if p.ExitCode != 0 {
		return core.Fail(coreerr.E("Process.Wait", core.Sprintf("process exited with code %d", p.ExitCode), nil))
	}
	return core.Ok(nil)
}

// Done returns a channel that closes when the process exits.
//
// Example:
//
//	<-proc.Done()
func (p *ManagedProcess) Done() <-chan struct{} {
	return p.done
}

// Kill forcefully terminates the process.
// If KillGroup is set, kills the entire process group.
//
// Example:
//
//	_ = proc.Kill()
func (p *ManagedProcess) Kill() core.Result {
	_, err := p.kill()
	return core.ResultOf(nil, err)
}

// kill terminates the process and reports whether a signal was actually sent.
func (p *ManagedProcess) kill() (bool, goError) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StatusRunning {
		return false, nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return false, nil
	}

	if p.killGroup {
		// Kill entire process group (negative PID)
		return true, syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	return true, p.cmd.Process.Kill()
}

// killTree forcefully terminates the process group when one exists.
func (p *ManagedProcess) killTree() (bool, goError) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StatusRunning {
		return false, nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return false, nil
	}

	return true, syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
}

// Shutdown gracefully stops the process: SIGTERM, then SIGKILL after grace period.
// If GracePeriod was not set (zero), falls back to immediate Kill().
// If KillGroup is set, signals are sent to the entire process group.
//
// Example:
//
//	_ = proc.Shutdown()
func (p *ManagedProcess) Shutdown() core.Result {
	p.mu.RLock()
	grace := p.gracePeriod
	p.mu.RUnlock()

	if grace <= 0 {
		return p.Kill()
	}

	// Send SIGTERM
	if r := p.terminate(); !r.OK {
		return p.Kill()
	}

	// Wait for exit or grace period
	select {
	case <-p.done:
		return core.Ok(nil)
	case <-time.After(grace):
		return p.Kill()
	}
}

// terminate sends SIGTERM to the process (or process group if KillGroup is set).
func (p *ManagedProcess) terminate() core.Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StatusRunning {
		return core.Ok(nil)
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return core.Ok(nil)
	}

	pid := p.cmd.Process.Pid
	if p.killGroup {
		pid = -pid
	}
	return core.ResultOf(nil, syscall.Kill(pid, syscall.SIGTERM))
}

// Signal sends a signal to the process.
//
// Example:
//
//	_ = proc.Signal(syscall.SIGINT)
func (p *ManagedProcess) Signal(sig syscall.Signal) core.Result {
	p.mu.RLock()
	status := p.Status
	cmd := p.cmd
	killGroup := p.killGroup
	p.mu.RUnlock()

	if status != StatusRunning {
		return core.Fail(ErrProcessNotRunning)
	}

	if cmd == nil || cmd.Process == nil {
		return core.Ok(nil)
	}

	if !killGroup {
		return core.ResultOf(nil, cmd.Process.Signal(sig))
	}

	if sig == 0 {
		return core.ResultOf(nil, syscall.Kill(-cmd.Process.Pid, 0))
	}

	if err := syscall.Kill(-cmd.Process.Pid, sig); err != nil {
		return core.Fail(err)
	}

	// Some shells briefly ignore or defer the signal while they are still
	// initialising child jobs. Retry a few times after short delays so the
	// whole process group is more reliably terminated. If the requested signal
	// still does not stop the group, escalate to SIGKILL so callers do not hang.
	go func(pid int, sig syscall.Signal, done <-chan struct{}) {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 5; i++ {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := syscall.Kill(-pid, sig); err != nil {
					return
				}
			}
		}

		select {
		case <-done:
			return
		default:
			if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
				return
			}
		}
	}(cmd.Process.Pid, sig, p.done)

	return core.Ok(nil)
}

// SendInput writes to the process stdin.
//
// Example:
//
//	_ = proc.SendInput("hello\n")
func (p *ManagedProcess) SendInput(input string) core.Result {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Status != StatusRunning {
		return core.Fail(ErrProcessNotRunning)
	}

	if p.stdin == nil {
		return core.Fail(ErrStdinNotAvailable)
	}

	_, err := p.stdin.Write([]byte(input))
	return core.ResultOf(nil, err)
}

// CloseStdin closes the process stdin pipe.
//
// Example:
//
//	_ = proc.CloseStdin()
func (p *ManagedProcess) CloseStdin() core.Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin == nil {
		return core.Ok(nil)
	}

	err := p.stdin.Close()
	p.stdin = nil
	return core.ResultOf(nil, err)
}
