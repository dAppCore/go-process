package command_test

import (
	"context"

	. "dappco.re/go"
	exec "dappco.re/go/process/exec"
)

func ExampleCommand() {
	cmd := exec.Command(context.Background(), "echo", "hello")
	Println(cmd != nil)
	// Output: true
}

func ExampleCmd_WithDir() {
	result := exec.Command(context.Background(), "pwd").WithDir(TempDir()).Output()
	Println(result.OK && Trim(string(result.Value.([]byte))) != "")
	// Output: true
}

func ExampleCmd_WithEnv() {
	result := exec.Command(context.Background(), "sh", "-c", "printf %s \"$EXAMPLE_ENV\"").WithEnv([]string{"EXAMPLE_ENV=ok"}).Output()
	Println(string(result.Value.([]byte)))
	// Output: ok
}

func ExampleCmd_WithStdin() {
	result := exec.Command(context.Background(), "cat").WithStdin(NewReader("input")).Output()
	Println(string(result.Value.([]byte)))
	// Output: input
}

func ExampleCmd_WithStdout() {
	stdout := NewBuilder()
	exec.Command(context.Background(), "echo", "captured").WithStdout(stdout).Run()
	Println(Trim(stdout.String()))
	// Output: captured
}

func ExampleCmd_WithStderr() {
	stderr := NewBuilder()
	exec.Command(context.Background(), "sh", "-c", "echo warn >&2").WithStderr(stderr).Run()
	Println(Trim(stderr.String()))
	// Output: warn
}

func ExampleCmd_WithLogger() {
	result := exec.Command(context.Background(), "true").WithLogger(exec.NopLogger{}).Run()
	Println(result.OK)
	// Output: true
}

func ExampleCmd_WithBackground() {
	result := exec.Command(context.Background(), "true").WithBackground(true).Run()
	Println(result.OK)
	// Output: true
}

func ExampleCmd_Start() {
	result := exec.Command(context.Background(), "true").WithBackground(true).Start()
	Println(result.OK)
	// Output: true
}

func ExampleCmd_Run() {
	result := exec.Command(context.Background(), "true").Run()
	Println(result.OK)
	// Output: true
}

func ExampleCmd_Output() {
	result := exec.Command(context.Background(), "echo", "hello").Output()
	Println(Trim(string(result.Value.([]byte))))
	// Output: hello
}

func ExampleCmd_CombinedOutput() {
	result := exec.Command(context.Background(), "sh", "-c", "echo out; echo err >&2").CombinedOutput()
	out := string(result.Value.([]byte))
	Println(Contains(out, "out"), Contains(out, "err"))
	// Output: true true
}

func ExampleRunQuiet() {
	result := exec.RunQuiet(context.Background(), "true")
	Println(result.OK)
	// Output: true
}
