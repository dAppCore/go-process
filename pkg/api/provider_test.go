// SPDX-Licence-Identifier: EUPL-1.2

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	core "dappco.re/go"
	goapi "dappco.re/go/api"
	process "dappco.re/go/process"
	processapi "dappco.re/go/process/pkg/api"
	corews "dappco.re/go/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestProcessProvider_Name_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	got := p.Name()
	assertEqual(t, "process", got)
}

func TestProcessProvider_BasePath_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	got := p.BasePath()
	assertEqual(t, "/api/process", got)
}

func TestProcessProvider_Channels_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	channels := p.Channels()
	assertContains(t, channels, "process.daemon.started")
	assertContains(t, channels, "process.daemon.stopped")
	assertContains(t, channels, "process.daemon.health")
}

func TestProcessProvider_Describe_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	descs := p.Describe()
	assertGreaterOrEqual(t, len(descs), 5)

	// Verify all descriptions have required fields
	for _, d := range descs {
		assertNotEmpty(t, d.Method)
		assertNotEmpty(t, d.Path)
		assertNotEmpty(t, d.Summary)
		assertNotEmpty(t, d.Tags)
	}

	foundPipelineRoute := false
	foundSignalRoute := false
	for _, d := range descs {
		if d.Method == "POST" && d.Path == "/pipelines/run" {
			foundPipelineRoute = true
		}
		if d.Method == "POST" && d.Path == "/processes/:id/signal" {
			foundSignalRoute = true
		}
	}
	assertTrue(t, foundPipelineRoute, "pipeline route should be described")
	assertTrue(t, foundSignalRoute, "signal route should be described")
}

func TestProcessProviderListDaemonsRoute(t *testing.T) {
	// Use a temp directory so the registry has no daemons
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	p := processapi.NewProvider(registry, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons", nil)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[[]any]
	err := unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	assertTrue(t, resp.Success)
}

func TestProcessProviderListDaemonsBroadcastsStarted(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	requireNoError(t, registry.Register(process.DaemonEntry{
		Code:   "test",
		Daemon: "serve",
		PID:    core.Getpid(),
	}))

	hub := corews.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	p := processapi.NewProvider(registry, nil, hub)
	server := httptest.NewServer(hub.Handler())
	defer server.Close()

	conn := connectWS(t, server.URL)
	defer conn.Close()

	requireEventually(t, func() bool {
		return hub.ClientCount() == 1
	}, time.Second, 10*time.Millisecond)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons", nil)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	events := readWSEvents(t, conn, "process.daemon.started")
	started := events["process.daemon.started"]
	requireNotNil(t, started)

	startedData := started.Data.(map[string]any)
	assertEqual(t, "test", startedData["code"])
	assertEqual(t, "serve", startedData["daemon"])
	assertEqual(t, float64(core.Getpid()), startedData["pid"])
}

func TestProcessProviderGetDaemonNotFound(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	p := processapi.NewProvider(registry, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons/test/nonexistent", nil)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusNotFound, w.Code)
}

func TestProcessProviderHealthCheckUnavailable(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)

	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream health check failed"))
	}))
	defer healthSrv.Close()

	hostPort := core.TrimPrefix(healthSrv.URL, "http://")
	requireNoError(t, registry.Register(process.DaemonEntry{
		Code:   "test",
		Daemon: "broken",
		PID:    core.Getpid(),
		Health: hostPort,
	}))

	p := processapi.NewProvider(registry, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons/test/broken/health", nil)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusServiceUnavailable, w.Code)

	var resp goapi.Response[map[string]any]
	err := unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)

	assertEqual(t, false, resp.Data["healthy"])
	assertEqual(t, hostPort, resp.Data["address"])
	assertEqual(t, "upstream health check failed", resp.Data["reason"])
}

func TestProcessProviderRegistersAsRouteGroup(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)

	engine, err := goapi.New()
	requireNoError(t, err)

	engine.Register(p)
	assertLen(t, engine.Groups(), 1)
	assertEqual(t, "process", engine.Groups()[0].Name())
}

func TestProcessProvider_Channels_RegisterAsStreamGroup_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)

	engine, err := goapi.New()
	requireNoError(t, err)

	engine.Register(p)

	// Engine.Channels() discovers StreamGroups
	channels := engine.Channels()
	assertContains(t, channels, "process.daemon.started")
}

