package process

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

var _ *Process = (*ManagedProcess)(nil)

func TestProcess_Info(t *testing.T) {
	svc, _ := newTestService(t)

	proc, err := svc.Start(context.Background(), "echo", "hello")
	requireNoError(t, err)

	<-proc.Done()

	info := proc.Info()
	assertEqual(t, proc.ID, info.ID)
	assertEqual(t, "echo", info.Command)
	assertEqual(t, []string{"hello"}, info.Args)
	assertFalse(t, info.Running)
	assertEqual(t, StatusExited, info.Status)
	assertEqual(t, 0, info.ExitCode)
	assertGreater(t, info.Duration, time.Duration(0))
}

func TestProcess_Info_Pending(t *testing.T) {
	proc := &Process{
		ID:     "pending",
		Status: StatusPending,
		done:   make(chan struct{}),
	}

	info := proc.Info()
	assertEqual(t, StatusPending, info.Status)
	assertFalse(t, info.Running)
}

func TestProcess_Info_RunningDuration(t *testing.T) {
	svc, _ := newTestService(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proc, err := svc.Start(ctx, "sleep", "10")
	requireNoError(t, err)

	time.Sleep(10 * time.Millisecond)
	info := proc.Info()
	assertTrue(t, info.Running)
	assertEqual(t, StatusRunning, info.Status)
	assertGreater(t, info.Duration, time.Duration(0))

	cancel()
	<-proc.Done()
}

func TestProcess_InfoSnapshot(t *testing.T) {
	svc, _ := newTestService(t)

	proc, err := svc.Start(context.Background(), "echo", "snapshot")
	requireNoError(t, err)

	<-proc.Done()

	info := proc.Info()
	requireNotEmpty(t, info.Args)

	info.Args[0] = "mutated"

	assertEqual(t, "snapshot", proc.Args[0])
	assertEqual(t, "mutated", info.Args[0])
}

func TestProcess_Output(t *testing.T) {
	t.Run("captures stdout", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "hello world")
		requireNoError(t, err)

		<-proc.Done()

		output := proc.Output()
		assertContains(t, output, "hello world")
	})

	t.Run("OutputBytes returns copy", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "test")
		requireNoError(t, err)

		<-proc.Done()

		bytes := proc.OutputBytes()
		assertNotNil(t, bytes)
		assertContains(t, string(bytes), "test")
	})
}

func TestProcess_IsRunning(t *testing.T) {
	t.Run("true while running", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "10")
		requireNoError(t, err)

		assertTrue(t, proc.IsRunning())
		assertTrue(t, proc.Info().Running)

		cancel()
		<-proc.Done()

		assertFalse(t, proc.IsRunning())
		assertFalse(t, proc.Info().Running)
	})

	t.Run("false after completion", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		requireNoError(t, err)

		<-proc.Done()

		assertFalse(t, proc.IsRunning())
	})
}

func TestProcess_Wait(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "ok")
		requireNoError(t, err)

		err = proc.Wait()
		assertNoError(t, err)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 1")
		requireNoError(t, err)

		err = proc.Wait()
		assertError(t, err)
	})
}

func TestProcess_Done(t *testing.T) {
	t.Run("channel closes on completion", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "test")
		requireNoError(t, err)

		select {
		case <-proc.Done():
			// Success - channel closed
		case <-time.After(5 * time.Second):
			t.Fatal("Done channel should have closed")
		}
	})
}

func TestProcess_Kill(t *testing.T) {
	t.Run("terminates running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		assertTrue(t, proc.IsRunning())

		err = proc.Kill()
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Good - process terminated
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("noop on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		requireNoError(t, err)

		<-proc.Done()

		err = proc.Kill()
		assertNoError(t, err)
	})
}

func TestProcess_SendInput(t *testing.T) {
	t.Run("writes to stdin", func(t *testing.T) {
		svc, _ := newTestService(t)

		// Use cat to echo back stdin
		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		err = proc.SendInput("hello\n")
		assertNoError(t, err)

		err = proc.CloseStdin()
		assertNoError(t, err)

		<-proc.Done()

		assertContains(t, proc.Output(), "hello")
	})

	t.Run("error on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		requireNoError(t, err)

		<-proc.Done()

		err = proc.SendInput("test")
		assertErrorIs(t, err, ErrProcessNotRunning)
	})
}

