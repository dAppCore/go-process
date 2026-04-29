package process

import (
	"context"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"syscall"
	"testing"
	"time"

	framework "dappco.re/go"
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

	cmd := commandContext(context.Background(), "sleep", "60")
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

func withGlobalDefault(t *testing.T) *Service {
	t.Helper()
	old := defaultService.Swap(nil)
	t.Cleanup(func() {
		if old != nil {
			defaultService.Store(old)
		} else {
			defaultService.Store(nil)
		}
	})
	svc := newGlobalTestService(t)
	requireNoError(t, SetDefault(svc))
	return svc
}

func withoutGlobalDefault(t *testing.T) {
	t.Helper()
	old := defaultService.Swap(nil)
	t.Cleanup(func() {
		if old != nil {
			defaultService.Store(old)
		} else {
			defaultService.Store(nil)
		}
	})
}

func TestProcessGlobal_Default_Good(t *testing.T) {
	svc := withGlobalDefault(t)
	got := Default()
	assertEqual(t, svc, got)
	assertNotNil(t, got)
}

func TestProcessGlobal_Default_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	got := Default()
	assertNil(t, got)
	assertEqual(t, (*Service)(nil), got)
}

func TestProcessGlobal_Default_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	first := Default()
	second := Default()
	assertNil(t, first)
	assertNil(t, second)
}

func TestProcessGlobal_SetDefault_Good(t *testing.T) {
	withoutGlobalDefault(t)
	svc := newGlobalTestService(t)
	err := SetDefault(svc)
	requireNoError(t, err)
	assertEqual(t, svc, Default())
}

func TestProcessGlobal_SetDefault_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	err := SetDefault(nil)
	assertErrorIs(t, err, ErrSetDefaultNil)
	assertNil(t, Default())
}

func TestProcessGlobal_SetDefault_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	first := newGlobalTestService(t)
	second := newGlobalTestService(t)
	requireNoError(t, SetDefault(first))
	requireNoError(t, SetDefault(second))
	assertEqual(t, second, Default())
}

func TestProcessGlobal_Init_Good(t *testing.T) {
	withoutGlobalDefault(t)
	defaultOnce = sync.Once{}
	defaultErr = nil
	err := Init(framework.New())
	requireNoError(t, err)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Init_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	defaultOnce = sync.Once{}
	defaultErr = nil
	err := Init(nil)
	requireNoError(t, err)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Init_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	defaultOnce = sync.Once{}
	defaultErr = nil
	requireNoError(t, Init(framework.New()))
	first := Default()
	requireNoError(t, Init(framework.New()))
	assertEqual(t, first, Default())
}

func TestProcessGlobal_Register_Good(t *testing.T) {
	c := framework.New()
	result := Register(c)
	requireTrue(t, result.OK)
	assertNotNil(t, result.Value)
}

func TestProcessGlobal_Register_Bad(t *testing.T) {
	result := Register(nil)
	requireTrue(t, result.OK)
	svc, ok := result.Value.(*Service)
	requireTrue(t, ok)
	assertNil(t, svc.coreApp())
}

func TestProcessGlobal_Register_Ugly(t *testing.T) {
	c := framework.New()
	first := Register(c)
	second := Register(c)
	requireTrue(t, first.OK)
	requireTrue(t, second.OK)
	assertFalse(t, first.Value == second.Value)
}

func TestProcessGlobal_Start_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	<-proc.Done()
	assertEqual(t, StatusExited, proc.Status)
}

func TestProcessGlobal_Start_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	proc, err := Start(context.Background(), "echo")
	assertNil(t, proc)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Start_Ugly(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "")
	assertNil(t, proc)
	assertError(t, err)
}

func TestProcessGlobal_Run_Good(t *testing.T) {
	withGlobalDefault(t)
	out, err := Run(context.Background(), "echo", "hello")
	requireNoError(t, err)
	assertContains(t, out, "hello")
}

func TestProcessGlobal_Run_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	out, err := Run(context.Background(), "echo")
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Run_Ugly(t *testing.T) {
	withGlobalDefault(t)
	out, err := Run(context.Background(), "false")
	assertEqual(t, "", out)
	assertError(t, err)
}

func TestProcessGlobal_Get_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	got, err := Get(proc.ID)
	requireNoError(t, err)
	assertEqual(t, proc, got)
}

func TestProcessGlobal_Get_Bad(t *testing.T) {
	withGlobalDefault(t)
	got, err := Get("missing")
	assertNil(t, got)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestProcessGlobal_Get_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	got, err := Get("")
	assertNil(t, got)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Output_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	<-proc.Done()
	out, err := Output(proc.ID)
	requireNoError(t, err)
	assertContains(t, out, "hello")
}

func TestProcessGlobal_Output_Bad(t *testing.T) {
	withGlobalDefault(t)
	out, err := Output("missing")
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestProcessGlobal_Output_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	out, err := Output("")
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Input_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "cat")
	requireNoError(t, err)
	requireNoError(t, Input(proc.ID, "hello\n"))
	requireNoError(t, CloseStdin(proc.ID))
	<-proc.Done()
	assertContains(t, proc.Output(), "hello")
}