func TestProcessProviderRunPipelineRoute(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()

	body := core.NewReader(`{
		"mode": "parallel",
		"specs": [
			{"name": "first", "command": "echo", "args": ["1"]},
			{"name": "second", "command": "echo", "args": ["2"]}
		]
	}`)
	req, err := http.NewRequest("POST", "/api/process/pipelines/run", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[process.RunAllResult]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	assertTrue(t, resp.Success)
	assertEqual(t, 2, resp.Data.Passed)
	assertLen(t, resp.Data.Results, 2)
}

func TestProcessProvider_RunPipeline_Unavailable(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/pipelines/run", core.NewReader(`{"mode":"all","specs":[]}`))
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusServiceUnavailable, w.Code)
}

func TestProcessProviderListProcessesRoute(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello-api")
	requireNoError(t, err)
	<-proc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[[]process.Info]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	requireLen(t, resp.Data, 1)
	assertEqual(t, proc.ID, resp.Data[0].ID)
	assertEqual(t, "echo", resp.Data[0].Command)
}

func TestProcessProviderListProcessesRunningOnly(t *testing.T) {
	svc := newTestProcessService(t)

	runningProc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)

	exitedProc, err := svc.Start(context.Background(), "echo", "done")
	requireNoError(t, err)
	<-exitedProc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes?runningOnly=true", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[[]process.Info]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	requireLen(t, resp.Data, 1)
	assertEqual(t, runningProc.ID, resp.Data[0].ID)
	assertEqual(t, process.StatusRunning, resp.Data[0].Status)

	requireNoError(t, svc.Kill(runningProc.ID))
	<-runningProc.Done()
}

func TestProcessProviderStartProcessRoute(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	body := core.NewReader(`{
		"command": "sleep",
		"args": ["60"],
		"detach": true,
		"killGroup": true
	}`)
	req, err := http.NewRequest("POST", "/api/process/processes", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[process.Info]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, "sleep", resp.Data.Command)
	assertEqual(t, process.StatusRunning, resp.Data.Status)
	assertTrue(t, resp.Data.Running)
	assertNotEmpty(t, resp.Data.ID)

	managed, err := svc.Get(resp.Data.ID)
	requireNoError(t, err)
	requireNoError(t, svc.Kill(managed.ID))
	select {
	case <-managed.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been killed after start test")
	}
}

func TestProcessProviderRunProcessRoute(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	body := core.NewReader(`{
		"command": "echo",
		"args": ["run-check"]
	}`)
	req, err := http.NewRequest("POST", "/api/process/processes/run", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[string]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertContains(t, resp.Data, "run-check")
}

func TestProcessProviderGetProcessRoute(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "single")
	requireNoError(t, err)
	<-proc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes/"+proc.ID, nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[process.Info]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, proc.ID, resp.Data.ID)
	assertEqual(t, "echo", resp.Data.Command)
}

func TestProcessProviderGetProcessOutputRoute(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "output-check")
	requireNoError(t, err)
	<-proc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes/"+proc.ID+"/output", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[string]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertContains(t, resp.Data, "output-check")
}

func TestProcessProviderWaitProcessRoute(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "wait-check")
	requireNoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/wait", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[process.Info]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, proc.ID, resp.Data.ID)
	assertEqual(t, process.StatusExited, resp.Data.Status)
	assertEqual(t, 0, resp.Data.ExitCode)
}

func TestProcessProviderWaitProcessNonZeroExit(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "sh", "-c", "exit 7")
	requireNoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/wait", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusConflict, w.Code)

	var resp goapi.Response[any]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireFalse(t, resp.Success)
	requireNotNil(t, resp.Error)
	assertEqual(t, "wait_failed", resp.Error.Code)
	assertContains(t, resp.Error.Message, "process exited with code 7")

	details, ok := resp.Error.Details.(map[string]any)
	requireTrue(t, ok)
	assertEqual(t, "exited", details["status"])
	assertEqual(t, float64(7), details["exitCode"])
	assertEqual(t, proc.ID, details["id"])
}

func TestProcessProviderInputAndCloseStdinRoutes(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "cat")
	requireNoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	inputReq := core.NewReader("{\"input\":\"hello-api\\n\"}")
	inputHTTPReq, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/input", inputReq)
	requireNoError(t, err)
	inputHTTPReq.Header.Set("Content-Type", "application/json")

	inputResp := httptest.NewRecorder()
	r.ServeHTTP(inputResp, inputHTTPReq)

	assertEqual(t, http.StatusOK, inputResp.Code)

	closeReq, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/close-stdin", nil)
	requireNoError(t, err)

	closeResp := httptest.NewRecorder()
	r.ServeHTTP(closeResp, closeReq)

	assertEqual(t, http.StatusOK, closeResp.Code)

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have exited after stdin was closed")
	}

	assertContains(t, proc.Output(), "hello-api")
}

