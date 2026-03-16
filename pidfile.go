package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	coreio "forge.lthn.ai/core/go-io"
)

// PIDFile manages a process ID file for single-instance enforcement.
type PIDFile struct {
	path string
	mu   sync.Mutex
}

// NewPIDFile creates a PID file manager.
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Acquire writes the current PID to the file.
// Returns error if another instance is running.
func (p *PIDFile) Acquire() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if data, err := coreio.Local.Read(p.path); err == nil {
		pid, err := strconv.Atoi(strings.TrimSpace(data))
		if err == nil && pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				if err := proc.Signal(syscall.Signal(0)); err == nil {
					return fmt.Errorf("another instance is running (PID %d)", pid)
				}
			}
		}
		_ = coreio.Local.Delete(p.path)
	}

	if dir := filepath.Dir(p.path); dir != "." {
		if err := coreio.Local.EnsureDir(dir); err != nil {
			return fmt.Errorf("failed to create PID directory: %w", err)
		}
	}

	pid := os.Getpid()
	if err := coreio.Local.Write(p.path, strconv.Itoa(pid)); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Release removes the PID file.
func (p *PIDFile) Release() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return coreio.Local.Delete(p.path)
}

// Path returns the PID file path.
func (p *PIDFile) Path() string {
	return p.path
}

// ReadPID reads a PID file and checks if the process is still running.
// Returns (pid, true) if the process is alive, (pid, false) if dead/stale,
// or (0, false) if the file doesn't exist or is invalid.
func ReadPID(path string) (int, bool) {
	data, err := coreio.Local.Read(path)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(data))
	if err != nil || pid <= 0 {
		return 0, false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return pid, false
	}

	return pid, true
}
