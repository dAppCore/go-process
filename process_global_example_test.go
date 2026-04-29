package process_test

import (
	"context"
	"syscall"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func useExampleDefault() {
	process.SetDefault(exampleService())
}

func ExampleDefault() {
	useExampleDefault()
	Println(process.Default() != nil)
	// Output: true
}

func ExampleSetDefault() {
	Println(process.SetDefault(exampleService()).OK)
	// Output: true
}

func ExampleInit() {
	Println(process.Init(New()).OK)
	// Output: true
}

func ExampleRegister() {
	result := process.Register(New())
	Println(result.OK)
	// Output: true
}

func ExampleStart() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "echo", "start")
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: start
}

func ExampleRun() {
	useExampleDefault()
	out, _ := process.Run(context.Background(), "echo", "run")
	Println(out)
	// Output: run
}

func ExampleGet() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "true")
	<-proc.Done()
	got, _ := process.Get(proc.ID)
	Println(got.ID == proc.ID)
	// Output: true
}

func ExampleOutput() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "echo", "output")
	<-proc.Done()
	out, _ := process.Output(proc.ID)
	Println(Trim(out))
	// Output: output
}

func ExampleInput() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "cat")
	process.Input(proc.ID, "input\n")
	process.CloseStdin(proc.ID)
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: input
}

func ExampleCloseStdin() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "cat")
	Println(process.CloseStdin(proc.ID).OK)
	// Output: true
}

func ExampleWait() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "true")
	info, _ := process.Wait(proc.ID)
	Println(info.Status)
	// Output: exited
}

func ExampleList() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "true")
	<-proc.Done()
	Println(len(process.List()) > 0)
	// Output: true
}

func ExampleKill() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "sleep", "5")
	Println(process.Kill(proc.ID).OK)
	// Output: true
}

func ExampleKillPID() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "sleep", "5")
	Println(process.KillPID(proc.Info().PID).OK)
	// Output: true
}

func ExampleSignal() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "sleep", "5")
	Println(process.Signal(proc.ID, syscall.SIGTERM).OK)
	// Output: true
}

func ExampleSignalPID() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "sleep", "5")
	Println(process.SignalPID(proc.Info().PID, syscall.SIGTERM).OK)
	// Output: true
}

func ExampleStartWithOptions() {
	useExampleDefault()
	proc, _ := process.StartWithOptions(context.Background(), process.RunOptions{Command: "echo", Args: []string{"options"}})
	<-proc.Done()
	Println(Trim(proc.Output()))
	// Output: options
}

func ExampleRunWithOptions() {
	useExampleDefault()
	out, _ := process.RunWithOptions(context.Background(), process.RunOptions{Command: "echo", Args: []string{"configured"}})
	Println(out)
	// Output: configured
}

func ExampleRunning() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "sleep", "1")
	defer process.Kill(proc.ID)
	Println(len(process.Running()) > 0)
	// Output: true
}

func ExampleRemove() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "true")
	<-proc.Done()
	Println(process.Remove(proc.ID).OK)
	// Output: true
}

func ExampleClear() {
	useExampleDefault()
	proc, _ := process.Start(context.Background(), "true")
	<-proc.Done()
	process.Clear()
	Println(len(process.List()))
	// Output: 0
}
