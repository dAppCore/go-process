package process

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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

func TestDaemon_StopMarksNotReadyBeforeShutdownCompletes(t *testing.T) {
	blockCheck := make(chan struct{})
	checkEntered := make(chan struct{})
	var once sync.Once

	d := NewDaemon(DaemonOptions{
		HealthAddr:      "127.0.0.1:0",
		ShutdownTimeout: 5 * time.Second,
		HealthChecks: []HealthCheck{
			func() error {
				once.Do(func() { close(checkEntered) })
				<-blockCheck
				return nil
			},
		},
	})

	err := d.Start()
	require.NoError(t, err)

	addr := d.HealthAddr()
	require.NotEmpty(t, addr)

	healthErr := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/health")
		if err != nil {
			healthErr <- err
			return
		}
		_ = resp.Body.Close()
		healthErr <- nil
	}()

	select {
	case <-checkEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("/health request did not enter the blocking check")
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- d.Stop()
	}()

	require.Eventually(t, func() bool {
		return !d.Ready()
	}, 500*time.Millisecond, 10*time.Millisecond, "daemon should become not ready before shutdown completes")

	select {
	case err := <-stopDone:
		t.Fatalf("daemon stopped too early: %v", err)
	default:
	}

	close(blockCheck)

	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("daemon stop did not finish after health check unblocked")
	}

	select {
	case err := <-healthErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("/health request did not finish")
	}
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
	assert.True(t, d.Ready())

	d.SetReady(false)
	assert.False(t, d.Ready())

	resp, _ = http.Get("http://" + addr + "/ready")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestDaemon_ReadyWithoutHealthServer(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assert.False(t, d.Ready())
}

func TestDaemon_NoHealthAddrReturnsEmpty(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assert.Empty(t, d.HealthAddr())
}

func TestDaemon_DefaultShutdownTimeout(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assert.Equal(t, 30*time.Second, d.opts.ShutdownTimeout)
}

func TestDaemon_RunBlocksUntilCancelled(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	err := d.Start()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()

	// Run should be blocking
	select {
	case <-done:
		t.Fatal("Run should block until context is cancelled")
	case <-time.After(50 * time.Millisecond):
		// Expected — still blocking
	}

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run should return after context cancellation")
	}
}

func TestDaemon_StopIdempotent(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	// Stop without Start should be a no-op
	err := d.Stop()
	assert.NoError(t, err)
}

func TestDaemon_AutoRegisters(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "daemons"))
	wd, err := os.Getwd()
	require.NoError(t, err)
	exe, err := os.Executable()
	require.NoError(t, err)

	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
		Registry:   reg,
		RegistryEntry: DaemonEntry{
			Code:   "test-app",
			Daemon: "serve",
		},
	})

	err = d.Start()
	require.NoError(t, err)

	// Should be registered
	entry, ok := reg.Get("test-app", "serve")
	require.True(t, ok)
	assert.Equal(t, os.Getpid(), entry.PID)
	assert.NotEmpty(t, entry.Health)
	assert.Equal(t, wd, entry.Project)
	assert.Equal(t, exe, entry.Binary)

	// Stop should unregister
	err = d.Stop()
	require.NoError(t, err)

	_, ok = reg.Get("test-app", "serve")
	assert.False(t, ok)
}

func TestDaemon_StartRollsBackOnRegistryFailure(t *testing.T) {
	dir := t.TempDir()

	pidPath := filepath.Join(dir, "daemon.pid")
	regDir := filepath.Join(dir, "registry")
	require.NoError(t, os.MkdirAll(regDir, 0o755))
	require.NoError(t, os.Chmod(regDir, 0o555))

	d := NewDaemon(DaemonOptions{
		PIDFile:    pidPath,
		HealthAddr: "127.0.0.1:0",
		Registry:   NewRegistry(regDir),
		RegistryEntry: DaemonEntry{
			Code:   "broken",
			Daemon: "start",
		},
	})

	err := d.Start()
	require.Error(t, err)

	_, statErr := os.Stat(pidPath)
	assert.True(t, os.IsNotExist(statErr))

	addr := d.HealthAddr()
	require.NotEmpty(t, addr)

	client := &http.Client{Timeout: 250 * time.Millisecond}
	resp, reqErr := client.Get("http://" + addr + "/health")
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.Error(t, reqErr)

	assert.NoError(t, d.Stop())
}
