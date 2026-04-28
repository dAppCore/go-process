package process

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"syscall"
	"testing"
	"time"

	framework "dappco.re/go"
)

func newTestService(t *testing.T) (*Service, *framework.Core) {
	t.Helper()

	c := framework.New()
	factory := NewService(Options{BufferSize: 1024})
	raw, err := factory(c)
	requireNoError(t, err)

	svc := raw.(*Service)
	return svc, c
}

// resultErr converts a core.Result returned from Startable/Stoppable hooks
// into the (error) shape the test suite was originally written against.
func resultErr(r framework.Result) error {
	if r.OK {
		return nil
	}
	if err, ok := r.Value.(error); ok {
		return err
	}
	return nil
}

// performTask invokes the package's handleTask switch directly, mirroring
// the legacy c.PERFORM(TaskProcess*) request/response pattern that named
// Actions do not support via broadcast. The legacy Perform contract returned
// the first OK result and an empty Result{} on failure, so we drop the error
// payload when the handler reports !OK to match those test expectations.
func performTask(svc *Service, c *framework.Core, msg framework.Message) framework.Result {
	r := svc.handleTask(c, msg)
	if !r.OK {
		return framework.Result{}
	}
	return r
}

func TestService_Start(t *testing.T) {
	t.Run("echo command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "hello")
		requireNoError(t, err)
		requireNotNil(t, proc)

		assertNotEmpty(t, proc.ID)
		assertEqual(t, "echo", proc.Command)
		assertEqual(t, []string{"hello"}, proc.Args)

		// Wait for completion
		<-proc.Done()

		assertEqual(t, StatusExited, proc.Status)
		assertEqual(t, 0, proc.ExitCode)
		assertContains(t, proc.Output(), "hello")
	})

	t.Run("works without core runtime", func(t *testing.T) {
		svc := &Service{
			processes: make(map[string]*Process),
			bufSize:   1024,
		}

		proc, err := svc.Start(context.Background(), "echo", "standalone")
		requireNoError(t, err)
		requireNotNil(t, proc)

		<-proc.Done()

		assertEqual(t, StatusExited, proc.Status)
		assertEqual(t, 0, proc.ExitCode)
		assertContains(t, proc.Output(), "standalone")
	})

	t.Run("failing command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 42")
		requireNoError(t, err)

		<-proc.Done()

		assertEqual(t, StatusExited, proc.Status)
		assertEqual(t, 42, proc.ExitCode)
	})

	t.Run("non-existent command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "nonexistent_command_xyz")
		assertError(t, err)
		requireNotNil(t, proc)
		assertEqual(t, StatusFailed, proc.Status)
		assertEqual(t, -1, proc.ExitCode)
		assertNotNil(t, proc.Done())
		<-proc.Done()

		got, getErr := svc.Get(proc.ID)
		requireNoError(t, getErr)
		assertEqual(t, proc.ID, got.ID)
		assertEqual(t, StatusFailed, got.Status)
	})

	t.Run("empty command is rejected", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.StartWithOptions(context.Background(), RunOptions{})
		requireError(t, err)
		assertContains(t, err.Error(), "command is required")
	})

	t.Run("nil context is rejected", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.StartWithOptions(nil, RunOptions{
			Command: "echo",
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrContextRequired)
	})

	t.Run("with working directory", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "pwd",
			Dir:     "/tmp",
		})
		requireNoError(t, err)

		<-proc.Done()

		// On macOS /tmp is a symlink to /private/tmp
		output := strings.TrimSpace(proc.Output())
		assertTrue(t, output == "/tmp" || output == "/private/tmp", "got: "+output)
	})

	t.Run("context cancellation", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		proc, err := svc.Start(ctx, "sleep", "10")
		requireNoError(t, err)

		// Cancel immediately
		cancel()

		select {
		case <-proc.Done():
			// Good - process was killed
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}
	})

	t.Run("disable capture", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:        "echo",
			Args:           []string{"no-capture"},
			DisableCapture: true,
		})
		requireNoError(t, err)
		<-proc.Done()

		assertEqual(t, StatusExited, proc.Status)
		assertEqual(t, "", proc.Output())
	})

	t.Run("with environment variables", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sh",
			Args:    []string{"-c", "echo $MY_TEST_VAR"},
			Env:     []string{"MY_TEST_VAR=hello_env"},
		})
		requireNoError(t, err)
		<-proc.Done()

		assertContains(t, proc.Output(), "hello_env")
	})

	t.Run("detach survives parent context", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())

		proc, err := svc.StartWithOptions(ctx, RunOptions{
			Command: "sh",
			Args:    []string{"-c", "sleep 0.2; echo detached"},
			Detach:  true,
		})
		requireNoError(t, err)

		// Cancel the parent context
		cancel()

		select {
		case <-proc.Done():
			t.Fatal("detached process should survive parent cancellation")
		case <-time.After(50 * time.Millisecond):
		}
		assertTrue(t, proc.IsRunning(), "detached process should remain running after parent cancellation")

		// Detached process should still complete normally
		select {
		case <-proc.Done():
			assertEqual(t, StatusExited, proc.Status)
			assertEqual(t, 0, proc.ExitCode)
			assertContains(t, proc.Output(), "detached")
		case <-time.After(2 * time.Second):
			t.Fatal("detached process should have completed")
		}
	})

	t.Run("kill group requires detach", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sleep",
			Args:      []string{"1"},
			KillGroup: true,
		})
		requireError(t, err)
		assertContains(t, err.Error(), "KillGroup requires Detach")
	})
}

