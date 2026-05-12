//go:build windows

package process

import "syscall"

import core "dappco.re/go"

// processSignal reports success for positive PIDs on Windows, where POSIX
// signals are not available through syscall.
func processSignal(pid int, sig syscall.Signal) core.Result {
	if pid <= 0 {
		return core.Fail(syscall.EINVAL)
	}
	return core.Ok(nil)
}

// processKillGroup best-effort kills the process-group leader on Windows.
func processKillGroup(pid int) core.Result {
	if pid <= 0 {
		return core.Fail(syscall.EINVAL)
	}
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return core.Fail(err)
	}
	defer syscall.CloseHandle(handle)
	return core.ResultOf(nil, syscall.TerminateProcess(handle, 1))
}

// processSignalGroup reports success for positive PIDs on Windows. Windows
// has no POSIX process-group signal primitive.
func processSignalGroup(pid int, sig syscall.Signal) core.Result {
	if pid <= 0 {
		return core.Fail(syscall.EINVAL)
	}
	return core.Ok(nil)
}

// signalCannotBeCaught reports signals callers must not handle gracefully.
func signalCannotBeCaught(sig syscall.Signal) bool {
	return sig == syscall.SIGKILL
}

// applyProcessGroup is a no-op until Windows Job Object lifecycle lands.
func applyProcessGroup(cmd *core.Cmd) {
}

// exitWasSignaled is always false on Windows.
func exitWasSignaled(status any) bool {
	return false
}

// exitSignalName is empty on Windows.
func exitSignalName(status any) string {
	return ""
}
