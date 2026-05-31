// SPDX-Licence-Identifier: EUPL-1.2

package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
)

// postJSON issues a POST with a JSON body and returns the recorder.
func postJSON(t *testing.T, r http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", path, bytes.NewReader([]byte(body)))
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestProcessProviderStartProcess_InvalidRequest_Bad(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	// Malformed JSON.
	w := postJSON(t, r, "/api/process/processes", `{bad json`)
	assertEqual(t, http.StatusBadRequest, w.Code)

	// Missing command.
	w = postJSON(t, r, "/api/process/processes", `{"args":["x"]}`)
	assertEqual(t, http.StatusBadRequest, w.Code)
}

func TestProcessProviderStartProcess_Unavailable_Ugly(t *testing.T) {
	r := setupRouter(NewProvider(nil, nil, nil))
	w := postJSON(t, r, "/api/process/processes", `{"command":"echo"}`)
	assertEqual(t, http.StatusServiceUnavailable, w.Code)
}

func TestProcessProviderStartProcessRFC_InvalidRequest_Bad(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	w := postJSON(t, r, "/api/process/process/start", `{not json`)
	assertEqual(t, http.StatusBadRequest, w.Code)

	w = postJSON(t, r, "/api/process/process/start", `{}`)
	assertEqual(t, http.StatusBadRequest, w.Code)
}

func TestProcessProviderStartProcessRFC_Unavailable_Ugly(t *testing.T) {
	r := setupRouter(NewProvider(nil, nil, nil))
	w := postJSON(t, r, "/api/process/process/start", `{"command":"echo"}`)
	assertEqual(t, http.StatusServiceUnavailable, w.Code)
}

func TestProcessProviderRunProcess_InvalidRequest_Bad(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	w := postJSON(t, r, "/api/process/processes/run", `{nope`)
	assertEqual(t, http.StatusBadRequest, w.Code)

	w = postJSON(t, r, "/api/process/processes/run", `{}`)
	assertEqual(t, http.StatusBadRequest, w.Code)
}

func TestProcessProviderRunProcess_Failed_Ugly(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	// A command that exits non-zero surfaces run_failed.
	w := postJSON(t, r, "/api/process/processes/run", `{"command":"sh","args":["-c","exit 7"]}`)
	assertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestProcessProviderInputProcess_InvalidRequest_Bad(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	// Malformed JSON body.
	w := postJSON(t, r, "/api/process/processes/anything/input", `{bad`)
	assertEqual(t, http.StatusBadRequest, w.Code)

	// Unknown process id is not-found.
	w = postJSON(t, r, "/api/process/processes/no-such/input", `{"input":"hi"}`)
	assertEqual(t, http.StatusNotFound, w.Code)
}

func TestProcessProviderCloseStdin_NotFound_Ugly(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	// Unknown process id surfaces not-found.
	w := postJSON(t, r, "/api/process/processes/no-such/close-stdin", ``)
	assertEqual(t, http.StatusNotFound, w.Code)
}

func TestProcessProviderGetProcessOutput_NotFound_Ugly(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/processes/no-such/output", nil)
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusNotFound, w.Code)
}

func TestProcessProviderGetProcess_NotFound_Ugly(t *testing.T) {
	svc := newTestProcessService(t)
	r := setupRouter(NewProvider(nil, svc, nil))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/processes/no-such", nil)
	r.ServeHTTP(w, req)
	assertEqual(t, http.StatusNotFound, w.Code)
}

func TestProcessProviderRunProcessRFCAlias_Unavailable_Ugly(t *testing.T) {
	r := setupRouter(NewProvider(nil, nil, nil))
	w := postJSON(t, r, "/api/process/process/run", `{"command":"echo"}`)
	assertEqual(t, http.StatusServiceUnavailable, w.Code)
}

func TestParseSignal_Good(t *testing.T) {
	cases := map[string]syscall.Signal{
		"SIGTERM": syscall.SIGTERM,
		"term":    syscall.SIGTERM,
		"SIGKILL": syscall.SIGKILL,
		"KILL":    syscall.SIGKILL,
		"SIGINT":  syscall.SIGINT,
		"int":     syscall.SIGINT,
		"SIGQUIT": syscall.SIGQUIT,
		"QUIT":    syscall.SIGQUIT,
		"SIGHUP":  syscall.SIGHUP,
		"HUP":     syscall.SIGHUP,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			r := parseSignal(name)
			requireTrue(t, r.OK)
			assertEqual(t, want, r.Value.(syscall.Signal))
		})
	}

	// A numeric signal is accepted as-is.
	r := parseSignal("9")
	requireTrue(t, r.OK)
	assertEqual(t, syscall.Signal(9), r.Value.(syscall.Signal))

	// A platform-specific name is delegated to parsePlatformSignal.
	r = parseSignal("SIGUSR1")
	requireTrue(t, r.OK)
}

func TestParseSignal_Bad(t *testing.T) {
	// Empty signal is required.
	r := parseSignal("")
	assertFalse(t, r.OK)

	// Whitespace trims to empty and is rejected.
	r = parseSignal("   ")
	assertFalse(t, r.OK)
}

func TestParseSignal_Ugly(t *testing.T) {
	// An unknown name falls through to the platform parser and is rejected.
	r := parseSignal("SIGNOTREAL")
	assertFalse(t, r.OK)
}
