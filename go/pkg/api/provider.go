// SPDX-Licence-Identifier: EUPL-1.2

// Package api provides a service provider that wraps go-process daemon
// management as REST endpoints with WebSocket event streaming.
package api

import (
	"context"
	"net/http"
	"strconv"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"syscall"
	"time"

	"dappco.re/go"
	process "dappco.re/go/process"
	"github.com/gin-gonic/gin"
)

// ProcessProvider wraps the go-process daemon Registry as a service provider.
type ProcessProvider struct {
	registry *process.Registry
	service  *process.Service
	runner   *process.Runner
	hub      any
	actions  sync.Once
}

// NewProvider creates a process provider backed by the given daemon registry
// and optional process service for pipeline execution.
//
// The WS hub is used to emit daemon state change events. Pass nil for hub
// if WebSocket streaming is not needed.
func NewProvider(registry *process.Registry, service *process.Service, hub any) *ProcessProvider {
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

// Name identifies the provider for structural route registration.
func (p *ProcessProvider) Name() string { return "process" }

// BasePath returns the mount point for structural route registration.
func (p *ProcessProvider) BasePath() string { return "/api/process" }

// Register mounts the provider on a Gin router using the provider base path.
func (p *ProcessProvider) Register(r gin.IRouter) {
	if p == nil || r == nil {
		return
	}
	p.RegisterRoutes(r.Group(p.BasePath()))
}

// Element declares the custom element used by GUI consumers.
func (p *ProcessProvider) Element() elementSpec {
	return elementSpec{
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

// RegisterRoutes mounts all process routes under rg.
func (p *ProcessProvider) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/daemons", p.listDaemons)
	rg.GET("/daemons/:code/:daemon", p.getDaemon)
	rg.POST("/daemons/:code/:daemon/stop", p.stopDaemon)
	rg.GET("/daemons/:code/:daemon/health", p.healthCheck)
	rg.GET("/processes", p.listProcesses)
	rg.POST("/processes", p.startProcess)
	rg.POST("/processes/run", p.runProcess)
	rg.GET("/processes/:id", p.getProcess)
	rg.GET("/processes/:id/output", p.getProcessOutput)
	rg.POST("/processes/:id/wait", p.waitProcess)
	rg.POST("/processes/:id/input", p.inputProcess)
	rg.POST("/processes/:id/close-stdin", p.closeProcessStdin)
	rg.POST("/processes/:id/kill", p.killProcess)
	rg.POST("/processes/:id/signal", p.signalProcess)

	// RFC-compatible singular aliases.
	rg.GET("/process/list", p.listProcessIDs)
	rg.POST("/process/start", p.startProcessRFC)
	rg.POST("/process/run", p.runProcess)
	rg.GET("/process/:id", p.getProcess)
	rg.GET("/process/:id/output", p.getProcessOutput)
	rg.POST("/process/kill", p.killProcessJSON)
	rg.POST("/process/:id/wait", p.waitProcess)
	rg.POST("/process/:id/input", p.inputProcess)
	rg.POST("/process/:id/close-stdin", p.closeProcessStdin)
	rg.POST("/process/:id/signal", p.signalProcess)

	rg.POST("/pipelines/run", p.runPipeline)
}

// -- Handlers -----------------------------------------------------------------

func (p *ProcessProvider) listDaemons(c *gin.Context) {
	result := p.registry.List()
	if !result.OK {
		c.JSON(http.StatusInternalServerError, fail("list_failed", result.Error()))
		return
	}
	entries := result.Value.([]process.DaemonEntry)
	if entries == nil {
		entries = []process.DaemonEntry{}
	}
	for _, entry := range entries {
		p.emitEvent("process.daemon.started", daemonEventPayload(entry))
	}
	c.JSON(http.StatusOK, core.Ok(entries))
}

func (p *ProcessProvider) getDaemon(c *gin.Context) {
	code := c.Param("code")
	daemon := c.Param("daemon")

	entry, ok := p.registry.Get(code, daemon)
	if !ok {
		c.JSON(http.StatusNotFound, fail("not_found", "daemon not found or not running"))
		return
	}
	p.emitEvent("process.daemon.started", daemonEventPayload(*entry))
	c.JSON(http.StatusOK, core.Ok(entry))
}

func (p *ProcessProvider) stopDaemon(c *gin.Context) {
	code := c.Param("code")
	daemon := c.Param("daemon")

	entry, ok := p.registry.Get(code, daemon)
	if !ok {
		c.JSON(http.StatusNotFound, fail("not_found", "daemon not found or not running"))
		return
	}

	// Send SIGTERM to the process.
	if err := syscall.Kill(entry.PID, syscall.SIGTERM); err != nil {
		c.JSON(http.StatusInternalServerError, fail("signal_failed", err.Error()))
		return
	}

	// Remove from registry
	if result := p.registry.Unregister(code, daemon); !result.OK {
		c.JSON(http.StatusInternalServerError, fail("unregister_failed", result.Error()))
		return
	}

	// Emit WS event
	p.emitEvent("process.daemon.stopped", map[string]any{
		"code":   code,
		"daemon": daemon,
		"pid":    entry.PID,
	})

	c.JSON(http.StatusOK, core.Ok(map[string]any{"stopped": true}))
}

func (p *ProcessProvider) healthCheck(c *gin.Context) {
	code := c.Param("code")
	daemon := c.Param("daemon")

	entry, ok := p.registry.Get(code, daemon)
	if !ok {
		c.JSON(http.StatusNotFound, fail("not_found", "daemon not found or not running"))
		return
	}

	if entry.Health == "" {
		c.JSON(http.StatusOK, core.Ok(map[string]any{
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
	c.JSON(statusCode, core.Ok(result))
}

func (p *ProcessProvider) listProcesses(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	procs := p.service.List()
	if runningOnly, _ := strconv.ParseBool(c.Query("runningOnly")); runningOnly {
		procs = p.service.Running()
	}
	infos := make([]process.Info, 0, len(procs))
	for _, proc := range procs {
		infos = append(infos, proc.Info())
	}

	c.JSON(http.StatusOK, core.Ok(infos))
}

func (p *ProcessProvider) startProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	var req process.TaskProcessStart
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}
	if core.Trim(req.Command) == "" {
		c.JSON(http.StatusBadRequest, fail("invalid_request", "command is required"))
		return
	}

	result := p.service.StartWithOptions(c.Request.Context(), startRunOptions(req))
	if !result.OK {
		c.JSON(http.StatusInternalServerError, fail("start_failed", result.Error()))
		return
	}
	proc := result.Value.(*process.Process)

	c.JSON(http.StatusOK, core.Ok(proc.Info()))
}

func (p *ProcessProvider) startProcessRFC(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	var req process.TaskProcessStart
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}
	if core.Trim(req.Command) == "" {
		c.JSON(http.StatusBadRequest, fail("invalid_request", "command is required"))
		return
	}

	result := p.service.StartWithOptions(c.Request.Context(), startRunOptions(req))
	if !result.OK {
		c.JSON(http.StatusInternalServerError, fail("start_failed", result.Error()))
		return
	}
	proc := result.Value.(*process.Process)

	c.JSON(http.StatusOK, core.Ok(proc.ID))
}

func (p *ProcessProvider) runProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	var req process.TaskProcessRun
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}
	if core.Trim(req.Command) == "" {
		c.JSON(http.StatusBadRequest, fail("invalid_request", "command is required"))
		return
	}

	result := p.service.RunWithOptions(c.Request.Context(), process.RunOptions(req))
	if !result.OK {
		c.JSON(http.StatusInternalServerError, failWithDetails("run_failed", result.Error(), map[string]any{
			"output": "",
		}))
		return
	}

	c.JSON(http.StatusOK, core.Ok(result.Value.(string)))
}

func (p *ProcessProvider) getProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	result := p.service.Get(c.Param("id"))
	if !result.OK {
		c.JSON(http.StatusNotFound, fail("not_found", result.Error()))
		return
	}
	proc := result.Value.(*process.Process)

	c.JSON(http.StatusOK, core.Ok(proc.Info()))
}

