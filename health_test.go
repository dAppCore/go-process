package process

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthServer_Endpoints(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	assert.True(t, hs.Ready())
	err := hs.Start()
	require.NoError(t, err)
	defer func() { _ = hs.Stop(context.Background()) }()

	addr := hs.Addr()
	require.NotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = http.Get("http://" + addr + "/ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	hs.SetReady(false)
	assert.False(t, hs.Ready())

	resp, err = http.Get("http://" + addr + "/ready")
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHealthServer_Ready(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	assert.True(t, hs.Ready())

	hs.SetReady(false)
	assert.False(t, hs.Ready())
}

func TestHealthServer_WithChecks(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	healthy := true
	hs.AddCheck(func() error {
		if !healthy {
			return assert.AnError
		}
		return nil
	})

	err := hs.Start()
	require.NoError(t, err)
	defer func() { _ = hs.Stop(context.Background()) }()

	addr := hs.Addr()

	resp, err := http.Get("http://" + addr + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	healthy = false

	resp, err = http.Get("http://" + addr + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHealthServer_NilCheckIgnored(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	var check HealthCheck
	hs.AddCheck(check)

	err := hs.Start()
	require.NoError(t, err)
	defer func() { _ = hs.Stop(context.Background()) }()

	addr := hs.Addr()

	resp, err := http.Get("http://" + addr + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHealthServer_ChecksSnapshotIsStable(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	hs.AddCheck(func() error { return nil })
	snapshot := hs.checksSnapshot()
	hs.AddCheck(func() error { return assert.AnError })

	require.Len(t, snapshot, 1)
	require.NotNil(t, snapshot[0])
}

func TestWaitForHealth_Reachable(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	require.NoError(t, hs.Start())
	defer func() { _ = hs.Stop(context.Background()) }()

	ok := WaitForHealth(hs.Addr(), 2_000)
	assert.True(t, ok)
}

func TestWaitForHealth_Unreachable(t *testing.T) {
	ok := WaitForHealth("127.0.0.1:19999", 500)
	assert.False(t, ok)
}

func TestWaitForReady_Reachable(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	require.NoError(t, hs.Start())
	defer func() { _ = hs.Stop(context.Background()) }()

	ok := WaitForReady(hs.Addr(), 2_000)
	assert.True(t, ok)
}

func TestWaitForReady_Unreachable(t *testing.T) {
	ok := WaitForReady("127.0.0.1:19999", 500)
	assert.False(t, ok)
}

func TestHealthServer_StopMarksNotReady(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	require.NoError(t, hs.Start())

	require.NotEmpty(t, hs.Addr())
	assert.True(t, hs.Ready())

	require.NoError(t, hs.Stop(context.Background()))

	assert.False(t, hs.Ready())
	assert.NotEmpty(t, hs.Addr())
}
