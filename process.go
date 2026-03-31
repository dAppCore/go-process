package process

import (
	"context"
	"strconv"
	"sync"
	"syscall"
	"time"

	"dappco.re/go/core"
)

type processStdin interface {
	Write(p []byte) (n int, err error)
	Close() error
}

// ManagedProcess represents a tracked external process started by the service.
type ManagedProcess struct {
	ID        string
	PID       int
	Command   string
	Args      []string
	Dir       string
	Env       []string
	StartedAt time.Time
	Status    Status
	ExitCode  int
	Duration  time.Duration

	cmd         *execCmd
	ctx         context.Context
	cancel      context.CancelFunc
	output      *RingBuffer
	stdin       processStdin
	done        chan struct{}
	mu          sync.RWMutex
	gracePeriod time.Duration
	killGroup   bool
	lastSignal  string
	killEmitted bool
}

// Process is kept as a compatibility alias for ManagedProcess.
type Process = ManagedProcess

// Info returns a snapshot of process state.
func (p *ManagedProcess) Info() ProcessInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProcessInfo{
		ID:        p.ID,
		Command:   p.Command,
		Args:      append([]string(nil), p.Args...),
		Dir:       p.Dir,
		StartedAt: p.StartedAt,
		Running:   p.Status == StatusRunning,
		Status:    p.Status,
		ExitCode:  p.ExitCode,
		Duration:  p.Duration,
		PID:       p.PID,
	}
}

// Output returns the captured output as a string.
func (p *ManagedProcess) Output() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.output == nil {
		return ""
	}
	return p.output.String()
}

// OutputBytes returns the captured output as bytes.
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
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Wait blocks until the process exits.
func (p *ManagedProcess) Wait() error {
	<-p.done
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Status == StatusFailed {
		return core.E("process.wait", core.Concat("process failed to start: ", p.ID), nil)
	}
	if p.Status == StatusKilled {
		return core.E("process.wait", core.Concat("process was killed: ", p.ID), nil)
	}
	if p.ExitCode != 0 {
		return core.E("process.wait", core.Concat("process exited with code ", strconv.Itoa(p.ExitCode)), nil)
	}
	return nil
}

// Done returns a channel that closes when the process exits.
func (p *ManagedProcess) Done() <-chan struct{} {
	return p.done
}

// Kill forcefully terminates the process.
// If KillGroup is set, kills the entire process group.
func (p *ManagedProcess) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != StatusRunning {
		return nil
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	p.lastSignal = "SIGKILL"
	if p.killGroup {
		// Kill entire process group (negative PID)
		return syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	return p.cmd.Process.Kill()
}

// Shutdown gracefully stops the process: SIGTERM, then SIGKILL after grace period.
// If GracePeriod was not set (zero), falls back to immediate Kill().
// If KillGroup is set, signals are sent to the entire process group.
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
	p.lastSignal = "SIGTERM"
	return syscall.Kill(pid, syscall.SIGTERM)
}

// SendInput writes to the process stdin.
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

func (p *ManagedProcess) requestedSignal() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastSignal
}

func (p *ManagedProcess) markKillEmitted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.killEmitted {
		return false
	}
	p.killEmitted = true
	return true
}
