// SPDX-Licence-Identifier: EUPL-1.2

// Package api provides a service provider that wraps go-process daemon
// management as REST endpoints with WebSocket event streaming.
package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"

	"dappco.re/go/core/api"
	"dappco.re/go/core/api/pkg/provider"
	process "dappco.re/go/core/process"
	"dappco.re/go/core/ws"
	"github.com/gin-gonic/gin"
)

// ProcessProvider wraps the go-process daemon Registry as a service provider.
// It implements provider.Provider, provider.Streamable, provider.Describable,
// and provider.Renderable.
type ProcessProvider struct {
	registry *process.Registry
	service  *process.Service
	runner   *process.Runner
	hub      *ws.Hub
}

// compile-time interface checks
var (
	_ provider.Provider    = (*ProcessProvider)(nil)
	_ provider.Streamable  = (*ProcessProvider)(nil)
	_ provider.Describable = (*ProcessProvider)(nil)
	_ provider.Renderable  = (*ProcessProvider)(nil)
)

// NewProvider creates a process provider backed by the given daemon registry
// and optional process service for pipeline execution.
//
// The WS hub is used to emit daemon state change events. Pass nil for hub
// if WebSocket streaming is not needed.
func NewProvider(registry *process.Registry, service *process.Service, hub *ws.Hub) *ProcessProvider {
	if registry == nil {
		registry = process.DefaultRegistry()
	}
	p := &ProcessProvider{
		registry: registry,
		service:  service,
		hub:      hub,
	}
	if service != nil {
		p.runner = process.NewRunner(service)
	}
	return p
}

// Name implements api.RouteGroup.
func (p *ProcessProvider) Name() string { return "process" }

// BasePath implements api.RouteGroup.
func (p *ProcessProvider) BasePath() string { return "/api/process" }

// Element implements provider.Renderable.
func (p *ProcessProvider) Element() provider.ElementSpec {
	return provider.ElementSpec{
		Tag:    "core-process-panel",
		Source: "/assets/core-process.js",
	}
}

// Channels implements provider.Streamable.
func (p *ProcessProvider) Channels() []string {
	return []string{
		"process.daemon.started",
		"process.daemon.stopped",
		"process.daemon.health",
		"process.started",
		"process.output",
		"process.exited",
		"process.killed",
	}
}

// RegisterRoutes implements api.RouteGroup.
func (p *ProcessProvider) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/daemons", p.listDaemons)
	rg.GET("/daemons/:code/:daemon", p.getDaemon)
	rg.POST("/daemons/:code/:daemon/stop", p.stopDaemon)
	rg.GET("/daemons/:code/:daemon/health", p.healthCheck)
	rg.GET("/processes", p.listProcesses)
	rg.GET("/processes/:id", p.getProcess)
	rg.GET("/processes/:id/output", p.getProcessOutput)
	rg.POST("/processes/:id/kill", p.killProcess)
	rg.POST("/pipelines/run", p.runPipeline)
}

