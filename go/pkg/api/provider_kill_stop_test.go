// SPDX-Licence-Identifier: EUPL-1.2

package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	core "dappco.re/go"
	process "dappco.re/go/process"
)

func TestProcessProviderKillProcessJSON_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := resultValue[*process.Process](svc.Start(context.Background(), "sleep", "60"))
	requireNoError(t, err)

	p := NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	body := bytes.NewReader([]byte(`{"id":"` + proc.ID + `"}`))
	req, err := http.NewRequest("POST", "/api/process/process/kill", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp resultEnvelope[map[string]any]
	requireNoError(t, unmarshalJSON(t, w.Body.Bytes(), &resp))
	requireTrue(t, resp.OK)
	assertEqual(t, true, resp.Value["killed"])

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been killed")
	}
	assertEqual(t, process.StatusKilled, proc.Status)
}

func TestProcessProviderKillProcessJSON_ByPID_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := resultValue[*process.Process](svc.Start(context.Background(), "sleep", "60"))
	requireNoError(t, err)
	t.Cleanup(func() {
		if proc.IsRunning() {
			_ = svc.Kill(proc.ID)
		}
	})

	p := NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	body := bytes.NewReader([]byte(`{"pid":` + strconv.Itoa(proc.Info().PID) + `}`))
	req, err := http.NewRequest("POST", "/api/process/process/kill", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)
	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been killed by PID")
	}
}

func TestProcessProviderKillProcessJSON_Bad(t *testing.T) {
	svc := newTestProcessService(t)
	p := NewProvider(nil, svc, nil)
	r := setupRouter(p)

	// Empty body: neither id nor pid supplied.
	w := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{}`))
	req, _ := http.NewRequest("POST", "/api/process/process/kill", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusBadRequest, w.Code)

	// Malformed JSON.
	w = httptest.NewRecorder()
	bad := bytes.NewReader([]byte(`{not json`))
	req, _ = http.NewRequest("POST", "/api/process/process/kill", bad)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusBadRequest, w.Code)
}

func TestProcessProviderKillProcessJSON_Ugly(t *testing.T) {
	// Service not configured returns service_unavailable.
	p := NewProvider(nil, nil, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{"id":"x"}`))
	req, _ := http.NewRequest("POST", "/api/process/process/kill", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusServiceUnavailable, w.Code)

	// Unknown id surfaces not-found.
	svc := newTestProcessService(t)
	p = NewProvider(nil, svc, nil)
	r = setupRouter(p)
	w = httptest.NewRecorder()
	body = bytes.NewReader([]byte(`{"id":"no-such-process"}`))
	req, _ = http.NewRequest("POST", "/api/process/process/kill", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusNotFound, w.Code)
}

// startManagedChild spawns a real child via the process service and returns
// its PID, so daemon-stop tests SIGTERM a throwaway process, never the test
// binary itself.
func startManagedChild(t *testing.T, svc *process.Service) (*process.Process, int) {
	t.Helper()
	proc, err := resultValue[*process.Process](svc.Start(context.Background(), "sleep", "60"))
	requireNoError(t, err)
	t.Cleanup(func() {
		if proc.IsRunning() {
			_ = svc.Kill(proc.ID)
		}
	})
	pid := proc.Info().PID
	requireTrue(t, pid > 0)
	return proc, pid
}

func TestProcessProviderStopDaemon_Good(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	svc := newTestProcessService(t)
	proc, pid := startManagedChild(t, svc)

	requireNoError(t, registry.Register(process.DaemonEntry{
		Code:   "test",
		Daemon: "serve",
		PID:    pid,
	}))

	p := NewProvider(registry, nil, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/process/daemons/test/serve/stop", nil)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)
	var resp resultEnvelope[map[string]any]
	requireNoError(t, unmarshalJSON(t, w.Body.Bytes(), &resp))
	requireTrue(t, resp.OK)
	assertEqual(t, true, resp.Value["stopped"])

	// The SIGTERM should propagate to the managed child.
	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("managed child should have received SIGTERM")
	}

	// The entry is gone from the registry after a successful stop.
	_, ok := registry.Get("test", "serve")
	assertFalse(t, ok)
}

func TestProcessProviderStopDaemon_Bad(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	p := NewProvider(registry, nil, nil)
	r := setupRouter(p)

	// Unknown daemon is a not-found.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/process/daemons/test/nonexistent/stop", nil)
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusNotFound, w.Code)
}

func TestProcessProviderStopDaemon_Ugly(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)

	// Register an entry with a dead PID. Registry.Get prunes dead-PID entries
	// on lookup, so the route resolves to not-found rather than reaching the
	// signal path — this asserts the prune-on-stale behaviour.
	requireNoError(t, registry.Register(process.DaemonEntry{
		Code:   "test",
		Daemon: "ghost",
		PID:    2147483646,
	}))

	p := NewProvider(registry, nil, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/process/daemons/test/ghost/stop", nil)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusNotFound, w.Code)
}

var _ = core.Ok
