package process_test

import (
	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleNewPIDFile() {
	pid := process.NewPIDFile(PathJoin(TempDir(), "example.pid"))
	Println(pid.Path() != "")
	// Output: true
}

func ExamplePIDFile_Acquire() {
	path := PathJoin(TempDir(), "go-process-example-acquire.pid")
	pid := process.NewPIDFile(path)
	defer pid.Release()
	Println(pid.Acquire().OK)
	// Output: true
}

func ExamplePIDFile_Release() {
	path := PathJoin(TempDir(), "go-process-example-release.pid")
	pid := process.NewPIDFile(path)
	pid.Acquire()
	Println(pid.Release().OK)
	// Output: true
}

func ExamplePIDFile_Path() {
	pid := process.NewPIDFile("/tmp/example.pid")
	Println(pid.Path())
	// Output: /tmp/example.pid
}

func ExampleReadPID() {
	path := PathJoin(TempDir(), "go-process-example-read.pid")
	pid := process.NewPIDFile(path)
	pid.Acquire()
	defer pid.Release()
	_, running := process.ReadPID(path)
	Println(running)
	// Output: true
}