func TestProcessProviderKillProcessRoute(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/kill", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[map[string]any]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, true, resp.Data["killed"])

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been killed")
	}
	assertEqual(t, process.StatusKilled, proc.Status)
}

func TestProcessProviderKillProcessByPIDRoute(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	proc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)
	t.Cleanup(func() {
		if proc.IsRunning() {
			_ = svc.Kill(proc.ID)
		}
		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
		}
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/process/processes/"+strconv.Itoa(proc.Info().PID)+"/kill", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[map[string]any]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, true, resp.Data["killed"])

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been killed by PID")
	}
}

func TestProcessProviderSignalProcessRoute(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/signal", core.NewReader(`{"signal":"SIGTERM"}`))
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[map[string]any]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, true, resp.Data["signalled"])

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been signalled")
	}
	assertEqual(t, process.StatusKilled, proc.Status)
}

func TestProcessProviderSignalProcessByPIDRoute(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	proc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)
	t.Cleanup(func() {
		if proc.IsRunning() {
			_ = svc.Kill(proc.ID)
		}
		select {
		case <-proc.Done():
		case <-time.After(2 * time.Second):
		}
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/process/processes/"+strconv.Itoa(proc.Info().PID)+"/signal", core.NewReader(`{"signal":"SIGTERM"}`))
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[map[string]any]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	requireTrue(t, resp.Success)
	assertEqual(t, true, resp.Data["signalled"])

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been signalled by PID")
	}
}

func TestProcessProviderSignalProcessInvalidSignal(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/signal", core.NewReader(`{"signal":"NOPE"}`))
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusBadRequest, w.Code)
	assertTrue(t, proc.IsRunning())

	requireNoError(t, svc.Kill(proc.ID))
	<-proc.Done()
}

func TestProcessProviderBroadcastsProcessEvents(t *testing.T) {
	svc := newTestProcessService(t)
	hub := corews.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	_ = processapi.NewProvider(nil, svc, hub)

	server := httptest.NewServer(hub.Handler())
	defer server.Close()

	conn := connectWS(t, server.URL)
	defer conn.Close()

	requireEventually(t, func() bool {
		return hub.ClientCount() == 1
	}, time.Second, 10*time.Millisecond)

	proc, err := svc.Start(context.Background(), "sh", "-c", "echo live-event")
	requireNoError(t, err)
	<-proc.Done()

	events := readWSEvents(t, conn, "process.started", "process.output", "process.exited")

	started := events["process.started"]
	requireNotNil(t, started)
	startedData := started.Data.(map[string]any)
	assertEqual(t, proc.ID, startedData["id"])
	assertEqual(t, "sh", startedData["command"])
	assertEqual(t, float64(proc.Info().PID), startedData["pid"])

	output := events["process.output"]
	requireNotNil(t, output)
	outputData := output.Data.(map[string]any)
	assertEqual(t, proc.ID, outputData["id"])
	assertEqual(t, "stdout", outputData["stream"])
	assertContains(t, outputData["line"], "live-event")

	exited := events["process.exited"]
	requireNotNil(t, exited)
	exitedData := exited.Data.(map[string]any)
	assertEqual(t, proc.ID, exitedData["id"])
	assertEqual(t, float64(0), exitedData["exitCode"])
}

func TestProcessProviderBroadcastsKilledEvents(t *testing.T) {
	svc := newTestProcessService(t)
	hub := corews.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	_ = processapi.NewProvider(nil, svc, hub)

	server := httptest.NewServer(hub.Handler())
	defer server.Close()

	conn := connectWS(t, server.URL)
	defer conn.Close()

	requireEventually(t, func() bool {
		return hub.ClientCount() == 1
	}, time.Second, 10*time.Millisecond)

	proc, err := svc.Start(context.Background(), "sleep", "60")
	requireNoError(t, err)

	requireNoError(t, svc.Kill(proc.ID))

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process should have been killed")
	}

	events := readWSEvents(t, conn, "process.killed", "process.exited")

	killed := events["process.killed"]
	requireNotNil(t, killed)
	killedData := killed.Data.(map[string]any)
	assertEqual(t, proc.ID, killedData["id"])
	assertEqual(t, "SIGKILL", killedData["signal"])
	assertEqual(t, float64(-1), killedData["exitCode"])

	exited := events["process.exited"]
	requireNotNil(t, exited)
	exitedData := exited.Data.(map[string]any)
	assertEqual(t, proc.ID, exitedData["id"])
	assertEqual(t, float64(-1), exitedData["exitCode"])
}

