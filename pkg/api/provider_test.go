// SPDX-Licence-Identifier: EUPL-1.2

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	core "dappco.re/go/core"
	goapi "dappco.re/go/core/api"
	process "dappco.re/go/core/process"
	processapi "dappco.re/go/core/process/pkg/api"
	corews "dappco.re/go/core/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestProcessProvider_Name_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	assert.Equal(t, "process", p.Name())
}

func TestProcessProvider_BasePath_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	assert.Equal(t, "/api/process", p.BasePath())
}

func TestProcessProvider_Channels_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	channels := p.Channels()
	assert.Contains(t, channels, "process.daemon.started")
	assert.Contains(t, channels, "process.daemon.stopped")
	assert.Contains(t, channels, "process.daemon.health")
}

func TestProcessProvider_Describe_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	descs := p.Describe()
	assert.GreaterOrEqual(t, len(descs), 5)

	// Verify all descriptions have required fields
	for _, d := range descs {
		assert.NotEmpty(t, d.Method)
		assert.NotEmpty(t, d.Path)
		assert.NotEmpty(t, d.Summary)
		assert.NotEmpty(t, d.Tags)
	}

	foundPipelineRoute := false
	for _, d := range descs {
		if d.Method == "POST" && d.Path == "/pipelines/run" {
			foundPipelineRoute = true
			break
		}
	}
	assert.True(t, foundPipelineRoute, "pipeline route should be described")
}

func TestProcessProvider_ListDaemons_Good(t *testing.T) {
	// Use a temp directory so the registry has no daemons
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	p := processapi.NewProvider(registry, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp goapi.Response[[]any]
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestProcessProvider_GetDaemon_Bad(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	p := processapi.NewProvider(registry, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons/test/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProcessProvider_HealthCheck_Bad(t *testing.T) {
	dir := t.TempDir()
	registry := newTestRegistry(dir)

	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream health check failed"))
	}))
	defer healthSrv.Close()

	hostPort := strings.TrimPrefix(healthSrv.URL, "http://")
	require.NoError(t, registry.Register(process.DaemonEntry{
		Code:   "test",
		Daemon: "broken",
		PID:    os.Getpid(),
		Health: hostPort,
	}))

	p := processapi.NewProvider(registry, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons/test/broken/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp goapi.Response[map[string]any]
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)

	assert.Equal(t, false, resp.Data["healthy"])
	assert.Equal(t, hostPort, resp.Data["address"])
	assert.Equal(t, "upstream health check failed", resp.Data["reason"])
}

func TestProcessProvider_RegistersAsRouteGroup_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)

	engine, err := goapi.New()
	require.NoError(t, err)

	engine.Register(p)
	assert.Len(t, engine.Groups(), 1)
	assert.Equal(t, "process", engine.Groups()[0].Name())
}

func TestProcessProvider_Channels_RegisterAsStreamGroup_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)

	engine, err := goapi.New()
	require.NoError(t, err)

	engine.Register(p)

	// Engine.Channels() discovers StreamGroups
	channels := engine.Channels()
	assert.Contains(t, channels, "process.daemon.started")
}

func TestProcessProvider_RunPipeline_Good(t *testing.T) {
	svc := newTestProcessService(t)
	p := processapi.NewProvider(nil, svc, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()

	body := strings.NewReader(`{
		"mode": "parallel",
		"specs": [
			{"name": "first", "command": "echo", "args": ["1"]},
			{"name": "second", "command": "echo", "args": ["2"]}
		]
	}`)
	req, err := http.NewRequest("POST", "/api/process/pipelines/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp goapi.Response[process.RunAllResult]
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 2, resp.Data.Passed)
	assert.Len(t, resp.Data.Results, 2)
}

func TestProcessProvider_RunPipeline_Unavailable(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/pipelines/run", strings.NewReader(`{"mode":"all","specs":[]}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestProcessProvider_ListProcesses_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "hello-api")
	require.NoError(t, err)
	<-proc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp goapi.Response[[]process.Info]
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, proc.ID, resp.Data[0].ID)
	assert.Equal(t, "echo", resp.Data[0].Command)
}

func TestProcessProvider_GetProcess_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "single")
	require.NoError(t, err)
	<-proc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes/"+proc.ID, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp goapi.Response[process.Info]
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.Equal(t, proc.ID, resp.Data.ID)
	assert.Equal(t, "echo", resp.Data.Command)
}

