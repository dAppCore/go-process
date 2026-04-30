package process_test

import (
	"context"
	"syscall"
	"time"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func exampleManagedProcess(command string, args ...string) *process.ManagedProcess {
	svc := exampleService()
	return exampleProcessResult(svc.Start(context.Background(), command, args...))
}

func ExampleManagedProcess_Info() {
	proc := exampleManagedProcess("echo", "info")
	<-proc.Done()
	Println(proc.Info().Command)
	// Output: echo
}

func ExampleManagedProcess_Output() {
	proc := exampleManagedProcess("echo", "output")
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: output
}

func ExampleManagedProcess_OutputBytes() {
	proc := exampleManagedProcess("echo", "data")
	<-proc.Done()
	Println(Trim(string(proc.OutputBytes())))
	// Output: data
}

func ExampleManagedProcess_IsRunning() {
	proc := exampleManagedProcess("sleep", "1")
	defer proc.Kill()
	Println(proc.IsRunning())
	// Output: true
}

func ExampleManagedProcess_Wait() {
	proc := exampleManagedProcess("true")
	Println(proc.Wait().OK)
	// Output: true
}

func ExampleManagedProcess_Done() {
	proc := exampleManagedProcess("true")
	<-proc.Done()
	Println(proc.Info().Status)
	// Output: exited
}

func ExampleManagedProcess_Kill() {
	proc := exampleManagedProcess("sleep", "5")
	Println(proc.Kill().OK)
	// Output: true
}

func ExampleManagedProcess_Shutdown() {
	svc := exampleService()
	proc := exampleProcessResult(svc.StartWithOptions(context.Background(), process.RunOptions{
		Command:     "sleep",
		Args:        []string{"5"},
		GracePeriod: time.Millisecond,
	}))
	Println(proc.Shutdown().OK)
	// Output: true
}

func ExampleManagedProcess_Signal() {
	proc := exampleManagedProcess("sleep", "5")
	Println(proc.Signal(syscall.SIGTERM).OK)
	// Output: true
}

func ExampleManagedProcess_SendInput() {
	proc := exampleManagedProcess("cat")
	proc.SendInput("hello\n")
	proc.CloseStdin()
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: hello
}

func ExampleManagedProcess_CloseStdin() {
	proc := exampleManagedProcess("cat")
	Println(proc.CloseStdin().OK)
	// Output: true
}
