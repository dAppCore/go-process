package process

import (
	"context"
	"testing"
	"time"
)

func TestServiceHandlers_handleRun_Good(t *testing.T) {
	svc, _ := newTestService(t)

	r := svc.handleRun(context.Background(), opts("command", "echo", "args", []string{"hello"}))
	requireTrue(t, r.OK)
	output, _ := r.Value.(string)
	assertContains(t, output, "hello")
}

func TestServiceHandlers_handleRun_Bad(t *testing.T) {
	svc, _ := newTestService(t)

	// Missing command is rejected before any spawn.
	r := svc.handleRun(context.Background(), opts())
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleRun_Ugly(t *testing.T) {
	svc, _ := newTestService(t)

	// A command that exits non-zero surfaces as a failed result.
	r := svc.handleRun(context.Background(), opts("command", "sh", "args", []string{"-c", "exit 3"}))
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleKill_Good(t *testing.T) {
	svc, _ := newTestService(t)

	proc, err := resultValue[*Process](svc.Start(context.Background(), "sleep", "30"))
	requireNoError(t, err)

	r := svc.handleKill(context.Background(), opts("id", proc.ID))
	requireTrue(t, r.OK)

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process should have been killed")
	}
	assertEqual(t, StatusKilled, proc.Status)
}

func TestServiceHandlers_handleKill_Bad(t *testing.T) {
	svc, _ := newTestService(t)

	// Neither id nor pid supplied is a target-parse error.
	r := svc.handleKill(context.Background(), opts())
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleKill_Ugly(t *testing.T) {
	svc, _ := newTestService(t)

	// Unknown id surfaces the Kill failure.
	r := svc.handleKill(context.Background(), opts("id", "no-such-process"))
	assertFalse(t, r.OK)

	// A PID that is not managed surfaces the KillPID failure.
	r = svc.handleKill(context.Background(), opts("pid", 999999))
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleList_Good(t *testing.T) {
	svc, _ := newTestService(t)

	proc, err := resultValue[*Process](svc.Start(context.Background(), "sleep", "30"))
	requireNoError(t, err)
	defer svc.Kill(proc.ID)

	r := svc.handleList(context.Background(), opts())
	requireTrue(t, r.OK)
	ids := r.Value.([]string)
	assertContains(t, ids, proc.ID)
}

func TestServiceHandlers_handleList_Bad(t *testing.T) {
	svc, _ := newTestService(t)

	// runningOnly filters out exited processes.
	done, err := resultValue[*Process](svc.Start(context.Background(), "echo", "bye"))
	requireNoError(t, err)
	<-done.Done()

	r := svc.handleList(context.Background(), opts("runningOnly", true))
	requireTrue(t, r.OK)
	ids := r.Value.([]string)
	assertFalse(t, containsValue(ids, done.ID))
}

func TestServiceHandlers_handleList_Ugly(t *testing.T) {
	svc, _ := newTestService(t)

	// Empty service returns an empty (non-nil) id slice.
	r := svc.handleList(context.Background(), opts())
	requireTrue(t, r.OK)
	ids := r.Value.([]string)
	assertNotNil(t, ids)
	assertLen(t, ids, 0)
}

func TestServiceHandlers_handleGet_Good(t *testing.T) {
	svc, _ := newTestService(t)

	proc, err := resultValue[*Process](svc.Start(context.Background(), "sleep", "30"))
	requireNoError(t, err)
	defer svc.Kill(proc.ID)

	r := svc.handleGet(context.Background(), opts("id", proc.ID))
	requireTrue(t, r.OK)
	info, ok := r.Value.(Info)
	requireTrue(t, ok)
	assertEqual(t, proc.ID, info.ID)
}

func TestServiceHandlers_handleGet_Bad(t *testing.T) {
	svc, _ := newTestService(t)

	// Missing id is rejected.
	r := svc.handleGet(context.Background(), opts())
	assertFalse(t, r.OK)

	// Whitespace-only id is trimmed to empty and rejected.
	r = svc.handleGet(context.Background(), opts("id", "   "))
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleGet_Ugly(t *testing.T) {
	svc, _ := newTestService(t)

	// Unknown id surfaces the Get failure.
	r := svc.handleGet(context.Background(), opts("id", "no-such-process"))
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleStart_Good(t *testing.T) {
	svc, _ := newTestService(t)

	r := svc.handleStart(context.Background(), opts("command", "sleep", "args", []string{"30"}))
	requireTrue(t, r.OK)
	id, _ := r.Value.(string)
	assertNotEmpty(t, id)
	defer svc.Kill(id)
}

func TestServiceHandlers_handleStart_Bad(t *testing.T) {
	svc, _ := newTestService(t)

	// Missing command is rejected.
	r := svc.handleStart(context.Background(), opts())
	assertFalse(t, r.OK)
}

func TestServiceHandlers_handleStart_Ugly(t *testing.T) {
	svc, _ := newTestService(t)

	// A non-existent binary surfaces the spawn failure.
	r := svc.handleStart(context.Background(), opts("command", "nonexistent_command_xyz"))
	assertFalse(t, r.OK)
}
