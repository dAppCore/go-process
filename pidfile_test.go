package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPIDFile_AcquireAndRelease(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "test.pid")
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

func TestPIDFile_StalePID(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "stale.pid")
	require.NoError(t, os.WriteFile(pidPath, []byte("999999999"), 0644))
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	require.NoError(t, err)
	err = pid.Release()
	require.NoError(t, err)
}

func TestPIDFile_CreatesParentDirectory(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "subdir", "nested", "test.pid")
	pid := NewPIDFile(pidPath)
	err := pid.Acquire()
	require.NoError(t, err)
	err = pid.Release()
	require.NoError(t, err)
}

func TestPIDFile_Path(t *testing.T) {
	pid := NewPIDFile("/tmp/test.pid")
	assert.Equal(t, "/tmp/test.pid", pid.Path())
}

func TestReadPID_Missing(t *testing.T) {
	pid, running := ReadPID("/nonexistent/path.pid")
	assert.Equal(t, 0, pid)
	assert.False(t, running)
}

func TestReadPID_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pid")
	require.NoError(t, os.WriteFile(path, []byte("notanumber"), 0644))
	pid, running := ReadPID(path)
	assert.Equal(t, 0, pid)
	assert.False(t, running)
}

func TestReadPID_StalePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")
	require.NoError(t, os.WriteFile(path, []byte("999999999"), 0644))
	pid, running := ReadPID(path)
	assert.Equal(t, 999999999, pid)
	assert.False(t, running)
}
