package process_test

import (
	"time"

	. "github.com/dappcore/go"
	process "github.com/dappcore/go-process/go"
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