func (p *ProcessProvider) listProcessIDs(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	procs := p.service.List()
	if runningOnly, _ := strconv.ParseBool(c.Query("runningOnly")); runningOnly {
		procs = p.service.Running()
	}

	ids := make([]string, 0, len(procs))
	for _, proc := range procs {
		ids = append(ids, proc.ID)
	}

	c.JSON(http.StatusOK, core.Ok(ids))
}

func (p *ProcessProvider) getProcessOutput(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	result := p.service.Output(c.Param("id"))
	if !result.OK {
		status := http.StatusInternalServerError
		if result.Value == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, fail("not_found", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(result.Value.(string)))
}

func (p *ProcessProvider) waitProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	result := p.service.Wait(c.Param("id"))
	if !result.OK {
		status := http.StatusInternalServerError
		info := process.Info{}
		if waitErr, ok := result.Value.(*process.TaskProcessWaitError); ok {
			info = waitErr.Info
		}
		switch {
		case result.Value == process.ErrProcessNotFound:
			status = http.StatusNotFound
		case info.Status == process.StatusExited || info.Status == process.StatusKilled:
			status = http.StatusConflict
		}
		c.JSON(status, failWithDetails("wait_failed", result.Error(), info))
		return
	}

	c.JSON(http.StatusOK, core.Ok(result.Value.(process.Info)))
}

