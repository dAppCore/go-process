//go:build !windows

package api

import (
	"syscall"

	core "dappco.re/go"
)

func signalPID(pid int, sig syscall.Signal) core.Result {
	return core.ResultOf(nil, syscall.Kill(pid, sig))
}

func pidAlive(pid int) bool {
	return syscall.Kill(pid, syscall.Signal(0)) == nil
}

func parsePlatformSignal(name string) core.Result {
	switch name {
	case "SIGSTOP", "STOP":
		return core.Ok(syscall.SIGSTOP)
	case "SIGCONT", "CONT":
		return core.Ok(syscall.SIGCONT)
	case "SIGUSR1", "USR1":
		return core.Ok(syscall.SIGUSR1)
	case "SIGUSR2", "USR2":
		return core.Ok(syscall.SIGUSR2)
	default:
		return core.Fail(core.E("ProcessProvider.parseSignal", "unsupported signal", nil))
	}
}
