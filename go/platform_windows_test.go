//go:build windows

package process

import (
	"syscall"
	"testing"

	core "dappco.re/go"
)

func testUncatchableSignals() []syscall.Signal {
	return []syscall.Signal{syscall.SIGKILL}
}

func TestPlatformWindowsSignalHelpers(t *testing.T) {
	pid := currentPID()

	if r := processSignal(pid, syscall.Signal(0)); !r.OK {
		t.Fatalf("processSignal current pid: %v", r.Error())
	}

	if r := processSignalGroup(pid, syscall.Signal(0)); !r.OK {
		t.Fatalf("processSignalGroup current pid: %v", r.Error())
	}

	cmd := &core.Cmd{}
	applyProcessGroup(cmd)
	if cmd.SysProcAttr != nil {
		t.Fatalf("applyProcessGroup configured SysProcAttr on Windows")
	}

	if exitWasSignaled(nil) {
		t.Fatalf("exitWasSignaled reported a signal on Windows")
	}
}

func TestPlatformWindowsKillGroupRejectsInvalidPID(t *testing.T) {
	if r := processKillGroup(0); r.OK {
		t.Fatalf("processKillGroup accepted invalid pid")
	}
}
