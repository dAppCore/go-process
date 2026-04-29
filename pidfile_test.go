package process

import (
	"testing"

	"dappco.re/go"
)

func TestPIDFile_Acquire_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	requireNoError(t, err)
	data := core.ReadFile(pidPath)
	requireTrue(t, data.OK, data.Error())
	assertNotEmpty(t, data.Value)
	err = pid.Release()
	requireNoError(t, err)
	stat := core.Stat(pidPath)
	assertFalse(t, stat.OK)
	assertTrue(t, core.IsNotExist(stat.Value.(error)))
}

func TestPIDFile_Acquire_Ugly(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "stale.pid")
	requireTrue(t, core.WriteFile(pidPath, []byte("999999999"), 0644).OK)
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	requireNoError(t, err)
	err = pid.Release()
	requireNoError(t, err)
}

func TestPIDFile_Acquire_CreatesDirectory(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "subdir", "nested", "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	requireNoError(t, err)
	err = pid.Release()
	requireNoError(t, err)
}

func TestPIDFile_Path_Good(t *testing.T) {
	pid := NewPIDFile("/tmp/test.pid")
	got := pid.Path()
	assertEqual(t, "/tmp/test.pid", got)
}

func TestPIDFile_Release_MissingIsNoop(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "absent.pid")
	pid := NewPIDFile(pidPath)
	requireNoError(t, pid.Release())
}

func TestReadPID_ReadPID_Missing(t *testing.T) {
	pid, running := ReadPID("/nonexistent/path.pid")
	assertEqual(t, 0, pid)
	assertFalse(t, running)
}

func TestReadPID_ReadPID_Invalid(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "bad.pid")
	requireTrue(t, core.WriteFile(path, []byte("notanumber"), 0644).OK)
	pid, running := ReadPID(path)
	assertEqual(t, 0, pid)
	assertFalse(t, running)
}

func TestReadPID_ReadPID_Stale(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "stale.pid")
	requireTrue(t, core.WriteFile(path, []byte("999999999"), 0644).OK)
	pid, running := ReadPID(path)
	assertEqual(t, 999999999, pid)
	assertFalse(t, running)
}

func TestPidfile_NewPIDFile_Good(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "good.pid")
	pid := NewPIDFile(path)
	assertNotNil(t, pid)
	assertEqual(t, path, pid.Path())
}

func TestPidfile_NewPIDFile_Bad(t *testing.T) {
	pid := NewPIDFile("")
	got := pid.Path()
	assertNotNil(t, pid)
	assertEqual(t, "", got)
}

func TestPidfile_NewPIDFile_Ugly(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "nested", "ugly.pid")
	pid := NewPIDFile(path)
	assertEqual(t, path, pid.Path())
	assertNotNil(t, pid)
}

func TestPidfile_PIDFile_Acquire_Good(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "acquire.pid")
	pid := NewPIDFile(path)
	err := pid.Acquire()
	requireNoError(t, err)
	assertTrue(t, fileExists(path))
}

func TestPidfile_PIDFile_Acquire_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "running.pid")
	requireTrue(t, core.WriteFile(path, []byte(core.Itoa(core.Getpid())), 0644).OK)
	pid := NewPIDFile(path)
	err := pid.Acquire()
	assertError(t, err)
	assertContains(t, err.Error(), "another instance")
}

func TestPidfile_PIDFile_Acquire_Ugly(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "stale.pid")
	requireTrue(t, core.WriteFile(path, []byte("999999999"), 0644).OK)
	pid := NewPIDFile(path)
	err := pid.Acquire()
	requireNoError(t, err)
	assertTrue(t, fileExists(path))
}

func TestPidfile_PIDFile_Release_Good(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "release.pid")
	pid := NewPIDFile(path)
	requireNoError(t, pid.Acquire())
	err := pid.Release()
	requireNoError(t, err)
	assertFalse(t, fileExists(path))
}

func TestPidfile_PIDFile_Release_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "missing.pid")
	pid := NewPIDFile(path)
	err := pid.Release()
	requireNoError(t, err)
	assertFalse(t, fileExists(path))
}

func TestPidfile_PIDFile_Release_Ugly(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "nested", "release.pid")
	pid := NewPIDFile(path)
	requireNoError(t, pid.Acquire())
	requireNoError(t, pid.Release())
	assertFalse(t, fileExists(path))
}

func TestPidfile_PIDFile_Path_Good(t *testing.T) {
	pid := NewPIDFile("/tmp/process.pid")
	got := pid.Path()
	assertEqual(t, "/tmp/process.pid", got)
	assertNotNil(t, pid)
}

func TestPidfile_PIDFile_Path_Bad(t *testing.T) {
	pid := NewPIDFile("")
	got := pid.Path()
	assertEqual(t, "", got)
	assertNotNil(t, pid)
}

func TestPidfile_PIDFile_Path_Ugly(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "space pid.pid")
	pid := NewPIDFile(path)
	got := pid.Path()
	assertEqual(t, path, got)
}

func TestPidfile_ReadPID_Good(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "current.pid")
	requireTrue(t, core.WriteFile(path, []byte(core.Itoa(core.Getpid())), 0644).OK)
	pid, running := ReadPID(path)
	assertEqual(t, core.Getpid(), pid)
	assertTrue(t, running)
}

func TestPidfile_ReadPID_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "missing.pid")
	pid, running := ReadPID(path)
	assertEqual(t, 0, pid)
	assertFalse(t, running)
}

func TestPidfile_ReadPID_Ugly(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "invalid.pid")
	requireTrue(t, core.WriteFile(path, []byte("not-a-pid"), 0644).OK)
	pid, running := ReadPID(path)
	assertEqual(t, 0, pid)
	assertFalse(t, running)
}
