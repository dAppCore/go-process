package process

import (
	"sort"
	"syscall"
	"time"

	"dappco.re/go"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
)

// DaemonEntry records a running daemon in the registry.
//
// Example:
//
//	entry := process.DaemonEntry{Code: "app", Daemon: "serve", PID: core.Getpid()}
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
	homeResult := core.UserHomeDir()
	home := core.TempDir()
	if homeResult.OK {
		home = homeResult.Value.(string)
	}
	return NewRegistry(core.PathJoin(home, ".core", "daemons"))
}

// Register writes a daemon entry to the registry directory.
// If Started is zero, it is set to the current time.
// The directory is created if it does not exist.
//
// Example:
//
//	_ = reg.Register(entry)
func (r *Registry) Register(entry DaemonEntry) core.Result {
	if entry.Started.IsZero() {
		entry.Started = time.Now()
	}

	if err := coreio.Local.EnsureDir(r.dir); err != nil {
		return core.Fail(coreerr.E("Registry.Register", "failed to create registry directory", err))
	}

	jsonResult := core.JSONMarshal(entry)
	if !jsonResult.OK {
		err, _ := jsonResult.Value.(error)
		return core.Fail(coreerr.E("Registry.Register", "failed to marshal entry", err))
	}

	data := jsonResult.Value.([]byte)
	if err := coreio.Local.Write(r.entryPath(entry.Code, entry.Daemon), string(data)); err != nil {
		return core.Fail(coreerr.E("Registry.Register", "failed to write entry file", err))
	}
	return core.Ok(nil)
}

// Unregister removes a daemon entry from the registry.
//
// Example:
//
//	_ = reg.Unregister("app", "serve")
func (r *Registry) Unregister(code, daemon string) core.Result {
	path := r.entryPath(code, daemon)
	if result := core.Remove(path); !result.OK {
		err, _ := result.Value.(error)
		if core.IsNotExist(err) {
			return core.Ok(nil)
		}
		return core.Fail(coreerr.E("Registry.Unregister", "failed to delete entry file", err))
	}
	return core.Ok(nil)
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
	if result := core.JSONUnmarshalString(data, &entry); !result.OK {
		if err := coreio.Local.Delete(path); err != nil {
			return nil, false
		}
		return nil, false
	}

	if !isAlive(entry.PID) {
		if err := coreio.Local.Delete(path); err != nil {
			return nil, false
		}
		return nil, false
	}

	return &entry, true
}

// List returns all alive daemon entries, pruning any with dead PIDs.
//
// Example:
//
//	result := reg.List()
func (r *Registry) List() core.Result {
	matches := core.PathGlob(core.PathJoin(r.dir, "*.json"))

	var alive []DaemonEntry
	for _, path := range matches {
		data, err := coreio.Local.Read(path)
		if err != nil {
			continue
		}

		var entry DaemonEntry
		if result := core.JSONUnmarshalString(data, &entry); !result.OK {
			if err := coreio.Local.Delete(path); err != nil {
				continue
			}
			continue
		}

		if !isAlive(entry.PID) {
			if err := coreio.Local.Delete(path); err != nil {
				continue
			}
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

	return core.Ok(alive)
}

// entryPath returns the filesystem path for a daemon entry.
func (r *Registry) entryPath(code, daemon string) string {
	name := core.Concat(core.Replace(code, "/", "-"), "-", core.Replace(daemon, "/", "-"), ".json")
	return core.PathJoin(r.dir, name)
}

// isAlive checks whether a process with the given PID is running.
func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return processSignal(pid, syscall.Signal(0)).OK
}
