//go:build !windows

package process

import "syscall"

import core "dappco.re/go"

// processSignal sends sig to pid.
func processSignal(pid int, sig syscall.Signal) core.Result {
	return core.ResultOf(nil, syscall.Kill(pid, sig))
}

// processKillGroup forcefully terminates the process group led by pid.
func processKillGroup(pid int) core.Result {
	return core.ResultOf(nil, syscall.Kill(-pid, syscall.SIGKILL))
}

// processSignalGroup sends sig to the process group led by pid.
func processSignalGroup(pid int, sig syscall.Signal) core.Result {
	return core.ResultOf(nil, syscall.Kill(-pid, sig))
}

// signalCannotBeCaught reports signals callers must not handle gracefully.
func signalCannotBeCaught(sig syscall.Signal) bool {
	switch sig {
	case syscall.SIGKILL, syscall.SIGSTOP:
		return true
	default:
		return false
	}
}

// applyProcessGroup starts cmd in a new process group.
func applyProcessGroup(cmd *core.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// exitWasSignaled reports whether wait status exited due to a signal.
func exitWasSignaled(status any) bool {
	ws, ok := status.(syscall.WaitStatus)
	return ok && ws.Signaled()
}

// exitSignalName returns the signal name from a signaled wait status.
func exitSignalName(status any) string {
	ws, ok := status.(syscall.WaitStatus)
	if !ok {
		return ""
	}
	return ws.Signal().String()
}