func TestService_Run(t *testing.T) {
	t.Run("returns output", func(t *testing.T) {
		svc, _ := newTestService(t)

		output, err := svc.Run(context.Background(), "echo", "hello world")
		requireNoError(t, err)
		assertContains(t, output, "hello world")
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Run(context.Background(), "sh", "-c", "exit 1")
		assertError(t, err)
		assertContains(t, err.Error(), "exited with code 1")
	})
}

func TestService_Actions(t *testing.T) {
	t.Run("broadcasts events", func(t *testing.T) {
		c := framework.New()

		// Register process service on Core
		factory := NewService(Options{})
		raw, err := factory(c)
		requireNoError(t, err)
		svc := raw.(*Service)

		var started []ActionProcessStarted
		var outputs []ActionProcessOutput
		var exited []ActionProcessExited
		var mu sync.Mutex

		c.RegisterAction(func(cc *framework.Core, msg framework.Message) framework.Result {
			mu.Lock()
			defer mu.Unlock()
			switch m := msg.(type) {
			case ActionProcessStarted:
				started = append(started, m)
			case ActionProcessOutput:
				outputs = append(outputs, m)
			case ActionProcessExited:
				exited = append(exited, m)
			}
			return framework.Result{OK: true}
		})
		proc, err := svc.Start(context.Background(), "echo", "test")
		requireNoError(t, err)

		<-proc.Done()

		// Give time for events to propagate
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assertLen(t, started, 1)
		assertEqual(t, "echo", started[0].Command)
		assertEqual(t, []string{"test"}, started[0].Args)

		assertNotEmpty(t, outputs)
		foundTest := false
		for _, o := range outputs {
			if strings.Contains(o.Line, "test") {
				foundTest = true
				break
			}
		}
		assertTrue(t, foundTest, "should have output containing 'test'")

		assertLen(t, exited, 1)
		assertEqual(t, 0, exited[0].ExitCode)
		assertNil(t, exited[0].Error)
	})

	t.Run("broadcasts killed events", func(t *testing.T) {
		c := framework.New()

		factory := NewService(Options{})
		raw, err := factory(c)
		requireNoError(t, err)
		svc := raw.(*Service)

		var killed []ActionProcessKilled
		var exited []ActionProcessExited
		var mu sync.Mutex

		c.RegisterAction(func(cc *framework.Core, msg framework.Message) framework.Result {
			mu.Lock()
			defer mu.Unlock()
			if m, ok := msg.(ActionProcessKilled); ok {
				killed = append(killed, m)
			}
			if m, ok := msg.(ActionProcessExited); ok {
				exited = append(exited, m)
			}
			return framework.Result{OK: true}
		})

		proc, err := svc.Start(context.Background(), "sleep", "60")
		requireNoError(t, err)

		err = svc.Kill(proc.ID)
		requireNoError(t, err)

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		requireLen(t, killed, 1)
		assertEqual(t, proc.ID, killed[0].ID)
		assertNotEmpty(t, killed[0].Signal)
		mu.Unlock()

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		assertLen(t, exited, 1)
		assertEqual(t, proc.ID, exited[0].ID)
		requireError(t, exited[0].Error)
		assertContains(t, exited[0].Error.Error(), "process was killed")
		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("broadcasts exited event on start failure", func(t *testing.T) {
		c := framework.New()

		factory := NewService(Options{})
		raw, err := factory(c)
		requireNoError(t, err)
		svc := raw.(*Service)

		var exited []ActionProcessExited
		var mu sync.Mutex

		c.RegisterAction(func(cc *framework.Core, msg framework.Message) framework.Result {
			mu.Lock()
			defer mu.Unlock()
			if m, ok := msg.(ActionProcessExited); ok {
				exited = append(exited, m)
			}
			return framework.Result{OK: true}
		})

		_, err = svc.Start(context.Background(), "definitely-not-a-real-binary-xyz")
		requireError(t, err)

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		requireLen(t, exited, 1)
		assertEqual(t, -1, exited[0].ExitCode)
		requireError(t, exited[0].Error)
		assertContains(t, exited[0].Error.Error(), "failed to start process")
	})

	t.Run("broadcasts exited error on non-zero exit", func(t *testing.T) {
		c := framework.New()

		factory := NewService(Options{})
		raw, err := factory(c)
		requireNoError(t, err)
		svc := raw.(*Service)

		var exited []ActionProcessExited
		var mu sync.Mutex

		c.RegisterAction(func(cc *framework.Core, msg framework.Message) framework.Result {
			mu.Lock()
			defer mu.Unlock()
			if m, ok := msg.(ActionProcessExited); ok {
				exited = append(exited, m)
			}
			return framework.Result{OK: true}
		})

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 7")
		requireNoError(t, err)

		<-proc.Done()
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		requireLen(t, exited, 1)
		assertEqual(t, 7, exited[0].ExitCode)
		requireError(t, exited[0].Error)
		assertContains(t, exited[0].Error.Error(), "process exited with code 7")
	})
}

func TestService_List(t *testing.T) {
	t.Run("tracks processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc1, _ := svc.Start(context.Background(), "echo", "1")
		proc2, _ := svc.Start(context.Background(), "echo", "2")

		<-proc1.Done()
		<-proc2.Done()

		list := svc.List()
		assertLen(t, list, 2)
		assertEqual(t, proc1.ID, list[0].ID)
		assertEqual(t, proc2.ID, list[1].ID)
	})

	t.Run("get by id", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, _ := svc.Start(context.Background(), "echo", "test")
		<-proc.Done()

		got, err := svc.Get(proc.ID)
		requireNoError(t, err)
		assertEqual(t, proc.ID, got.ID)
	})

	t.Run("get not found", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Get("nonexistent")
		assertErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_Remove(t *testing.T) {
	t.Run("removes completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, _ := svc.Start(context.Background(), "echo", "test")
		<-proc.Done()

		err := svc.Remove(proc.ID)
		requireNoError(t, err)

		_, err = svc.Get(proc.ID)
		assertErrorIs(t, err, ErrProcessNotFound)
	})

	t.Run("cannot remove running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, _ := svc.Start(ctx, "sleep", "10")

		err := svc.Remove(proc.ID)
		assertError(t, err)

		cancel()
		<-proc.Done()
	})
}

