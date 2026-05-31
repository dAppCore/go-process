package process

import (
	"testing"

	core "dappco.re/go"
)

// unknownMessage is a Core message (core.Message is any) the process service
// does not handle, used to exercise handleTask's default branch.
type unknownMessage struct{}

func TestServiceHandleTask_MissingTarget_Bad(t *testing.T) {
	svc, c := newTestService(t)

	// Each of these task variants requires an id (or id/pid) and must fail
	// with a descriptive error when none is supplied.
	cases := []core.Message{
		TaskProcessKill{},
		TaskProcessSignal{},
		TaskProcessGet{},
		TaskProcessWait{},
		TaskProcessOutput{},
		TaskProcessInput{},
		TaskProcessCloseStdin{},
		TaskProcessRemove{},
	}
	for _, msg := range cases {
		r := svc.handleTask(c, msg)
		assertFalse(t, r.OK)
	}
}

func TestServiceHandleTask_Default_Ugly(t *testing.T) {
	svc, c := newTestService(t)

	// An unhandled message type returns the zero Result (not OK, no value).
	r := svc.handleTask(c, unknownMessage{})
	assertFalse(t, r.OK)
	assertNil(t, r.Value)
}

func TestServiceHandleTask_Clear_Good(t *testing.T) {
	svc, c := newTestService(t)

	// Clear is always OK even on an empty service.
	r := svc.handleTask(c, TaskProcessClear{})
	assertTrue(t, r.OK)
}

func TestServiceHandleTask_KillUnknownID_Ugly(t *testing.T) {
	svc, c := newTestService(t)

	// A non-empty but unknown id surfaces the Kill failure.
	r := svc.handleTask(c, TaskProcessKill{ID: "no-such"})
	assertFalse(t, r.OK)

	// A positive but unmanaged PID surfaces the KillPID failure.
	r = svc.handleTask(c, TaskProcessKill{PID: 999999})
	assertFalse(t, r.OK)
}

func TestServiceHandleTask_SignalUnknownID_Ugly(t *testing.T) {
	svc, c := newTestService(t)

	r := svc.handleTask(c, TaskProcessSignal{ID: "no-such"})
	assertFalse(t, r.OK)
}

func TestServiceHandleTask_RemoveUnknownID_Ugly(t *testing.T) {
	svc, c := newTestService(t)

	r := svc.handleTask(c, TaskProcessRemove{ID: "no-such"})
	assertFalse(t, r.OK)
}
