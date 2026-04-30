package process_test

import (
	"context"
	"time"

	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleNewDaemon() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
	Println(daemon.Ready())
	// Output: true
}

func ExampleDaemon_Start() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
	Println(daemon.Start().OK)
	daemon.Stop()
	// Output: true
}

func ExampleDaemon_Run() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
	daemon.Start()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Println(daemon.Run(ctx).OK)
	// Output: true
}

func ExampleDaemon_Stop() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0", ShutdownTimeout: time.Second})
	daemon.Start()
	Println(daemon.Stop().OK)
	// Output: true
}

func ExampleDaemon_SetReady() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
	daemon.SetReady(false)
	Println(daemon.Ready())
	// Output: false
}

func ExampleDaemon_Ready() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
	Println(daemon.Ready())
	// Output: true
}

func ExampleDaemon_HealthAddr() {
	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
	daemon.Start()
	defer daemon.Stop()
	Println(Contains(daemon.HealthAddr(), "127.0.0.1:"))
	// Output: true
}