type processInputRequest struct {
	Input string `json:"input"`
}

func (p *ProcessProvider) inputProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	var req processInputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}

	if result := p.service.Input(c.Param("id"), req.Input); !result.OK {
		err, _ := result.Value.(error)
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound || err == process.ErrProcessNotRunning {
			status = http.StatusNotFound
		}
		c.JSON(status, fail("input_failed", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(map[string]any{"written": true}))
}

func (p *ProcessProvider) closeProcessStdin(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	if result := p.service.CloseStdin(c.Param("id")); !result.OK {
		err, _ := result.Value.(error)
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, fail("close_stdin_failed", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(map[string]any{"closed": true}))
}

func (p *ProcessProvider) killProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	id := c.Param("id")
	pid, _ := pidFromString(id)
	if result := p.killProcessByTarget(id, pid); !result.OK {
		err, _ := result.Value.(error)
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, fail("kill_failed", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(map[string]any{"killed": true}))
}

type processKillRequest struct {
	ID  string `json:"id"`
	PID int    `json:"pid"`
}

func (p *ProcessProvider) killProcessJSON(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	var req processKillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}
	if req.ID == "" && req.PID <= 0 {
		c.JSON(http.StatusBadRequest, fail("invalid_request", "id or pid is required"))
		return
	}
	if req.PID <= 0 {
		if parsedPID, ok := pidFromString(req.ID); ok {
			req.PID = parsedPID
		}
	}

	if result := p.killProcessByTarget(req.ID, req.PID); !result.OK {
		err, _ := result.Value.(error)
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, fail("kill_failed", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(map[string]any{"killed": true}))
}

func (p *ProcessProvider) killProcessByTarget(id string, pid int) core.Result {
	if result := p.service.Kill(id); !result.OK {
		if pid <= 0 {
			return result
		}
		if pidResult := p.service.KillPID(pid); !pidResult.OK {
			return pidResult
		}
		return core.Ok(nil)
	}
	if id == "" && pid > 0 {
		if result := p.service.KillPID(pid); !result.OK {
			return result
		}
	}
	return core.Ok(nil)
}

type processSignalRequest struct {
	Signal string `json:"signal"`
}

func (p *ProcessProvider) signalProcess(c *gin.Context) {
	if p.service == nil {
		c.JSON(http.StatusServiceUnavailable, fail("service_unavailable", "process service is not configured"))
		return
	}

	var req processSignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}

	sigResult := parseSignal(req.Signal)
	if !sigResult.OK {
		c.JSON(http.StatusBadRequest, fail("invalid_signal", sigResult.Error()))
		return
	}
	sig := sigResult.Value.(syscall.Signal)

	id := c.Param("id")
	if result := p.service.Signal(id, sig); !result.OK {
		err, _ := result.Value.(error)
		if pid, ok := pidFromString(id); ok {
			pidResult := p.service.SignalPID(pid, sig)
			if pidResult.OK {
				c.JSON(http.StatusOK, core.Ok(map[string]any{"signalled": true}))
				return
			}
			result = pidResult
			err, _ = result.Value.(error)
		}
		status := http.StatusInternalServerError
		if err == process.ErrProcessNotFound || err == process.ErrProcessNotRunning {
			status = http.StatusNotFound
		}
		c.JSON(status, fail("signal_failed", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(map[string]any{"signalled": true}))
}

type pipelineRunRequest struct {
	Mode  string            `json:"mode"`
	Specs []process.RunSpec `json:"specs"`
}

func (p *ProcessProvider) runPipeline(c *gin.Context) {
	if p.runner == nil {
		c.JSON(http.StatusServiceUnavailable, fail("runner_unavailable", "pipeline runner is not configured"))
		return
	}

	var req pipelineRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, fail("invalid_request", err.Error()))
		return
	}

	mode := core.Lower(core.Trim(req.Mode))
	if mode == "" {
		mode = "all"
	}

	ctx := c.Request.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var result core.Result

	switch mode {
	case "all":
		result = p.runner.RunAll(ctx, req.Specs)
	case "sequential":
		result = p.runner.RunSequential(ctx, req.Specs)
	case "parallel":
		result = p.runner.RunParallel(ctx, req.Specs)
	default:
		c.JSON(http.StatusBadRequest, fail("invalid_mode", "mode must be one of: all, sequential, parallel"))
		return
	}
	if !result.OK {
		c.JSON(http.StatusBadRequest, fail("pipeline_failed", result.Error()))
		return
	}

	c.JSON(http.StatusOK, core.Ok(result.Value.(*process.RunAllResult)))
}

// emitEvent sends a WS event if the hub is available.
func (p *ProcessProvider) emitEvent(channel string, data any) {
	if p.hub == nil {
		return
	}
	emitHubEvent(p.hub, channel, data)
}

func daemonEventPayload(entry process.DaemonEntry) map[string]any {
	return map[string]any{
		"code":      entry.Code,
		"daemon":    entry.Daemon,
		"pid":       entry.PID,
		"health":    entry.Health,
		"project":   entry.Project,
		"binary":    entry.Binary,
		"started":   entry.Started,
		"startedAt": entry.Started,
	}
}

// PIDAlive checks whether a PID is still running. Exported for use by
// consumers that need to verify daemon liveness outside the REST API.
func PIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, syscall.Signal(0)) == nil
}

