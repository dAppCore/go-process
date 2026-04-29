package process

import (
	"context"
	"net/http"
	"testing"
)

func TestHealthServer_Endpoints(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	assertTrue(t, hs.Ready())
	requireNoError(t, hs.Start())
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

	requireNoError(t, hs.Start())
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

	requireNoError(t, hs.Start())
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
	healthy, reason := ProbeHealth("127.0.0.1:19999", 0)
	assertFalse(t, ok)
	assertFalse(t, healthy)
	assertNotEmpty(t, reason)
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
	ready, reason := ProbeReady("127.0.0.1:19999", 0)
	assertFalse(t, ok)
	assertFalse(t, ready)
	assertNotEmpty(t, reason)
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

func TestHealth_NewHealthServer_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	assertNotNil(t, hs)
	assertTrue(t, hs.Ready())
	assertEqual(t, "127.0.0.1:0", hs.Addr())
}

func TestHealth_NewHealthServer_Bad(t *testing.T) {
	hs := NewHealthServer("")
	assertNotNil(t, hs)
	assertTrue(t, hs.Ready())
	assertEqual(t, "", hs.Addr())
}

func TestHealth_NewHealthServer_Ugly(t *testing.T) {
	hs := NewHealthServer(":0")
	assertNotNil(t, hs)
	assertTrue(t, hs.Ready())
	assertEqual(t, ":0", hs.Addr())
}

func TestHealth_HealthServer_AddCheck_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.AddCheck(func() error { return nil })
	checks := hs.checksSnapshot()
	requireLen(t, checks, 1)
	assertNil(t, checks[0]())
}

func TestHealth_HealthServer_AddCheck_Bad(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.AddCheck(func() error { return errSentinel })
	checks := hs.checksSnapshot()
	requireLen(t, checks, 1)
	assertErrorIs(t, checks[0](), errSentinel)
}

func TestHealth_HealthServer_AddCheck_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	var check HealthCheck
	hs.AddCheck(check)
	checks := hs.checksSnapshot()
	requireLen(t, checks, 1)
	assertNil(t, checks[0])
}

func TestHealth_HealthServer_SetReady_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	assertFalse(t, hs.Ready())
	hs.SetReady(true)
	assertTrue(t, hs.Ready())
}

func TestHealth_HealthServer_SetReady_Bad(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	got := hs.Ready()
	assertFalse(t, got)
}

func TestHealth_HealthServer_SetReady_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	for i := 0; i < 3; i++ {
		hs.SetReady(i%2 == 0)
	}
	assertTrue(t, hs.Ready())
}

func TestHealth_HealthServer_Ready_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	got := hs.Ready()
	assertTrue(t, got)
	assertEqual(t, "127.0.0.1:0", hs.Addr())
}

func TestHealth_HealthServer_Ready_Bad(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	got := hs.Ready()
	assertFalse(t, got)
}

func TestHealth_HealthServer_Ready_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	hs.SetReady(true)
	assertTrue(t, hs.Ready())
}

func TestHealth_HealthServer_Start_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	assertNotEmpty(t, hs.Addr())
}

func TestHealth_HealthServer_Start_Bad(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	second := NewHealthServer(hs.Addr())
	assertError(t, second.Start())
}

func TestHealth_HealthServer_Start_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.AddCheck(nil)
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	assertTrue(t, WaitForHealth(hs.Addr(), 2_000))
}

func TestHealth_HealthServer_Stop_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	err := hs.Stop(context.Background())
	requireNoError(t, err)
	assertFalse(t, hs.Ready())
}

func TestHealth_HealthServer_Stop_Bad(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	err := hs.Stop(context.Background())
	requireNoError(t, err)
	assertFalse(t, hs.Ready())
}

func TestHealth_HealthServer_Stop_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	requireNoError(t, hs.Stop(context.Background()))
	err := hs.Stop(context.Background())
	requireNoError(t, err)
}

func TestHealth_HealthServer_Addr_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	got := hs.Addr()
	assertNotEmpty(t, got)
}

func TestHealth_HealthServer_Addr_Bad(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	got := hs.Addr()
	assertEqual(t, "127.0.0.1:0", got)
	assertTrue(t, hs.Ready())
}

func TestHealth_HealthServer_Addr_Ugly(t *testing.T) {
	hs := NewHealthServer("")
	got := hs.Addr()
	assertEqual(t, "", got)
	assertTrue(t, hs.Ready())
}

func TestHealth_WaitForHealth_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok := WaitForHealth(hs.Addr(), 2_000)
	assertTrue(t, ok)
}

func TestHealth_WaitForHealth_Bad(t *testing.T) {
	ok := WaitForHealth("127.0.0.1:1", 0)
	assertFalse(t, ok)
	assertTrue(t, true)
}

func TestHealth_WaitForHealth_Ugly(t *testing.T) {
	ok := WaitForHealth("", 0)
	assertFalse(t, ok)
	assertTrue(t, true)
}

func TestHealth_ProbeHealth_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok, reason := ProbeHealth(hs.Addr(), 2_000)
	assertTrue(t, ok)
	assertEqual(t, "", reason)
}

func TestHealth_ProbeHealth_Bad(t *testing.T) {
	ok, reason := ProbeHealth("127.0.0.1:1", 0)
	assertFalse(t, ok)
	assertNotEmpty(t, reason)
}

func TestHealth_ProbeHealth_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.AddCheck(func() error { return errSentinel })
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok, reason := ProbeHealth(hs.Addr(), 1)
	assertFalse(t, ok)
	assertContains(t, reason, "unhealthy")
}

func TestHealth_WaitForReady_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok := WaitForReady(hs.Addr(), 2_000)
	assertTrue(t, ok)
}

func TestHealth_WaitForReady_Bad(t *testing.T) {
	ok := WaitForReady("127.0.0.1:1", 0)
	assertFalse(t, ok)
	assertTrue(t, true)
}

func TestHealth_WaitForReady_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok := WaitForReady(hs.Addr(), 1)
	assertFalse(t, ok)
}

func TestHealth_ProbeReady_Good(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok, reason := ProbeReady(hs.Addr(), 2_000)
	assertTrue(t, ok)
	assertEqual(t, "", reason)
}

func TestHealth_ProbeReady_Bad(t *testing.T) {
	ok, reason := ProbeReady("127.0.0.1:1", 0)
	assertFalse(t, ok)
	assertNotEmpty(t, reason)
}

func TestHealth_ProbeReady_Ugly(t *testing.T) {
	hs := NewHealthServer("127.0.0.1:0")
	hs.SetReady(false)
	requireNoError(t, hs.Start())
	defer func() { requireNoError(t, hs.Stop(context.Background())) }()
	ok, reason := ProbeReady(hs.Addr(), 1)
	assertFalse(t, ok)
	assertContains(t, reason, "not ready")
}
