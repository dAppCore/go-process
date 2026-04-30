//go:build windows

package process

import "syscall"

import core "dappco.re/go"

// processSignal reports success for positive PIDs on Windows, where POSIX
// signals are not available through syscall.
//
// Example:
//
//	err := processSignal(1234, syscall.Signal(0))
func processSignal(pid int, sig syscall.Signal) core.Result {
	if pid <= 0 {
		return core.Fail(syscall.EINVAL)
	}
	return core.Ok(nil)
}

// currentPID returns the PID of the current process.
//
// Example:
//
//	pid := currentPID()
func currentPID() int {
	return syscall.Getpid()
}
