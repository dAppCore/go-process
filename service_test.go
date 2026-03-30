package process

import (
	"context"
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
	r := Register(c)
	require.True(t, r.OK)
	return r.Value.(*Service), c
}

func newStartedTestService(t *testing.T) (*Service, *framework.Core) {
	t.Helper()

	svc, c := newTestService(t)
	r := svc.OnStartup(context.Background())
	require.True(t, r.OK)
	return svc, c
}

func TestService_Register_Good(t *testing.T) {
	c := framework.New(framework.WithService(Register))

	svc, ok := framework.ServiceFor[*Service](c, "process")
	require.True(t, ok)
	assert.NotNil(t, svc)
}

func TestService_OnStartup_Good(t *testing.T) {
	svc, c := newTestService(t)

	r := svc.OnStartup(context.Background())
	require.True(t, r.OK)

	assert.True(t, c.Action("process.run").Exists())
	assert.True(t, c.Action("process.start").Exists())
	assert.True(t, c.Action("process.kill").Exists())
	assert.True(t, c.Action("process.list").Exists())
	assert.True(t, c.Action("process.get").Exists())
}

func TestService_HandleRun_Good(t *testing.T) {
	_, c := newStartedTestService(t)

	r := c.Action("process.run").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "echo"},
		framework.Option{Key: "args", Value: []string{"hello"}},
	))
	require.True(t, r.OK)
	assert.Contains(t, r.Value.(string), "hello")
}

func TestService_HandleRun_Bad(t *testing.T) {
	_, c := newStartedTestService(t)

	r := c.Action("process.run").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "nonexistent_command_xyz"},
	))
	assert.False(t, r.OK)
}

func TestService_HandleRun_Ugly(t *testing.T) {
	_, c := newStartedTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := c.Action("process.run").Run(ctx, framework.NewOptions(
		framework.Option{Key: "command", Value: "sleep"},
		framework.Option{Key: "args", Value: []string{"1"}},
	))
	assert.False(t, r.OK)
}

func TestService_HandleStart_Good(t *testing.T) {
	svc, c := newStartedTestService(t)

	r := c.Action("process.start").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "sleep"},
		framework.Option{Key: "args", Value: []string{"60"}},
	))
	require.True(t, r.OK)

	id := r.Value.(string)
	proc, err := svc.Get(id)
	require.NoError(t, err)
	assert.True(t, proc.IsRunning())

	kill := c.Action("process.kill").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "id", Value: id},
	))
	require.True(t, kill.OK)
	<-proc.Done()

	t.Run("respects detach=false", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		start := c.Action("process.start").Run(ctx, framework.NewOptions(
			framework.Option{Key: "command", Value: "sleep"},
			framework.Option{Key: "args", Value: []string{"60"}},
			framework.Option{Key: "detach", Value: false},
		))
		require.True(t, start.OK)

		id := start.Value.(string)
		proc, err := svc.Get(id)
		require.NoError(t, err)

		cancel()

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should honor detached=false context cancellation")
		}
	})
}

func TestService_HandleStart_Bad(t *testing.T) {
	_, c := newStartedTestService(t)

	r := c.Action("process.start").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "nonexistent_command_xyz"},
	))
	assert.False(t, r.OK)
}

func TestService_HandleKill_Good(t *testing.T) {
	svc, c := newStartedTestService(t)

	start := c.Action("process.start").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "sleep"},
		framework.Option{Key: "args", Value: []string{"60"}},
	))
	require.True(t, start.OK)

	id := start.Value.(string)
	proc, err := svc.Get(id)
	require.NoError(t, err)

	kill := c.Action("process.kill").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "id", Value: id},
	))
	require.True(t, kill.OK)

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process should have been killed")
	}
}

func TestService_HandleKill_Bad(t *testing.T) {
	_, c := newStartedTestService(t)

	r := c.Action("process.kill").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "id", Value: "missing"},
	))
	assert.False(t, r.OK)
}

func TestService_HandleList_Good(t *testing.T) {
	svc, c := newStartedTestService(t)

	startOne := c.Action("process.start").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "sleep"},
		framework.Option{Key: "args", Value: []string{"60"}},
	))
	require.True(t, startOne.OK)
	startTwo := c.Action("process.start").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "sleep"},
		framework.Option{Key: "args", Value: []string{"60"}},
	))
	require.True(t, startTwo.OK)

	r := c.Action("process.list").Run(context.Background(), framework.NewOptions())
	require.True(t, r.OK)

	ids := r.Value.([]string)
	assert.Len(t, ids, 2)

	for _, id := range ids {
		proc, err := svc.Get(id)
		require.NoError(t, err)
		_ = proc.Kill()
		<-proc.Done()
	}
}

