package command_test

import (
	. "dappco.re/go"
	exec "dappco.re/go/process/exec"
)

func ExampleNopLogger_Debug() {
	logger := exec.NopLogger{}
	logger.Debug("ignored", "key", "value")
	Println("debug ignored")
	// Output: debug ignored
}

func ExampleNopLogger_Error() {
	logger := exec.NopLogger{}
	logger.Error("ignored", "err", NewError("boom"))
	Println("error ignored")
	// Output: error ignored
}

func ExampleSetDefaultLogger() {
	exec.SetDefaultLogger(exec.NopLogger{})
	Println(exec.DefaultLogger() != nil)
	// Output: true
}

func ExampleDefaultLogger() {
	exec.SetDefaultLogger(exec.NopLogger{})
	Println(exec.DefaultLogger() != nil)
	// Output: true
}
