package process

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	framework "dappco.re/go/core"
)

func TestGlobal_DefaultNotInitialized(t *testing.T) {
	// Reset global state for this test
	old := defaultService.Swap(nil)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	assertNil(t, Default())

	_, err := Start(context.Background(), "echo", "test")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	_, err = Run(context.Background(), "echo", "test")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	_, err = Get("proc-1")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	_, err = Output("proc-1")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = Input("proc-1", "test")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = CloseStdin("proc-1")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	assertNil(t, List())
	assertNil(t, Running())

	err = Remove("proc-1")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	// Clear is a no-op without a default service.
	Clear()

	err = Kill("proc-1")
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = KillPID(1234)
	assertErrorIs(t, err, ErrServiceNotInitialized)

	_, err = StartWithOptions(context.Background(), RunOptions{Command: "echo"})
	assertErrorIs(t, err, ErrServiceNotInitialized)

	_, err = RunWithOptions(context.Background(), RunOptions{Command: "echo"})
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func newGlobalTestService(t *testing.T) *Service {
	t.Helper()
	c := framework.New()
	factory := NewService(Options{})
	raw, err := factory(c)
	requireNoError(t, err)
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
		requireNoError(t, err)
		assertEqual(t, svc, Default())
	})

	t.Run("errors on nil", func(t *testing.T) {
		err := SetDefault(nil)
		assertError(t, err)
	})
}

func TestGlobal_Register(t *testing.T) {
	c := framework.New()

	result := Register(c)
	requireTrue(t, result.OK)

	svc, ok := result.Value.(*Service)
	requireTrue(t, ok)
	requireNotNil(t, svc)
	assertNotNil(t, svc.ServiceRuntime)
	assertEqual(t, DefaultBufferSize, svc.bufSize)
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
	requireNoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := Default()
			assertNotNil(t, s)
			assertEqual(t, svc, s)
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
	assertNotNil(t, final)

	found := false
	for _, svc := range services {
		if svc == final {
			found = true
			break
		}
	}
	assertTrue(t, found, "Default should be one of the set services")
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
	requireNoError(t, err)

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

	assertLen(t, processes, 20)

	var wg2 sync.WaitGroup
	for _, p := range processes {
		wg2.Add(1)
		go func(id string) {
			defer wg2.Done()
			got, err := Get(id)
			assertNoError(t, err)
			assertNotNil(t, got)
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
	requireNoError(t, err)

	<-proc.Done()

	assertEqual(t, 0, proc.ExitCode)
	assertContains(t, proc.Output(), "with options")
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
	requireNoError(t, err)
	assertContains(t, output, "run options")
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
	requireNoError(t, err)
	<-proc.Done()

	output, err := Output(proc.ID)
	requireNoError(t, err)
	assertContains(t, output, "global-output")
}

func TestGlobal_InputAndCloseStdin(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	proc, err := Start(context.Background(), "cat")
	requireNoError(t, err)

	err = Input(proc.ID, "global-input\n")
	requireNoError(t, err)

	err = CloseStdin(proc.ID)
	requireNoError(t, err)

	<-proc.Done()

	assertContains(t, proc.Output(), "global-input")
}

func TestGlobal_Wait(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	proc, err := Start(context.Background(), "echo", "global-wait")
	requireNoError(t, err)

	info, err := Wait(proc.ID)
	requireNoError(t, err)
	assertEqual(t, proc.ID, info.ID)
	assertEqual(t, StatusExited, info.Status)
	assertEqual(t, 0, info.ExitCode)
}

func TestGlobal_Signal(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

	proc, err := Start(context.Background(), "sleep", "60")
	requireNoError(t, err)

	err = Signal(proc.ID, syscall.SIGTERM)
	requireNoError(t, err)

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process should have been signalled through the global helper")
	}
}

func TestGlobal_SignalPID(t *testing.T) {
	svc, _ := newTestService(t)

	old := defaultService.Swap(svc)
	defer func() {
		if old != nil {
			defaultService.Store(old)
		}
	}()

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

	err := SignalPID(cmd.Process.Pid, syscall.SIGTERM)
	requireNoError(t, err)

	select {
	case err := <-waitCh:
		requireError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("unmanaged process should have been signalled through the global helper")
	}
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
	requireNoError(t, err)

	running := Running()
	assertLen(t, running, 1)
	assertEqual(t, proc.ID, running[0].ID)

	cancel()
	<-proc.Done()

	running = Running()
	assertLen(t, running, 0)
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
	requireNoError(t, err)
	<-proc.Done()

	err = Remove(proc.ID)
	requireNoError(t, err)

	_, err = Get(proc.ID)
	requireErrorIs(t, err, ErrProcessNotFound)

	proc2, err := Start(context.Background(), "echo", "clear-me")
	requireNoError(t, err)
	<-proc2.Done()

	Clear()

	_, err = Get(proc2.ID)
	requireErrorIs(t, err, ErrProcessNotFound)
}
