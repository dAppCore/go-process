package process

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcess_Info_Good(t *testing.T) {
	svc, _ := newTestService(t)

	proc := startProc(t, svc, context.Background(), "echo", "hello")

	<-proc.Done()

	info := proc.Info()
	assert.Equal(t, proc.ID, info.ID)
	assert.Equal(t, "echo", info.Command)
	assert.Equal(t, []string{"hello"}, info.Args)
	assert.Equal(t, StatusExited, info.Status)
	assert.Equal(t, 0, info.ExitCode)
	assert.Greater(t, info.Duration, time.Duration(0))
}

func TestProcess_Info_Pending_Good(t *testing.T) {
	proc := &ManagedProcess{
		ID:     "pending",
		Status: StatusPending,
		done:   make(chan struct{}),
	}

	info := proc.Info()
	assert.Equal(t, StatusPending, info.Status)
	assert.False(t, info.Running)
}

func TestProcess_Output_Good(t *testing.T) {
	t.Run("captures stdout", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "hello world")
		<-proc.Done()
		assert.Contains(t, proc.Output(), "hello world")
	})

	t.Run("OutputBytes returns copy", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "test")
		<-proc.Done()
		bytes := proc.OutputBytes()
		assert.NotNil(t, bytes)
		assert.Contains(t, string(bytes), "test")
	})
}

func TestProcess_IsRunning_Good(t *testing.T) {
	t.Run("true while running", func(t *testing.T) {
		svc, _ := newTestService(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc := startProc(t, svc, ctx, "sleep", "10")
		assert.True(t, proc.IsRunning())

		cancel()
		<-proc.Done()
		assert.False(t, proc.IsRunning())
	})

	t.Run("false after completion", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "done")
		<-proc.Done()
		assert.False(t, proc.IsRunning())
	})
}

func TestProcess_Wait_Good(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "ok")
		err := proc.Wait()
		assert.NoError(t, err)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "sh", "-c", "exit 1")
		err := proc.Wait()
		assert.Error(t, err)
	})
}

func TestProcess_Done_Good(t *testing.T) {
	t.Run("channel closes on completion", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "test")

		select {
		case <-proc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("Done channel should have closed")
		}
	})
}

func TestProcess_Kill_Good(t *testing.T) {
	t.Run("terminates running process", func(t *testing.T) {
		svc, _ := newTestService(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc := startProc(t, svc, ctx, "sleep", "60")
		assert.True(t, proc.IsRunning())

		err := proc.Kill()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}
		assert.Equal(t, StatusKilled, proc.Status)
		assert.Equal(t, -1, proc.ExitCode)
	})

	t.Run("noop on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "done")
		<-proc.Done()
		err := proc.Kill()
		assert.NoError(t, err)
	})
}

func TestProcess_SendInput_Good(t *testing.T) {
	t.Run("writes to stdin", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "cat")

		err := proc.SendInput("hello\n")
		assert.NoError(t, err)
		err = proc.CloseStdin()
		assert.NoError(t, err)
		<-proc.Done()
		assert.Contains(t, proc.Output(), "hello")
	})

	t.Run("error on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "done")
		<-proc.Done()
		err := proc.SendInput("test")
		assert.ErrorIs(t, err, ErrProcessNotRunning)
	})
}

func TestProcess_Signal_Good(t *testing.T) {
	t.Run("sends signal to running process", func(t *testing.T) {
		svc, _ := newTestService(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc := startProc(t, svc, ctx, "sleep", "60")
		err := proc.Signal(os.Interrupt)
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been terminated by signal")
		}
		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("error on completed process", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "echo", "done")
		<-proc.Done()
		err := proc.Signal(os.Interrupt)
		assert.ErrorIs(t, err, ErrProcessNotRunning)
	})
}

func TestProcess_CloseStdin_Good(t *testing.T) {
	t.Run("closes stdin pipe", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "cat")
		err := proc.CloseStdin()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("cat should exit when stdin is closed")
		}
	})

	t.Run("double close is safe", func(t *testing.T) {
		svc, _ := newTestService(t)
		proc := startProc(t, svc, context.Background(), "cat")
		err := proc.CloseStdin()
		assert.NoError(t, err)
		<-proc.Done()
		err = proc.CloseStdin()
		assert.NoError(t, err)
	})
}

func TestProcess_Timeout_Good(t *testing.T) {
	t.Run("kills process after timeout", func(t *testing.T) {
		svc, _ := newTestService(t)
		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sleep",
			Args:    []string{"60"},
			Timeout: 200 * time.Millisecond,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		select {
		case <-proc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("process should have been killed by timeout")
		}
		assert.False(t, proc.IsRunning())
		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("no timeout when zero", func(t *testing.T) {
		svc, _ := newTestService(t)
		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "echo",
			Args:    []string{"fast"},
			Timeout: 0,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)
		<-proc.Done()
		assert.Equal(t, 0, proc.ExitCode)
	})
}

func TestProcess_Shutdown_Good(t *testing.T) {
	t.Run("graceful with grace period", func(t *testing.T) {
		svc, _ := newTestService(t)
		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command:     "sleep",
			Args:        []string{"60"},
			GracePeriod: 100 * time.Millisecond,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		assert.True(t, proc.IsRunning())
		err := proc.Shutdown()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown should have completed")
		}
	})

	t.Run("immediate kill without grace period", func(t *testing.T) {
		svc, _ := newTestService(t)
		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sleep",
			Args:    []string{"60"},
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		err := proc.Shutdown()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("kill should be immediate")
		}
	})
}

func TestProcess_KillGroup_Good(t *testing.T) {
	t.Run("kills child processes", func(t *testing.T) {
		svc, _ := newTestService(t)
		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command:   "sh",
			Args:      []string{"-c", "sleep 60 & wait"},
			Detach:    true,
			KillGroup: true,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		time.Sleep(100 * time.Millisecond)
		err := proc.Kill()
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("process group should have been killed")
		}
	})
}

func TestProcess_TimeoutWithGrace_Good(t *testing.T) {
	t.Run("timeout triggers graceful shutdown", func(t *testing.T) {
		svc, _ := newTestService(t)
		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command:     "sleep",
			Args:        []string{"60"},
			Timeout:     200 * time.Millisecond,
			GracePeriod: 100 * time.Millisecond,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		select {
		case <-proc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("process should have been killed by timeout")
		}
		assert.Equal(t, StatusKilled, proc.Status)
	})
}
