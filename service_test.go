package process

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	framework "dappco.re/go/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*Service, *framework.Core) {
	t.Helper()

	c := framework.New()
	factory := NewService(Options{BufferSize: 1024})
	raw, err := factory(c)
	require.NoError(t, err)

	svc := raw.(*Service)
	return svc, c
}

func TestService_Start(t *testing.T) {
	t.Run("echo command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "hello")
		require.NoError(t, err)
		require.NotNil(t, proc)

		assert.NotEmpty(t, proc.ID)
		assert.Equal(t, "echo", proc.Command)
		assert.Equal(t, []string{"hello"}, proc.Args)

		// Wait for completion
		<-proc.Done()

		assert.Equal(t, StatusExited, proc.Status)
		assert.Equal(t, 0, proc.ExitCode)
		assert.Contains(t, proc.Output(), "hello")
	})

	t.Run("failing command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "sh", "-c", "exit 42")
		require.NoError(t, err)

		<-proc.Done()

		assert.Equal(t, StatusExited, proc.Status)
		assert.Equal(t, 42, proc.ExitCode)
	})

	t.Run("non-existent command", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Start(context.Background(), "nonexistent_command_xyz")
		assert.Error(t, err)
	})

	t.Run("empty command is rejected", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.StartWithOptions(context.Background(), RunOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("with working directory", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "pwd",
			Dir:     "/tmp",
		})
		require.NoError(t, err)

		<-proc.Done()

		// On macOS /tmp is a symlink to /private/tmp
		output := strings.TrimSpace(proc.Output())
		assert.True(t, output == "/tmp" || output == "/private/tmp", "got: %s", output)
	})

	t.Run("context cancellation", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		proc, err := svc.Start(ctx, "sleep", "10")
		require.NoError(t, err)

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
		require.NoError(t, err)
		<-proc.Done()

		assert.Equal(t, StatusExited, proc.Status)
		assert.Equal(t, "", proc.Output(), "output should be empty when capture is disabled")
	})

	t.Run("with environment variables", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sh",
			Args:    []string{"-c", "echo $MY_TEST_VAR"},
			Env:     []string{"MY_TEST_VAR=hello_env"},
		})
		require.NoError(t, err)
		<-proc.Done()

		assert.Contains(t, proc.Output(), "hello_env")
	})

	t.Run("detach survives parent context", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())

		proc, err := svc.StartWithOptions(ctx, RunOptions{
			Command: "echo",
			Args:    []string{"detached"},
			Detach:  true,
		})
		require.NoError(t, err)

		// Cancel the parent context
		cancel()

		// Detached process should still complete normally
		select {
		case <-proc.Done():
			assert.Equal(t, StatusExited, proc.Status)
			assert.Equal(t, 0, proc.ExitCode)
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
		require.Error(t, err)
		assert.Contains(t, err.Error(), "KillGroup requires Detach")
	})
}

func TestService_Run(t *testing.T) {
	t.Run("returns output", func(t *testing.T) {
		svc, _ := newTestService(t)

		output, err := svc.Run(context.Background(), "echo", "hello world")
		require.NoError(t, err)
		assert.Contains(t, output, "hello world")
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Run(context.Background(), "sh", "-c", "exit 1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exited with code 1")
	})
}

func TestService_Actions(t *testing.T) {
	t.Run("broadcasts events", func(t *testing.T) {
		c := framework.New()

		// Register process service on Core
		factory := NewService(Options{})
		raw, err := factory(c)
		require.NoError(t, err)
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
		require.NoError(t, err)

		<-proc.Done()

		// Give time for events to propagate
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.Len(t, started, 1)
		assert.Equal(t, "echo", started[0].Command)
		assert.Equal(t, []string{"test"}, started[0].Args)

		assert.NotEmpty(t, outputs)
		foundTest := false
		for _, o := range outputs {
			if strings.Contains(o.Line, "test") {
				foundTest = true
				break
			}
		}
		assert.True(t, foundTest, "should have output containing 'test'")

		assert.Len(t, exited, 1)
		assert.Equal(t, 0, exited[0].ExitCode)
	})

	t.Run("broadcasts killed events", func(t *testing.T) {
		c := framework.New()

		factory := NewService(Options{})
		raw, err := factory(c)
		require.NoError(t, err)
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
		require.NoError(t, err)

		err = svc.Kill(proc.ID)
		require.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		assert.Len(t, killed, 1)
		assert.Equal(t, proc.ID, killed[0].ID)
		assert.NotEmpty(t, killed[0].Signal)
		assert.Len(t, exited, 1)
		assert.Equal(t, proc.ID, exited[0].ID)
		assert.Error(t, exited[0].Error)
		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("broadcasts exited event on start failure", func(t *testing.T) {
		c := framework.New()

		factory := NewService(Options{})
		raw, err := factory(c)
		require.NoError(t, err)
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
		require.Error(t, err)

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, exited, 1)
		assert.Equal(t, -1, exited[0].ExitCode)
		assert.Error(t, exited[0].Error)
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
		assert.Len(t, list, 2)
		assert.Equal(t, proc1.ID, list[0].ID)
		assert.Equal(t, proc2.ID, list[1].ID)
	})

	t.Run("get by id", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, _ := svc.Start(context.Background(), "echo", "test")
		<-proc.Done()

		got, err := svc.Get(proc.ID)
		require.NoError(t, err)
		assert.Equal(t, proc.ID, got.ID)
	})

	t.Run("get not found", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Get("nonexistent")
		assert.ErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_Remove(t *testing.T) {
	t.Run("removes completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, _ := svc.Start(context.Background(), "echo", "test")
		<-proc.Done()

		err := svc.Remove(proc.ID)
		require.NoError(t, err)

		_, err = svc.Get(proc.ID)
		assert.ErrorIs(t, err, ErrProcessNotFound)
	})

	t.Run("cannot remove running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, _ := svc.Start(ctx, "sleep", "10")

		err := svc.Remove(proc.ID)
		assert.Error(t, err)

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

		assert.Len(t, svc.List(), 2)

		svc.Clear()

		assert.Len(t, svc.List(), 0)
	})
}

