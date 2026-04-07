// SPDX-Licence-Identifier: EUPL-1.2

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { connectProcessEvents, type ProcessEvent } from './shared/events.js';
import { ProcessApi } from './shared/api.js';

interface OutputLine {
  text: string;
  stream: 'stdout' | 'stderr';
  timestamp: number;
}

/**
 * <core-process-output> — Live stdout/stderr stream for a selected process.
 *
 * Connects to the WS endpoint and filters for process.output events matching
 * the given process-id. Displays output in a terminal-style scrollable area
 * with colour-coded stream indicators (stdout/stderr).
 */
@customElement('core-process-output')
export class ProcessOutput extends LitElement {
  static styles = css`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
      margin-top: 0.75rem;
    }

    .output-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.5rem 0.75rem;
      background: #1e1e1e;
      border-radius: 0.5rem 0.5rem 0 0;
      colour: #d4d4d4;
      font-size: 0.75rem;
    }

    .output-title {
      font-weight: 600;
    }

    .output-actions {
      display: flex;
      gap: 0.5rem;
    }

    button.clear-btn {
      padding: 0.25rem 0.5rem;
      border-radius: 0.25rem;
      font-size: 0.6875rem;
      cursor: pointer;
      background: #333;
      colour: #d4d4d4;
      border: 1px solid #555;
      transition: background 0.15s;
    }

    button.clear-btn:hover {
      background: #444;
    }

    .auto-scroll-toggle {
      display: flex;
      align-items: center;
      gap: 0.25rem;
      font-size: 0.6875rem;
      colour: #d4d4d4;
      cursor: pointer;
    }

    .auto-scroll-toggle input {
      cursor: pointer;
    }

    .output-body {
      background: #1e1e1e;
      border-radius: 0 0 0.5rem 0.5rem;
      padding: 0.5rem 0.75rem;
      max-height: 24rem;
      overflow-y: auto;
      font-family: 'SF Mono', 'Monaco', 'Menlo', 'Consolas', monospace;
      font-size: 0.8125rem;
      line-height: 1.5;
    }

    .line {
      white-space: pre-wrap;
      word-break: break-all;
    }

    .line.stdout {
      colour: #d4d4d4;
    }

    .line.stderr {
      colour: #f87171;
    }

    .stream-tag {
      display: inline-block;
      width: 3rem;
      font-size: 0.625rem;
      font-weight: 600;
      text-transform: uppercase;
      opacity: 0.5;
      margin-right: 0.5rem;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #6b7280;
      font-size: 0.8125rem;
    }

    .waiting {
      colour: #9ca3af;
      font-style: italic;
      padding: 1rem;
      text-align: center;
      font-size: 0.8125rem;
    }
  `;

  @property({ attribute: 'api-url' }) apiUrl = '';
  @property({ attribute: 'ws-url' }) wsUrl = '';
  @property({ attribute: 'process-id' }) processId = '';

  @state() private lines: OutputLine[] = [];
  @state() private autoScroll = true;
  @state() private connected = false;
  @state() private loadingSnapshot = false;

  private ws: WebSocket | null = null;
  private api = new ProcessApi(this.apiUrl);
  private syncToken = 0;

  connectedCallback() {
    super.connectedCallback();
    this.syncSources();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.disconnect();
  }

  updated(changed: Map<string, unknown>) {
    if (changed.has('apiUrl')) {
      this.api = new ProcessApi(this.apiUrl);
    }

    if (changed.has('processId') || changed.has('wsUrl') || changed.has('apiUrl')) {
      this.syncSources();
    }

    if (this.autoScroll) {
      this.scrollToBottom();
    }
  }

  private syncSources() {
    this.disconnect();
    this.lines = [];
    if (!this.processId) {
      return;
    }

    void this.loadSnapshotAndConnect();
  }

  private async loadSnapshotAndConnect() {
    const token = ++this.syncToken;

    if (!this.processId) {
      return;
    }

    if (this.apiUrl) {
      this.loadingSnapshot = true;
      try {
        const output = await this.api.getProcessOutput(this.processId);
        if (token !== this.syncToken) {
          return;
        }
        const snapshot = this.linesFromOutput(output);
        if (snapshot.length > 0) {
          this.lines = snapshot;
        }
      } catch {
        // Ignore missing snapshot data and continue with live streaming.
      } finally {
        if (token === this.syncToken) {
          this.loadingSnapshot = false;
        }
      }
    }

    if (token === this.syncToken && this.wsUrl) {
      this.connect();
    }
  }

  private linesFromOutput(output: string): OutputLine[] {
    if (!output) {
      return [];
    }

    const normalized = output.replace(/\r\n/g, '\n');
    const parts = normalized.split('\n');
    if (parts.length > 0 && parts[parts.length - 1] === '') {
      parts.pop();
    }

    return parts.map((text) => ({
      text,
      stream: 'stdout' as const,
      timestamp: Date.now(),
    }));
  }

  private connect() {
    this.ws = connectProcessEvents(this.wsUrl, (event: ProcessEvent) => {
      const data = event.data;
      if (!data) return;

      // Filter for output events matching our process ID
      const channel = event.channel ?? event.type ?? '';
      if (channel === 'process.output' && data.id === this.processId) {
        this.lines = [
          ...this.lines,
          {
            text: data.line ?? '',
            stream: data.stream === 'stderr' ? 'stderr' : 'stdout',
            timestamp: Date.now(),
          },
        ];
      }
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

  private handleClear() {
    this.lines = [];
  }

  private handleAutoScrollToggle() {
    this.autoScroll = !this.autoScroll;
  }

  private scrollToBottom() {
    const body = this.shadowRoot?.querySelector('.output-body');
    if (body) {
      body.scrollTop = body.scrollHeight;
    }
  }

  render() {
    if (!this.processId) {
      return html`<div class="empty">Select a process to view its output.</div>`;
    }

    return html`
      <div class="output-header">
        <span class="output-title">Output: ${this.processId}</span>
        <div class="output-actions">
          <label class="auto-scroll-toggle">
            <input
              type="checkbox"
              ?checked=${this.autoScroll}
              @change=${this.handleAutoScrollToggle}
            />
            Auto-scroll
          </label>
          <button class="clear-btn" @click=${this.handleClear}>Clear</button>
        </div>
      </div>
      <div class="output-body">
        ${this.loadingSnapshot && this.lines.length === 0
          ? html`<div class="waiting">Loading snapshot\u2026</div>`
          : this.lines.length === 0
          ? html`<div class="waiting">Waiting for output\u2026</div>`
          : this.lines.map(
              (line) => html`
                <div class="line ${line.stream}">
                  <span class="stream-tag">${line.stream}</span>${line.text}
                </div>
              `,
            )}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'core-process-output': ProcessOutput;
  }
}