func TestService_Clear(t *testing.T) {
	t.Run("clears completed processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc1, _ := svc.Start(context.Background(), "echo", "1")
		proc2, _ := svc.Start(context.Background(), "echo", "2")

		<-proc1.Done()
		<-proc2.Done()

		assertLen(t, svc.List(), 2)

		svc.Clear()

		assertLen(t, svc.List(), 0)
	})
}

func TestService_Kill(t *testing.T) {
	t.Run("kills running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		err = svc.Kill(proc.ID)
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Process killed successfully
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		err := svc.Kill("nonexistent")
		assertErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_KillPID(t *testing.T) {
	t.Run("terminates unmanaged process with SIGKILL", func(t *testing.T) {
		svc, _ := newTestService(t)

		// Ignore SIGTERM so the test proves KillPID uses a forceful signal.
		cmd := exec.Command("sh", "-c", "trap '' TERM; while :; do :; done")
		requireNoError(t, cmd.Start())

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		t.Cleanup(func() {
			if cmd.ProcessState == nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			select {
			case <-waitCh:
			case <-time.After(2 * time.Second):
			}
		})

		err := svc.KillPID(cmd.Process.Pid)
		requireNoError(t, err)

		select {
		case err := <-waitCh:
			requireError(t, err)
			var exitErr *exec.ExitError
			requireErrorAs(t, err, &exitErr)
			ws, ok := exitErr.Sys().(syscall.WaitStatus)
			requireTrue(t, ok)
			assertTrue(t, ws.Signaled())
			assertEqual(t, syscall.SIGKILL, ws.Signal())
		case <-time.After(2 * time.Second):
			t.Fatal("unmanaged process should have been killed")
		}
	})
}

func TestService_Signal(t *testing.T) {
	t.Run("rejects uncatchable signals", func(t *testing.T) {
		svc, _ := newTestService(t)

		for _, tc := range []struct {
			name string
			send func(syscall.Signal) error
		}{
			{
				name: "by id",
				send: func(sig syscall.Signal) error {
					return svc.Signal("nonexistent", sig)
				},
			},
			{
				name: "by pid",
				send: func(sig syscall.Signal) error {
					return svc.SignalPID(999999, sig)
				},
			},
		} {
			for _, sig := range []syscall.Signal{syscall.SIGKILL, syscall.SIGSTOP} {
				t.Run(tc.name+"/"+strconv.Itoa(int(sig)), func(t *testing.T) {
					err := tc.send(sig)
					requireError(t, err)
					assertErrorIs(t, err, ErrUncatchableSignal)
					assertContains(t, err.Error(), "signal "+strconv.Itoa(int(sig))+" cannot be caught")
				})
			}
		}
	})

	t.Run("signals running process by id", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "sleep", "60")
		requireNoError(t, err)

		err = svc.Signal(proc.ID, syscall.SIGTERM)
		assertNoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been signalled")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("signals unmanaged process by pid", func(t *testing.T) {
		svc, _ := newTestService(t)

		cmd := exec.Command("sleep", "60")
		requireNoError(t, cmd.Start())

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		t.Cleanup(func() {
			if cmd.ProcessState == nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			select {
			case <-waitCh:
			case <-time.After(2 * time.Second):
			}
		})

		err := svc.SignalPID(cmd.Process.Pid, syscall.SIGTERM)
		requireNoError(t, err)

		select {
		case err := <-waitCh:
			requireError(t, err)
			var exitErr *exec.ExitError
			requireErrorAs(t, err, &exitErr)
			ws, ok := exitErr.Sys().(syscall.WaitStatus)
			requireTrue(t, ok)
			assertTrue(t, ws.Signaled())
			assertEqual(t, syscall.SIGTERM, ws.Signal())
		case <-time.After(2 * time.Second):
			t.Fatal("unmanaged process should have been signalled")
		}
	})
}

