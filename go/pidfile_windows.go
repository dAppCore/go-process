//go:build windows

package process

import "syscall"

// currentPID returns the PID of the current process.
//
// Example:
//
//	pid := currentPID()
func currentPID() int {
	return syscall.Getpid()
}
