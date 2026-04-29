package process

import (
	"context"
	"net/http"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"syscall"
	"testing"
	"time"

	core "dappco.re/go"
)

func TestDaemon_StartAndStop(t *testing.T) {
	pidPath := core.PathJoin(t.TempDir(), "test.pid")

	d := NewDaemon(DaemonOptions{
		PIDFile:         pidPath,
		HealthAddr:      "127.0.0.1:0",
		ShutdownTimeout: 5 * time.Second,
	})

	requireNoError(t, d.Start())

	addr := d.HealthAddr()
	requireNotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/health")
	requireNoError(t, err)
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	requireNoError(t, d.Stop())
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

	requireNoError(t, d.Start())

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
	reg := NewRegistry(core.PathJoin(dir, "registry"))

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

	requireNoError(t, d.Start())

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

	stopDone := make(chan core.Result, 1)
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

	requireNoError(t, d.Start())
	defer func() { _ = d.Stop() }()

	result := d.Start()
	assertError(t, result)
	assertContains(t, result.Error(), "already running")
}

func TestDaemon_RunWithoutStartFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := d.Run(ctx)
	assertError(t, result)
	assertContains(t, result.Error(), "not started")
}

func TestDaemon_RunNilContextFails(t *testing.T) {
	d := NewDaemon(DaemonOptions{})

	result := d.Run(nil)
	requireError(t, result)
	assertErrorIs(t, result, ErrDaemonContextRequired)
}

func TestDaemon_SetReady(t *testing.T) {
	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
	})

	requireNoError(t, d.Start())
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

	requireNoError(t, d.Start())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan core.Result, 1)
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
	assertNoError(t, d.Stop())
	assertFalse(t, d.Ready())
}

func TestDaemon_AutoRegisters(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(core.PathJoin(dir, "daemons"))
	wd := core.Getwd()
	requireTrue(t, wd.OK)
	args := core.Args()
	requireNotEmpty(t, args)

	d := NewDaemon(DaemonOptions{
		HealthAddr: "127.0.0.1:0",
		Registry:   reg,
		RegistryEntry: DaemonEntry{
			Code:   "test-app",
			Daemon: "serve",
		},
	})

	requireNoError(t, d.Start())

	// Should be registered
	entry, ok := reg.Get("test-app", "serve")
	requireTrue(t, ok)
	assertEqual(t, core.Getpid(), entry.PID)
	assertNotEmpty(t, entry.Health)
	assertEqual(t, wd.Value.(string), entry.Project)
	assertEqual(t, args[0], entry.Binary)

	// Stop should unregister
	requireNoError(t, d.Stop())

	_, ok = reg.Get("test-app", "serve")
	assertFalse(t, ok)
}

func TestDaemon_StartRollsBackOnRegistryFailure(t *testing.T) {
	dir := t.TempDir()

	pidPath := core.PathJoin(dir, "daemon.pid")
	regDir := core.PathJoin(dir, "registry")
	requireTrue(t, core.MkdirAll(regDir, 0o755).OK)
	requireNoError(t, syscall.Chmod(regDir, 0o555))

	d := NewDaemon(DaemonOptions{
		PIDFile:    pidPath,
		HealthAddr: "127.0.0.1:0",
		Registry:   NewRegistry(regDir),
		RegistryEntry: DaemonEntry{
			Code:   "broken",
			Daemon: "start",
		},
	})

	result := d.Start()
	requireError(t, result)

	stat := core.Stat(pidPath)
	assertFalse(t, stat.OK)
	assertTrue(t, core.IsNotExist(stat.Value.(error)))

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
	pidPath := core.PathJoin(t.TempDir(), "daemon.pid")
	d := NewDaemon(DaemonOptions{PIDFile: pidPath})
	assertNotNil(t, d)
	assertNotNil(t, d.pid)
	assertEqual(t, pidPath, d.pid.Path())
}

func TestDaemon_Daemon_Start_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	defer func() { requireNoError(t, d.Stop()) }()
	assertNotEmpty(t, d.HealthAddr())
}

func TestDaemon_Daemon_Start_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	defer func() { requireNoError(t, d.Stop()) }()
	result := d.Start()
	assertError(t, result)
}

func TestDaemon_Daemon_Start_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "256.256.256.256:1"})
	result := d.Start()
	assertError(t, result)
	assertFalse(t, d.running)
}

func TestDaemon_Daemon_Run_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requireNoError(t, d.Run(ctx))
}

func TestDaemon_Daemon_Run_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	result := d.Run(nil)
	assertError(t, result)
	assertErrorIs(t, result, ErrDaemonContextRequired)
}

func TestDaemon_Daemon_Run_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := d.Run(ctx)
	assertError(t, result)
}

func TestDaemon_Daemon_Stop_Good(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	requireNoError(t, d.Stop())
	assertFalse(t, d.running)
}

func TestDaemon_Daemon_Stop_Bad(t *testing.T) {
	d := NewDaemon(DaemonOptions{})
	requireNoError(t, d.Stop())
	assertFalse(t, d.running)
}

func TestDaemon_Daemon_Stop_Ugly(t *testing.T) {
	d := NewDaemon(DaemonOptions{HealthAddr: "127.0.0.1:0"})
	requireNoError(t, d.Start())
	requireNoError(t, d.Stop())
	requireNoError(t, d.Stop())
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