// Describe implements api.DescribableGroup.
func (p *ProcessProvider) Describe() []api.RouteDescription {
	return []api.RouteDescription{
		{
			Method:      "GET",
			Path:        "/daemons",
			Summary:     "List running daemons",
			Description: "Returns all alive daemon entries from the registry, pruning any with dead PIDs.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":    map[string]any{"type": "string"},
						"daemon":  map[string]any{"type": "string"},
						"pid":     map[string]any{"type": "integer"},
						"health":  map[string]any{"type": "string"},
						"project": map[string]any{"type": "string"},
						"binary":  map[string]any{"type": "string"},
						"started": map[string]any{"type": "string", "format": "date-time"},
					},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/daemons/:code/:daemon",
			Summary:     "Get daemon status",
			Description: "Returns a single daemon entry if its process is alive.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"code":    map[string]any{"type": "string"},
					"daemon":  map[string]any{"type": "string"},
					"pid":     map[string]any{"type": "integer"},
					"health":  map[string]any{"type": "string"},
					"started": map[string]any{"type": "string", "format": "date-time"},
				},
			},
		},
		{
			Method:      "POST",
			Path:        "/daemons/:code/:daemon/stop",
			Summary:     "Stop a daemon",
			Description: "Sends SIGTERM to the daemon process and removes it from the registry.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"stopped": map[string]any{"type": "boolean"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/daemons/:code/:daemon/health",
			Summary:     "Check daemon health",
			Description: "Probes the daemon's health endpoint and returns the result, including a failure reason when unhealthy.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"healthy": map[string]any{"type": "boolean"},
					"address": map[string]any{"type": "string"},
					"reason":  map[string]any{"type": "string"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/processes",
			Summary:     "List managed processes",
			Description: "Returns the current process service snapshot as serialisable process info entries.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":        map[string]any{"type": "string"},
						"command":   map[string]any{"type": "string"},
						"args":      map[string]any{"type": "array"},
						"dir":       map[string]any{"type": "string"},
						"startedAt": map[string]any{"type": "string", "format": "date-time"},
						"running":   map[string]any{"type": "boolean"},
						"status":    map[string]any{"type": "string"},
						"exitCode":  map[string]any{"type": "integer"},
						"duration":  map[string]any{"type": "integer"},
						"pid":       map[string]any{"type": "integer"},
					},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/processes/:id",
			Summary:     "Get a managed process",
			Description: "Returns a single managed process by ID as a process info snapshot.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":        map[string]any{"type": "string"},
					"command":   map[string]any{"type": "string"},
					"args":      map[string]any{"type": "array"},
					"dir":       map[string]any{"type": "string"},
					"startedAt": map[string]any{"type": "string", "format": "date-time"},
					"running":   map[string]any{"type": "boolean"},
					"status":    map[string]any{"type": "string"},
					"exitCode":  map[string]any{"type": "integer"},
					"duration":  map[string]any{"type": "integer"},
					"pid":       map[string]any{"type": "integer"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/processes/:id/output",
			Summary:     "Get process output",
			Description: "Returns the captured stdout and stderr for a managed process.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "string",
			},
		},
		{
			Method:      "POST",
			Path:        "/processes/:id/kill",
			Summary:     "Kill a managed process",
			Description: "Sends SIGKILL to the managed process identified by ID.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"killed": map[string]any{"type": "boolean"},
				},
			},
		},
		{
			Method:      "POST",
			Path:        "/pipelines/run",
			Summary:     "Run a process pipeline",
			Description: "Executes a list of process specs using the configured runner in sequential, parallel, or dependency-aware mode.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"results": map[string]any{
						"type": "array",
					},
					"duration": map[string]any{"type": "integer"},
					"passed":   map[string]any{"type": "integer"},
					"failed":   map[string]any{"type": "integer"},
					"skipped":  map[string]any{"type": "integer"},
				},
			},
		},
	}
}

// -- Handlers -----------------------------------------------------------------

func (p *ProcessProvider) listDaemons(c *gin.Context) {
	entries, err := p.registry.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Fail("list_failed", err.Error()))
		return
	}
	if entries == nil {
		entries = []process.DaemonEntry{}
	}
	c.JSON(http.StatusOK, api.OK(entries))
}

func (p *ProcessProvider) getDaemon(c *gin.Context) {
	code := c.Param("code")
	daemon := c.Param("daemon")

	entry, ok := p.registry.Get(code, daemon)
	if !ok {
		c.JSON(http.StatusNotFound, api.Fail("not_found", "daemon not found or not running"))
		return
	}
	c.JSON(http.StatusOK, api.OK(entry))
}