func TestProcess_Signal(t *testing.T) {
	t.Run("sends signal to running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		requireNoError(t, err)

		err = proc.Signal(os.Interrupt)
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Process terminated by signal
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been terminated by signal")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("error on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		requireNoError(t, err)
		<-proc.Done()

		err = proc.Signal(os.Interrupt)
		assertErrorIs(t, err, ErrProcessNotRunning)
	})

	t.Run("signals process group when kill group is enabled", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sh",
			Args:      []string{"-c", "trap '' INT; sh -c 'trap - INT; sleep 60' & wait"},
			Detach:    true,
			KillGroup: true,
		})
		requireNoError(t, err)

		err = proc.Signal(os.Interrupt)
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Good - the whole process group responded to the signal.
		case <-time.After(5 * time.Second):
			t.Fatal("process group should have been terminated by signal")
		}
	})

	t.Run("signal zero only probes process group liveness", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sh",
			Args:      []string{"-c", "sleep 60 & wait"},
			Detach:    true,
			KillGroup: true,
		})
		requireNoError(t, err)

		err = proc.Signal(syscall.Signal(0))
		assertNoError(t, err)

		time.Sleep(300 * time.Millisecond)
		assertTrue(t, proc.IsRunning())

		err = proc.Kill()
		assertNoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("process group should have been killed for cleanup")
		}
	})
}

func TestProcess_CloseStdin(t *testing.T) {
	t.Run("closes stdin pipe", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		err = proc.CloseStdin()
		assertNoError(t, err)

		// Process should exit now that stdin is closed
		select {
		case <-proc.Done():
			// Good
		case <-time.After(2 * time.Second):
			t.Fatal("cat should exit when stdin is closed")
		}
	})

	t.Run("double close is safe", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "cat")
		requireNoError(t, err)

		// First close
		err = proc.CloseStdin()
		assertNoError(t, err)

		<-proc.Done()

		// Second close should be safe (stdin already nil)
		err = proc.CloseStdin()
		assertNoError(t, err)
	})
}

func TestProcess_Timeout(t *testing.T) {
	t.Run("kills process after timeout", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sleep",
			Args:    []string{"60"},
			Timeout: 200 * time.Millisecond,
		})
		requireNoError(t, err)

		select {
		case <-proc.Done():
			// Good — process was killed by timeout
		case <-time.After(5 * time.Second):
			t.Fatal("process should have been killed by timeout")
		}

		assertFalse(t, proc.IsRunning())
		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("no timeout when zero", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "echo",
			Args:    []string{"fast"},
			Timeout: 0,
		})
		requireNoError(t, err)

		<-proc.Done()
		assertEqual(t, 0, proc.ExitCode)
	})
}

func TestProcess_Shutdown(t *testing.T) {
	t.Run("graceful with grace period", func(t *testing.T) {
		svc, _ := newTestService(t)

		// Use a process that traps SIGTERM
		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:     "sleep",
			Args:        []string{"60"},
			GracePeriod: 100 * time.Millisecond,
		})
		requireNoError(t, err)

		assertTrue(t, proc.IsRunning())

		err = proc.Shutdown()
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Good
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown should have completed")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})

	t.Run("immediate kill without grace period", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sleep",
			Args:    []string{"60"},
		})
		requireNoError(t, err)

		err = proc.Shutdown()
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Good
		case <-time.After(2 * time.Second):
			t.Fatal("kill should be immediate")
		}
	})
}

func TestProcess_KillGroup(t *testing.T) {
	t.Run("kills child processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		// Spawn a parent that spawns a child — KillGroup should kill both
		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sh",
			Args:      []string{"-c", "sleep 60 & wait"},
			Detach:    true,
			KillGroup: true,
		})
		requireNoError(t, err)

		// Give child time to spawn
		time.Sleep(100 * time.Millisecond)

		err = proc.Kill()
		assertNoError(t, err)

		select {
		case <-proc.Done():
			// Good — whole group killed
		case <-time.After(5 * time.Second):
			t.Fatal("process group should have been killed")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})
}

func TestProcess_TimeoutWithGrace(t *testing.T) {
	t.Run("timeout triggers graceful shutdown", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:     "sleep",
			Args:        []string{"60"},
			Timeout:     200 * time.Millisecond,
			GracePeriod: 100 * time.Millisecond,
		})
		requireNoError(t, err)

		select {
		case <-proc.Done():
			// Good — timeout + grace triggered
		case <-time.After(5 * time.Second):
			t.Fatal("process should have been killed by timeout")
		}

		assertEqual(t, StatusKilled, proc.Status)
	})
}

func TestProcess_ManagedProcess_Info_Good(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "hello\n")
	info := proc.Info()
	assertEqual(t, proc.ID, info.ID)
	assertEqual(t, StatusExited, info.Status)
	assertFalse(t, info.Running)
}

func TestProcess_ManagedProcess_Info_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusFailed, -1, "")
	info := proc.Info()
	assertEqual(t, StatusFailed, info.Status)
	assertEqual(t, -1, info.ExitCode)
	assertFalse(t, info.Running)
}

func TestProcess_ManagedProcess_Info_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	info := proc.Info()
	assertTrue(t, info.Running)
	assertEqual(t, StatusRunning, info.Status)
	assertGreaterOrEqual(t, info.Duration, time.Duration(0))
}

