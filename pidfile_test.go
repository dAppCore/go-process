package process

import (
	"os"
	"testing"

	"dappco.re/go/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPIDFile_Acquire_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	require.NoError(t, err)
	data, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	err = pid.Release()
	require.NoError(t, err)
	_, err = os.Stat(pidPath)
	assert.True(t, os.IsNotExist(err))
}

func TestPIDFile_AcquireStale_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "stale.pid")
	require.NoError(t, os.WriteFile(pidPath, []byte("999999999"), 0644))
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	require.NoError(t, err)
	err = pid.Release()
	require.NoError(t, err)
}

func TestPIDFile_CreateDirectory_Good(t *testing.T) {
	pidPath := core.JoinPath(t.TempDir(), "subdir", "nested", "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	require.NoError(t, err)
	err = pid.Release()
	require.NoError(t, err)
}

func TestPIDFile_Path_Good(t *testing.T) {
	pid := NewPIDFile("/tmp/test.pid")
	assert.Equal(t, "/tmp/test.pid", pid.Path())
}

func TestReadPID_Missing_Bad(t *testing.T) {
	pid, running := ReadPID("/nonexistent/path.pid")
	assert.Equal(t, 0, pid)
	assert.False(t, running)
}

func TestReadPID_Invalid_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "bad.pid")
	require.NoError(t, os.WriteFile(path, []byte("notanumber"), 0644))
	pid, running := ReadPID(path)
	assert.Equal(t, 0, pid)
	assert.False(t, running)
}

func TestReadPID_Stale_Bad(t *testing.T) {
	path := core.JoinPath(t.TempDir(), "stale.pid")
	require.NoError(t, os.WriteFile(path, []byte("999999999"), 0644))
	pid, running := ReadPID(path)
	assert.Equal(t, 999999999, pid)
	assert.False(t, running)
}
