//go:build windows

package api

import (
	"syscall"

	core "dappco.re/go"
)

func signalPID(pid int, sig syscall.Signal) core.Result {
	if pid <= 0 {
		return core.Fail(syscall.EINVAL)
	}
	return core.Ok(nil)
}

func pidAlive(pid int) bool {
	return pid > 0
}

func parsePlatformSignal(name string) core.Result {
	return core.Fail(core.E("ProcessProvider.parseSignal", "unsupported signal", nil))
}
