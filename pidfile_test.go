package process

import (
	"os"
	"testing"

	"dappco.re/go/core"
)

func TestPIDFile_Acquire_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	requireNoError(t, err)
	data, err := os.ReadFile(pidPath)
	requireNoError(t, err)
	assertNotEmpty(t, data)
	err = pid.Release()
	requireNoError(t, err)
	_, err = os.Stat(pidPath)
	assertTrue(t, os.IsNotExist(err))
}

func TestPIDFile_AcquireStale_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "stale.pid")
	requireNoError(t, os.WriteFile(pidPath, []byte("999999999"), 0644))
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	requireNoError(t, err)
	err = pid.Release()
	requireNoError(t, err)
}

func TestPIDFile_CreateDirectory_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "subdir", "nested", "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	requireNoError(t, err)
	err = pid.Release()
	requireNoError(t, err)
}

func TestPIDFile_Path_Good(t *testing.T) {
	pid := NewPIDFile("/tmp/test.pid")
	assertEqual(t, "/tmp/test.pid", pid.Path())
}

func TestPIDFile_Release_MissingIsNoop(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "absent.pid")
	pid := NewPIDFile(pidPath)
	requireNoError(t, pid.Release())
}

func TestReadPID_Missing_Bad(t *testing.T) {
	pid, running := ReadPID("/nonexistent/path.pid")
	assertEqual(t, 0, pid)
	assertFalse(t, running)
}

func TestReadPID_Invalid_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "bad.pid")
	requireNoError(t, os.WriteFile(path, []byte("notanumber"), 0644))
	pid, running := ReadPID(path)
	assertEqual(t, 0, pid)
	assertFalse(t, running)
}

func TestReadPID_Stale_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "stale.pid")
	requireNoError(t, os.WriteFile(path, []byte("999999999"), 0644))
	pid, running := ReadPID(path)
	assertEqual(t, 999999999, pid)
	assertFalse(t, running)
}
