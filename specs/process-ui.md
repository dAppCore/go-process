# @core/process-ui
**Import:** `@core/process-ui`
**Files:** 8

## Types

### `DaemonEntry`
`interface`

Daemon-registry row returned by `ProcessApi.listDaemons` and `ProcessApi.getDaemon`.

Properties:
- `code: string`: Application or component code.
- `daemon: string`: Daemon name.
- `pid: number`: Process ID.
- `health?: string`: Optional health-endpoint address.
- `project?: string`: Optional project label.
- `binary?: string`: Optional binary label.
- `started: string`: Start timestamp string from the API.

### `HealthResult`
`interface`

Result returned by the daemon health endpoint.

Properties:
- `healthy: boolean`: Health outcome.
- `address: string`: Health endpoint address that was checked.
- `reason?: string`: Optional explanation such as the absence of a health endpoint.

### `ProcessInfo`
`interface`

Process snapshot shape used by the UI package.

Properties:
- `id: string`: Managed-process identifier.
- `command: string`: Executable name.
- `args: string[]`: Command arguments.
- `dir: string`: Working directory.
- `startedAt: string`: Start timestamp string.
- `status: 'pending' | 'running' | 'exited' | 'failed' | 'killed'`: Process status string.
- `exitCode: number`: Exit code.
- `duration: number`: Numeric duration value from the API payload.
- `pid: number`: Child PID.

### `RunResult`
`interface`

Pipeline result row used by `ProcessRunner`.

Properties:
- `name: string`: Spec name.
- `exitCode: number`: Exit code.
- `duration: number`: Numeric duration value.
- `output: string`: Captured output.
- `error?: string`: Optional error message.
- `skipped: boolean`: Whether the spec was skipped.
- `passed: boolean`: Whether the spec passed.

### `RunAllResult`
`interface`

Aggregate pipeline result consumed by `ProcessRunner`.

Properties:
- `results: RunResult[]`: Per-spec results.
- `duration: number`: Aggregate duration.
- `passed: number`: Count of passed specs.
- `failed: number`: Count of failed specs.
- `skipped: number`: Count of skipped specs.
- `success: boolean`: Aggregate success flag.

### `ProcessApi`
`class`

Typed fetch client for `/api/process/*`.

Public API:
- `new ProcessApi(baseUrl?: string)`: Stores an optional URL prefix. The default is `""`.
- `listDaemons(): Promise<DaemonEntry[]>`: Fetches `GET /api/process/daemons`.
- `getDaemon(code: string, daemon: string): Promise<DaemonEntry>`: Fetches one daemon entry.
- `stopDaemon(code: string, daemon: string): Promise<{ stopped: boolean }>`: Sends `POST /api/process/daemons/:code/:daemon/stop`.
- `healthCheck(code: string, daemon: string): Promise<HealthResult>`: Fetches `GET /api/process/daemons/:code/:daemon/health`.

### `ProcessEvent`
`interface`

Event envelope consumed by `connectProcessEvents`.

Properties:
- `type: string`: Event type.
- `channel?: string`: Optional channel name.
- `data?: any`: Event payload.
- `timestamp?: string`: Optional timestamp string.

### `ProcessPanel`
`class`

Top-level custom element registered as `<core-process-panel>`.

Public properties:
- `apiUrl: string`: Forwarded to child elements through the `api-url` attribute.
- `wsUrl: string`: WebSocket endpoint URL from the `ws-url` attribute.

Behavior:
- Renders tabbed daemon, process, and pipeline views.
- Opens a process-event WebSocket when `wsUrl` is set.
- Shows the last received process channel or event type in the footer.

### `ProcessDaemons`
`class`

Daemon-list custom element registered as `<core-process-daemons>`.

Public properties:
- `apiUrl: string`: Base URL prefix for `ProcessApi`.

Behavior:
- Loads daemon entries on connect.
- Can trigger per-daemon health checks and stop requests.
- Emits `daemon-stopped` after a successful stop request.

### `ProcessList`
`class`

