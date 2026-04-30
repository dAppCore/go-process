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

	err := resultErr(Start(context.Background(), "echo", "test"))
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = resultErr(Run(context.Background(), "echo", "test"))
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = resultErr(Get("proc-1"))
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = resultErr(Output("proc-1"))
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

	err = resultErr(StartWithOptions(context.Background(), RunOptions{Command: "echo"}))
	assertErrorIs(t, err, ErrServiceNotInitialized)

	err = resultErr(RunWithOptions(context.Background(), RunOptions{Command: "echo"}))
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func newGlobalTestService(t *testing.T) *Service {
	t.Helper()
	c := framework.New()
	factory := NewService(Options{})
	result := factory(c)
	requireNoError(t, result)
	return result.Value.(*Service)
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
			result := Start(context.Background(), "echo", "concurrent")
			if result.OK {
				proc := result.Value.(*Process)
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
			got, err := resultValue[*Process](Get(id))
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

	proc := requireResultValue[*Process](t, StartWithOptions(context.Background(), RunOptions{
		Command: "echo",
		Args:    []string{"with", "options"},
	}))

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

	output := requireResultValue[string](t, RunWithOptions(context.Background(), RunOptions{
		Command: "echo",
		Args:    []string{"run", "options"},
	}))
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

	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "global-output"))
	<-proc.Done()

	output := requireResultValue[string](t, Output(proc.ID))
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

	proc := requireResultValue[*Process](t, Start(context.Background(), "cat"))

	err := Input(proc.ID, "global-input\n")
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

	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "global-wait"))

	info := requireResultValue[Info](t, Wait(proc.ID))
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

	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "60"))

	err := Signal(proc.ID, syscall.SIGTERM)
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

	proc := requireResultValue[*Process](t, Start(ctx, "sleep", "60"))

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

	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "remove-me"))
	<-proc.Done()

	removeResult := Remove(proc.ID)
	requireNoError(t, removeResult)

	err := resultErr(Get(proc.ID))
	requireErrorIs(t, err, ErrProcessNotFound)

	proc2 := requireResultValue[*Process](t, Start(context.Background(), "echo", "clear-me"))
	<-proc2.Done()

	Clear()

	err = resultErr(Get(proc2.ID))
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
	defaultResult = framework.Result{}
	err := Init(framework.New())
	requireNoError(t, err)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Init_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	defaultOnce = sync.Once{}
	defaultResult = framework.Result{}
	err := Init(nil)
	requireNoError(t, err)
	assertNotNil(t, Default())
}

func TestProcessGlobal_Init_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	defaultOnce = sync.Once{}
	defaultResult = framework.Result{}
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "hello"))
	<-proc.Done()
	assertEqual(t, StatusExited, proc.Status)
}

func TestProcessGlobal_Start_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	proc, err := resultValue[*Process](Start(context.Background(), "echo"))
	assertNil(t, proc)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Start_Ugly(t *testing.T) {
	withGlobalDefault(t)
	proc, err := resultValue[*Process](Start(context.Background(), ""))
	assertNil(t, proc)
	assertError(t, err)
}

func TestProcessGlobal_Run_Good(t *testing.T) {
	withGlobalDefault(t)
	out := requireResultValue[string](t, Run(context.Background(), "echo", "hello"))
	assertContains(t, out, "hello")
}

func TestProcessGlobal_Run_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	out, err := resultValue[string](Run(context.Background(), "echo"))
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Run_Ugly(t *testing.T) {
	withGlobalDefault(t)
	out, err := resultValue[string](Run(context.Background(), "false"))
	assertEqual(t, "", out)
	assertError(t, err)
}

func TestProcessGlobal_Get_Good(t *testing.T) {
	withGlobalDefault(t)
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "hello"))
	got := requireResultValue[*Process](t, Get(proc.ID))
	assertEqual(t, proc, got)
}

