// SPDX-Licence-Identifier: EUPL-1.2

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { ProcessApi, type DaemonEntry, type HealthResult } from './shared/api.js';

/**
 * <core-process-daemons> — Daemon registry list.
 * Shows registered daemons with status badges, health indicators, and stop buttons.
 */
@customElement('core-process-daemons')
export class ProcessDaemons extends LitElement {
  static styles = css`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
    }

    .list {
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
    }

    .item {
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      padding: 1rem;
      background: #fff;
      display: flex;
      justify-content: space-between;
      align-items: center;
      transition: box-shadow 0.15s;
    }

    .item:hover {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
    }

    .item-info {
      flex: 1;
    }

    .item-name {
      font-weight: 600;
      font-size: 0.9375rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .item-code {
      font-family: monospace;
      font-size: 0.8125rem;
      colour: #6366f1;
    }

    .item-meta {
      font-size: 0.75rem;
      colour: #6b7280;
      margin-top: 0.25rem;
      display: flex;
      gap: 1rem;
    }

    .pid-badge {
      font-family: monospace;
      background: #f3f4f6;
      padding: 0.0625rem 0.375rem;
      border-radius: 0.25rem;
      font-size: 0.6875rem;
    }

    .health-badge {
      font-size: 0.6875rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .health-badge.healthy {
      background: #dcfce7;
      colour: #166534;
    }

    .health-badge.unhealthy {
      background: #fef2f2;
      colour: #991b1b;
    }

    .health-badge.unknown {
      background: #f3f4f6;
      colour: #6b7280;
    }

    .health-badge.checking {
      background: #fef3c7;
      colour: #92400e;
    }

    .item-actions {
      display: flex;
      gap: 0.5rem;
    }

    button {
      padding: 0.375rem 0.75rem;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      cursor: pointer;
      transition: background 0.15s;
    }

    button.health-btn {
      background: #fff;
      colour: #6366f1;
      border: 1px solid #6366f1;
    }

    button.health-btn:hover {
      background: #eef2ff;
    }

    button.health-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    button.stop-btn {
      background: #fff;
      colour: #dc2626;
      border: 1px solid #dc2626;
    }

    button.stop-btn:hover {
      background: #fef2f2;
    }

    button.stop-btn:disabled {
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
  `;

  @property({ attribute: 'api-url' }) apiUrl = '';

  @state() private daemons: DaemonEntry[] = [];
  @state() private loading = true;
  @state() private error = '';
  @state() private stopping = new Set<string>();
  @state() private checking = new Set<string>();
  @state() private healthResults = new Map<string, HealthResult>();

  private api!: ProcessApi;

  connectedCallback() {
    super.connectedCallback();
    this.api = new ProcessApi(this.apiUrl);
    this.loadDaemons();
  }

  async loadDaemons() {
    this.loading = true;
    this.error = '';
    try {
      this.daemons = await this.api.listDaemons();
    } catch (e: any) {
      this.error = e.message ?? 'Failed to load daemons';
    } finally {
      this.loading = false;
    }
  }

  private daemonKey(d: DaemonEntry): string {
    return `${d.code}/${d.daemon}`;
  }

  private async handleStop(d: DaemonEntry) {
    const key = this.daemonKey(d);
    this.stopping = new Set([...this.stopping, key]);
    try {
      await this.api.stopDaemon(d.code, d.daemon);
      this.dispatchEvent(
        new CustomEvent('daemon-stopped', {
          detail: { code: d.code, daemon: d.daemon },
          bubbles: true,
        }),
      );
      await this.loadDaemons();
    } catch (e: any) {
      this.error = e.message ?? 'Failed to stop daemon';
    } finally {
      const next = new Set(this.stopping);
      next.delete(key);
      this.stopping = next;
    }
  }

  private async handleHealthCheck(d: DaemonEntry) {
    const key = this.daemonKey(d);
    this.checking = new Set([...this.checking, key]);
    try {
      const result = await this.api.healthCheck(d.code, d.daemon);
      const next = new Map(this.healthResults);
      next.set(key, result);
      this.healthResults = next;
    } catch (e: any) {
      this.error = e.message ?? 'Health check failed';
    } finally {
      const next = new Set(this.checking);
      next.delete(key);
      this.checking = next;
    }
  }

  private formatDate(iso: string): string {
    try {
      return new Date(iso).toLocaleDateString('en-GB', {
        day: 'numeric',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return iso;
    }
  }

  private renderHealthBadge(d: DaemonEntry) {
    const key = this.daemonKey(d);

    if (this.checking.has(key)) {
      return html`<span class="health-badge checking">Checking\u2026</span>`;
    }

    const result = this.healthResults.get(key);
    if (result) {
      return html`<span class="health-badge ${result.healthy ? 'healthy' : 'unhealthy'}">
        ${result.healthy ? 'Healthy' : 'Unhealthy'}
      </span>`;
    }

    if (!d.health) {
      return html`<span class="health-badge unknown">No health endpoint</span>`;
    }

    return html`<span class="health-badge unknown">Unchecked</span>`;
  }

  render() {
    if (this.loading) {
      return html`<div class="loading">Loading daemons\u2026</div>`;
    }

    return html`
      ${this.error ? html`<div class="error">${this.error}</div>` : nothing}
      ${this.daemons.length === 0
        ? html`<div class="empty">No daemons registered.</div>`
        : html`
            <div class="list">
              ${this.daemons.map((d) => {
                const key = this.daemonKey(d);
                return html`
                  <div class="item">
                    <div class="item-info">
                      <div class="item-name">
                        <span class="item-code">${d.code}</span>
                        <span>${d.daemon}</span>
                        ${this.renderHealthBadge(d)}
                      </div>
                      <div class="item-meta">
                        <span class="pid-badge">PID ${d.pid}</span>
                        <span>Started ${this.formatDate(d.started)}</span>
                        ${d.project ? html`<span>${d.project}</span>` : nothing}
                        ${d.binary ? html`<span>${d.binary}</span>` : nothing}
                      </div>
                    </div>
                    <div class="item-actions">
                      ${d.health
                        ? html`
                            <button
                              class="health-btn"
                              ?disabled=${this.checking.has(key)}
                              @click=${() => this.handleHealthCheck(d)}
                            >
                              ${this.checking.has(key) ? 'Checking\u2026' : 'Health'}
                            </button>
                          `
                        : nothing}
                      <button
                        class="stop-btn"
                        ?disabled=${this.stopping.has(key)}
                        @click=${() => this.handleStop(d)}
                      >
                        ${this.stopping.has(key) ? 'Stopping\u2026' : 'Stop'}
                      </button>
                    </div>
                  </div>
                `;
              })}
            </div>
          `}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'core-process-daemons': ProcessDaemons;
  }
}