func TestProcess_ManagedProcess_Output_Good(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "hello\n")
	got := proc.Output()
	assertEqual(t, "hello\n", got)
	assertEqual(t, 6, proc.output.Len())
}

func TestProcess_ManagedProcess_Output_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	proc.output = nil
	got := proc.Output()
	assertEqual(t, "", got)
}

func TestProcess_ManagedProcess_Output_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	got := proc.Output()
	assertEqual(t, "", got)
	assertEqual(t, 0, proc.output.Len())
}

func TestProcess_ManagedProcess_OutputBytes_Good(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "hello")
	got := proc.OutputBytes()
	assertEqual(t, []byte("hello"), got)
	assertEqual(t, 5, len(got))
}

func TestProcess_ManagedProcess_OutputBytes_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	proc.output = nil
	got := proc.OutputBytes()
	assertNil(t, got)
}

func TestProcess_ManagedProcess_OutputBytes_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	got := proc.OutputBytes()
	assertNil(t, got)
	assertEqual(t, "", proc.Output())
}

func TestProcess_ManagedProcess_IsRunning_Good(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	got := proc.IsRunning()
	assertTrue(t, got)
	assertEqual(t, StatusRunning, proc.Status)
}

func TestProcess_ManagedProcess_IsRunning_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	got := proc.IsRunning()
	assertFalse(t, got)
	assertEqual(t, StatusExited, proc.Status)
}

func TestProcess_ManagedProcess_IsRunning_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusKilled, -1, "")
	got := proc.IsRunning()
	assertFalse(t, got)
	assertEqual(t, StatusKilled, proc.Status)
}

func TestProcess_ManagedProcess_Wait_Good(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	err := proc.Wait()
	requireNoError(t, err)
	assertEqual(t, 0, proc.ExitCode)
}

func TestProcess_ManagedProcess_Wait_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 7, "")
	err := proc.Wait()
	assertError(t, err)
	assertContains(t, err.Error(), "code 7")
}

func TestProcess_ManagedProcess_Wait_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusKilled, -1, "")
	err := proc.Wait()
	assertError(t, err)
	assertContains(t, err.Error(), "killed")
}

func TestProcess_ManagedProcess_Done_Good(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	done := proc.Done()
	_, ok := <-done
	assertFalse(t, ok)
}

func TestProcess_ManagedProcess_Done_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	done := proc.Done()
	assertNotNil(t, done)
	assertTrue(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_Done_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusFailed, -1, "")
	done := proc.Done()
	_, ok := <-done
	assertFalse(t, ok)
}

func TestProcess_ManagedProcess_Kill_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = proc.Kill()
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcess_ManagedProcess_Kill_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	err := proc.Kill()
	requireNoError(t, err)
	assertFalse(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_Kill_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.cmd = nil
	err := proc.Kill()
	requireNoError(t, err)
	assertTrue(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_Shutdown_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.StartWithOptions(context.Background(), RunOptions{Command: "sleep", Args: []string{"5"}, GracePeriod: 10 * time.Millisecond})
	requireNoError(t, err)
	err = proc.Shutdown()
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcess_ManagedProcess_Shutdown_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	err := proc.Shutdown()
	requireNoError(t, err)
	assertFalse(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_Shutdown_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.cmd = nil
	err := proc.Shutdown()
	requireNoError(t, err)
	assertTrue(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_Signal_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcess_ManagedProcess_Signal_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	err := proc.Signal(syscall.SIGTERM)
	assertErrorIs(t, err, ErrProcessNotRunning)
	assertFalse(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_Signal_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.cmd = nil
	err := proc.Signal(syscall.Signal(0))
	requireNoError(t, err)
	assertTrue(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_SendInput_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "cat")
	requireNoError(t, err)
	requireNoError(t, proc.SendInput("hello\n"))
	requireNoError(t, proc.CloseStdin())
	<-proc.Done()
	assertContains(t, proc.Output(), "hello")
}

func TestProcess_ManagedProcess_SendInput_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	err := proc.SendInput("hello")
	assertErrorIs(t, err, ErrProcessNotRunning)
	assertFalse(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_SendInput_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.stdin = nil
	err := proc.SendInput("")
	assertErrorIs(t, err, ErrStdinNotAvailable)
	assertTrue(t, proc.IsRunning())
}

func TestProcess_ManagedProcess_CloseStdin_Good(t *testing.T) {
	svc, _ := newTestService(t)
	proc, err := svc.Start(context.Background(), "cat")
	requireNoError(t, err)
	err = proc.CloseStdin()
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcess_ManagedProcess_CloseStdin_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	err := proc.CloseStdin()
	requireNoError(t, err)
	assertNil(t, proc.stdin)
}

func TestProcess_ManagedProcess_CloseStdin_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.stdin = nil
	err := proc.CloseStdin()
	requireNoError(t, err)
	assertTrue(t, proc.IsRunning())
}