func TestService_Output(t *testing.T) {
	t.Run("returns captured output", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "captured")
		requireNoError(t, err)
		<-proc.Done()

		output, err := svc.Output(proc.ID)
		requireNoError(t, err)
		assertContains(t, output, "captured")
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Output("nonexistent")
		assertErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_Input(t *testing.T) {
	t.Run("writes to stdin", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		err = svc.Input(proc.ID, "service-input\n")
		requireNoError(t, err)

		err = svc.CloseStdin(proc.ID)
		requireNoError(t, err)

		<-proc.Done()

		assertContains(t, proc.Output(), "service-input")
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		err := svc.Input("nonexistent", "test")
		assertErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_CloseStdin(t *testing.T) {
	t.Run("closes stdin pipe", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		err = svc.CloseStdin(proc.ID)
		requireNoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("cat should exit when stdin is closed")
		}
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		err := svc.CloseStdin("nonexistent")
		assertErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_Wait(t *testing.T) {
	t.Run("returns final info on success", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "waited")
		requireNoError(t, err)

		info, err := svc.Wait(proc.ID)
		requireNoError(t, err)
		assertEqual(t, proc.ID, info.ID)
		assertEqual(t, StatusExited, info.Status)
		assertEqual(t, 0, info.ExitCode)
	})

	t.Run("returns error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Wait("nonexistent")
		assertErrorIs(t, err, ErrProcessNotFound)
	})

	t.Run("returns info alongside failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 7")
		requireNoError(t, err)

		info, err := svc.Wait(proc.ID)
		requireError(t, err)
		assertEqual(t, proc.ID, info.ID)
		assertEqual(t, StatusExited, info.Status)
		assertEqual(t, 7, info.ExitCode)
	})
}

func TestService_OnShutdown(t *testing.T) {
	t.Run("kills all running processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc1, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)
		proc2, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		assertTrue(t, proc1.IsRunning())
		assertTrue(t, proc2.IsRunning())

		assertNoError(t, resultErr(svc.OnShutdown(context.Background())))

		select {
		case <-proc1.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("proc1 should have been killed")
		}
		select {
		case <-proc2.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("proc2 should have been killed")
		}
	})

	t.Run("does not wait for process grace period", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:     "sh",
			Args:        []string{"-c", "trap '' TERM; sleep 60"},
			GracePeriod: 5 * time.Second,
		})
		requireNoError(t, err)
		requireTrue(t, proc.IsRunning())

		start := time.Now()
		requireNoError(t, resultErr(svc.OnShutdown(context.Background())))

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed immediately on shutdown")
		}

		assertLess(t, time.Since(start), 2*time.Second)
		assertEqual(t, StatusKilled, proc.Status)
	})
}

