// SPDX-Licence-Identifier: EUPL-1.2

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	core "dappco.re/go/core"
	goapi "dappco.re/go/core/api"
	process "dappco.re/go/core/process"
	processapi "dappco.re/go/core/process/pkg/api"
	"github.com/gin-gonic/gin"
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
