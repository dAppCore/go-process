package process

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// DaemonEntry records a running daemon in the registry.
//
// Example:
//
//	entry := process.DaemonEntry{Code: "app", Daemon: "serve", PID: os.Getpid()}
type DaemonEntry struct {
	Code      string            `json:"code"`
	Daemon    string            `json:"daemon"`
	PID       int               `json:"pid"`
	Health    string            `json:"health,omitempty"`
	Project   string            `json:"project,omitempty"`
	Binary    string            `json:"binary,omitempty"`
	Started   time.Time         `json:"started,omitempty"`
	StartedAt time.Time         `json:"startedAt,omitempty"`
	Config    map[string]string `json:"config,omitempty"`
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
	return NewRegistry(core.Path(home, ".core", "daemons"))
}

// Register writes a daemon entry to the registry directory.
// If Started and StartedAt are zero, they are set to the current time.
// The directory is created if it does not exist.
//
// Example:
//
//	_ = reg.Register(entry)
func (r *Registry) Register(entry DaemonEntry) error {
	now := time.Now()

	if entry.StartedAt.IsZero() {
		entry.StartedAt = now
	}
	if entry.Started.IsZero() {
		entry.Started = entry.StartedAt
	}
	if entry.Config != nil {
		entry.Config = cloneDaemonConfig(entry.Config)
	}

	entryPath := r.entryPath(entry.Code, entry.Daemon)
	if err := coreio.Local.EnsureDir(filepath.Dir(entryPath)); err != nil {
		return coreerr.E("Registry.Register", "failed to create registry directory", err)
	}

	r1 := core.JSONMarshal(entry)
	if !r1.OK {
		return coreerr.E("Registry.Register", "failed to marshal entry", r1.Value.(error))
	}

	payload := string(r1.Value.([]byte))
	if err := coreio.Local.Write(entryPath, payload); err != nil {
		return coreerr.E("Registry.Register", "failed to write entry file", err)
	}

	legacyPath := r.legacyEntryPath(entry.Code, entry.Daemon)
	if legacyPath != entryPath {
		_ = coreio.Local.Write(legacyPath, payload)
	}

	return nil
}

// Unregister removes a daemon entry from the registry.
//
// Example:
//
//	_ = reg.Unregister("app", "serve")
func (r *Registry) Unregister(code, daemon string) error {
	paths := r.lookupPaths(code, daemon)
	for _, path := range paths {
		if err := coreio.Local.Delete(path); err != nil {
			if core.Is(err, os.ErrNotExist) {
				continue
			}
			return coreerr.E("Registry.Unregister", "failed to delete entry file", err)
		}
	}

	return nil
}

// Get reads a single daemon entry and checks whether its process is alive.
// If the process is dead, stale files are removed and (nil, false) is returned.
//
// Example:
//
//	entry, ok := reg.Get("app", "serve")
func (r *Registry) Get(code, daemon string) (*DaemonEntry, bool) {
	var candidate DaemonEntry
	candidateSet := false
	var candidatePath string
	var stalePaths []string

	for _, path := range r.lookupPaths(code, daemon) {
		data, err := coreio.Local.Read(path)
		if err != nil {
			continue
		}

		var entry DaemonEntry
		if r1 := core.JSONUnmarshalString(data, &entry); !r1.OK {
			stalePaths = append(stalePaths, path)
			continue
		}

		normalizeDaemonEntry(&entry)
		if !isAlive(entry.PID) {
			stalePaths = append(stalePaths, path)
			continue
		}

		if !candidateSet || entryStartedAt(entry).After(entryStartedAt(candidate)) {
			if candidatePath != "" {
				stalePaths = append(stalePaths, candidatePath)
			}
			candidate = entry
			candidateSet = true
			candidatePath = path
		}
	}

	for _, path := range stalePaths {
		_ = coreio.Local.Delete(path)
	}

	if !candidateSet {
		return nil, false
	}
	return &candidate, true
}

