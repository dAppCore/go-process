package process_test

import (
	"context"
	"syscall"
	"time"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func exampleService() *process.Service {
	result := process.NewService(process.Options{BufferSize: 4096})(New())
	if !result.OK {
		return nil
	}
	return result.Value.(*process.Service)
}

func exampleProcessResult(result Result) *process.Process {
	if !result.OK {
		return nil
	}
	return result.Value.(*process.Process)
}

func exampleStringResult(result Result) string {
	if !result.OK {
		return ""
	}
	return result.Value.(string)
}

func exampleInfoResult(result Result) process.Info {
	if !result.OK {
		return process.Info{}
	}
	return result.Value.(process.Info)
}

func exampleRunAllResult(result Result) *process.RunAllResult {
	if !result.OK {
		return nil
	}
	return result.Value.(*process.RunAllResult)
}

func ExampleNewService() {
	svc := exampleService()
	Println(svc != nil)
	// Output: true
}

func ExampleService_OnStartup() {
	svc := exampleService()
	Println(svc.OnStartup(context.Background()).OK)
	// Output: true
}

func ExampleService_OnShutdown() {
	svc := exampleService()
	Println(svc.OnShutdown(context.Background()).OK)
	// Output: true
}

func ExampleService_Start() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "echo", "started"))
	<-proc.Done()
	Println(proc.Info().Status)
	// Output: exited
}

func ExampleService_StartWithOptions() {
	svc := exampleService()
	proc := exampleProcessResult(svc.StartWithOptions(context.Background(), process.RunOptions{Command: "echo", Args: []string{"options"}}))
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: options
}

func ExampleService_Get() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "echo", "get"))
	<-proc.Done()
	got := exampleProcessResult(svc.Get(proc.ID))
	Println(got.Command)
	// Output: echo
}

func ExampleService_List() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "echo", "list"))
	<-proc.Done()
	Println(len(svc.List()) > 0)
	// Output: true
}

func ExampleService_Running() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "sleep", "1"))
	defer svc.Kill(proc.ID)
	Println(len(svc.Running()) > 0)
	// Output: true
}

func ExampleService_Kill() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "sleep", "5"))
	Println(svc.Kill(proc.ID).OK)
	// Output: true
}

func ExampleService_KillPID() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "sleep", "5"))
	Println(svc.KillPID(proc.Info().PID).OK)
	// Output: true
}

func ExampleService_Signal() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "sleep", "5"))
	Println(svc.Signal(proc.ID, syscall.SIGTERM).OK)
	// Output: true
}

func ExampleService_SignalPID() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "sleep", "5"))
	Println(svc.SignalPID(proc.Info().PID, syscall.SIGTERM).OK)
	// Output: true
}

func ExampleService_Remove() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "true"))
	<-proc.Done()
	Println(svc.Remove(proc.ID).OK)
	// Output: true
}

func ExampleService_Clear() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "true"))
	<-proc.Done()
	svc.Clear()
	Println(len(svc.List()))
	// Output: 0
}

func ExampleService_Output() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "echo", "captured"))
	<-proc.Done()
	out := exampleStringResult(svc.Output(proc.ID))
	Println(Trim(out))
	// Output: captured
}

func ExampleService_Input() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "cat"))
	svc.Input(proc.ID, "stdin\n")
	svc.CloseStdin(proc.ID)
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: stdin
}

func ExampleService_CloseStdin() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "cat"))
	Println(svc.CloseStdin(proc.ID).OK)
	// Output: true
}

func ExampleService_Wait() {
	svc := exampleService()
	proc := exampleProcessResult(svc.Start(context.Background(), "true"))
	info := exampleInfoResult(svc.Wait(proc.ID))
	Println(info.Status)
	// Output: exited
}

func ExampleService_Run() {
	svc := exampleService()
	out := exampleStringResult(svc.Run(context.Background(), "echo", "run"))
	Println(out)
	// Output: run
}

func ExampleService_RunWithOptions() {
	svc := exampleService()
	out := exampleStringResult(svc.RunWithOptions(context.Background(), process.RunOptions{
		Command: "echo",
		Args:    []string{"configured"},
		Timeout: time.Second,
	}))
	Println(out)
	// Output: configured
}
