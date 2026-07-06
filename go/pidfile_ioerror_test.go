package process

import (
	"testing"

	core "dappco.re/go"
)

func TestPidfileIOError_Acquire_InvalidContent_Ugly(t *testing.T) {
	// Non-numeric PID content fails strconv.Atoi, so the liveness check is
	// skipped and the stale file is removed and rewritten with our PID.
	path := core.JoinPath(t.TempDir(), "garbage.pid")
	requireTrue(t, core.WriteFile(path, []byte("not-a-number"), 0o644).OK)

	pid := NewPIDFile(path)
	requireNoError(t, pid.Acquire())
	assertTrue(t, fileExists(path))
	requireNoError(t, pid.Release())
}

func TestPidfileIOError_Acquire_WriteFails_Bad(t *testing.T) {
	// Pointing the PID file at an existing non-empty directory makes the
	// final WriteFile fail (cannot write a file over a directory). ReadFile
	// on the directory also fails, so the stale-removal block is skipped.
	dir := t.TempDir()
	requireTrue(t, core.WriteFile(core.JoinPath(dir, "child"), []byte("x"), 0o644).OK)

	pid := NewPIDFile(dir)
	r := pid.Acquire()
	assertFalse(t, r.OK)
}

func TestPidfileIOError_Release_RemoveFails_Bad(t *testing.T) {
	// A non-empty directory at the PID path: Stat succeeds, but Remove of a
	// non-empty directory fails, exercising the release error branch.
	dir := t.TempDir()
	requireTrue(t, core.WriteFile(core.JoinPath(dir, "child"), []byte("x"), 0o644).OK)

	pid := NewPIDFile(dir)
	r := pid.Release()
	assertFalse(t, r.OK)
	assertContains(t, r.Error(), "failed to remove PID file")
}