// List returns all alive daemon entries, pruning any with dead PIDs.
//
// Example:
//
//	entries, err := reg.List()
func (r *Registry) List() ([]DaemonEntry, error) {
	matches, err := r.findEntryPaths()
	if err != nil {
		return nil, coreerr.E("Registry.List", "failed to scan registry files", err)
	}

	byKey := make(map[string]DaemonEntry)
	for _, path := range matches {
		data, err := coreio.Local.Read(path)
		if err != nil {
			continue
		}

		var entry DaemonEntry
		if r1 := core.JSONUnmarshalString(data, &entry); !r1.OK {
			_ = coreio.Local.Delete(path)
			continue
		}

		normalizeDaemonEntry(&entry)
		if !isAlive(entry.PID) {
			_ = coreio.Local.Delete(path)
			continue
		}

		key := entry.Code + "\x00" + entry.Daemon
		existing, ok := byKey[key]
		if !ok || entryStartedAt(entry).After(entryStartedAt(existing)) {
			byKey[key] = entry
		}
	}

	alive := make([]DaemonEntry, 0, len(byKey))
	for _, entry := range byKey {
		alive = append(alive, entry)
	}

	slices.SortFunc(alive, func(a, b DaemonEntry) int {
		ta := entryStartedAt(a)
		tb := entryStartedAt(b)

		if ta.Equal(tb) {
			if a.Code == b.Code {
				if a.Daemon == b.Daemon {
					return 0
				}
				if a.Daemon < b.Daemon {
					return -1
				}
				return 1
			}
			if a.Code < b.Code {
				return -1
			}
			return 1
		}

		if ta.Before(tb) {
			return -1
		}
		return 1
	})

	return alive, nil
}

// findEntryPaths returns all JSON files below the registry directory recursively.
func (r *Registry) findEntryPaths() ([]string, error) {
	entries := make([]string, 0)
	_, err := os.Stat(r.dir)
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}

	if err := filepath.WalkDir(r.dir, func(path string, dirEntry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".json") {
			entries = append(entries, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return entries, nil
}

// lookupPaths returns both modern and legacy layout paths for the daemon key.
func (r *Registry) lookupPaths(code, daemon string) []string {
	paths := []string{r.entryPath(code, daemon)}
	legacy := r.legacyEntryPath(code, daemon)
	if legacy != paths[0] {
		paths = append(paths, legacy)
	}
	return paths
}

// entryPath returns the filesystem path for a daemon entry.
// RFC layout: {dir}/{code}/{daemon}.json.
func (r *Registry) entryPath(code, daemon string) string {
	return core.Path(r.dir, sanitizeRegistrySegment(code), core.Concat(sanitizeRegistrySegment(daemon), ".json"))
}

// legacyEntryPath returns the old flat layout for compatibility.
func (r *Registry) legacyEntryPath(code, daemon string) string {
	name := core.Concat(core.Replace(code, "/", "-"), "-", core.Replace(daemon, "/", "-"), ".json")
	name = strings.ReplaceAll(name, `\`, "-")
	return core.Path(r.dir, name)
}

func entryStartedAt(entry DaemonEntry) time.Time {
	if !entry.StartedAt.IsZero() {
		return entry.StartedAt
	}
	return entry.Started
}

func normalizeDaemonEntry(entry *DaemonEntry) {
	if entry == nil {
		return
	}

	if entry.StartedAt.IsZero() {
		entry.StartedAt = entry.Started
	}
	if entry.Started.IsZero() {
		entry.Started = entry.StartedAt
	}

	entry.Config = cloneDaemonConfig(entry.Config)
}

func cloneDaemonConfig(cfg map[string]string) map[string]string {
	if len(cfg) == 0 {
		return nil
	}
	out := make(map[string]string, len(cfg))
	for key, value := range cfg {
		out[key] = value
	}
	return out
}

func sanitizeRegistrySegment(value string) string {
	sanitized := core.Trim(value)
	if sanitized == "" {
		sanitized = "default"
	}
	return strings.ReplaceAll(sanitized, "..", "-")
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