func TestService_OnStartup(t *testing.T) {
	t.Run("registers detached process.start action", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		ctx, cancel := context.WithCancel(context.Background())
		result := c.Action("process.start").Run(ctx, framework.NewOptions(
			framework.Option{Key: "command", Value: "sleep"},
			framework.Option{Key: "args", Value: []string{"0.1"}},
		))
		requireTrue(t, result.OK)

		id, ok := result.Value.(string)
		requireTrue(t, ok)
		proc, err := svc.Get(id)
		requireNoError(t, err)

		cancel()

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("detached action-started process should complete")
		}

		assertEqual(t, StatusExited, proc.Status)
		assertEqual(t, 0, proc.ExitCode)
	})

	t.Run("registers process.start task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		result := performTask(svc, c, TaskProcessStart{
			Command: "sleep",
			Args:    []string{"1"},
		})

		requireTrue(t, result.OK)

		info, ok := result.Value.(Info)
		requireTrue(t, ok)
		assertNotEmpty(t, info.ID)
		assertEqual(t, StatusRunning, info.Status)
		assertTrue(t, info.Running)

		proc, err := svc.Get(info.ID)
		requireNoError(t, err)
		assertTrue(t, proc.IsRunning())

		<-proc.Done()
		assertEqual(t, StatusExited, proc.Status)
	})

	t.Run("registers process.run task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		result := performTask(svc, c, TaskProcessRun{
			Command: "echo",
			Args:    []string{"action-run"},
		})

		requireTrue(t, result.OK)
		assertContains(t, result.Value.(string), "action-run")
	})

	t.Run("forwards task execution options", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		result := performTask(svc, c, TaskProcessRun{
			Command:     "sleep",
			Args:        []string{"60"},
			Timeout:     100 * time.Millisecond,
			GracePeriod: 50 * time.Millisecond,
		})

		requireFalse(t, result.OK)
		assertNil(t, result.Value)
	})

	t.Run("registers process.kill task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "sleep", "60")
		requireNoError(t, err)
		requireTrue(t, proc.IsRunning())

		var killed []ActionProcessKilled
		c.RegisterAction(func(cc *framework.Core, msg framework.Message) framework.Result {
			if m, ok := msg.(ActionProcessKilled); ok {
				killed = append(killed, m)
			}
			return framework.Result{OK: true}
		})

		result := performTask(svc, c, TaskProcessKill{PID: proc.Info().PID})
		requireTrue(t, result.OK)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed by pid")
		}

		assertEqual(t, StatusKilled, proc.Status)
		requireLen(t, killed, 1)
		assertEqual(t, proc.ID, killed[0].ID)
		assertNotEmpty(t, killed[0].Signal)
	})

	t.Run("registers process.signal task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "sleep", "60")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessSignal{
			ID:     proc.ID,
			Signal: syscall.SIGTERM,
		})
		requireTrue(t, result.OK)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been signalled through core")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("allows signal zero liveness checks", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessSignal{
			ID:     proc.ID,
			Signal: syscall.Signal(0),
		})
		requireTrue(t, result.OK)

		assertTrue(t, proc.IsRunning())

		cancel()
		<-proc.Done()
	})

	t.Run("signal zero does not kill process groups", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sh",
			Args:      []string{"-c", "sleep 60 & wait"},
			Detach:    true,
			KillGroup: true,
		})
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessSignal{
			ID:     proc.ID,
			Signal: syscall.Signal(0),
		})
		requireTrue(t, result.OK)

		time.Sleep(300 * time.Millisecond)
		assertTrue(t, proc.IsRunning())

		err = proc.Kill()
		requireNoError(t, err)
		<-proc.Done()
	})

	t.Run("registers process.wait task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "echo", "action-wait")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessWait{ID: proc.ID})
		requireTrue(t, result.OK)

		info, ok := result.Value.(Info)
		requireTrue(t, ok)
		assertEqual(t, proc.ID, info.ID)
		assertEqual(t, StatusExited, info.Status)
		assertEqual(t, 0, info.ExitCode)
	})

	t.Run("preserves final snapshot when process.wait task fails", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 7")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessWait{ID: proc.ID})
		requireTrue(t, result.OK)

		errValue, ok := result.Value.(error)
		requireTrue(t, ok)
		var waitErr *TaskProcessWaitError
		requireErrorAs(t, errValue, &waitErr)
		assertContains(t, waitErr.Error(), "process exited with code 7")
		assertEqual(t, proc.ID, waitErr.Info.ID)
		assertEqual(t, StatusExited, waitErr.Info.Status)
		assertEqual(t, 7, waitErr.Info.ExitCode)
	})

	t.Run("registers process.list task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessList{RunningOnly: true})
		requireTrue(t, result.OK)

		infos, ok := result.Value.([]Info)
		requireTrue(t, ok)
		requireLen(t, infos, 1)
		assertEqual(t, proc.ID, infos[0].ID)
		assertTrue(t, infos[0].Running)

		cancel()
		<-proc.Done()
	})

	t.Run("registers process.get task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "echo", "snapshot")
		requireNoError(t, err)
		<-proc.Done()

		result := performTask(svc, c, TaskProcessGet{ID: proc.ID})
		requireTrue(t, result.OK)

		info, ok := result.Value.(Info)
		requireTrue(t, ok)
		assertEqual(t, proc.ID, info.ID)
		assertEqual(t, proc.Command, info.Command)
		assertEqual(t, proc.Args, info.Args)
		assertEqual(t, proc.Status, info.Status)
		assertEqual(t, proc.ExitCode, info.ExitCode)
		assertEqual(t, proc.Info().PID, info.PID)
	})

	t.Run("registers process.remove task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "echo", "remove-through-core")
		requireNoError(t, err)
		<-proc.Done()

		result := performTask(svc, c, TaskProcessRemove{ID: proc.ID})
		requireTrue(t, result.OK)

		_, err = svc.Get(proc.ID)
		assertErrorIs(t, err, ErrProcessNotFound)
	})

	t.Run("registers process.clear task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		first, err := svc.Start(context.Background(), "echo", "clear-through-core-1")
		requireNoError(t, err)
		second, err := svc.Start(context.Background(), "echo", "clear-through-core-2")
		requireNoError(t, err)
		<-first.Done()
		<-second.Done()

		requireLen(t, svc.List(), 2)

		result := performTask(svc, c, TaskProcessClear{})
		requireTrue(t, result.OK)
		assertLen(t, svc.List(), 0)
	})

	t.Run("registers process.output task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "echo", "snapshot-output")
		requireNoError(t, err)
		<-proc.Done()

		result := performTask(svc, c, TaskProcessOutput{ID: proc.ID})
		requireTrue(t, result.OK)

		output, ok := result.Value.(string)
		requireTrue(t, ok)
		assertContains(t, output, "snapshot-output")
	})

	t.Run("registers process.input task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessInput{
			ID:    proc.ID,
			Input: "typed-through-core\n",
		})
		requireTrue(t, result.OK)

		err = proc.CloseStdin()
		requireNoError(t, err)

		<-proc.Done()

		assertContains(t, proc.Output(), "typed-through-core")
	})

	t.Run("registers process.close_stdin task", func(t *testing.T) {
		svc, c := newTestService(t)

		requireNoError(t, resultErr(svc.OnStartup(context.Background())))

		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		result := performTask(svc, c, TaskProcessInput{
			ID:    proc.ID,
			Input: "close-through-core\n",
		})
		requireTrue(t, result.OK)

		result = performTask(svc, c, TaskProcessCloseStdin{ID: proc.ID})
		requireTrue(t, result.OK)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have exited after stdin was closed")
		}

		assertContains(t, proc.Output(), "close-through-core")
	})
}

