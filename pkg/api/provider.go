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
	"sync"
	"syscall"
	"time"

	"dappco.re/go/core"
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
	actions  sync.Once
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
	p.registerProcessEvents()
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
	rg.POST("/processes/:id/wait", p.waitProcess)
	rg.POST("/processes/:id/input", p.inputProcess)
	rg.POST("/processes/:id/close-stdin", p.closeProcessStdin)
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
			Path:        "/processes/:id/wait",
			Summary:     "Wait for a managed process",
			Description: "Blocks until the process exits and returns the final process snapshot. Non-zero exits include the snapshot in the error details payload.",
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
			Method:      "POST",
			Path:        "/processes/:id/input",
			Summary:     "Write process input",
			Description: "Writes the provided input string to a managed process stdin pipe.",
			Tags:        []string{"process"},
			RequestBody: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input": map[string]any{"type": "string"},
				},
				"required": []string{"input"},
			},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"written": map[string]any{"type": "boolean"},
				},
			},
		},
		{
			Method:      "POST",
			Path:        "/processes/:id/close-stdin",
			Summary:     "Close process stdin",
			Description: "Closes the stdin pipe of a managed process so it can exit cleanly.",
			Tags:        []string{"process"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"closed": map[string]any{"type": "boolean"},
				},
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
	for _, entry := range entries {
		p.emitEvent("process.daemon.started", daemonEventPayload(entry))
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
	p.emitEvent("process.daemon.started", daemonEventPayload(*entry))
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

func (p *ProcessProvider) waitProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	info, err := p.service.Wait(c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case err == process.ErrProcessNotFound:
			status = http.StatusNotFound
		case info.Status == process.StatusExited || info.Status == process.StatusKilled:
			status = http.StatusConflict
		}
		c.JSON(status, api.FailWithDetails("wait_failed", err.Error(), info))
		return
	}

	c.JSON(http.StatusOK, api.OK(info))
}

type processInputRequest struct {
	Input string `json:"input"`
}

func (p *ProcessProvider) inputProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	var req processInputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Fail("invalid_request", err.Error()))
		return
	}

	if err := p.service.Input(c.Param("id"), req.Input); err != nil {
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound || err == process.ErrProcessNotRunning {
			status = http.StatusNotFound
		}
		c.JSON(status, api.Fail("input_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.OK(map[string]any{"written": true}))
}

func (p *ProcessProvider) closeProcessStdin(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, api.Fail("service_unavailable", "process service is not configured"))
		return
	}

	if err := p.service.CloseStdin(c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, api.Fail("close_stdin_failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.OK(map[string]any{"closed": true}))
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
	msg := ws.Message{
		Type: ws.TypeEvent,
		Data: data,
	}
	_ = p.hub.Broadcast(ws.Message{
		Type:    msg.Type,
		Channel: channel,
		Data:    data,
	})
	_ = p.hub.SendToChannel(channel, msg)
}

func daemonEventPayload(entry process.DaemonEntry) map[string]any {
	return map[string]any{
		"code":    entry.Code,
		"daemon":  entry.Daemon,
		"pid":     entry.PID,
		"health":  entry.Health,
		"project": entry.Project,
		"binary":  entry.Binary,
		"started": entry.Started,
	}
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

func (p *ProcessProvider) registerProcessEvents() {
	if p == nil || p.hub == nil || p.service == nil {
		return
	}

	coreApp := p.service.Core()
	if coreApp == nil {
		return
	}

	p.actions.Do(func() {
		coreApp.RegisterAction(func(_ *core.Core, msg core.Message) core.Result {
			p.forwardProcessEvent(msg)
			return core.Result{OK: true}
		})
	})
}

func (p *ProcessProvider) forwardProcessEvent(msg core.Message) {
	switch m := msg.(type) {
	case process.ActionProcessStarted:
		payload := p.processEventPayload(m.ID)
		payload["id"] = m.ID
		payload["command"] = m.Command
		payload["args"] = append([]string(nil), m.Args...)
		payload["dir"] = m.Dir
		payload["pid"] = m.PID
		if _, ok := payload["startedAt"]; !ok {
			payload["startedAt"] = time.Now().UTC()
		}
		p.emitEvent("process.started", payload)
	case process.ActionProcessOutput:
		p.emitEvent("process.output", map[string]any{
			"id":     m.ID,
			"line":   m.Line,
			"stream": m.Stream,
		})
	case process.ActionProcessExited:
		payload := p.processEventPayload(m.ID)
		payload["id"] = m.ID
		payload["exitCode"] = m.ExitCode
		payload["duration"] = m.Duration
		if m.Error != nil {
			payload["error"] = m.Error.Error()
		}
		p.emitEvent("process.exited", payload)
	case process.ActionProcessKilled:
		payload := p.processEventPayload(m.ID)
		payload["id"] = m.ID
		payload["signal"] = m.Signal
		payload["exitCode"] = -1
		p.emitEvent("process.killed", payload)
	}
}

func (p *ProcessProvider) processEventPayload(id string) map[string]any {
	if p == nil || p.service == nil || id == "" {
		return map[string]any{}
	}

	proc, err := p.service.Get(id)
	if err != nil {
		return map[string]any{}
	}

	info := proc.Info()
	return map[string]any{
		"id":        info.ID,
		"command":   info.Command,
		"args":      append([]string(nil), info.Args...),
		"dir":       info.Dir,
		"startedAt": info.StartedAt,
		"running":   info.Running,
		"status":    info.Status,
		"exitCode":  info.ExitCode,
		"duration":  info.Duration,
		"pid":       info.PID,
	}
}
