package process_test

import (
	"context"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleProgram_Find() {
	program := &process.Program{Name: "echo"}
	result := program.Find()
	Println(result.OK, program.Path != "")
	// Output: true true
}

func ExampleProgram_Run() {
	program := &process.Program{Name: "echo"}
	out := exampleStringResult(program.Run(context.Background(), "hello"))
	Println(out)
	// Output: hello
}

func ExampleProgram_RunDir() {
	program := &process.Program{Name: "pwd"}
	out := exampleStringResult(program.RunDir(context.Background(), TempDir()))
	Println(out != "")
	// Output: true
}
