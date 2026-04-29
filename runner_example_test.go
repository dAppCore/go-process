package process_test

import (
	"context"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleNewRunner() {
	runner := process.NewRunner(exampleService())
	Println(runner != nil)
	// Output: true
}

func ExampleRunResult_Passed() {
	result := process.RunResult{Name: "unit", ExitCode: 0}
	Println(result.Passed())
	// Output: true
}

func ExampleRunAllResult_Success() {
	result := process.RunAllResult{Passed: 2, Failed: 0}
	Println(result.Success())
	// Output: true
}

func ExampleRunner_RunAll() {
	runner := process.NewRunner(exampleService())
	result, _ := runner.RunAll(context.Background(), []process.RunSpec{
		{Name: "echo", Command: "echo", Args: []string{"ok"}},
	})
	Println(result.Success())
	// Output: true
}

func ExampleRunner_RunSequential() {
	runner := process.NewRunner(exampleService())
	result, _ := runner.RunSequential(context.Background(), []process.RunSpec{
		{Name: "first", Command: "echo", Args: []string{"first"}},
		{Name: "second", Command: "echo", Args: []string{"second"}, After: []string{"first"}},
	})
	Println(result.Passed)
	// Output: 2
}

func ExampleRunner_RunParallel() {
	runner := process.NewRunner(exampleService())
	result, _ := runner.RunParallel(context.Background(), []process.RunSpec{
		{Name: "one", Command: "true"},
		{Name: "two", Command: "true"},
	})
	Println(result.Passed)
	// Output: 2
}
