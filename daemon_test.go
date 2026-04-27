package process

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"testing"
	"time"
)

func TestDaemon_StartAndStop(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "test.pid")

	d := NewDaemon(DaemonOptions{
		PIDFile:         pidPath,
		HealthAddr:      "127.0.0.1:0",
		ShutdownTimeout: 5 * time.Second,
	})

	err := d.Start()
	requireNoError(t, err)

	addr := d.HealthAddr()
	requireNotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/health")
	requireNoError(t, err)
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	err = d.Stop()
	requireNoError(t, err)
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
	requireNoError(t, err)

	addr := d.HealthAddr()
	requireNotEmpty(t, addr)

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

	requireEventually(t, func() bool {
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
		requireNoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("daemon stop did not finish after health check unblocked")
	}

	select {
	case err := <-healthErr:
		requireNoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("/health request did not finish")
	}
}

func TestDaemon_StopUnregistersBeforeHealthShutdownCompletes(t *testing.T) {
	blockCheck := make(chan struct{})
	checkEntered := make(chan struct{})
	var once sync.Once
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "registry"))

	d := NewDaemon(DaemonOptions{
		HealthAddr:      "127.0.0.1:0",
		ShutdownTimeout: 5 * time.Second,
		Registry:        reg,
		RegistryEntry: DaemonEntry{
			Code:   "test-app",
			Daemon: "serve",
		},
		HealthChecks: []HealthCheck{
			func() error {
				once.Do(func() { close(checkEntered) })
				<-blockCheck
				return nil
			},
		},
	})

	err := d.Start()
	requireNoError(t, err)

	addr := d.HealthAddr()
	requireNotEmpty(t, addr)

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

	requireEventually(t, func() bool {
		return !d.Ready()
	}, 500*time.Millisecond, 10*time.Millisecond, "daemon should become not ready before shutdown completes")

	_, ok := reg.Get("test-app", "serve")
	assertFalse(t, ok, "daemon should unregister before health shutdown completes")

	select {
	case err := <-stopDone:
		t.Fatalf("daemon stopped too early: %v", err)
	default:
	}

	close(blockCheck)

	select {
	case err := <-stopDone:
		requireNoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("daemon stop did not finish after health check unblocked")
	}

	requireEventually(t, func() bool {
		_, ok := reg.Get("test-app", "serve")
		return !ok
	}, 500*time.Millisecond, 10*time.Millisecond, "daemon should remain unregistered after health shutdown completes")

	select {
	case err := <-healthErr:
		requireNoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("/health request did not finish")
	}
}

func TestDaemon_DoubleStartFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	err := d.Start()
	requireNoError(t, err)
	defer func() { _ = d.Stop() }()

	err = d.Start()
	assertError(t, err)
	assertContains(t, err.Error(), "already running")
}

func TestDaemon_RunWithoutStartFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.Run(ctx)
	assertError(t, err)
	assertContains(t, err.Error(), "not started")
}

func TestDaemon_RunNilContextFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	err := d.Run(nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrDaemonContextRequired)
}

func TestDaemon_SetReady(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	err := d.Start()
	requireNoError(t, err)
	defer func() { _ = d.Stop() }()

	addr := d.HealthAddr()

	resp, _ := http.Get("http://" + addr + "/ready")
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	assertTrue(t, d.Ready())

	d.SetReady(false)
	assertFalse(t, d.Ready())

	resp, _ = http.Get("http://" + addr + "/ready")
	assertEqual(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestDaemon_ReadyWithoutHealthServer(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assertFalse(t, d.Ready())
}

func TestDaemon_NoHealthAddrReturnsEmpty(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assertEqual(t, "", d.HealthAddr())
}

func TestDaemon_DefaultShutdownTimeout(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	assertEqual(t, 30*time.Second, d.opts.ShutdownTimeout)
}

func TestDaemon_RunBlocksUntilCancelled(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	err := d.Start()
	requireNoError(t, err)

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
		assertNoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run should return after context cancellation")
	}
}

func TestDaemon_StopIdempotent(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	// Stop without Start should be a no-op
	err := d.Stop()
	assertNoError(t, err)
}

func TestDaemon_AutoRegisters(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "daemons"))
	wd, err := os.Getwd()
	requireNoError(t, err)
	exe, err := os.Executable()
	requireNoError(t, err)

	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
		Registry:   reg,
		RegistryEntry: DaemonEntry{
			Code:   "test-app",
			Daemon: "serve",
		},
	})

	err = d.Start()
	requireNoError(t, err)

	// Should be registered
	entry, ok := reg.Get("test-app", "serve")
	requireTrue(t, ok)
	assertEqual(t, os.Getpid(), entry.PID)
	assertNotEmpty(t, entry.Health)
	assertEqual(t, wd, entry.Project)
	assertEqual(t, exe, entry.Binary)

	// Stop should unregister
	err = d.Stop()
	requireNoError(t, err)

	_, ok = reg.Get("test-app", "serve")
	assertFalse(t, ok)
}

func TestDaemon_StartRollsBackOnRegistryFailure(t *testing.T) {
	dir := t.TempDir()

	pidPath := filepath.Join(dir, "daemon.pid")
	regDir := filepath.Join(dir, "registry")
	requireNoError(t, os.MkdirAll(regDir, 0o755))
	requireNoError(t, os.Chmod(regDir, 0o555))

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
	requireError(t, err)

	_, statErr := os.Stat(pidPath)
	assertTrue(t, os.IsNotExist(statErr))

	addr := d.HealthAddr()
	requireNotEmpty(t, addr)

	client := &http.Client{Timeout: 250 * time.Millisecond}
	resp, reqErr := client.Get("http://" + addr + "/health")
	if resp != nil {
		_ = resp.Body.Close()
	}
	assertError(t, reqErr)

	assertNoError(t, d.Stop())
}
