package command_test

import (
	. "dappco.re/go"
	command "dappco.re/go/process/exec"
)

func ExampleNopLogger_Debug() {
	logger := command.NopLogger{}
	logger.Debug("ignored", "key", "value")
	Println("debug ignored")
	// Output: debug ignored
}

func ExampleNopLogger_Error() {
	logger := command.NopLogger{}
	logger.Error("ignored", "err", NewError("boom"))
	Println("error ignored")
	// Output: error ignored
}

func ExampleSetDefaultLogger() {
	command.SetDefaultLogger(command.NopLogger{})
	Println(command.DefaultLogger() != nil)
	// Output: true
}

func ExampleDefaultLogger() {
	command.SetDefaultLogger(command.NopLogger{})
	Println(command.DefaultLogger() != nil)
	// Output: true
}
