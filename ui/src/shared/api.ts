// SPDX-Licence-Identifier: EUPL-1.2

/**
 * Daemon entry as returned by the process registry REST API.
 */
export interface DaemonEntry {
  code: string;
  daemon: string;
  pid: number;
  health?: string;
  project?: string;
  binary?: string;
  started: string;
}

/**
 * Health check result from the daemon health endpoint.
 */
export interface HealthResult {
  healthy: boolean;
  address: string;
  reason?: string;
}

/**
 * Process info snapshot as returned by the process service.
 */
export interface ProcessInfo {
  id: string;
  command: string;
  args: string[];
  dir: string;
  startedAt: string;
  running: boolean;
  status: 'pending' | 'running' | 'exited' | 'failed' | 'killed';
  exitCode: number;
  duration: number;
  pid: number;
}

/**
 * RunSpec payload for pipeline execution.
 */
export interface RunSpec {
  name: string;
  command: string;
  args?: string[];
  dir?: string;
  env?: string[];
  after?: string[];
  allowFailure?: boolean;
}

/**
 * Pipeline run result for a single spec.
 */
export interface RunResult {
  name: string;
  exitCode: number;
  duration: number;
  output: string;
  error?: string;
  skipped: boolean;
  passed: boolean;
}

/**
 * Aggregate pipeline run result.
 */
export interface RunAllResult {
  results: RunResult[];
  duration: number;
  passed: number;
  failed: number;
  skipped: number;
  success: boolean;
}

/**
 * Process start and run payload shared by the control endpoints.
 */
export interface ProcessControlRequest {
  command: string;
  args?: string[];
  dir?: string;
  env?: string[];
  disableCapture?: boolean;
  detach?: boolean;
  timeout?: number;
  gracePeriod?: number;
  killGroup?: boolean;
}

/**
 * ProcessApi provides a typed fetch wrapper for the /api/process/* endpoints.
 */
export class ProcessApi {
  constructor(private baseUrl: string = '') {}

  private get base(): string {
    return `${this.baseUrl}/api/process`;
  }

  private async request<T>(path: string, opts?: RequestInit): Promise<T> {
    const res = await fetch(`${this.base}${path}`, opts);
    const json = await res.json();
    if (!json.success) {
      throw new Error(json.error?.message ?? 'Request failed');
    }
    return json.data as T;
  }

  /** List all alive daemons from the registry. */
  listDaemons(): Promise<DaemonEntry[]> {
    return this.request<DaemonEntry[]>('/daemons');
  }

  /** Get a single daemon entry by code and daemon name. */
  getDaemon(code: string, daemon: string): Promise<DaemonEntry> {
    return this.request<DaemonEntry>(`/daemons/${code}/${daemon}`);
  }

  /** Stop a daemon (SIGTERM + unregister). */
  stopDaemon(code: string, daemon: string): Promise<{ stopped: boolean }> {
    return this.request<{ stopped: boolean }>(`/daemons/${code}/${daemon}/stop`, {
      method: 'POST',
    });
  }

  /** Check daemon health endpoint. */
  healthCheck(code: string, daemon: string): Promise<HealthResult> {
    return this.request<HealthResult>(`/daemons/${code}/${daemon}/health`);
  }

  /** List all managed processes. */
  listProcesses(runningOnly = false): Promise<ProcessInfo[]> {
    const query = runningOnly ? '?runningOnly=true' : '';
    return this.request<ProcessInfo[]>(`/processes${query}`);
  }

  /** Get a single managed process by ID. */
  getProcess(id: string): Promise<ProcessInfo> {
    return this.request<ProcessInfo>(`/processes/${id}`);
  }

  /** Get the captured stdout/stderr for a managed process by ID. */
  getProcessOutput(id: string): Promise<string> {
    return this.request<string>(`/processes/${id}/output`);
  }

  /** Start a managed process asynchronously. */
  startProcess(opts: ProcessControlRequest): Promise<ProcessInfo> {
    return this.request<ProcessInfo>('/processes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(opts),
    });
  }

  /** Run a managed process synchronously and return its combined output. */
  runProcess(opts: ProcessControlRequest): Promise<string> {
    return this.request<string>('/processes/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(opts),
    });
  }

  /** Wait for a managed process to exit and return its final snapshot. */
  waitProcess(id: string): Promise<ProcessInfo> {
    return this.request<ProcessInfo>(`/processes/${id}/wait`, {
      method: 'POST',
    });
  }

  /** Write input to a managed process stdin pipe. */
  inputProcess(id: string, input: string): Promise<{ written: boolean }> {
    return this.request<{ written: boolean }>(`/processes/${id}/input`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input }),
    });
  }

  /** Close a managed process stdin pipe. */
  closeProcessStdin(id: string): Promise<{ closed: boolean }> {
    return this.request<{ closed: boolean }>(`/processes/${id}/close-stdin`, {
      method: 'POST',
    });
  }

  /** Kill a managed process by ID. */
  killProcess(id: string): Promise<{ killed: boolean }> {
    return this.request<{ killed: boolean }>(`/processes/${id}/kill`, {
      method: 'POST',
    });
  }

  /** Send a signal to a managed process by ID. */
  signalProcess(id: string, signal: string | number): Promise<{ signalled: boolean }> {
    return this.request<{ signalled: boolean }>(`/processes/${id}/signal`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ signal: String(signal) }),
    });
  }

  /** Run a process pipeline using the configured runner. */
  runPipeline(mode: 'all' | 'sequential' | 'parallel', specs: RunSpec[]): Promise<RunAllResult> {
    return this.request<RunAllResult>('/pipelines/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode, specs }),
    });
  }
}