func TestProcessGlobal_Input_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := Input("missing", "hello")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Input_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := Input("", "")
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_CloseStdin_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "cat")
	requireNoError(t, err)
	err = CloseStdin(proc.ID)
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcessGlobal_CloseStdin_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := CloseStdin("missing")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertNotNil(t, Default())
}

func TestProcessGlobal_CloseStdin_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := CloseStdin("")
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_Wait_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	info, err := Wait(proc.ID)
	requireNoError(t, err)
	assertEqual(t, proc.ID, info.ID)
}

func TestProcessGlobal_Wait_Bad(t *testing.T) {
	withGlobalDefault(t)
	info, err := Wait("missing")
	assertEqual(t, Info{}, info)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestProcessGlobal_Wait_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	info, err := Wait("")
	assertEqual(t, Info{}, info)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_List_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "hello")
	requireNoError(t, err)
	<-proc.Done()
	procs := List()
	assertLen(t, procs, 1)
}

func TestProcessGlobal_List_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	procs := List()
	assertNil(t, procs)
	assertEqual(t, 0, len(procs))
}

func TestProcessGlobal_List_Ugly(t *testing.T) {
	withGlobalDefault(t)
	procs := List()
	assertEmpty(t, procs)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Kill_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = Kill(proc.ID)
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcessGlobal_Kill_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := Kill("missing")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Kill_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := Kill("")
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_KillPID_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	pid := proc.Info().PID
	err = KillPID(pid)
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcessGlobal_KillPID_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := KillPID(0)
	assertError(t, err)
	assertNotNil(t, Default())
}

func TestProcessGlobal_KillPID_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := KillPID(1)
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_Signal_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	err = Signal(proc.ID, syscall.SIGTERM)
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcessGlobal_Signal_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := Signal("missing", syscall.SIGTERM)
	assertErrorIs(t, err, ErrProcessNotFound)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Signal_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := Signal("", syscall.SIGTERM)
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_SignalPID_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	pid := proc.Info().PID
	err = SignalPID(pid, syscall.SIGTERM)
	requireNoError(t, err)
	<-proc.Done()
}

func TestProcessGlobal_SignalPID_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := SignalPID(0, syscall.SIGTERM)
	assertError(t, err)
	assertNotNil(t, Default())
}

func TestProcessGlobal_SignalPID_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := SignalPID(1, syscall.SIGTERM)
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_StartWithOptions_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := StartWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}})
	requireNoError(t, err)
	<-proc.Done()
	assertEqual(t, StatusExited, proc.Status)
}

func TestProcessGlobal_StartWithOptions_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	proc, err := StartWithOptions(context.Background(), RunOptions{Command: "echo"})
	assertNil(t, proc)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_StartWithOptions_Ugly(t *testing.T) {
	withGlobalDefault(t)
	proc, err := StartWithOptions(context.Background(), RunOptions{})
	assertNil(t, proc)
	assertError(t, err)
}

func TestProcessGlobal_RunWithOptions_Good(t *testing.T) {
	withGlobalDefault(t)
	out, err := RunWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}})
	requireNoError(t, err)
	assertContains(t, out, "hello")
}

func TestProcessGlobal_RunWithOptions_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	out, err := RunWithOptions(context.Background(), RunOptions{Command: "echo"})
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_RunWithOptions_Ugly(t *testing.T) {
	withGlobalDefault(t)
	out, err := RunWithOptions(context.Background(), RunOptions{Command: "false"})
	assertEqual(t, "", out)
	assertError(t, err)
}

func TestProcessGlobal_Running_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	running := Running()
	requireLen(t, running, 1)
	assertEqual(t, proc.ID, running[0].ID)
	requireNoError(t, Kill(proc.ID))
}

func TestProcessGlobal_Running_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	running := Running()
	assertNil(t, running)
	assertEqual(t, 0, len(running))
}

func TestProcessGlobal_Running_Ugly(t *testing.T) {
	withGlobalDefault(t)
	running := Running()
	assertEmpty(t, running)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Remove_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "remove")
	requireNoError(t, err)
	<-proc.Done()
	err = Remove(proc.ID)
	requireNoError(t, err)
}

func TestProcessGlobal_Remove_Bad(t *testing.T) {
	withGlobalDefault(t)
	err := Remove("missing")
	assertErrorIs(t, err, ErrProcessNotFound)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Remove_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	err := Remove("")
	assertErrorIs(t, err, ErrServiceNotInitialized)
	assertNil(t, Default())
}

func TestProcessGlobal_Clear_Good(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "echo", "clear")
	requireNoError(t, err)
	<-proc.Done()
	Clear()
	assertEmpty(t, List())
}

func TestProcessGlobal_Clear_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	Clear()
	got := Default()
	assertNil(t, got)
}

func TestProcessGlobal_Clear_Ugly(t *testing.T) {
	withGlobalDefault(t)
	proc, err := Start(context.Background(), "sleep", "5")
	requireNoError(t, err)
	Clear()
	assertLen(t, Running(), 1)
	requireNoError(t, Kill(proc.ID))
}