// intParam parses a URL param as int, returning 0 on failure.
func pidFromString(value string) (int, bool) {
	pid, err := strconv.Atoi(core.Trim(value))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func startRunOptions(req process.TaskProcessStart) process.RunOptions {
	return process.RunOptions{
		Command:        req.Command,
		Args:           req.Args,
		Dir:            req.Dir,
		Env:            req.Env,
		DisableCapture: req.DisableCapture,
		// RFC semantics for process.start are detached/background execution.
		Detach:      true,
		Timeout:     req.Timeout,
		GracePeriod: req.GracePeriod,
		KillGroup:   req.KillGroup,
	}
}

func parseSignal(value string) core.Result {
	trimmed := core.Trim(core.Upper(value))
	if trimmed == "" {
		return core.Fail(core.E("ProcessProvider.parseSignal", "signal is required", nil))
	}

	if n, err := strconv.Atoi(trimmed); err == nil {
		return core.Ok(syscall.Signal(n))
	}

	switch trimmed {
	case "SIGTERM", "TERM":
		return core.Ok(syscall.SIGTERM)
	case "SIGKILL", "KILL":
		return core.Ok(syscall.SIGKILL)
	case "SIGINT", "INT":
		return core.Ok(syscall.SIGINT)
	case "SIGQUIT", "QUIT":
		return core.Ok(syscall.SIGQUIT)
	case "SIGHUP", "HUP":
		return core.Ok(syscall.SIGHUP)
	case "SIGSTOP", "STOP":
		return core.Ok(syscall.SIGSTOP)
	case "SIGCONT", "CONT":
		return core.Ok(syscall.SIGCONT)
	case "SIGUSR1", "USR1":
		return core.Ok(syscall.SIGUSR1)
	case "SIGUSR2", "USR2":
		return core.Ok(syscall.SIGUSR2)
	default:
		return core.Fail(core.E("ProcessProvider.parseSignal", "unsupported signal", nil))
	}
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
			return core.Ok(nil)
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

	result := p.service.Get(id)
	if !result.OK {
		return map[string]any{}
	}
	proc := result.Value.(*process.Process)

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