func TestService_RunWithOptions(t *testing.T) {
	t.Run("returns output on success", func(t *testing.T) {
		svc, _ := newTestService(t)

		output, err := svc.RunWithOptions(context.Background(), RunOptions{
			Command: "echo",
			Args:    []string{"opts-test"},
		})
		requireNoError(t, err)
		assertContains(t, output, "opts-test")
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.RunWithOptions(context.Background(), RunOptions{
			Command: "sh",
			Args:    []string{"-c", "exit 2"},
		})
		assertError(t, err)
		assertContains(t, err.Error(), "exited with code 2")
	})

	t.Run("rejects nil context", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.RunWithOptions(nil, RunOptions{
			Command: "echo",
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrContextRequired)
	})
}

func TestService_Running(t *testing.T) {
	t.Run("returns only running processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc1, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		doneProc, err := svc.Start(context.Background(), "echo", "done")
		requireNoError(t, err)
		<-doneProc.Done()

		running := svc.Running()
		assertLen(t, running, 1)
		assertEqual(t, proc1.ID, running[0].ID)

		proc2, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		running = svc.Running()
		assertLen(t, running, 2)
		assertEqual(t, proc1.ID, running[0].ID)
		assertEqual(t, proc2.ID, running[1].ID)

		cancel()
		<-proc1.Done()
		<-proc2.Done()
	})
}

func TestService_NewService_Good(t *testing.T) {
	factory := NewService(Options{BufferSize: 64})
	raw, err := factory(framework.New())
	requireNoError(t, err)
	svc := raw.(*Service)
	assertEqual(t, 64, svc.bufSize)
}

func TestService_NewService_Bad(t *testing.T) {
	factory := NewService(Options{})
	raw, err := factory(nil)
	requireNoError(t, err)
	svc := raw.(*Service)
	assertEqual(t, DefaultBufferSize, svc.bufSize)
}

