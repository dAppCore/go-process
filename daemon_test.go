package process

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_StartAndStop(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "test.pid")

	d := NewDaemon(DaemonOptions{
		PIDFile:         pidPath,
		HealthAddr:      "127.0.0.1:0",
		ShutdownTimeout: 5 * time.Second,
	})

	err := d.Start()
	require.NoError(t, err)

	addr := d.HealthAddr()
	require.NotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	err = d.Stop()
	require.NoError(t, err)
}

func TestDaemon_DoubleStartFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	err := d.Start()
	require.NoError(t, err)
	defer func() { _ = d.Stop() }()

	err = d.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestDaemon_RunWithoutStartFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.Run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestDaemon_SetReady(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	err := d.Start()
	require.NoError(t, err)
	defer func() { _ = d.Stop() }()

	addr := d.HealthAddr()

	resp, _ := http.Get("http://" + addr + "/ready")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	d.SetReady(false)

	resp, _ = http.Get("http://" + addr + "/ready")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestDaemon_NoHealthAddrReturnsEmpty(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assert.Empty(t, d.HealthAddr())
}

func TestDaemon_DefaultShutdownTimeout(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assert.Equal(t, 30*time.Second, d.opts.ShutdownTimeout)
}

func TestDaemon_AutoRegisters(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "daemons"))

	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
		Registry:   reg,
		RegistryEntry: DaemonEntry{
			Code:   "test-app",
			Daemon: "serve",
		},
	})

	err := d.Start()
	require.NoError(t, err)

	// Should be registered
	entry, ok := reg.Get("test-app", "serve")
	require.True(t, ok)
	assert.Equal(t, os.Getpid(), entry.PID)
	assert.NotEmpty(t, entry.Health)

	// Stop should unregister
	err = d.Stop()
	require.NoError(t, err)

	_, ok = reg.Get("test-app", "serve")
	assert.False(t, ok)
}