func TestProcessProvider_GetProcessOutput_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "echo", "output-check")
	require.NoError(t, err)
	<-proc.Done()

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/api/process/processes/"+proc.ID+"/output", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp goapi.Response[string]
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.Contains(t, resp.Data, "output-check")
}

func TestProcessProvider_KillProcess_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := svc.Start(context.Background(), "sleep", "60")
	require.NoError(t, err)

	p := processapi.NewProvider(nil, svc, nil)
	r := setupRouter(p)
	w := httptest.NewRecorder()

	req, err := http.NewRequest("POST", "/api/process/processes/"+proc.ID+"/kill", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp goapi.Response[map[string]any]
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.Equal(t, true, resp.Data["killed"])

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process should have been killed")
	}
	assert.Equal(t, process.StatusKilled, proc.Status)
}

func TestProcessProvider_BroadcastsProcessEvents_Good(t *testing.T) {
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

	require.Eventually(t, func() bool {
		return hub.ClientCount() == 1
	}, time.Second, 10*time.Millisecond)

	proc, err := svc.Start(context.Background(), "sh", "-c", "echo live-event")
	require.NoError(t, err)
	<-proc.Done()

	events := readWSEvents(t, conn, "process.started", "process.output", "process.exited")

	started := events["process.started"]
	require.NotNil(t, started)
	startedData := started.Data.(map[string]any)
	assert.Equal(t, proc.ID, startedData["id"])
	assert.Equal(t, "sh", startedData["command"])
	assert.Equal(t, float64(proc.Info().PID), startedData["pid"])

	output := events["process.output"]
	require.NotNil(t, output)
	outputData := output.Data.(map[string]any)
	assert.Equal(t, proc.ID, outputData["id"])
	assert.Equal(t, "stdout", outputData["stream"])
	assert.Contains(t, outputData["line"], "live-event")

	exited := events["process.exited"]
	require.NotNil(t, exited)
	exitedData := exited.Data.(map[string]any)
	assert.Equal(t, proc.ID, exitedData["id"])
	assert.Equal(t, float64(0), exitedData["exitCode"])
}

func TestProcessProvider_BroadcastsKilledEvents_Good(t *testing.T) {
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

	require.Eventually(t, func() bool {
		return hub.ClientCount() == 1
	}, time.Second, 10*time.Millisecond)

	proc, err := svc.Start(context.Background(), "sleep", "60")
	require.NoError(t, err)

	require.NoError(t, svc.Kill(proc.ID))

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process should have been killed")
	}

	events := readWSEvents(t, conn, "process.killed", "process.exited")

	killed := events["process.killed"]
	require.NotNil(t, killed)
	killedData := killed.Data.(map[string]any)
	assert.Equal(t, proc.ID, killedData["id"])
	assert.Equal(t, "SIGKILL", killedData["signal"])
	assert.Equal(t, float64(-1), killedData["exitCode"])

	exited := events["process.exited"]
	require.NotNil(t, exited)
	exitedData := exited.Data.(map[string]any)
	assert.Equal(t, proc.ID, exitedData["id"])
	assert.Equal(t, float64(-1), exitedData["exitCode"])
}

func TestProcessProvider_ProcessRoutes_Unavailable(t *testing.T) {
	p := processapi.NewProvider(nil, nil, nil)
	r := setupRouter(p)

	cases := []string{
		"/api/process/processes",
		"/api/process/processes/anything",
		"/api/process/processes/anything/output",
		"/api/process/processes/anything/kill",
	}

	for _, path := range cases {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", path, nil)
		if strings.HasSuffix(path, "/kill") {
			req, err = http.NewRequest("POST", path, nil)
		}
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	}
}

// -- Test helpers -------------------------------------------------------------

func setupRouter(p *processapi.ProcessProvider) *gin.Engine {
	r := gin.New()
	rg := r.Group(p.BasePath())
	p.RegisterRoutes(rg)
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
	require.NoError(t, err)

	return raw.(*process.Service)
}

func connectWS(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
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
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))

		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)

		for _, line := range strings.Split(strings.TrimSpace(string(payload)), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}

			var msg corews.Message
			require.NoError(t, json.Unmarshal([]byte(line), &msg))

			if _, ok := want[msg.Channel]; ok {
				events[msg.Channel] = msg
			}
		}
	}

	require.Len(t, events, len(channels))
	return events
}