func TestService_NewService_Ugly(t *testing.T) {
	factory := NewService(Options{BufferSize: -1})
	raw, err := factory(framework.New())
	requireNoError(t, err)
	svc := raw.(*Service)
	assertEqual(t, -1, svc.bufSize)
}

func TestService_Service_OnStartup_Good(t *testing.T) {
	svc, c := newTestService(t)
	r := svc.OnStartup(context.Background())
	requireTrue(t, r.OK)
	assertTrue(t, c.Action("process.run").Exists())
}

func TestService_Service_OnStartup_Bad(t *testing.T) {
	factory := NewService(Options{})
	raw, err := factory(nil)
	requireNoError(t, err)
	svc := raw.(*Service)
	r := svc.OnStartup(context.Background())
	assertTrue(t, r.OK)
}

func TestService_Service_OnStartup_Ugly(t *testing.T) {
	svc, c := newTestService(t)
	requireTrue(t, svc.OnStartup(context.Background()).OK)
	requireTrue(t, svc.OnStartup(context.Background()).OK)
	assertTrue(t, c.Action("process.start").Exists())
}

func TestService_Service_OnShutdown_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	r := svc.OnShutdown(context.Background())
	requireTrue(t, r.OK)
	<-proc.Done()
}

func TestService_Service_OnShutdown_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	r := svc.OnShutdown(context.Background())
	assertTrue(t, r.OK)
	assertEmpty(t, svc.Running())
}

func TestService_Service_OnShutdown_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	r := svc.OnShutdown(nil)
	assertTrue(t, r.OK)
	assertEmpty(t, svc.List())
}

func TestService_Service_Start_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	<-proc.Done()
	assertEqual(t, StatusExited, proc.Status)
}

func TestService_Service_Start_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "")
	assertNil(t, proc)
	assertError(t, err)
}

func TestService_Service_Start_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(nil, "echo")
	assertNil(t, proc)
	assertErrorIs(t, err, ErrContextRequired)
}

func TestService_Service_StartWithOptions_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.StartWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}})
	requireNoError(t, err)
	<-proc.Done()
	assertEqual(t, StatusExited, proc.Status)
}

func TestService_Service_StartWithOptions_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.StartWithOptions(context.Background(), RunOptions{})
	assertNil(t, proc)
	assertError(t, err)
}

func TestService_Service_StartWithOptions_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.StartWithOptions(context.Background(), RunOptions{Command: "sleep", Args: []string{"1"}, KillGroup: true})
	assertNil(t, proc)
	assertError(t, err)
}

func TestService_Service_Get_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	got, err := svc.Get(proc.ID)
	requireNoError(t, err)
	assertEqual(t, proc, got)
}

func TestService_Service_Get_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	got, err := svc.Get("missing")
	assertNil(t, got)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestService_Service_Get_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	got, err := svc.Get("")
	assertNil(t, got)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestService_Service_List_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	<-proc.Done()
	assertLen(t, svc.List(), 1)
}

func TestService_Service_List_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	procs := svc.List()
	assertEmpty(t, procs)
	assertEqual(t, 0, len(procs))
}

func TestService_Service_List_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	procs := svc.List()
	procs = append(procs, &Process{ID: "mutated"})
	assertEmpty(t, svc.List())
}

func TestService_Service_Running_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	running := svc.Running()
	requireLen(t, running, 1)
	assertEqual(t, proc.ID, running[0].ID)
	requireNoError(t, svc.Kill(proc.ID))
}

func TestService_Service_Running_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	running := svc.Running()
	assertEmpty(t, running)
	assertEqual(t, 0, len(running))
}

func TestService_Service_Running_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "done")
	requireNoError(t, err)
	<-proc.Done()
	assertEmpty(t, svc.Running())
}

func TestService_Service_Kill_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = svc.Kill(proc.ID)
	requireNoError(t, err)
	<-proc.Done()
}

func TestService_Service_Kill_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Kill("missing")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertEmpty(t, svc.List())
}

func TestService_Service_Kill_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "done")
	requireNoError(t, err)
	<-proc.Done()
	err = svc.Kill(proc.ID)
	requireNoError(t, err)
}

func TestService_Service_KillPID_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = svc.KillPID(proc.Info().PID)
	requireNoError(t, err)
	<-proc.Done()
}

func TestService_Service_KillPID_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.KillPID(0)
	assertError(t, err)
	assertEmpty(t, svc.List())
}

func TestService_Service_KillPID_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.KillPID(-1)
	assertError(t, err)
	assertEmpty(t, svc.List())
}

func TestService_Service_Signal_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = svc.Signal(proc.ID, syscall.SIGTERM)
	requireNoError(t, err)
	<-proc.Done()
}

func TestService_Service_Signal_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Signal("missing", syscall.SIGTERM)
	assertErrorIs(t, err, ErrProcessNotFound)
	assertEmpty(t, svc.List())
}

