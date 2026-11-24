//go:build windows

package process

import "syscall"

import core "github.com/dappcore/go"

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

// createNoWindow is CREATE_NO_WINDOW: the child gets no console of its own.
const createNoWindow = 0x08000000

// applyProcessGroup starts the child without a console window. Its output is
// always piped back to the service, so a console would only be an empty
// black window flashing up over a GUI app (-H windowsgui) for every command.
// Process-group lifecycle (Job Objects) is still to come on Windows.
func applyProcessGroup(cmd *core.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// exitWasSignaled is always false on Windows.
func exitWasSignaled(status any) bool {
	return false
}

// exitSignalName is empty on Windows.
func exitSignalName(status any) string {
	return ""
}
