// SPDX-Licence-Identifier: EUPL-1.2

// Package api provides a service provider that wraps go-process daemon
// management as REST endpoints with WebSocket event streaming.
package api

import (
	"net/http"
	"os"
	"strconv"
	"syscall"

	process "dappco.re/go/core/process"
	"dappco.re/go/core/ws"
	"forge.lthn.ai/core/api"
	"forge.lthn.ai/core/api/pkg/provider"
	"github.com/gin-gonic/gin"
)

// ProcessProvider wraps the go-process daemon Registry as a service provider.
// It implements provider.Provider, provider.Streamable, provider.Describable,
// and provider.Renderable.
type ProcessProvider struct {
	registry *process.Registry
	hub      *ws.Hub
}

// compile-time interface checks
var (
	_ provider.Provider    = (*ProcessProvider)(nil)
	_ provider.Streamable  = (*ProcessProvider)(nil)
	_ provider.Describable = (*ProcessProvider)(nil)
	_ provider.Renderable  = (*ProcessProvider)(nil)
)

// NewProvider creates a process provider backed by the given daemon registry.
// The WS hub is used to emit daemon state change events. Pass nil for hub
// if WebSocket streaming is not needed.
func NewProvider(registry *process.Registry, hub *ws.Hub) *ProcessProvider {
	if registry == nil {
		registry = process.DefaultRegistry()
	}
	return &ProcessProvider{
		registry: registry,
		hub:      hub,
	}
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
					"project": map[string]any{"type": "string"},
					"binary":  map[string]any{"type": "string"},
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
			Description: "Probes the daemon's health endpoint and returns the result.",
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

	healthy := process.WaitForHealth(entry.Health, 2000)
	reason := ""
	if !healthy {
		reason = "health endpoint did not report healthy"
	}

	result := map[string]any{
		"healthy": healthy,
		"address": entry.Health,
		"reason":  reason,
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