func TestService_Kill(t *testing.T) {
	t.Run("kills running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, err := svc.Start(ctx, "sleep", "60")
		require.NoError(t, err)

		err = svc.Kill(proc.ID)
		assert.NoError(t, err)

		select {
		case <-proc.Done():
			// Process killed successfully
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}

		assert.Equal(t, StatusKilled, proc.Status)
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		err := svc.Kill("nonexistent")
		assert.ErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_Output(t *testing.T) {
	t.Run("returns captured output", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc, err := svc.Start(context.Background(), "echo", "captured")
		require.NoError(t, err)
		<-proc.Done()

		output, err := svc.Output(proc.ID)
		require.NoError(t, err)
		assert.Contains(t, output, "captured")
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.Output("nonexistent")
		assert.ErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_OnShutdown(t *testing.T) {
	t.Run("kills all running processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc1, err := svc.Start(ctx, "sleep", "60")
		require.NoError(t, err)
		proc2, err := svc.Start(ctx, "sleep", "60")
		require.NoError(t, err)

		assert.True(t, proc1.IsRunning())
		assert.True(t, proc2.IsRunning())

		err = svc.OnShutdown(context.Background())
		assert.NoError(t, err)

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
}

func TestService_OnStartup(t *testing.T) {
	t.Run("registers process.run task", func(t *testing.T) {
		svc, c := newTestService(t)

		err := svc.OnStartup(context.Background())
		require.NoError(t, err)

		result := c.PERFORM(TaskProcessRun{
			Command: "echo",
			Args:    []string{"action-run"},
		})

		require.True(t, result.OK)
		assert.Contains(t, result.Value.(string), "action-run")
	})

	t.Run("forwards task execution options", func(t *testing.T) {
		svc, c := newTestService(t)

		err := svc.OnStartup(context.Background())
		require.NoError(t, err)

		result := c.PERFORM(TaskProcessRun{
			Command:     "sleep",
			Args:        []string{"60"},
			Timeout:     100 * time.Millisecond,
			GracePeriod: 50 * time.Millisecond,
		})

		require.False(t, result.OK)
		assert.Nil(t, result.Value)
	})

	t.Run("registers process.kill task", func(t *testing.T) {
		svc, c := newTestService(t)

		err := svc.OnStartup(context.Background())
		require.NoError(t, err)

		proc, err := svc.Start(context.Background(), "sleep", "60")
		require.NoError(t, err)
		require.True(t, proc.IsRunning())

		result := c.PERFORM(TaskProcessKill{PID: proc.Info().PID})
		require.True(t, result.OK)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed by pid")
		}

		assert.Equal(t, StatusKilled, proc.Status)
	})
}

func TestService_RunWithOptions(t *testing.T) {
	t.Run("returns output on success", func(t *testing.T) {
		svc, _ := newTestService(t)

		output, err := svc.RunWithOptions(context.Background(), RunOptions{
			Command: "echo",
			Args:    []string{"opts-test"},
		})
		require.NoError(t, err)
		assert.Contains(t, output, "opts-test")
	})

	t.Run("returns error on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		_, err := svc.RunWithOptions(context.Background(), RunOptions{
			Command: "sh",
			Args:    []string{"-c", "exit 2"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exited with code 2")
	})
}

func TestService_Running(t *testing.T) {
	t.Run("returns only running processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc1, err := svc.Start(ctx, "sleep", "60")
		require.NoError(t, err)

		doneProc, err := svc.Start(context.Background(), "echo", "done")
		require.NoError(t, err)
		<-doneProc.Done()

		running := svc.Running()
		assert.Len(t, running, 1)
		assert.Equal(t, proc1.ID, running[0].ID)

		proc2, err := svc.Start(ctx, "sleep", "60")
		require.NoError(t, err)

		running = svc.Running()
		assert.Len(t, running, 2)
		assert.Equal(t, proc1.ID, running[0].ID)
		assert.Equal(t, proc2.ID, running[1].ID)

		cancel()
		<-proc1.Done()
		<-proc2.Done()
	})
}
