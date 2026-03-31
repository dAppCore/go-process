// SPDX-Licence-Identifier: EUPL-1.2

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { connectProcessEvents, type ProcessEvent } from './shared/events.js';
import type { ProcessInfo } from './shared/api.js';

/**
 * <core-process-list> — Running processes with status and actions.
 *
 * Displays managed processes from the process service. Shows status badges,
 * uptime, exit codes, and provides kill/select actions.
 *
 * Emits `process-selected` event when a process row is clicked, carrying
 * the process ID for the output viewer.
 *
 * Note: Requires process-level REST endpoints (GET /processes, POST /processes/:id/kill)
 * that are not yet in the provider. The element renders from WS events and local state
 * until those endpoints are available.
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

  private ws: WebSocket | null = null;

  connectedCallback() {
    super.connectedCallback();
    this.loadProcesses();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.disconnect();
  }

  updated(changed: Map<string, unknown>) {
    if (changed.has('wsUrl')) {
      this.disconnect();
      this.processes = [];
      this.loadProcesses();
    }
  }

  async loadProcesses() {
    // The process list is built from the shared process event stream.
    this.error = '';
    this.loading = false;

    if (!this.wsUrl) {
      this.processes = [];
      return;
    }

    this.connect();
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

  private connect() {
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
    const data = (event.data ?? {}) as Partial<ProcessInfo> & {
      id?: string;
      signal?: string;
    };

    if (!data.id) {
      return;
    }

    const next = new Map(this.processes.map((proc) => [proc.id, proc] as const));
    const current = next.get(data.id);

    if (channel === 'process.started') {
      next.set(data.id, this.normalizeProcess(data, current, 'running'));
      this.processes = this.sortProcesses(next);
      return;
    }

    if (channel === 'process.exited') {
      next.set(data.id, this.normalizeProcess(data, current, 'exited'));
      this.processes = this.sortProcesses(next);
      return;
    }

    if (channel === 'process.killed') {
      next.set(data.id, this.normalizeProcess(data, current, 'killed'));
      this.processes = this.sortProcesses(next);
      return;
    }
  }

  private normalizeProcess(
    data: Partial<ProcessInfo> & { id: string; signal?: string },
    current: ProcessInfo | undefined,
    status: ProcessInfo['status'],
  ): ProcessInfo {
    return {
      id: data.id,
      command: data.command ?? current?.command ?? '',
      args: data.args ?? current?.args ?? [],
      dir: data.dir ?? current?.dir ?? '',
      startedAt: data.startedAt ?? current?.startedAt ?? new Date().toISOString(),
      status,
      exitCode: data.exitCode ?? current?.exitCode ?? (status === 'killed' ? -1 : 0),
      duration: data.duration ?? current?.duration ?? 0,
      pid: data.pid ?? current?.pid ?? 0,
    };
  }

  private sortProcesses(processes: Map<string, ProcessInfo>): ProcessInfo[] {
    return [...processes.values()].sort(
      (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
    );
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
                  ? 'Waiting for process events from the WebSocket feed.'
                  : 'Connecting to the process event stream...'
                : 'Set a WebSocket URL to receive live process events.'}
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
                              disabled
                              @click=${(e: Event) => {
                                e.stopPropagation();
                              }}
                            >
                              Live only
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
