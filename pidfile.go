package process

import (
	"bytes"
	"path"
	"strconv"
	"sync"
	"syscall"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
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
		pid, err := strconv.Atoi(string(bytes.TrimSpace([]byte(data))))
		if err == nil && pid > 0 {
			if proc, err := processHandle(pid); err == nil {
				if err := proc.Signal(syscall.Signal(0)); err == nil {
					return core.E("pidfile.acquire", core.Concat("another instance is running (PID ", strconv.Itoa(pid), ")"), nil)
				}
			}
		}
		_ = coreio.Local.Delete(p.path)
	}

	if dir := path.Dir(p.path); dir != "." {
		if err := coreio.Local.EnsureDir(dir); err != nil {
			return core.E("pidfile.acquire", "failed to create PID directory", err)
		}
	}

	pid := currentPID()
	if err := coreio.Local.Write(p.path, strconv.Itoa(pid)); err != nil {
		return core.E("pidfile.acquire", "failed to write PID file", err)
	}

	return nil
}

// Release removes the PID file.
func (p *PIDFile) Release() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := coreio.Local.Delete(p.path); err != nil {
		return core.E("pidfile.release", "failed to remove PID file", err)
	}
	return nil
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

	pid, err := strconv.Atoi(string(bytes.TrimSpace([]byte(data))))
	if err != nil || pid <= 0 {
		return 0, false
	}

	proc, err := processHandle(pid)
	if err != nil {
		return pid, false
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return pid, false
	}

	return pid, true
}
