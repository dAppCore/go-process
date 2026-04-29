package process_test

import (
	. "dappco.re/go"
	process "dappco.re/go/process"
)

func exampleRegistry() *process.Registry {
	return process.NewRegistry(PathJoin(TempDir(), "go-process-registry-"+ID()))
}

func ExampleNewRegistry() {
	reg := process.NewRegistry(PathJoin(TempDir(), "daemons"))
	Println(reg != nil)
	// Output: true
}

func ExampleDefaultRegistry() {
	reg := process.DefaultRegistry()
	Println(reg != nil)
	// Output: true
}

func ExampleRegistry_Register() {
	reg := exampleRegistry()
	entry := process.DaemonEntry{Code: "app", Daemon: "web", PID: Getpid()}
	Println(reg.Register(entry).OK)
	// Output: true
}

func ExampleRegistry_Unregister() {
	reg := exampleRegistry()
	entry := process.DaemonEntry{Code: "app", Daemon: "web", PID: Getpid()}
	reg.Register(entry)
	Println(reg.Unregister("app", "web").OK)
	// Output: true
}

func ExampleRegistry_Get() {
	reg := exampleRegistry()
	entry := process.DaemonEntry{Code: "app", Daemon: "web", PID: Getpid()}
	reg.Register(entry)
	got, ok := reg.Get("app", "web")
	Println(ok, got.Code)
	// Output: true app
}

func ExampleRegistry_List() {
	reg := exampleRegistry()
	reg.Register(process.DaemonEntry{Code: "app", Daemon: "web", PID: Getpid()})
	entries, _ := reg.List()
	Println(len(entries))
	// Output: 1
}
