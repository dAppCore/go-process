# api
**Import:** `dappco.re/go/core/process/pkg/api`
**Files:** 2

## Types

### `ProcessProvider`
`struct`

Service provider that wraps the go-process daemon registry and bundled UI entrypoint.

Exported fields:
- None.

## Functions

### Package Functions

- `func NewProvider(registry *process.Registry, hub *ws.Hub) *ProcessProvider`: Returns a `ProcessProvider` for the supplied registry and WebSocket hub. When `registry` is `nil`, it uses `process.DefaultRegistry()`.
- `func PIDAlive(pid int) bool`: Returns `false` for non-positive PIDs and otherwise reports whether `os.FindProcess(pid)` followed by signal `0` succeeds.

### `ProcessProvider` Methods

- `func (p *ProcessProvider) Name() string`: Returns `"process"`.
- `func (p *ProcessProvider) BasePath() string`: Returns `"/api/process"`.
- `func (p *ProcessProvider) Element() provider.ElementSpec`: Returns an element spec with tag `core-process-panel` and source `/assets/core-process.js`.
- `func (p *ProcessProvider) Channels() []string`: Returns `process.daemon.started`, `process.daemon.stopped`, `process.daemon.health`, `process.started`, `process.output`, `process.exited`, and `process.killed`.
- `func (p *ProcessProvider) RegisterRoutes(rg *gin.RouterGroup)`: Registers the daemon list, daemon lookup, daemon stop, and daemon health routes.
- `func (p *ProcessProvider) Describe() []api.RouteDescription`: Returns static route descriptions for the registered daemon routes.
