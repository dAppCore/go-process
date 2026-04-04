package process

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	got, ok := reg.Get("myapp", "worker")
	require.True(t, ok)
	assert.Equal(t, "myapp", got.Code)
	assert.Equal(t, "worker", got.Daemon)
	assert.Equal(t, os.Getpid(), got.PID)
	assert.Equal(t, "healthy", got.Health)
	assert.Equal(t, "test-project", got.Project)
	assert.Equal(t, "/usr/bin/worker", got.Binary)
	assert.Equal(t, started, got.Started)
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
	require.NoError(t, err)

	// File should exist
	path := filepath.Join(dir, "myapp-server.json")
	_, err = os.Stat(path)
	require.NoError(t, err)

	err = reg.Unregister("myapp", "server")
	require.NoError(t, err)

	// File should be gone
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestRegistry_List(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	err := reg.Register(DaemonEntry{Code: "app1", Daemon: "web", PID: os.Getpid()})
	require.NoError(t, err)
	err = reg.Register(DaemonEntry{Code: "app2", Daemon: "api", PID: os.Getpid()})
	require.NoError(t, err)

	entries, err := reg.List()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "app1", entries[0].Code)
	assert.Equal(t, "app2", entries[1].Code)
}

func TestRegistry_List_PrunesStale(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	err := reg.Register(DaemonEntry{Code: "dead", Daemon: "proc", PID: 999999999})
	require.NoError(t, err)

	// File should exist before listing
	path := filepath.Join(dir, "dead-proc.json")
	_, err = os.Stat(path)
	require.NoError(t, err)

	entries, err := reg.List()
	require.NoError(t, err)
	assert.Empty(t, entries)

	// Stale file should be removed
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestRegistry_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	got, ok := reg.Get("nope", "missing")
	assert.Nil(t, got)
	assert.False(t, ok)
}

func TestRegistry_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep", "daemons")
	reg := NewRegistry(dir)

	err := reg.Register(DaemonEntry{Code: "app", Daemon: "srv", PID: os.Getpid()})
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	assert.NotNil(t, reg)
}
