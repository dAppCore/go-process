package process

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// DaemonEntry records a running daemon in the registry.
//
// Example:
//
//	entry := process.DaemonEntry{Code: "app", Daemon: "serve", PID: os.Getpid()}
type DaemonEntry struct {
	Code    string    `json:"code"`
	Daemon  string    `json:"daemon"`
	PID     int       `json:"pid"`
	Health  string    `json:"health,omitempty"`
	Project string    `json:"project,omitempty"`
	Binary  string    `json:"binary,omitempty"`
	Started time.Time `json:"started"`
}

// Registry tracks running daemons via JSON files in a directory.
type Registry struct {
	dir string
}

// NewRegistry creates a registry backed by the given directory.
//
// Example:
//
//	reg := process.NewRegistry("/tmp/daemons")
func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir}
}

// DefaultRegistry returns a registry using ~/.core/daemons/.
//
// Example:
//
//	reg := process.DefaultRegistry()
func DefaultRegistry() *Registry {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return NewRegistry(filepath.Join(home, ".core", "daemons"))
}

// Register writes a daemon entry to the registry directory.
// If Started is zero, it is set to the current time.
// The directory is created if it does not exist.
//
// Example:
//
//	_ = reg.Register(entry)
func (r *Registry) Register(entry DaemonEntry) error {
	if entry.Started.IsZero() {
		entry.Started = time.Now()
	}

	if err := coreio.Local.EnsureDir(r.dir); err != nil {
		return coreerr.E("Registry.Register", "failed to create registry directory", err)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return coreerr.E("Registry.Register", "failed to marshal entry", err)
	}

	if err := coreio.Local.Write(r.entryPath(entry.Code, entry.Daemon), string(data)); err != nil {
		return coreerr.E("Registry.Register", "failed to write entry file", err)
	}
	return nil
}

// Unregister removes a daemon entry from the registry.
//
// Example:
//
//	_ = reg.Unregister("app", "serve")
func (r *Registry) Unregister(code, daemon string) error {
	if err := coreio.Local.Delete(r.entryPath(code, daemon)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return coreerr.E("Registry.Unregister", "failed to delete entry file", err)
	}
	return nil
}

// Get reads a single daemon entry and checks whether its process is alive.
// If the process is dead, the stale file is removed and (nil, false) is returned.
//
// Example:
//
//	entry, ok := reg.Get("app", "serve")
func (r *Registry) Get(code, daemon string) (*DaemonEntry, bool) {
	path := r.entryPath(code, daemon)

	data, err := coreio.Local.Read(path)
	if err != nil {
		return nil, false
	}

	var entry DaemonEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		_ = coreio.Local.Delete(path)
		return nil, false
	}

	if !isAlive(entry.PID) {
		_ = coreio.Local.Delete(path)
		return nil, false
	}

	return &entry, true
}

// List returns all alive daemon entries, pruning any with dead PIDs.
//
// Example:
//
//	entries, err := reg.List()
func (r *Registry) List() ([]DaemonEntry, error) {
	matches, err := filepath.Glob(filepath.Join(r.dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var alive []DaemonEntry
	for _, path := range matches {
		data, err := coreio.Local.Read(path)
		if err != nil {
			continue
		}

		var entry DaemonEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			_ = coreio.Local.Delete(path)
			continue
		}

		if !isAlive(entry.PID) {
			_ = coreio.Local.Delete(path)
			continue
		}

		alive = append(alive, entry)
	}

	sort.Slice(alive, func(i, j int) bool {
		if alive[i].Started.Equal(alive[j].Started) {
			if alive[i].Code == alive[j].Code {
				return alive[i].Daemon < alive[j].Daemon
			}
			return alive[i].Code < alive[j].Code
		}
		return alive[i].Started.Before(alive[j].Started)
	})

	return alive, nil
}

// entryPath returns the filesystem path for a daemon entry.
func (r *Registry) entryPath(code, daemon string) string {
	name := strings.ReplaceAll(code, "/", "-") + "-" + strings.ReplaceAll(daemon, "/", "-") + ".json"
	return filepath.Join(r.dir, name)
}

// isAlive checks whether a process with the given PID is running.
func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
