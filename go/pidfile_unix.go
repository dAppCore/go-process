//go:build !windows

package process

import "syscall"

import core "dappco.re/go"

// processSignal sends sig to pid.
//
// Example:
//
//	err := processSignal(1234, syscall.Signal(0))
func processSignal(pid int, sig syscall.Signal) core.Result {
	return core.ResultOf(nil, syscall.Kill(pid, sig))
}

// currentPID returns the PID of the current process.
//
// Example:
//
//	pid := currentPID()
func currentPID() int {
	return syscall.Getpid()
}