Managed-process list custom element registered as `<core-process-list>`.

Public properties:
- `apiUrl: string`: Declared API prefix property.
- `selectedId: string`: Selected process ID, reflected from `selected-id`.

Behavior:
- Emits `process-selected` when a row is chosen.
- Currently renders from local state only because the process REST endpoints referenced by the component are not implemented in this package.

### `ProcessOutput`
`class`

Live output custom element registered as `<core-process-output>`.

Public properties:
- `apiUrl: string`: Declared API prefix property. The current implementation does not use it.
- `wsUrl: string`: WebSocket endpoint URL.
- `processId: string`: Selected process ID from the `process-id` attribute.

Behavior:
- Connects to the WebSocket when both `wsUrl` and `processId` are present.
- Filters for `process.output` events whose payload `data.id` matches `processId`.
- Appends output lines and auto-scrolls by default.

### `ProcessRunner`
`class`

Pipeline-results custom element registered as `<core-process-runner>`.

Public properties:
- `apiUrl: string`: Declared API prefix property.
- `result: RunAllResult | null`: Aggregate pipeline result used for rendering.

Behavior:
- Renders summary counts plus expandable per-spec output.
- Depends on the `result` property today because pipeline REST endpoints are not implemented in the package.

## Functions

### Package Functions

- `function connectProcessEvents(wsUrl: string, handler: (event: ProcessEvent) => void): WebSocket`: Opens a WebSocket, parses incoming JSON, forwards only messages whose `type` or `channel` starts with `process.`, ignores malformed payloads, and returns the `WebSocket` instance.

### `ProcessPanel` Methods

- `connectedCallback(): void`: Calls the LitElement base implementation and opens the WebSocket when `wsUrl` is set.
- `disconnectedCallback(): void`: Calls the LitElement base implementation and closes the current WebSocket.
- `render(): unknown`: Renders the header, tab strip, active child element, and connection footer.

### `ProcessDaemons` Methods

- `connectedCallback(): void`: Instantiates `ProcessApi` and loads daemon data.
- `loadDaemons(): Promise<void>`: Fetches daemon entries, stores them in component state, and records any request error message.
- `render(): unknown`: Renders the daemon list, loading state, empty state, and action buttons.

### `ProcessList` Methods

- `connectedCallback(): void`: Calls the LitElement base implementation and invokes `loadProcesses`.
- `loadProcesses(): Promise<void>`: Current placeholder implementation that clears state because the referenced process REST endpoints are not implemented yet.
- `render(): unknown`: Renders the process list or an informational empty state explaining the missing REST support.

### `ProcessOutput` Methods

- `connectedCallback(): void`: Calls the LitElement base implementation and opens the WebSocket when `wsUrl` and `processId` are both set.
- `disconnectedCallback(): void`: Calls the LitElement base implementation and closes the current WebSocket.
- `updated(changed: Map<string, unknown>): void`: Reconnects when `processId` or `wsUrl` changes, resets buffered lines on reconnection, and auto-scrolls when enabled.
- `render(): unknown`: Renders the output panel, waiting state, and accumulated stdout or stderr lines.

### `ProcessRunner` Methods

- `connectedCallback(): void`: Calls the LitElement base implementation and invokes `loadResults`.
- `loadResults(): Promise<void>`: Current placeholder method. The implementation is empty because pipeline endpoints are not present.
- `render(): unknown`: Renders the empty-state notice when `result` is absent, or the aggregate summary plus per-spec details when `result` is present.

### `ProcessApi` Methods

- `listDaemons(): Promise<DaemonEntry[]>`: Returns the `data` field from a successful daemon-list response.
- `getDaemon(code: string, daemon: string): Promise<DaemonEntry>`: Returns one daemon entry from the provider API.
- `stopDaemon(code: string, daemon: string): Promise<{ stopped: boolean }>`: Issues the stop request and returns the provider's `{ stopped }` payload.
- `healthCheck(code: string, daemon: string): Promise<HealthResult>`: Returns the daemon-health payload.
