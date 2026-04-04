// SPDX-Licence-Identifier: EUPL-1.2

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { connectProcessEvents, type ProcessEvent } from './shared/events.js';
import { ProcessApi, type ProcessInfo } from './shared/api.js';

/**
 * <core-process-list> — Running processes with status and actions.
 *
 * Displays managed processes from the process service. Shows status badges,
 * uptime, exit codes, and provides kill/select actions.
 *
 * Emits `process-selected` event when a process row is clicked, carrying
 * the process ID for the output viewer.
 *
 * The list is seeded from the REST API and then kept in sync with the live
 * process event stream when a WebSocket URL is configured.
 */
@customElement('core-process-list')
export class ProcessList extends LitElement {
  static styles = css`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
    }

    .list {
      display: flex;
      flex-direction: column;
      gap: 0.375rem;
    }

    .item {
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      padding: 0.75rem 1rem;
      background: #fff;
      display: flex;
      justify-content: space-between;
      align-items: center;
      cursor: pointer;
      transition: box-shadow 0.15s, border-colour 0.15s;
    }

    .item:hover {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
    }

    .item.selected {
      border-colour: #6366f1;
      box-shadow: 0 0 0 1px #6366f1;
    }

    .item-info {
      flex: 1;
    }

    .item-command {
      font-weight: 600;
      font-size: 0.9375rem;
      font-family: monospace;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .item-meta {
      font-size: 0.75rem;
      colour: #6b7280;
      margin-top: 0.25rem;
      display: flex;
      gap: 1rem;
    }

    .status-badge {
      font-size: 0.6875rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .status-badge.running {
      background: #dbeafe;
      colour: #1e40af;
    }

    .status-badge.pending {
      background: #fef3c7;
      colour: #92400e;
    }

    .status-badge.exited {
      background: #dcfce7;
      colour: #166534;
    }

    .status-badge.failed {
      background: #fef2f2;
      colour: #991b1b;
    }

    .status-badge.killed {
      background: #fce7f3;
      colour: #9d174d;
    }

    .exit-code {
      font-family: monospace;
      font-size: 0.6875rem;
      background: #f3f4f6;
      padding: 0.0625rem 0.375rem;
      border-radius: 0.25rem;
    }

    .exit-code.nonzero {
      background: #fef2f2;
      colour: #991b1b;
    }

    .pid-badge {
      font-family: monospace;
      background: #f3f4f6;
      padding: 0.0625rem 0.375rem;
      border-radius: 0.25rem;
      font-size: 0.6875rem;
    }

    .item-actions {
      display: flex;
      gap: 0.5rem;
    }

    button.kill-btn {
      padding: 0.375rem 0.75rem;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      cursor: pointer;
      transition: background 0.15s;
      background: #fff;
      colour: #dc2626;
      border: 1px solid #dc2626;
    }

    button.kill-btn:hover {
      background: #fef2f2;
    }

    button.kill-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #9ca3af;
      font-size: 0.875rem;
    }

    .loading {
      text-align: center;
      padding: 2rem;
      colour: #6b7280;
    }

    .error {
      colour: #dc2626;
      padding: 0.75rem;
      background: #fef2f2;
      border-radius: 0.375rem;
      font-size: 0.875rem;
      margin-bottom: 1rem;
    }

    .info-notice {
      padding: 0.75rem;
      background: #eff6ff;
      border: 1px solid #bfdbfe;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      colour: #1e40af;
      margin-bottom: 1rem;
    }
  `;

  @property({ attribute: 'api-url' }) apiUrl = '';
  @property({ attribute: 'ws-url' }) wsUrl = '';
  @property({ attribute: 'selected-id' }) selectedId = '';

  @state() private processes: ProcessInfo[] = [];
  @state() private loading = false;
  @state() private error = '';
  @state() private connected = false;
  @state() private killing = new Set<string>();

  private api!: ProcessApi;
  private ws: WebSocket | null = null;

  connectedCallback() {
    super.connectedCallback();
    this.api = new ProcessApi(this.apiUrl);
    this.loadProcesses();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.disconnect();
  }

  updated(changed: Map<string, unknown>) {
    if (changed.has('apiUrl')) {
      this.api = new ProcessApi(this.apiUrl);
    }

    if (changed.has('wsUrl') || changed.has('apiUrl')) {
      this.disconnect();
      void this.loadProcesses();
    }
  }

  async loadProcesses() {
    this.loading = true;
    this.error = '';
    try {
      this.processes = await this.api.listProcesses();
      if (this.wsUrl) {
        this.connect();
      }
    } catch (e: any) {
      this.error = e.message ?? 'Failed to load processes';
      this.processes = [];
    } finally {
      this.loading = false;
    }
  }

