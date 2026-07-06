package process

import (
	"context"
	"testing"
	"time"
)

func TestProcessLifecycle_terminate_NotRunning_Bad(t *testing.T) {
	// A non-running process is a no-op success.
	proc := newProcessForTest(t, StatusExited, 0, "")
	r := proc.terminate()
	assertTrue(t, r.OK)
}

func TestProcessLifecycle_terminate_NilCmd_Ugly(t *testing.T) {
	// Running status but no underlying cmd is also a no-op success.
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.cmd = nil
	r := proc.terminate()
	assertTrue(t, r.OK)
}

func TestProcessLifecycle_terminate_Good(t *testing.T) {
	// A live process receives SIGTERM and exits.
	svc, _ := newTestService(t)
	proc := requireResultValue[*Process](t, svc.Start(context.Background(), "sleep", "30"))

	r := proc.terminate()
	assertTrue(t, r.OK)

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM should have terminated the process")
	}
}

func TestProcessLifecycle_killTree_NotRunning_Bad(t *testing.T) {
	proc := newProcessForTest(t, StatusExited, 0, "")
	r := proc.killTree()
	requireTrue(t, r.OK)
	assertEqual(t, false, r.Value.(bool))
}

func TestProcessLifecycle_killTree_NilCmd_Ugly(t *testing.T) {
	proc := newProcessForTest(t, StatusRunning, 0, "")
	proc.cmd = nil
	r := proc.killTree()
	requireTrue(t, r.OK)
	assertEqual(t, false, r.Value.(bool))
}

func TestProcessLifecycle_killTree_Group_Good(t *testing.T) {
	// A detached, group-managed process is killed at the group level.
	svc, _ := newTestService(t)
	proc := requireResultValue[*Process](t, svc.StartWithOptions(context.Background(), RunOptions{
		Command:   "sh",
		Args:      []string{"-c", "sleep 60 & wait"},
		Detach:    true,
		KillGroup: true,
	}))
	time.Sleep(100 * time.Millisecond)

	r := proc.killTree()
	requireTrue(t, r.OK)
	assertEqual(t, true, r.Value.(bool))

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("killTree should have killed the process group")
	}
}

func TestProcessLifecycle_terminate_Group_Good(t *testing.T) {
	// terminate on a group-managed process signals the whole group with TERM.
	svc, _ := newTestService(t)
	proc := requireResultValue[*Process](t, svc.StartWithOptions(context.Background(), RunOptions{
		Command:   "sh",
		Args:      []string{"-c", "sleep 60 & wait"},
		Detach:    true,
		KillGroup: true,
	}))
	time.Sleep(100 * time.Millisecond)

	r := proc.terminate()
	assertTrue(t, r.OK)

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("group SIGTERM should have terminated the process")
	}
}
