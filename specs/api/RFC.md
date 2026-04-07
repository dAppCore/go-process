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

- `func NewProvider(registry *process.Registry, service *process.Service, hub *ws.Hub) *ProcessProvider`: Returns a `ProcessProvider` for the supplied registry, optional process service, and WebSocket hub. When `registry` is `nil`, it uses `process.DefaultRegistry()`. When `service` is non-`nil`, provider-runner features are enabled.
- `func PIDAlive(pid int) bool`: Returns `false` for non-positive PIDs and otherwise reports whether `os.FindProcess(pid)` followed by signal `0` succeeds.

### `ProcessProvider` Methods

- `func (p *ProcessProvider) Name() string`: Returns `"process"`.
- `func (p *ProcessProvider) BasePath() string`: Returns `"/api/process"`.
- `func (p *ProcessProvider) Element() provider.ElementSpec`: Returns an element spec with tag `core-process-panel` and source `/assets/core-process.js`.
- `func (p *ProcessProvider) Channels() []string`: Returns `process.daemon.started`, `process.daemon.stopped`, `process.daemon.health`, `process.started`, `process.output`, `process.exited`, and `process.killed`.
- `func (p *ProcessProvider) RegisterRoutes(rg *gin.RouterGroup)`: Registers daemon management routes (`GET /daemons`, `GET /daemons/:code/:daemon`, `POST /daemons/:code/:daemon/stop`, `GET /daemons/:code/:daemon/health`), process management routes (`GET /processes`, `POST /processes`, `POST /processes/run`, `GET /processes/:id`, `GET /processes/:id/output`, `POST /processes/:id/wait`, `POST /processes/:id/input`, `POST /processes/:id/close-stdin`, `POST /processes/:id/kill`, `POST /processes/:id/signal`), and pipeline routes (`POST /pipelines/run`).
- `func (p *ProcessProvider) Describe() []api.RouteDescription`: Returns static route descriptions for the registered daemon routes.