func (p *ProcessProvider) stopDaemon(c *gin.Context) {
	code := c.Param("code")
	daemon := c.Param("daemon")

	entry, ok := p.registry.Get(code, daemon)
	if !ok {
		c.JSON(http.StatusNotFound, api.Fail("not_found", "daemon not found or not running"))
		return
	}

	// Send SIGTERM to the process
	proc, err := os.FindProcess(entry.PID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Fail("signal_failed", err.Error()))
		return
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		c.JSON(http.StatusInternalServerError, api.Fail("signal_failed", err.Error()))
		return
	}

	// Remove from registry
	_ = p.registry.Unregister(code, daemon)

	// Emit WS event
	p.emitEvent("process.daemon.stopped", map[string]any{
		"code":   code,
		"daemon": daemon,
		"pid":    entry.PID,
	})

	c.JSON(http.StatusOK, api.OK(map[string]any{"stopped": true}))
}

func (p *ProcessProvider) healthCheck(c *gin.Context) {
	code := c.Param("code")
	daemon := c.Param("daemon")

	entry, ok := p.registry.Get(code, daemon)
	if !ok {
		c.JSON(http.StatusNotFound, api.Fail("not_found", "daemon not found or not running"))
		return
	}

	if entry.Health == "" {
		c.JSON(http.StatusOK, api.OK(map[string]any{
			"healthy": false,
			"address": "",
			"reason":  "no health endpoint configured",
		}))
		return
	}

	healthy, reason := process.ProbeHealth(entry.Health, 2000)

	result := map[string]any{
		"healthy": healthy,
		"address": entry.Health,
	}
	if !healthy && reason != "" {
		result["reason"] = reason
	}

	// Emit health event
	p.emitEvent("process.daemon.health", map[string]any{
		"code":    code,
		"daemon":  daemon,
		"healthy": healthy,
		"reason":  reason,
	})

	statusCode := http.StatusOK
	if !healthy {
		statusCode = http.StatusServiceUnavailable
	}
	c.JSON(statusCode, api.OK(result))
}

func (p *ProcessProvider) listProcesses(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	procs := p.service.List()
	infos := make([]process.Info, 0, len(procs))
	for _, proc := range procs {
		infos = append(infos, proc.Info())
	}

	c.JSON(http.StatusOK, api.OK(infos))
}

func (p *ProcessProvider) getProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	proc, err := p.service.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, api.Fail("not_found", err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.OK(proc.Info()))
}

func (p *ProcessProvider) getProcessOutput(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	output, err := p.service.Output(c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, api.Fail("not_found", err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.OK(output))
}

func (p *ProcessProvider) killProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	if err := p.service.Kill(c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, api.Fail("kill_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.OK(map[string]any{"killed": true}))
}

type pipelineRunRequest struct {
	Mode  string            `json:"mode"`
	Specs []process.RunSpec `json:"specs"`
}

func (p *ProcessProvider) runPipeline(c *gin.Context) {
	if p.runner == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("runner_unavailable", "pipeline runner is not configured"))
		return
	}

	var req pipelineRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Fail("invalid_request", err.Error()))
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "all"
	}

	ctx := c.Request.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		result *process.RunAllResult
		err    error
	)

	switch mode {
	case "all":
		result, err = p.runner.RunAll(ctx, req.Specs)
	case "sequential":
		result, err = p.runner.RunSequential(ctx, req.Specs)
	case "parallel":
		result, err = p.runner.RunParallel(ctx, req.Specs)
	default:
		c.JSON(http.StatusBadRequest, api.Fail("invalid_mode", "mode must be one of: all, sequential, parallel"))
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, api.Fail("pipeline_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.OK(result))
}

// emitEvent sends a WS event if the hub is available.
func (p *ProcessProvider) emitEvent(channel string, data any) {
	if p.hub == nil {
		return
	}
	_ = p.hub.SendToChannel(channel, ws.Message{
		Type: ws.TypeEvent,
		Data: data,
	})
}

// PIDAlive checks whether a PID is still running. Exported for use by
// consumers that need to verify daemon liveness outside the REST API.
func PIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// intParam parses a URL param as int, returning 0 on failure.
func intParam(c *gin.Context, name string) int {
	v, _ := strconv.Atoi(c.Param(name))
	return v
}
