package process

import (
	// Note: banned-imports exception: go-process is THE implementation of core.Process and cannot depend on itself; core.* helpers are downstream and unavailable at this layer.
	"context"
	// Note: banned-imports exception: core.* string/format helpers are downstream from this core.Process primitive and unavailable here.
	"fmt"
	// Note: banned-imports exception: go-process is THE implementation of core.Process and cannot depend on itself; OS handles and signals are intrinsic to process management.
	"os"
	// Note: banned-imports exception: os/exec is intrinsic to process management in THE implementation of core.Process, which cannot depend on itself.
	"os/exec"
	// Note: banned-imports exception: synchronization is intrinsic to this core.Process primitive; go-process cannot depend on downstream core.* helpers.
	"sync"
	// Note: banned-imports exception: syscall is intrinsic to process management in THE implementation of core.Process, which cannot depend on itself.
	"syscall"
	// Note: banned-imports exception: process lifecycle timing is intrinsic here; core.* helpers are downstream and unavailable at this layer.
	"time"

	coreerr "dappco.re/go/log"
	// Note: banned-imports exception: stdlib io is intrinsic for process pipes; go-process is THE core.Process implementation and cannot self-depend.
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

	cmd          *exec.Cmd
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
//	fmt.Println(proc.Output())
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
//	if err := proc.Wait(); err != nil { return err }
func (p *ManagedProcess) Wait() error {
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
func (p *ManagedProcess) Done() <-chan struct{} {
	return p.done
}

// Kill forcefully terminates the process.
// If KillGroup is set, kills the entire process group.
//
// Example:
//
//	_ = proc.Kill()
func (p *ManagedProcess) Kill() error {
	_, err := p.kill()
	return err
}

// kill terminates the process and reports whether a signal was actually sent.
func (p *ManagedProcess) kill() (bool, error) {
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
func (p *ManagedProcess) killTree() (bool, error) {
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
func (p *ManagedProcess) Shutdown() error {
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
func (p *ManagedProcess) terminate() error {
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
func (p *ManagedProcess) Signal(sig os.Signal) error {
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

	if sysSig == 0 {
		return syscall.Kill(-cmd.Process.Pid, 0)
	}

	if err := syscall.Kill(-cmd.Process.Pid, sysSig); err != nil {
		return err
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
				_ = syscall.Kill(-pid, sig)
			}
		}

		select {
		case <-done:
			return
		default:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	}(cmd.Process.Pid, sysSig, p.done)

	return nil
}

// SendInput writes to the process stdin.
//
// Example:
//
//	_ = proc.SendInput("hello\n")
func (p *ManagedProcess) SendInput(input string) error {
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
func (p *ManagedProcess) CloseStdin() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin == nil {
		return nil
	}

	err := p.stdin.Close()
	p.stdin = nil
	return err
}
