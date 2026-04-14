package process

import "os"

// processHandle returns an os.Process for the given PID.
//
//	proc, err := processHandle(12345)
func processHandle(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

// currentPID returns the calling process ID.
//
//	pid := currentPID()
func currentPID() int {
	return os.Getpid()
}