func TestProcessProvider_ProcessRoutes_Unavailable(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	r := setupRouter(p)

	cases := []string{
		"/api/process/processes",
		"/api/process/processes/anything",
		"/api/process/processes/anything/output",
		"/api/process/processes/anything/wait",
		"/api/process/processes/anything/input",
		"/api/process/processes/anything/close-stdin",
		"/api/process/processes/anything/kill",
	}

	for _, path := range cases {
		w := httptest.NewRecorder()
		method := "GET"
		switch {
		case core.HasSuffix(path, "/kill"),
			core.HasSuffix(path, "/wait"),
			core.HasSuffix(path, "/input"),
			core.HasSuffix(path, "/close-stdin"):
			method = "POST"
		}
		req, err := http.NewRequest(method, path, nil)
		requireNoError(t, err)
		r.ServeHTTP(w, req)
		assertEqual(t, http.StatusServiceUnavailable, w.Code)
	}
}

func TestProcessProviderRFCListAlias(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	proc, err := svc.Start(context.Background(), "sleep", "0.1")
	requireNoError(t, err)
	t.Cleanup(func() {
		_ = svc.Kill(proc.ID)
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/process/process/list?runningOnly=true", nil)
	requireNoError(t, err)
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[[]string]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	assertTrue(t, resp.Success)
	assertContains(t, resp.Data, proc.ID)
}

func TestProcessProviderRFCStartAlias(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)

	body := core.NewReader(`{"command":"sleep","args":["0.1"]}`)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/process/process/start", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assertEqual(t, http.StatusOK, w.Code)

	var resp goapi.Response[string]
	err = unmarshalJSON(t, w.Body.Bytes(), &resp)
	requireNoError(t, err)
	assertTrue(t, resp.Success)
	assertNotEmpty(t, resp.Data)

	proc, err := svc.Get(resp.Data)
	requireNoError(t, err)

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("RFC alias start should detach from the HTTP request context")
	}

	assertEqual(t, process.StatusExited, proc.Status)
	assertEqual(t, 0, proc.ExitCode)
}

// -- Test helpers -------------------------------------------------------------

func unmarshalJSON(t *testing.T, data []byte, target any) (err error) {
	t.Helper()
	result := core.JSONUnmarshal(data, target)
	if result.OK {
		return nil
	}
	err, _ = result.Value.(error)
	return err
}

func setupRouter(p *processapi.ProcessProvider) *gin.Engine {
	r := gin.New()
	p.Register(r)
	return r
}

// newTestRegistry creates a process.Registry backed by a test directory.
func newTestRegistry(dir string) *process.Registry {
	return process.NewRegistry(dir)
}

func newTestProcessService(t *testing.T) *process.Service {
	t.Helper()

	c := core.New()
	factory := process.NewService(process.Options{})
	raw, err := factory(c)
	requireNoError(t, err)

	return raw.(*process.Service)
}

func connectWS(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + core.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	requireNoError(t, err)
	return conn
}

func readWSEvents(t *testing.T, conn *websocket.Conn, channels ...string) map[string]corews.Message {
	t.Helper()

	want := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		want[channel] = struct{}{}
	}

	events := make(map[string]corews.Message, len(channels))
	deadline := time.Now().Add(3 * time.Second)

	for len(events) < len(channels) && time.Now().Before(deadline) {
		requireNoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))

		_, payload, err := conn.ReadMessage()
		requireNoError(t, err)

		for _, line := range core.Split(core.Trim(string(payload)), "\n") {
			if core.Trim(line) == "" {
				continue
			}

			var msg corews.Message
			requireNoError(t, unmarshalJSON(t, []byte(line), &msg))

			if _, ok := want[msg.Channel]; ok {
				events[msg.Channel] = msg
			}
		}
	}

	requireLen(t, events, len(channels))
	return events
}

func TestProvider_NewProvider_Good(t *testing.T) {
	registry := newTestRegistry(t.TempDir())
	service := newTestProcessService(t)
	p := processapi.NewProvider(registry, service, nil)
	assertNotNil(t, p)
	assertEqual(t, "process", p.Name())
}

func TestProvider_NewProvider_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	assertNotNil(t, p)
	assertEqual(t, "process", p.Name())
}

func TestProvider_NewProvider_Ugly(t *testing.T) {
	hub := corews.NewHub()
	p := processapi.NewProvider(nil, nil, hub)
	assertNotNil(t, p)
	assertContains(t, p.Channels(), "process.output")
}

func TestProvider_ProcessProvider_Name_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	got := p.Name()
	assertEqual(t, "process", got)
}

