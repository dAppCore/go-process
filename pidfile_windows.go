//go:build windows

package process

import "os"

// processHandle returns the OS process handle for the given PID.
//
// Example:
//
//	proc, err := processHandle(1234)
func processHandle(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

// currentPID returns the PID of the current process.
//
// Example:
//
//	pid := currentPID()
func currentPID() int {
	return os.Getpid()
}
