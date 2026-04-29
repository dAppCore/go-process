package process_test

import (
	"time"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleStatus() {
	Println(process.StatusRunning)
	// Output: running
}

func ExampleStream() {
	Println(process.StreamStdout)
	// Output: stdout
}

func ExampleRunOptions() {
	opts := process.RunOptions{Command: "echo", Timeout: time.Second}
	Println(opts.Command, opts.Timeout == time.Second)
	// Output: echo true
}

func ExampleInfo() {
	info := process.Info{ID: "proc-1", Status: process.StatusExited}
	Println(info.ID, info.Status)
	// Output: proc-1 exited
}