func TestService_Service_Signal_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Signal("missing", syscall.SIGKILL)
	assertErrorIs(t, err, ErrUncatchableSignal)
	assertEmpty(t, svc.List())
}

func TestService_Service_SignalPID_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = svc.SignalPID(proc.Info().PID, syscall.SIGTERM)
	requireNoError(t, err)
	<-proc.Done()
}

func TestService_Service_SignalPID_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.SignalPID(0, syscall.SIGTERM)
	assertError(t, err)
	assertEmpty(t, svc.List())
}

func TestService_Service_SignalPID_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.SignalPID(1, syscall.SIGKILL)
	assertErrorIs(t, err, ErrUncatchableSignal)
	assertEmpty(t, svc.List())
}

func TestService_Service_Remove_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "remove")
	requireNoError(t, err)
	<-proc.Done()
	err = svc.Remove(proc.ID)
	requireNoError(t, err)
}

func TestService_Service_Remove_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Remove("missing")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertEmpty(t, svc.List())
}

func TestService_Service_Remove_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = svc.Remove(proc.ID)
	assertError(t, err)
	requireNoError(t, svc.Kill(proc.ID))
}

func TestService_Service_Clear_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "clear")
	requireNoError(t, err)
	<-proc.Done()
	svc.Clear()
	assertEmpty(t, svc.List())
}

func TestService_Service_Clear_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	svc.Clear()
	assertEmpty(t, svc.List())
	assertEmpty(t, svc.Running())
}

func TestService_Service_Clear_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	svc.Clear()
	assertLen(t, svc.Running(), 1)
	requireNoError(t, svc.Kill(proc.ID))
}

func TestService_Service_Output_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	<-proc.Done()
	out, err := svc.Output(proc.ID)
	requireNoError(t, err)
	assertContains(t, out, "hello")
}

func TestService_Service_Output_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Output("missing")
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestService_Service_Output_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.StartWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}, DisableCapture: true})
	requireNoError(t, err)
	<-proc.Done()
	out, err := svc.Output(proc.ID)
	requireNoError(t, err)
	assertEqual(t, "", out)
}

func TestService_Service_Input_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "cat")
	requireNoError(t, err)
	requireNoError(t, svc.Input(proc.ID, "hello\n"))
	requireNoError(t, svc.CloseStdin(proc.ID))
	<-proc.Done()
	assertContains(t, proc.Output(), "hello")
}

func TestService_Service_Input_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Input("missing", "hello")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertEmpty(t, svc.List())
}

func TestService_Service_Input_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "done")
	requireNoError(t, err)
	<-proc.Done()
	err = svc.Input(proc.ID, "late")
	assertErrorIs(t, err, ErrProcessNotRunning)
}

func TestService_Service_CloseStdin_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "cat")
	requireNoError(t, err)
	err = svc.CloseStdin(proc.ID)
	requireNoError(t, err)
	<-proc.Done()
}

func TestService_Service_CloseStdin_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.CloseStdin("missing")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertEmpty(t, svc.List())
}

func TestService_Service_CloseStdin_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "cat")
	requireNoError(t, err)
	requireNoError(t, svc.CloseStdin(proc.ID))
	err = svc.CloseStdin(proc.ID)
	requireNoError(t, err)
}

func TestService_Service_Wait_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	info, err := svc.Wait(proc.ID)
	requireNoError(t, err)
	assertEqual(t, proc.ID, info.ID)
}

func TestService_Service_Wait_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	info, err := svc.Wait("missing")
	assertEqual(t, Info{}, info)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestService_Service_Wait_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "false")
	requireNoError(t, err)
	info, err := svc.Wait(proc.ID)
	assertEqual(t, proc.ID, info.ID)
	assertError(t, err)
}

func TestService_Service_Run_Good(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Run(context.Background(), "echo", "hello")
	requireNoError(t, err)
	assertContains(t, out, "hello")
}

func TestService_Service_Run_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Run(context.Background(), "false")
	assertEqual(t, "", out)
	assertError(t, err)
}

func TestService_Service_Run_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.Run(nil, "echo", "hello")
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrContextRequired)
}

func TestService_Service_RunWithOptions_Good(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.RunWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}})
	requireNoError(t, err)
	assertContains(t, out, "hello")
}

func TestService_Service_RunWithOptions_Bad(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.RunWithOptions(context.Background(), RunOptions{Command: "false"})
	assertEqual(t, "", out)
	assertError(t, err)
}

func TestService_Service_RunWithOptions_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	out, err := svc.RunWithOptions(nil, RunOptions{Command: "echo"})
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrContextRequired)
}
