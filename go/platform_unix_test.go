//go:build !windows

package process

import "syscall"

func testUncatchableSignals() []syscall.Signal {
	return []syscall.Signal{syscall.SIGKILL, syscall.SIGSTOP}
}
