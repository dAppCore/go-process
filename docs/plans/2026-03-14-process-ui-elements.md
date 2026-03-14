# Process UI Elements — Lit Custom Elements

**Date**: 2026-03-14
**Status**: Complete

## Overview

Add Lit custom elements for process management, following the go-scm UI pattern exactly. The UI provides a tabbed panel with views for daemons, processes, process output, and pipeline runner status.

## 1. Review Existing ProcessProvider

The ProcessProvider (`pkg/api/provider.go`) already has:

- **REST endpoints**:
  - `GET /api/process/daemons` — List running daemons (auto-prunes dead PIDs)
  - `GET /api/process/daemons/:code/:daemon` — Get single daemon status
  - `POST /api/process/daemons/:code/:daemon/stop` — Stop daemon (SIGTERM + unregister)
  - `GET /api/process/daemons/:code/:daemon/health` — Health check probe

- **WS channels** (via `provider.Streamable`):
  - `process.daemon.started`
  - `process.daemon.stopped`
  - `process.daemon.health`

- **Implements**: `provider.Provider`, `provider.Streamable`, `provider.Describable`

- **Missing**: `provider.Renderable` — needs `Element()` method returning `ElementSpec{Tag, Source}`

- **Missing channels**: Process-level events (`process.output`, `process.started`, `process.exited`, `process.killed`) — the Go IPC actions exist but are not declared as WS channels

### Changes needed to provider.go

1. Add `provider.Renderable` interface compliance
2. Add `Element()` method returning `{Tag: "core-process-panel", Source: "/assets/core-process.js"}`
3. Add process-level WS channels to `Channels()`

## 2. Create ui/ directory

Scaffold the same structure as go-scm/ui:

```
ui/
├── package.json          # @core/process-ui, lit dep
├── tsconfig.json         # ES2021 + decorators
├── vite.config.ts        # lib mode → ../pkg/api/ui/dist/core-process.js
├── index.html            # Demo page
└── src/
    ├── index.ts           # Bundle entry, exports all elements
    ├── process-panel.ts   # <core-process-panel> — top-level tabbed panel
    ├── process-daemons.ts # <core-process-daemons> — daemon registry list
    ├── process-list.ts    # <core-process-list> — running processes
    ├── process-output.ts  # <core-process-output> — live stdout/stderr stream
    ├── process-runner.ts  # <core-process-runner> — pipeline run results
    └── shared/
        ├── api.ts         # ProcessApi fetch wrapper
        └── events.ts      # WS event filter for process.* events
```

## 3. Lit Elements

### 3.1 shared/api.ts — ProcessApi

Typed fetch wrapper for `/api/process/*` endpoints:

- `listDaemons()` → `GET /daemons`
- `getDaemon(code, daemon)` → `GET /daemons/:code/:daemon`
- `stopDaemon(code, daemon)` → `POST /daemons/:code/:daemon/stop`
- `healthCheck(code, daemon)` → `GET /daemons/:code/:daemon/health`

### 3.2 shared/events.ts — connectProcessEvents

WebSocket event filter for `process.*` channels. Same pattern as go-scm `connectScmEvents`.

### 3.3 process-panel.ts — `<core-process-panel>`

Top-level panel with tabs: Daemons / Processes / Pipelines. HLCRF layout:
- H: Title bar ("Process Manager") + Refresh button
- H-L: Tab navigation
- C: Active tab content
- F: WS connection status + last event

### 3.4 process-daemons.ts — `<core-process-daemons>`

Daemon registry list showing:
- Code + Daemon name
- PID + Started timestamp
- Health status badge (healthy/unhealthy/no endpoint)
- Project + Binary metadata
- Stop button (POST stop endpoint)
- Health check button (GET health endpoint)

### 3.5 process-list.ts — `<core-process-list>`

Running processes from Service.List():
- Process ID + Command + Args
- Status badge (pending/running/exited/failed/killed)
- PID + uptime (since StartedAt)
- Exit code (if exited)
- Kill button (for running processes)
- Click to select → emits `process-selected` event for output viewer

Note: This element requires process-level REST endpoints that do not exist in the current provider. The element will be built but will show placeholder state until those endpoints are added.

### 3.6 process-output.ts — `<core-process-output>`

Live stdout/stderr stream for a selected process:
- Receives process ID via attribute
- Subscribes to `process.output` WS events filtered by ID
- Terminal-style output area with monospace font
- Auto-scroll to bottom
- Stream type indicator (stdout/stderr with colour coding)

### 3.7 process-runner.ts — `<core-process-runner>`

Pipeline execution results display:
- RunSpec list with name, command, status
- Dependency chain visualisation (After field)
- Pass/Fail/Skip badges
- Duration display
- Aggregate summary (passed/failed/skipped counts)

Note: Like process-list, this needs runner REST endpoints. Built as display-ready element.

## 4. Create pkg/api/embed.go

```go
//go:embed all:ui/dist
var Assets embed.FS
```

Same pattern as go-scm. The `ui/dist/` directory must exist (created by `npm run build` in ui/).

## 5. Update ProcessProvider

Add to `provider.go`:

1. Compile-time check: `_ provider.Renderable = (*ProcessProvider)(nil)`
2. `Element()` method
3. Extend `Channels()` with process-level events

## 6. Build Verification

1. `cd ui && npm install && npm run build` — produces `pkg/api/ui/dist/core-process.js`
2. `go build ./...` from go-process root — verifies embed and provider compile

## Tasks

- [x] Write implementation plan
- [x] Create ui/package.json, tsconfig.json, vite.config.ts
- [x] Create ui/index.html demo page
- [x] Create ui/src/shared/api.ts
- [x] Create ui/src/shared/events.ts
- [x] Create ui/src/process-panel.ts
- [x] Create ui/src/process-daemons.ts
- [x] Create ui/src/process-list.ts
- [x] Create ui/src/process-output.ts
- [x] Create ui/src/process-runner.ts
- [x] Create ui/src/index.ts
- [x] Create pkg/api/ui/dist/.gitkeep (placeholder for embed)
- [x] Create pkg/api/embed.go
- [x] Update pkg/api/provider.go — add Renderable + channels
- [x] Build UI: npm install + npm run build (57.50 kB, gzip: 13.64 kB)
- [x] Verify: go build ./... (pass) + go test ./pkg/api/... (8/8 pass)
