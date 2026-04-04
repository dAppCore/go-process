package process

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcess_Info(t *testing.T) {
	svc, _ := newTestService(t)

	proc, err := svc.Start(context.Background(), "echo", "hello")
	require.NoError(t, err)

	<-proc.Done()

	info := proc.Info()
	assert.Equal(t, proc.ID, info.ID)
	assert.Equal(t, "echo", info.Command)
	assert.Equal(t, []string{"hello"}, info.Args)
	assert.False(t, info.Running)
	assert.Equal(t, StatusExited, info.Status)
	assert.Equal(t, 0, info.ExitCode)
	assert.Greater(t, info.Duration, time.Duration(0))
}

func TestProcess_Output(t *testing.T) {
	t.Run("captures stdout", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "hello world")
		require.NoError(t, err)

		<-proc.Done()

		output := proc.Output()
		assert.Contains(t, output, "hello world")
	})

	t.Run("OutputBytes returns copy", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "test")
		require.NoError(t, err)

		<-proc.Done()

		bytes := proc.OutputBytes()
		assert.NotNil(t, bytes)
		assert.Contains(t, string(bytes), "test")
	})
}

func TestProcess_IsRunning(t *testing.T) {
	t.Run("true while running", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "10")
		require.NoError(t, err)

		assert.True(t, proc.IsRunning())
		assert.True(t, proc.Info().Running)

		cancel()
		<-proc.Done()

		assert.False(t, proc.IsRunning())
		assert.False(t, proc.Info().Running)
	})

	t.Run("false after completion", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		require.NoError(t, err)

		<-proc.Done()

		assert.False(t, proc.IsRunning())
	})
}

func TestProcess_Wait(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "ok")
		require.NoError(t, err)

		err = proc.Wait()
		assert.NoError(t, err)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 1")
		require.NoError(t, err)

		err = proc.Wait()
		assert.Error(t, err)
	})
}

func TestProcess_Done(t *testing.T) {
	t.Run("channel closes on completion", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "test")
		require.NoError(t, err)

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
		require.NoError(t, err)

		assert.True(t, proc.IsRunning())

		err = proc.Kill()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
			// Good - process terminated
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}

		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("noop on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		require.NoError(t, err)

		<-proc.Done()

		err = proc.Kill()
		assert.NoError(t, err)
	})
}

func TestProcess_SendInput(t *testing.T) {
	t.Run("writes to stdin", func(t *testing.T) {
		svc, _ := newTestService(t)

		// Use cat to echo back stdin
		proc, err := svc.Start(context.Background(), "cat")
		require.NoError(t, err)

		err = proc.SendInput("hello\n")
		assert.NoError(t, err)

		err = proc.CloseStdin()
		assert.NoError(t, err)

		<-proc.Done()

		assert.Contains(t, proc.Output(), "hello")
	})

	t.Run("error on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		require.NoError(t, err)

		<-proc.Done()

		err = proc.SendInput("test")
		assert.ErrorIs(t, err, ErrProcessNotRunning)
	})
}

func TestProcess_Signal(t *testing.T) {
	t.Run("sends signal to running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		require.NoError(t, err)

		err = proc.Signal(os.Interrupt)
		assert.NoError(t, err)

		select {
		case <-proc.Done():
			// Process terminated by signal
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been terminated by signal")
		}

		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("error on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "done")
		require.NoError(t, err)
		<-proc.Done()

		err = proc.Signal(os.Interrupt)
		assert.ErrorIs(t, err, ErrProcessNotRunning)
	})

	t.Run("signals process group when kill group is enabled", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sh",
			Args:      []string{"-c", "trap '' INT; sh -c 'trap - INT; sleep 60' & wait"},
			Detach:    true,
			KillGroup: true,
		})
		require.NoError(t, err)

		err = proc.Signal(os.Interrupt)
		assert.NoError(t, err)

		select {
		case <-proc.Done():
			// Good - the whole process group responded to the signal.
		case <-time.After(5 * time.Second):
			t.Fatal("process group should have been terminated by signal")
		}
	})
}

func TestProcess_CloseStdin(t *testing.T) {
	t.Run("closes stdin pipe", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "cat")
		require.NoError(t, err)

		err = proc.CloseStdin()
		assert.NoError(t, err)

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
		require.NoError(t, err)

		// First close
		err = proc.CloseStdin()
		assert.NoError(t, err)

		<-proc.Done()

		// Second close should be safe (stdin already nil)
		err = proc.CloseStdin()
		assert.NoError(t, err)
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
		require.NoError(t, err)

		select {
		case <-proc.Done():
			// Good — process was killed by timeout
		case <-time.After(5 * time.Second):
			t.Fatal("process should have been killed by timeout")
		}

		assert.False(t, proc.IsRunning())
		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("no timeout when zero", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "echo",
			Args:    []string{"fast"},
			Timeout: 0,
		})
		require.NoError(t, err)

		<-proc.Done()
		assert.Equal(t, 0, proc.ExitCode)
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
		require.NoError(t, err)

		assert.True(t, proc.IsRunning())

		err = proc.Shutdown()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
			// Good
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown should have completed")
		}

		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("immediate kill without grace period", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sleep",
			Args:    []string{"60"},
		})
		require.NoError(t, err)

		err = proc.Shutdown()
		assert.NoError(t, err)

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
		require.NoError(t, err)

		// Give child time to spawn
		time.Sleep(100 * time.Millisecond)

		err = proc.Kill()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
			// Good — whole group killed
		case <-time.After(5 * time.Second):
			t.Fatal("process group should have been killed")
		}

		assert.Equal(t, StatusKilled, proc.Status)
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
		require.NoError(t, err)

		select {
		case <-proc.Done():
			// Good — timeout + grace triggered
		case <-time.After(5 * time.Second):
			t.Fatal("process should have been killed by timeout")
		}

		assert.Equal(t, StatusKilled, proc.Status)
	})
}
