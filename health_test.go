package process

import (
	"context"
	"net/http"
	"testing"
)

func TestHealthServer_Endpoints(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	assertTrue(t, hs.Ready())
	err := hs.Start()
	requireNoError(t, err)
	defer func() { _ = hs.Stop(context.Background()) }()

	addr := hs.Addr()
	requireNotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/health")
	requireNoError(t, err)
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = http.Get("http://" + addr + "/ready")
	requireNoError(t, err)
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	hs.SetReady(false)
	assertFalse(t, hs.Ready())

	resp, err = http.Get("http://" + addr + "/ready")
	requireNoError(t, err)
	assertEqual(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHealthServer_Ready(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	assertTrue(t, hs.Ready())

	hs.SetReady(false)
	assertFalse(t, hs.Ready())
}

func TestHealthServer_WithChecks(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	healthy := true
	hs.AddCheck(func() error {
		if !healthy {
			return errSentinel
		}
		return nil
	})

	err := hs.Start()
	requireNoError(t, err)
	defer func() { _ = hs.Stop(context.Background()) }()

	addr := hs.Addr()

	resp, err := http.Get("http://" + addr + "/health")
	requireNoError(t, err)
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	healthy = false

	resp, err = http.Get("http://" + addr + "/health")
	requireNoError(t, err)
	assertEqual(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHealthServer_NilCheckIgnored(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	var check HealthCheck
	hs.AddCheck(check)

	err := hs.Start()
	requireNoError(t, err)
	defer func() { _ = hs.Stop(context.Background()) }()

	addr := hs.Addr()

	resp, err := http.Get("http://" + addr + "/health")
	requireNoError(t, err)
	assertEqual(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHealthServer_ChecksSnapshotIsStable(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")

	hs.AddCheck(func() error { return nil })
	snapshot := hs.checksSnapshot()
	hs.AddCheck(func() error { return errSentinel })

	requireLen(t, snapshot, 1)
	requireNotNil(t, snapshot[0])
}

func TestWaitForHealth_Reachable(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { _ = hs.Stop(context.Background()) }()

	ok := WaitForHealth(hs.Addr(), 2_000)
	assertTrue(t, ok)
}

func TestWaitForHealth_Unreachable(t *testing.T) {
	ok := WaitForHealth("127.0.0.1:19999", 500)
	assertFalse(t, ok)
}

func TestWaitForReady_Reachable(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { _ = hs.Stop(context.Background()) }()

	ok := WaitForReady(hs.Addr(), 2_000)
	assertTrue(t, ok)
}

func TestWaitForReady_Unreachable(t *testing.T) {
	ok := WaitForReady("127.0.0.1:19999", 500)
	assertFalse(t, ok)
}

func TestHealthServer_StopMarksNotReady(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())

	requireNotEmpty(t, hs.Addr())
	assertTrue(t, hs.Ready())

	requireNoError(t, hs.Stop(context.Background()))

	assertFalse(t, hs.Ready())
	assertNotEmpty(t, hs.Addr())
}