func TestProcessGlobal_Get_Bad(t *testing.T) {
	withGlobalDefault(t)
	got, err := resultValue[*Process](Get("missing"))
	assertNil(t, got)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestProcessGlobal_Get_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	got, err := resultValue[*Process](Get(""))
	assertNil(t, got)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Output_Good(t *testing.T) {
	withGlobalDefault(t)
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "hello"))
	<-proc.Done()
	out := requireResultValue[string](t, Output(proc.ID))
	assertContains(t, out, "hello")
}

func TestProcessGlobal_Output_Bad(t *testing.T) {
	withGlobalDefault(t)
	out, err := resultValue[string](Output("missing"))
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestProcessGlobal_Output_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	out, err := resultValue[string](Output(""))
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_Input_Good(t *testing.T) {
	withGlobalDefault(t)
	proc := requireResultValue[*Process](t, Start(context.Background(), "cat"))
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "cat"))
	err := CloseStdin(proc.ID)
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "hello"))
	info := requireResultValue[Info](t, Wait(proc.ID))
	assertEqual(t, proc.ID, info.ID)
}

func TestProcessGlobal_Wait_Bad(t *testing.T) {
	withGlobalDefault(t)
	info, err := resultValue[Info](Wait("missing"))
	assertEqual(t, Info{}, info)
	assertErrorIs(t, err, ErrProcessNotFound)
}

func TestProcessGlobal_Wait_Ugly(t *testing.T) {
	withoutGlobalDefault(t)
	info, err := resultValue[Info](Wait(""))
	assertEqual(t, Info{}, info)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_List_Good(t *testing.T) {
	withGlobalDefault(t)
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "hello"))
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "5"))
	err := Kill(proc.ID)
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "5"))
	pid := proc.Info().PID
	err := KillPID(pid)
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "5"))
	err := Signal(proc.ID, syscall.SIGTERM)
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "5"))
	pid := proc.Info().PID
	err := SignalPID(pid, syscall.SIGTERM)
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
	proc := requireResultValue[*Process](t, StartWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}}))
	<-proc.Done()
	assertEqual(t, StatusExited, proc.Status)
}

func TestProcessGlobal_StartWithOptions_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	proc, err := resultValue[*Process](StartWithOptions(context.Background(), RunOptions{Command: "echo"}))
	assertNil(t, proc)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_StartWithOptions_Ugly(t *testing.T) {
	withGlobalDefault(t)
	proc, err := resultValue[*Process](StartWithOptions(context.Background(), RunOptions{}))
	assertNil(t, proc)
	assertError(t, err)
}

func TestProcessGlobal_RunWithOptions_Good(t *testing.T) {
	withGlobalDefault(t)
	out := requireResultValue[string](t, RunWithOptions(context.Background(), RunOptions{Command: "echo", Args: []string{"hello"}}))
	assertContains(t, out, "hello")
}

func TestProcessGlobal_RunWithOptions_Bad(t *testing.T) {
	withoutGlobalDefault(t)
	out, err := resultValue[string](RunWithOptions(context.Background(), RunOptions{Command: "echo"}))
	assertEqual(t, "", out)
	assertErrorIs(t, err, ErrServiceNotInitialized)
}

func TestProcessGlobal_RunWithOptions_Ugly(t *testing.T) {
	withGlobalDefault(t)
	out, err := resultValue[string](RunWithOptions(context.Background(), RunOptions{Command: "false"}))
	assertEqual(t, "", out)
	assertError(t, err)
}

func TestProcessGlobal_Running_Good(t *testing.T) {
	withGlobalDefault(t)
	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "5"))
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "remove"))
	<-proc.Done()
	err := Remove(proc.ID)
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "echo", "clear"))
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
	proc := requireResultValue[*Process](t, Start(context.Background(), "sleep", "5"))
	Clear()
	assertLen(t, Running(), 1)
	requireNoError(t, Kill(proc.ID))
}
