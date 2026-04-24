package process

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	started := time.Now().UTC().Truncate(time.Second)
	entry := DaemonEntry{
		Code:    "myapp",
		Daemon:  "worker",
		PID:     os.Getpid(),
		Health:  "healthy",
		Project: "test-project",
		Binary:  "/usr/bin/worker",
		Started: started,
	}

	err := reg.Register(entry)
	requireNoError(t, err)

	got, ok := reg.Get("myapp", "worker")
	requireTrue(t, ok)
	assertEqual(t, "myapp", got.Code)
	assertEqual(t, "worker", got.Daemon)
	assertEqual(t, os.Getpid(), got.PID)
	assertEqual(t, "healthy", got.Health)
	assertEqual(t, "test-project", got.Project)
	assertEqual(t, "/usr/bin/worker", got.Binary)
	assertEqual(t, started, got.Started)
}

func TestRegistry_Unregister(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	entry := DaemonEntry{
		Code:   "myapp",
		Daemon: "server",
		PID:    os.Getpid(),
	}

	err := reg.Register(entry)
	requireNoError(t, err)

	// File should exist
	path := filepath.Join(dir, "myapp-server.json")
	_, err = os.Stat(path)
	requireNoError(t, err)

	err = reg.Unregister("myapp", "server")
	requireNoError(t, err)

	// File should be gone
	_, err = os.Stat(path)
	assertTrue(t, os.IsNotExist(err))
}

func TestRegistry_UnregisterMissingIsNoop(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	err := reg.Unregister("missing", "entry")
	requireNoError(t, err)
}

func TestRegistry_List(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	err := reg.Register(DaemonEntry{Code: "app1", Daemon: "web", PID: os.Getpid()})
	requireNoError(t, err)
	err = reg.Register(DaemonEntry{Code: "app2", Daemon: "api", PID: os.Getpid()})
	requireNoError(t, err)

	entries, err := reg.List()
	requireNoError(t, err)
	requireLen(t, entries, 2)
	assertEqual(t, "app1", entries[0].Code)
	assertEqual(t, "app2", entries[1].Code)
}

func TestRegistry_List_PrunesStale(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	err := reg.Register(DaemonEntry{Code: "dead", Daemon: "proc", PID: 999999999})
	requireNoError(t, err)

	// File should exist before listing
	path := filepath.Join(dir, "dead-proc.json")
	_, err = os.Stat(path)
	requireNoError(t, err)

	entries, err := reg.List()
	requireNoError(t, err)
	assertEmpty(t, entries)

	// Stale file should be removed
	_, err = os.Stat(path)
	assertTrue(t, os.IsNotExist(err))
}

func TestRegistry_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	got, ok := reg.Get("nope", "missing")
	assertNil(t, got)
	assertFalse(t, ok)
}

func TestRegistry_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep", "daemons")
	reg := NewRegistry(dir)

	err := reg.Register(DaemonEntry{Code: "app", Daemon: "srv", PID: os.Getpid()})
	requireNoError(t, err)

	info, err := os.Stat(dir)
	requireNoError(t, err)
	assertTrue(t, info.IsDir())
}

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	assertNotNil(t, reg)
}
