// SPDX-Licence-Identifier: EUPL-1.2

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	goapi "forge.lthn.ai/core/api"
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
	p := processapi.NewProvider(nil, nil)
	assert.Equal(t, "process", p.Name())
}

func TestProcessProvider_BasePath_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil)
	assert.Equal(t, "/api/process", p.BasePath())
}

func TestProcessProvider_Channels_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil)
	channels := p.Channels()
	assert.Contains(t, channels, "process.daemon.started")
	assert.Contains(t, channels, "process.daemon.stopped")
	assert.Contains(t, channels, "process.daemon.health")
}

func TestProcessProvider_Describe_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil)
	descs := p.Describe()
	assert.GreaterOrEqual(t, len(descs), 4)

	// Verify all descriptions have required fields
	for _, d := range descs {
		assert.NotEmpty(t, d.Method)
		assert.NotEmpty(t, d.Path)
		assert.NotEmpty(t, d.Summary)
		assert.NotEmpty(t, d.Tags)
	}
}

func TestProcessProvider_ListDaemons_Good(t *testing.T) {
	// Use a temp directory so the registry has no daemons
	dir := t.TempDir()
	registry := newTestRegistry(dir)
	p := processapi.NewProvider(registry, nil)

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
	p := processapi.NewProvider(registry, nil)

	r := setupRouter(p)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/process/daemons/test/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProcessProvider_RegistersAsRouteGroup_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil)

	engine, err := goapi.New()
	require.NoError(t, err)

	engine.Register(p)
	assert.Len(t, engine.Groups(), 1)
	assert.Equal(t, "process", engine.Groups()[0].Name())
}

func TestProcessProvider_Channels_RegisterAsStreamGroup_Good(t *testing.T) {
	p := processapi.NewProvider(nil, nil)

	engine, err := goapi.New()
	require.NoError(t, err)

	engine.Register(p)

	// Engine.Channels() discovers StreamGroups
	channels := engine.Channels()
	assert.Contains(t, channels, "process.daemon.started")
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