func TestService_HandleGet_Good(t *testing.T) {
	svc, c := newStartedTestService(t)

	start := c.Action("process.start").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "command", Value: "sleep"},
		framework.Option{Key: "args", Value: []string{"60"}},
	))
	require.True(t, start.OK)

	id := start.Value.(string)
	r := c.Action("process.get").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "id", Value: id},
	))
	require.True(t, r.OK)

	info := r.Value.(ProcessInfo)
	assert.Equal(t, id, info.ID)
	assert.Equal(t, "sleep", info.Command)
	assert.True(t, info.Running)
	assert.Equal(t, StatusRunning, info.Status)
	assert.Positive(t, info.PID)

	proc, err := svc.Get(id)
	require.NoError(t, err)
	_ = proc.Kill()
	<-proc.Done()
}

func TestService_HandleGet_Bad(t *testing.T) {
	_, c := newStartedTestService(t)

	missingID := c.Action("process.get").Run(context.Background(), framework.NewOptions())
	assert.False(t, missingID.OK)

	missingProc := c.Action("process.get").Run(context.Background(), framework.NewOptions(
		framework.Option{Key: "id", Value: "missing"},
	))
	assert.False(t, missingProc.OK)
}

func TestService_Ugly_PermissionModel(t *testing.T) {
	c := framework.New()

	r := c.Process().Run(context.Background(), "echo", "blocked")
	assert.False(t, r.OK)

	c = framework.New(framework.WithService(Register))
	startup := c.ServiceStartup(context.Background(), nil)
	require.True(t, startup.OK)
	defer func() {
		shutdown := c.ServiceShutdown(context.Background())
		assert.True(t, shutdown.OK)
	}()

	r = c.Process().Run(context.Background(), "echo", "allowed")
	require.True(t, r.OK)
	assert.Contains(t, r.Value.(string), "allowed")
}

func startProc(t *testing.T, svc *Service, ctx context.Context, command string, args ...string) *Process {
	t.Helper()
	r := svc.Start(ctx, command, args...)
	require.True(t, r.OK)
	return r.Value.(*Process)
}

func TestService_Start_Good(t *testing.T) {
	t.Run("echo command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc := startProc(t, svc, context.Background(), "echo", "hello")

		assert.NotEmpty(t, proc.ID)
		assert.Positive(t, proc.PID)
		assert.Equal(t, "echo", proc.Command)
		assert.Equal(t, []string{"hello"}, proc.Args)

		<-proc.Done()

		assert.Equal(t, StatusExited, proc.Status)
		assert.Equal(t, 0, proc.ExitCode)
		assert.Contains(t, proc.Output(), "hello")
	})

	t.Run("failing command", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc := startProc(t, svc, context.Background(), "sh", "-c", "exit 42")

		<-proc.Done()

		assert.Equal(t, StatusExited, proc.Status)
		assert.Equal(t, 42, proc.ExitCode)
	})

	t.Run("non-existent command", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.Start(context.Background(), "nonexistent_command_xyz")
		assert.False(t, r.OK)
	})

	t.Run("with working directory", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "pwd",
			Dir:     "/tmp",
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		<-proc.Done()

		output := framework.Trim(proc.Output())
		assert.True(t, output == "/tmp" || output == "/private/tmp", "got: %s", output)
	})

	t.Run("context cancellation", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		proc := startProc(t, svc, ctx, "sleep", "10")

		cancel()

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}
	})

	t.Run("disable capture", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command:        "echo",
			Args:           []string{"no-capture"},
			DisableCapture: true,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)
		<-proc.Done()

		assert.Equal(t, StatusExited, proc.Status)
		assert.Equal(t, "", proc.Output(), "output should be empty when capture is disabled")
	})

	t.Run("with environment variables", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.StartWithOptions(context.Background(), RunOptions{
			Command: "sh",
			Args:    []string{"-c", "echo $MY_TEST_VAR"},
			Env:     []string{"MY_TEST_VAR=hello_env"},
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)
		<-proc.Done()

		assert.Contains(t, proc.Output(), "hello_env")
	})

	t.Run("detach survives parent context", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())

		r := svc.StartWithOptions(ctx, RunOptions{
			Command: "echo",
			Args:    []string{"detached"},
			Detach:  true,
		})
		require.True(t, r.OK)
		proc := r.Value.(*Process)

		cancel()

		select {
		case <-proc.Done():
			assert.Equal(t, StatusExited, proc.Status)
			assert.Equal(t, 0, proc.ExitCode)
		case <-time.After(2 * time.Second):
			t.Fatal("detached process should have completed")
		}
	})
}

func TestService_Run_Good(t *testing.T) {
	t.Run("returns output", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.Run(context.Background(), "echo", "hello world")
		assert.True(t, r.OK)
		assert.Contains(t, r.Value.(string), "hello world")
	})

	t.Run("returns !OK on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.Run(context.Background(), "sh", "-c", "exit 1")
		assert.False(t, r.OK)
	})
}

