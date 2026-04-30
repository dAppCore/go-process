package process

import (
	"strconv"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"syscall"

	"dappco.re/go"
)

// PIDFile manages a process ID file for single-instance enforcement.
//
// Example:
//
//	pidFile := process.NewPIDFile("/var/run/myapp.pid")
type PIDFile struct {
	path string
	mu   sync.Mutex
}

// NewPIDFile creates a PID file manager.
//
// Example:
//
//	pidFile := process.NewPIDFile("/var/run/myapp.pid")
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Acquire writes the current PID to the file.
// Returns error if another instance is running.
//
// Example:
//
//	if err := pidFile.Acquire(); err != nil { return err }
func (p *PIDFile) Acquire() core.Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if read := core.ReadFile(p.path); read.OK {
		data := string(read.Value.([]byte))
		pid, err := strconv.Atoi(core.Trim(data))
		if err == nil && pid > 0 {
			if r := processSignal(pid, syscall.Signal(0)); r.OK {
				return core.Fail(core.E("pidfile.acquire", core.Concat("another instance is running (PID ", strconv.Itoa(pid), ")"), nil))
			}
		}
		if remove := core.Remove(p.path); !remove.OK {
			err, _ := remove.Value.(error)
			return core.Fail(core.E("pidfile.acquire", "failed to remove stale PID file", err))
		}
	}

	if dir := core.PathDir(p.path); dir != "." {
		if mkdir := core.MkdirAll(dir, 0755); !mkdir.OK {
			err, _ := mkdir.Value.(error)
			return core.Fail(core.E("pidfile.acquire", "failed to create PID directory", err))
		}
	}

	pid := currentPID()
	if write := core.WriteFile(p.path, []byte(strconv.Itoa(pid)), 0644); !write.OK {
		err, _ := write.Value.(error)
		return core.Fail(core.E("pidfile.acquire", "failed to write PID file", err))
	}

	return core.Ok(nil)
}

// Release removes the PID file.
// Returns nil if the PID file does not exist.
//
// Example:
//
//	_ = pidFile.Release()
func (p *PIDFile) Release() core.Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !core.Stat(p.path).OK {
		return core.Ok(nil)
	}
	if remove := core.Remove(p.path); !remove.OK {
		err, _ := remove.Value.(error)
		return core.Fail(core.E("pidfile.release", "failed to remove PID file", err))
	}
	return core.Ok(nil)
}

// Path returns the PID file path.
//
// Example:
//
//	path := pidFile.Path()
func (p *PIDFile) Path() string {
	return p.path
}

// ReadPID reads a PID file and checks if the process is still running.
// Returns (pid, true) if the process is alive, (pid, false) if dead/stale,
// or (0, false) if the file doesn't exist or is invalid.
//
// Example:
//
//	pid, running := process.ReadPID("/var/run/myapp.pid")
func ReadPID(path string) (int, bool) {
	read := core.ReadFile(path)
	if !read.OK {
		return 0, false
	}

	data := string(read.Value.([]byte))
	pid, err := strconv.Atoi(core.Trim(data))
	if err != nil || pid <= 0 {
		return 0, false
	}

	if r := processSignal(pid, syscall.Signal(0)); !r.OK {
		return pid, false
	}

	return pid, true
}
