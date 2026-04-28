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
	ready := d.Ready()
	assertFalse(t, ready)
	assertEqual(t, "", d.HealthAddr())
}

func TestDaemon_NoHealthAddrReturnsEmpty(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	addr := d.HealthAddr()
	assertEqual(t, "", addr)
	assertFalse(t, d.Ready())
}

func TestDaemon_DefaultShutdownTimeout(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	timeout := d.opts.ShutdownTimeout
	assertEqual(t, 30*time.Second, timeout)
	assertFalse(t, d.running)
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

func TestDaemon_NewDaemon_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	assertNotNil(t, d)
	assertNotNil(t, d.health)
	assertEqual(t, 30*time.Second, d.opts.ShutdownTimeout)
}

func TestDaemon_NewDaemon_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{ShutdownTimeout: time.Second})
	assertNotNil(t, d)
	assertNil(t, d.health)
	assertEqual(t, time.Second, d.opts.ShutdownTimeout)
}

func TestDaemon_NewDaemon_Ugly(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "daemon.pid")
	d := NewDaemon(DaemonOptions{PIDFile: pidPath})
	assertNotNil(t, d)
	assertNotNil(t, d.pid)
	assertEqual(t, pidPath, d.pid.Path())
}

func TestDaemon_Daemon_Start_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	err := d.Start()
	requireNoError(t, err)
	defer func() { requireNoError(t, d.Stop()) }()
	assertNotEmpty(t, d.HealthAddr())
}

func TestDaemon_Daemon_Start_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	defer func() { requireNoError(t, d.Stop()) }()
	err := d.Start()
	assertError(t, err)
}

func TestDaemon_Daemon_Start_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "256.256.256.256:1"})
	err := d.Start()
	assertError(t, err)
	assertFalse(t, d.running)
}

func TestDaemon_Daemon_Run_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.Run(ctx)
	requireNoError(t, err)
}

func TestDaemon_Daemon_Run_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	err := d.Run(nil)
	assertError(t, err)
	assertErrorIs(t, err, ErrDaemonContextRequired)
}

func TestDaemon_Daemon_Run_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := d.Run(ctx)
	assertError(t, err)
}

func TestDaemon_Daemon_Stop_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	err := d.Stop()
	requireNoError(t, err)
	assertFalse(t, d.running)
}

func TestDaemon_Daemon_Stop_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	err := d.Stop()
	requireNoError(t, err)
	assertFalse(t, d.running)
}

func TestDaemon_Daemon_Stop_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	requireNoError(t, d.Stop())
	err := d.Stop()
	requireNoError(t, err)
}

func TestDaemon_Daemon_SetReady_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	d.SetReady(false)
	assertFalse(t, d.Ready())
	d.SetReady(true)
	assertTrue(t, d.Ready())
}

func TestDaemon_Daemon_SetReady_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	d.SetReady(false)
	assertFalse(t, d.Ready())
	assertEqual(t, "", d.HealthAddr())
}

func TestDaemon_Daemon_SetReady_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	d.SetReady(false)
	d.SetReady(false)
	assertFalse(t, d.Ready())
}

func TestDaemon_Daemon_Ready_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	got := d.Ready()
	assertTrue(t, got)
	assertNotNil(t, d.health)
}

func TestDaemon_Daemon_Ready_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	got := d.Ready()
	assertFalse(t, got)
	assertNil(t, d.health)
}

func TestDaemon_Daemon_Ready_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	d.SetReady(false)
	got := d.Ready()
	assertFalse(t, got)
}

func TestDaemon_Daemon_HealthAddr_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	defer func() { requireNoError(t, d.Stop()) }()
	got := d.HealthAddr()
	assertNotEmpty(t, got)
}

func TestDaemon_Daemon_HealthAddr_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	got := d.HealthAddr()
	assertEqual(t, "", got)
	assertFalse(t, d.Ready())
}

func TestDaemon_Daemon_HealthAddr_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	got := d.HealthAddr()
	assertEqual(t, "127.0.0.1:0", got)
	assertTrue(t, d.Ready())
}
