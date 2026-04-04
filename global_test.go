package process

import (
	"context"
	"sync"
	"testing"

	framework "dappco.re/go/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobal_DefaultNotInitialized(t *testing.T) {
	// Reset global state for this test
	old := defaultService.Swap(nil)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	assert.Nil(t, Default())

	_, err := Start(context.Background(), "echo", "test")
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	_, err = Run(context.Background(), "echo", "test")
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	_, err = Get("proc-1")
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	_, err = Output("proc-1")
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	assert.Nil(t, List())
	assert.Nil(t, Running())

	err = Remove("proc-1")
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	// Clear is a no-op without a default service.
	Clear()

	err = Kill("proc-1")
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	err = KillPID(1234)
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	_, err = StartWithOptions(context.Background(), RunOptions{Command: "echo"})
	assert.ErrorIs(t, err, ErrServiceNotInitialized)

	_, err = RunWithOptions(context.Background(), RunOptions{Command: "echo"})
	assert.ErrorIs(t, err, ErrServiceNotInitialized)
}

func newGlobalTestService(t *testing.T) *Service {
	t.Helper()
	c := framework.New()
	factory := NewService(Options{})
	raw, err := factory(c)
	require.NoError(t, err)
	return raw.(*Service)
}

func TestGlobal_SetDefault(t *testing.T) {
	t.Run("sets and retrieves service", func(t *testing.T) {
		old := defaultService.Swap(nil)
		defer func() {
			if old != nil {
				defaultService.Store(old)
			}
		}()

		svc := newGlobalTestService(t)

		err := SetDefault(svc)
		require.NoError(t, err)
		assert.Equal(t, svc, Default())
	})

	t.Run("errors on nil", func(t *testing.T) {
		err := SetDefault(nil)
		assert.Error(t, err)
	})
}

func TestGlobal_ConcurrentDefault(t *testing.T) {
	old := defaultService.Swap(nil)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	svc := newGlobalTestService(t)

	err := SetDefault(svc)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := Default()
			assert.NotNil(t, s)
			assert.Equal(t, svc, s)
		}()
	}
	wg.Wait()
}

func TestGlobal_ConcurrentSetDefault(t *testing.T) {
	old := defaultService.Swap(nil)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	var services []*Service
	for i := 0; i < 10; i++ {
		svc := newGlobalTestService(t)
		services = append(services, svc)
	}

	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Add(1)
		go func(s *Service) {
			defer wg.Done()
			_ = SetDefault(s)
		}(svc)
	}
	wg.Wait()

	final := Default()
	assert.NotNil(t, final)

	found := false
	for _, svc := range services {
		if svc == final {
			found = true
			break
		}
	}
	assert.True(t, found, "Default should be one of the set services")
}

func TestGlobal_ConcurrentOperations(t *testing.T) {
	old := defaultService.Swap(nil)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	svc := newGlobalTestService(t)

	err := SetDefault(svc)
	require.NoError(t, err)

	var wg sync.WaitGroup
	var processes []*Process
	var procMu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proc, err := Start(context.Background(), "echo", "concurrent")
			if err == nil {
				procMu.Lock()
				processes = append(processes, proc)
				procMu.Unlock()
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = List()
			_ = Running()
		}()
	}

	wg.Wait()

	procMu.Lock()
	for _, p := range processes {
		<-p.Done()
	}
	procMu.Unlock()

	assert.Len(t, processes, 20)

	var wg2 sync.WaitGroup
	for _, p := range processes {
		wg2.Add(1)
		go func(id string) {
			defer wg2.Done()
			got, err := Get(id)
			assert.NoError(t, err)
			assert.NotNil(t, got)
		}(p.ID)
	}
	wg2.Wait()
}

func TestGlobal_StartWithOptions(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	proc, err := StartWithOptions(context.Background(), RunOptions{
		Command: "echo",
		Args:    []string{"with", "options"},
	})
	require.NoError(t, err)

	<-proc.Done()

	assert.Equal(t, 0, proc.ExitCode)
	assert.Contains(t, proc.Output(), "with options")
}

func TestGlobal_RunWithOptions(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	output, err := RunWithOptions(context.Background(), RunOptions{
		Command: "echo",
		Args:    []string{"run", "options"},
	})
	require.NoError(t, err)
	assert.Contains(t, output, "run options")
}

func TestGlobal_Output(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	proc, err := Start(context.Background(), "echo", "global-output")
	require.NoError(t, err)
	<-proc.Done()

	output, err := Output(proc.ID)
	require.NoError(t, err)
	assert.Contains(t, output, "global-output")
}

func TestGlobal_Running(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proc, err := Start(ctx, "sleep", "60")
	require.NoError(t, err)

	running := Running()
	assert.Len(t, running, 1)
	assert.Equal(t, proc.ID, running[0].ID)

	cancel()
	<-proc.Done()

	running = Running()
	assert.Len(t, running, 0)
}

func TestGlobal_RemoveAndClear(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	proc, err := Start(context.Background(), "echo", "remove-me")
	require.NoError(t, err)
	<-proc.Done()

	err = Remove(proc.ID)
	require.NoError(t, err)

	_, err = Get(proc.ID)
	require.ErrorIs(t, err, ErrProcessNotFound)

	proc2, err := Start(context.Background(), "echo", "clear-me")
	require.NoError(t, err)
	<-proc2.Done()

	Clear()

	_, err = Get(proc2.ID)
	require.ErrorIs(t, err, ErrProcessNotFound)
}