func TestProvider_ProcessProvider_Name_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	got := p.Name()
	assertNotEmpty(t, got)
	assertEqual(t, "process", got)
}

func TestProvider_ProcessProvider_Name_Ugly(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	first := p.Name()
	second := p.Name()
	assertEqual(t, first, second)
}

func TestProvider_ProcessProvider_BasePath_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	got := p.BasePath()
	assertEqual(t, "/api/process", got)
}

func TestProvider_ProcessProvider_BasePath_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	got := p.BasePath()
	assertContains(t, got, "/api")
	assertEqual(t, "/api/process", got)
}

func TestProvider_ProcessProvider_BasePath_Ugly(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	first := p.BasePath()
	second := p.BasePath()
	assertEqual(t, first, second)
}

func TestProvider_ProcessProvider_Register_Good(t *testing.T) {
	p := processapi.NewProvider(newTestRegistry(t.TempDir()), nil, nil)
	router := gin.New()
	p.Register(router)
	assertNotNil(t, router)
}

func TestProvider_ProcessProvider_Register_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	p.Register(nil)
	assertEqual(t, "process", p.Name())
}

func TestProvider_ProcessProvider_Register_Ugly(t *testing.T) {
	var p *processapi.ProcessProvider
	router := gin.New()
	p.Register(router)
	assertNotNil(t, router)
}

func TestProvider_ProcessProvider_Element_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	element := p.Element()
	assertEqual(t, "core-process-panel", element.Tag)
	assertContains(t, element.Source, "core-process")
}

func TestProvider_ProcessProvider_Element_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	element := p.Element()
	assertNotEmpty(t, element.Tag)
	assertNotEmpty(t, element.Source)
}

func TestProvider_ProcessProvider_Element_Ugly(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	first := p.Element()
	second := p.Element()
	assertEqual(t, first, second)
}

func TestProvider_ProcessProvider_Channels_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	channels := p.Channels()
	assertContains(t, channels, "process.started")
	assertContains(t, channels, "process.exited")
}

func TestProvider_ProcessProvider_Channels_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	channels := p.Channels()
	assertNotEmpty(t, channels)
	assertGreaterOrEqual(t, len(channels), 1)
}

func TestProvider_ProcessProvider_Channels_Ugly(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	channels := p.Channels()
	channels[0] = "mutated"
	assertContains(t, p.Channels(), "process.daemon.started")
}

func TestProvider_ProcessProvider_RegisterRoutes_Good(t *testing.T) {
	p := processapi.NewProvider(newTestRegistry(t.TempDir()), nil, nil)
	router := gin.New()
	p.RegisterRoutes(router.Group("/x"))
	assertNotNil(t, router)
}

func TestProvider_ProcessProvider_RegisterRoutes_Bad(t *testing.T) {
	p := processapi.NewProvider(newTestRegistry(t.TempDir()), nil, nil)
	router := gin.New()
	p.RegisterRoutes(router.Group(""))
	assertEqual(t, "process", p.Name())
}

func TestProvider_ProcessProvider_RegisterRoutes_Ugly(t *testing.T) {
	p := processapi.NewProvider(newTestRegistry(t.TempDir()), nil, nil)
	router := gin.New()
	p.RegisterRoutes(router.Group("/api/process"))
	req, _ := http.NewRequest("GET", "/api/process/daemons", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assertEqual(t, http.StatusOK, w.Code)
}

func TestProvider_ProcessProvider_Describe_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	descs := p.Describe()
	assertNotEmpty(t, descs)
	assertEqual(t, "GET", descs[0].Method)
}

func TestProvider_ProcessProvider_Describe_Bad(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	descs := p.Describe()
	assertGreaterOrEqual(t, len(descs), 1)
	assertNotEmpty(t, descs[0].Path)
}

func TestProvider_ProcessProvider_Describe_Ugly(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	descs := p.Describe()
	descs[0].Path = "mutated"
	assertFalse(t, "mutated" == p.Describe()[0].Path)
}

func TestProvider_PIDAlive_Good(t *testing.T) {
	alive := processapi.PIDAlive(core.Getpid())
	assertTrue(t, alive)
	assertGreater(t, core.Getpid(), 0)
}

func TestProvider_PIDAlive_Bad(t *testing.T) {
	alive := processapi.PIDAlive(0)
	assertFalse(t, alive)
	assertFalse(t, processapi.PIDAlive(-999999))
}

func TestProvider_PIDAlive_Ugly(t *testing.T) {
	alive := processapi.PIDAlive(-1)
	assertFalse(t, alive)
	assertLess(t, -1, 0)
}