func TestService_Actions_Good(t *testing.T) {
	t.Run("broadcasts events", func(t *testing.T) {
		svc, c := newTestService(t)

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
		proc := startProc(t, svc, context.Background(), "echo", "test")

		<-proc.Done()

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.Len(t, started, 1)
		assert.Equal(t, "echo", started[0].Command)
		assert.Equal(t, []string{"test"}, started[0].Args)

		assert.NotEmpty(t, outputs)
		foundTest := false
		for _, o := range outputs {
			if framework.Contains(o.Line, "test") {
				foundTest = true
				break
			}
		}
		assert.True(t, foundTest, "should have output containing 'test'")

		assert.Len(t, exited, 1)
		assert.Equal(t, 0, exited[0].ExitCode)
	})

	t.Run("broadcasts killed event", func(t *testing.T) {
		svc, c := newTestService(t)

		var killed []ActionProcessKilled
		var mu sync.Mutex

		c.RegisterAction(func(cc *framework.Core, msg framework.Message) framework.Result {
			mu.Lock()
			defer mu.Unlock()
			if m, ok := msg.(ActionProcessKilled); ok {
				killed = append(killed, m)
			}
			return framework.Result{OK: true}
		})

		proc := startProc(t, svc, context.Background(), "sleep", "60")
		err := svc.Kill(proc.ID)
		require.NoError(t, err)
		<-proc.Done()

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		require.Len(t, killed, 1)
		assert.Equal(t, proc.ID, killed[0].ID)
		assert.Equal(t, "SIGKILL", killed[0].Signal)
	})
}

func TestService_List_Good(t *testing.T) {
	t.Run("tracks processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc1 := startProc(t, svc, context.Background(), "echo", "1")
		proc2 := startProc(t, svc, context.Background(), "echo", "2")

		<-proc1.Done()
		<-proc2.Done()

		list := svc.List()
		assert.Len(t, list, 2)
	})

	t.Run("get by id", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc := startProc(t, svc, context.Background(), "echo", "test")
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

func TestService_Remove_Good(t *testing.T) {
	t.Run("removes completed process", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc := startProc(t, svc, context.Background(), "echo", "test")
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

		proc := startProc(t, svc, ctx, "sleep", "10")

		err := svc.Remove(proc.ID)
		assert.Error(t, err)

		cancel()
		<-proc.Done()
	})
}

func TestService_Clear_Good(t *testing.T) {
	t.Run("clears completed processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc1 := startProc(t, svc, context.Background(), "echo", "1")
		proc2 := startProc(t, svc, context.Background(), "echo", "2")

		<-proc1.Done()
		<-proc2.Done()

		assert.Len(t, svc.List(), 2)

		svc.Clear()

		assert.Len(t, svc.List(), 0)
	})
}

func TestService_Kill_Good(t *testing.T) {
	t.Run("kills running process", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc := startProc(t, svc, ctx, "sleep", "60")

		err := svc.Kill(proc.ID)
		assert.NoError(t, err)

		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process should have been killed")
		}
	})

	t.Run("error on unknown id", func(t *testing.T) {
		svc, _ := newTestService(t)

		err := svc.Kill("nonexistent")
		assert.ErrorIs(t, err, ErrProcessNotFound)
	})
}

func TestService_Output_Good(t *testing.T) {
	t.Run("returns captured output", func(t *testing.T) {
		svc, _ := newTestService(t)

		proc := startProc(t, svc, context.Background(), "echo", "captured")
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

func TestService_OnShutdown_Good(t *testing.T) {
	t.Run("kills all running processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc1 := startProc(t, svc, ctx, "sleep", "60")
		proc2 := startProc(t, svc, ctx, "sleep", "60")

		assert.True(t, proc1.IsRunning())
		assert.True(t, proc2.IsRunning())

		r := svc.OnShutdown(context.Background())
		assert.True(t, r.OK)

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

func TestService_RunWithOptions_Good(t *testing.T) {
	t.Run("returns output on success", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.RunWithOptions(context.Background(), RunOptions{
			Command: "echo",
			Args:    []string{"opts-test"},
		})
		assert.True(t, r.OK)
		assert.Contains(t, r.Value.(string), "opts-test")
	})

	t.Run("returns !OK on failure", func(t *testing.T) {
		svc, _ := newTestService(t)

		r := svc.RunWithOptions(context.Background(), RunOptions{
			Command: "sh",
			Args:    []string{"-c", "exit 2"},
		})
		assert.False(t, r.OK)
	})
}

func TestService_Running_Good(t *testing.T) {
	t.Run("returns only running processes", func(t *testing.T) {
		svc, _ := newTestService(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc1 := startProc(t, svc, ctx, "sleep", "60")
		proc2 := startProc(t, svc, context.Background(), "echo", "done")
		<-proc2.Done()

		running := svc.Running()
		assert.Len(t, running, 1)
		assert.Equal(t, proc1.ID, running[0].ID)

		cancel()
		<-proc1.Done()
	})
}
