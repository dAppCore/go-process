package process_test

import (
	"context"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleNewHealthServer() {
	hs := process.NewHealthServer("127.0.0.1:0")
	Println(hs.Ready())
	// Output: true
}

func ExampleHealthServer_AddCheck() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.AddCheck(func() error { return nil })
	Println(hs.Start().OK)
	hs.Stop(context.Background())
	// Output: true
}

func ExampleHealthServer_SetReady() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	Println(hs.Ready())
	// Output: false
}

func ExampleHealthServer_Ready() {
	hs := process.NewHealthServer("127.0.0.1:0")
	Println(hs.Ready())
	// Output: true
}

func ExampleHealthServer_Start() {
	hs := process.NewHealthServer("127.0.0.1:0")
	Println(hs.Start().OK)
	hs.Stop(context.Background())
	// Output: true
}

func ExampleHealthServer_Stop() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.Start()
	Println(hs.Stop(context.Background()).OK)
	// Output: true
}

func ExampleHealthServer_Addr() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.Start()
	defer hs.Stop(context.Background())
	Println(Contains(hs.Addr(), "127.0.0.1:"))
	// Output: true
}

func ExampleWaitForHealth() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.Start()
	defer hs.Stop(context.Background())
	Println(process.WaitForHealth(hs.Addr(), 1000))
	// Output: true
}

func ExampleProbeHealth() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.AddCheck(func() error { return E("example", "down", nil) })
	hs.Start()
	defer hs.Stop(context.Background())
	ok, reason := process.ProbeHealth(hs.Addr(), 1000)
	Println(ok, Contains(reason, "down"))
	// Output: false true
}

func ExampleWaitForReady() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.Start()
	defer hs.Stop(context.Background())
	Println(process.WaitForReady(hs.Addr(), 1000))
	// Output: true
}

func ExampleProbeReady() {
	hs := process.NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	hs.Start()
	defer hs.Stop(context.Background())
	ok, reason := process.ProbeReady(hs.Addr(), 1000)
	Println(ok, Contains(reason, "not ready"))
	// Output: false true
}
