package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	coreerr "dappco.re/go/core/log"
)

// Process represents a managed external process.
//
// Example:
//
//	proc, err := svc.Start(ctx, "echo", "hello")
type Process struct {
	ID        string
	Command   string
	Args      []string
	Dir       string
	Env       []string
	StartedAt time.Time
	Status    Status
	ExitCode  int
	Duration  time.Duration

	cmd         *exec.Cmd
	ctx         context.Context
	cancel      context.CancelFunc
	output      *RingBuffer
	stdin       io.WriteCloser
	done        chan struct{}
	mu          sync.RWMutex
	gracePeriod time.Duration
	killGroup   bool
}

// Info returns a snapshot of process state.
//
// Example:
//
//	info := proc.Info()
func (p *Process) Info() Info {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pid := 0
	if p.cmd != nil && p.cmd.Process != nil {
		pid = p.cmd.Process.Pid
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
		Duration:  p.Duration,
		PID:       pid,
	}
}

// Output returns the captured output as a string.
//
// Example:
//
//	fmt.Println(proc.Output())
func (p *Process) Output() string {
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
func (p *Process) OutputBytes() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.output == nil {
		return nil
	}
	return p.output.Bytes()
}

// IsRunning returns true if the process is still executing.
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status == StatusRunning
}

// Wait blocks until the process exits.
//
// Example:
//
//	if err := proc.Wait(); err != nil { return err }
func (p *Process) Wait() error {
	<-p.done
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Status == StatusFailed {
		return coreerr.E("Process.Wait", fmt.Sprintf("process failed to start: %s", p.ID), nil)
	}
	if p.Status == StatusKilled {
		return coreerr.E("Process.Wait", fmt.Sprintf("process was killed: %s", p.ID), nil)
	}
	if p.ExitCode != 0 {
		return coreerr.E("Process.Wait", fmt.Sprintf("process exited with code %d", p.ExitCode), nil)
	}
	return nil
}

// Done returns a channel that closes when the process exits.
//
// Example:
//
//	<-proc.Done()
func (p *Process) Done() <-chan struct{} {
	return p.done
}

// Kill forcefully terminates the process.
// If KillGroup is set, kills the entire process group.
//
// Example:
//
//	_ = proc.Kill()
func (p *Process) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StatusRunning {
		return nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	if p.killGroup {
		// Kill entire process group (negative PID)
		return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	return p.cmd.Process.Kill()
}

// Shutdown gracefully stops the process: SIGTERM, then SIGKILL after grace period.
// If GracePeriod was not set (zero), falls back to immediate Kill().
// If KillGroup is set, signals are sent to the entire process group.
//
// Example:
//
//	_ = proc.Shutdown()
func (p *Process) Shutdown() error {
	p.mu.RLock()
	grace := p.gracePeriod
	p.mu.RUnlock()

	if grace <= 0 {
		return p.Kill()
	}

	// Send SIGTERM
	if err := p.terminate(); err != nil {
		return p.Kill()
	}

	// Wait for exit or grace period
	select {
	case <-p.done:
		return nil
	case <-time.After(grace):
		return p.Kill()
	}
}

// terminate sends SIGTERM to the process (or process group if KillGroup is set).
func (p *Process) terminate() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StatusRunning {
		return nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	pid := p.cmd.Process.Pid
	if p.killGroup {
		pid = -pid
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// Signal sends a signal to the process.
//
// Example:
//
//	_ = proc.Signal(os.Interrupt)
func (p *Process) Signal(sig os.Signal) error {
	p.mu.RLock()
	status := p.Status
	cmd := p.cmd
	killGroup := p.killGroup
	p.mu.RUnlock()

	if status != StatusRunning {
		return ErrProcessNotRunning
	}

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if !killGroup {
		return cmd.Process.Signal(sig)
	}

	sysSig, ok := sig.(syscall.Signal)
	if !ok {
		return cmd.Process.Signal(sig)
	}

	if err := syscall.Kill(-cmd.Process.Pid, sysSig); err != nil {
		return err
	}

	// Some shells briefly ignore or defer the signal while they are still
	// initialising child jobs. Retry once after a short delay so the whole
	// process group is more reliably terminated.
	go func(pid int, sig syscall.Signal, done <-chan struct{}) {
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()

		select {
		case <-done:
			return
		case <-timer.C:
		}

		_ = syscall.Kill(-pid, sig)
	}(cmd.Process.Pid, sysSig, p.done)

	return nil
}

// SendInput writes to the process stdin.
//
// Example:
//
//	_ = proc.SendInput("hello\n")
func (p *Process) SendInput(input string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Status != StatusRunning {
		return ErrProcessNotRunning
	}

	if p.stdin == nil {
		return ErrStdinNotAvailable
	}

	_, err := p.stdin.Write([]byte(input))
	return err
}

// CloseStdin closes the process stdin pipe.
//
// Example:
//
//	_ = proc.CloseStdin()
func (p *Process) CloseStdin() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin == nil {
		return nil
	}

	err := p.stdin.Close()
	p.stdin = nil
	return err
}