  private handleSelect(proc: ProcessInfo) {
    this.dispatchEvent(
      new CustomEvent('process-selected', {
        detail: { id: proc.id },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private async handleKill(proc: ProcessInfo) {
    this.killing = new Set([...this.killing, proc.id]);
    try {
      await this.api.killProcess(proc.id);
      await this.loadProcesses();
    } catch (e: any) {
      this.error = e.message ?? 'Failed to kill process';
    } finally {
      const next = new Set(this.killing);
      next.delete(proc.id);
      this.killing = next;
    }
  }

  private connect() {
    if (!this.wsUrl || this.ws) {
      return;
    }

    this.ws = connectProcessEvents(this.wsUrl, (event: ProcessEvent) => {
      this.applyEvent(event);
    });

    this.ws.onopen = () => {
      this.connected = true;
    };
    this.ws.onclose = () => {
      this.connected = false;
    };
  }

  private disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.connected = false;
  }

  private applyEvent(event: ProcessEvent) {
    const channel = event.channel ?? event.type ?? '';
    const data = (event.data ?? {}) as Partial<ProcessInfo> & { id?: string };

    if (!data.id) {
      return;
    }

    const next = new Map(this.processes.map((proc) => [proc.id, proc] as const));
    const current = next.get(data.id);

    switch (channel) {
      case 'process.started':
        next.set(data.id, this.normalizeProcess(data, current, 'running'));
        break;
      case 'process.exited':
        next.set(data.id, this.normalizeProcess(data, current, data.exitCode === -1 && data.error ? 'failed' : 'exited'));
        break;
      case 'process.killed':
        next.set(data.id, this.normalizeProcess(data, current, 'killed'));
        break;
      default:
        return;
    }

    this.processes = this.sortProcesses(next);
  }

  private normalizeProcess(
    data: Partial<ProcessInfo> & { id: string; error?: unknown },
    current: ProcessInfo | undefined,
    status: ProcessInfo['status'],
  ): ProcessInfo {
    const startedAt = data.startedAt ?? current?.startedAt ?? new Date().toISOString();
    return {
      id: data.id,
      command: data.command ?? current?.command ?? '',
      args: data.args ?? current?.args ?? [],
      dir: data.dir ?? current?.dir ?? '',
      startedAt,
      running: status === 'running',
      status,
      exitCode: data.exitCode ?? current?.exitCode ?? (status === 'killed' ? -1 : 0),
      duration: data.duration ?? current?.duration ?? 0,
      pid: data.pid ?? current?.pid ?? 0,
    };
  }

  private sortProcesses(processes: Map<string, ProcessInfo>): ProcessInfo[] {
    return [...processes.values()].sort((a, b) => {
      const aStarted = new Date(a.startedAt).getTime();
      const bStarted = new Date(b.startedAt).getTime();
      if (aStarted === bStarted) {
        return a.id.localeCompare(b.id);
      }
      return aStarted - bStarted;
    });
  }

  private formatUptime(started: string): string {
    try {
      const ms = Date.now() - new Date(started).getTime();
      const seconds = Math.floor(ms / 1000);
      if (seconds < 60) return `${seconds}s`;
      const minutes = Math.floor(seconds / 60);
      if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
      const hours = Math.floor(minutes / 60);
      return `${hours}h ${minutes % 60}m`;
    } catch {
      return 'unknown';
    }
  }

  render() {
    if (this.loading) {
      return html`<div class="loading">Loading processes\u2026</div>`;
    }

    return html`
      ${this.error ? html`<div class="error">${this.error}</div>` : nothing}
      ${this.processes.length === 0
        ? html`
            <div class="info-notice">
              ${this.wsUrl
                ? this.connected
                  ? 'Receiving live process updates.'
                  : 'Connecting to the process event stream...'
                : 'Managed processes are loaded from the process REST API.'}
            </div>
            <div class="empty">No managed processes.</div>
          `
        : html`
            <div class="list">
              ${this.processes.map(
                (proc) => html`
                  <div
                    class="item ${this.selectedId === proc.id ? 'selected' : ''}"
                    @click=${() => this.handleSelect(proc)}
                  >
                    <div class="item-info">
                      <div class="item-command">
                        <span>${proc.command} ${proc.args?.join(' ') ?? ''}</span>
                        <span class="status-badge ${proc.status}">${proc.status}</span>
                      </div>
                      <div class="item-meta">
                        <span class="pid-badge">PID ${proc.pid}</span>
                        <span>${proc.id}</span>
                        ${proc.dir ? html`<span>${proc.dir}</span>` : nothing}
                        ${proc.status === 'running'
                          ? html`<span>Up ${this.formatUptime(proc.startedAt)}</span>`
                          : nothing}
                        ${proc.status === 'exited'
                          ? html`<span class="exit-code ${proc.exitCode !== 0 ? 'nonzero' : ''}">
                              exit ${proc.exitCode}
                            </span>`
                          : nothing}
                      </div>
                    </div>
                    ${proc.status === 'running'
                      ? html`
                          <div class="item-actions">
                            <button
                              class="kill-btn"
                              ?disabled=${this.killing.has(proc.id)}
                              @click=${(e: Event) => {
                                e.stopPropagation();
                                void this.handleKill(proc);
                              }}
                            >
                              ${this.killing.has(proc.id) ? 'Killing\u2026' : 'Kill'}
                            </button>
                          </div>
                        `
                      : nothing}
                  </div>
                `,
              )}
            </div>
          `}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'core-process-list': ProcessList;
  }
}
