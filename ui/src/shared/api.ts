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
  status: 'pending' | 'running' | 'exited' | 'failed' | 'killed';
  exitCode: number;
  duration: number;
  pid: number;
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
}
