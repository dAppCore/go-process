//go:build windows

package process

import (
	"syscall"
	"testing"

	core "github.com/dappcore/go"
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
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("applyProcessGroup should start children without a console window: %+v", cmd.SysProcAttr)
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
