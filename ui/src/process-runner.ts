// SPDX-Licence-Identifier: EUPL-1.2

import { LitElement, html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import type { RunResult, RunAllResult } from './shared/api.js';

/**
 * <core-process-runner> — Pipeline execution status display.
 *
 * Shows RunSpec execution results with pass/fail/skip badges, duration,
 * dependency chains, and aggregate summary.
 *
 * Note: Pipeline runner REST endpoints are not yet in the provider.
 * This element renders from WS events and accepts data via properties
 * until those endpoints are available.
 */
@customElement('core-process-runner')
export class ProcessRunner extends LitElement {
  static styles = css`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
    }

    .summary {
      display: flex;
      gap: 1rem;
      padding: 0.75rem 1rem;
      background: #fff;
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      margin-bottom: 1rem;
      align-items: center;
    }

    .summary-stat {
      display: flex;
      flex-direction: column;
      align-items: center;
    }

    .summary-value {
      font-weight: 700;
      font-size: 1.25rem;
    }

    .summary-label {
      font-size: 0.6875rem;
      colour: #6b7280;
      text-transform: uppercase;
      letter-spacing: 0.025em;
    }

    .summary-value.passed {
      colour: #166534;
    }

    .summary-value.failed {
      colour: #991b1b;
    }

    .summary-value.skipped {
      colour: #92400e;
    }

    .summary-duration {
      margin-left: auto;
      font-size: 0.8125rem;
      colour: #6b7280;
    }

    .overall-badge {
      font-size: 0.75rem;
      padding: 0.25rem 0.75rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .overall-badge.success {
      background: #dcfce7;
      colour: #166534;
    }

    .overall-badge.failure {
      background: #fef2f2;
      colour: #991b1b;
    }

    .list {
      display: flex;
      flex-direction: column;
      gap: 0.375rem;
    }

    .spec {
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      padding: 0.75rem 1rem;
      background: #fff;
      transition: box-shadow 0.15s;
    }

    .spec:hover {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
    }

    .spec-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .spec-name {
      font-weight: 600;
      font-size: 0.9375rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .spec-meta {
      font-size: 0.75rem;
      colour: #6b7280;
      margin-top: 0.25rem;
      display: flex;
      gap: 1rem;
    }

    .result-badge {
      font-size: 0.6875rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .result-badge.passed {
      background: #dcfce7;
      colour: #166534;
    }

    .result-badge.failed {
      background: #fef2f2;
      colour: #991b1b;
    }

    .result-badge.skipped {
      background: #fef3c7;
      colour: #92400e;
    }

    .duration {
      font-family: monospace;
      font-size: 0.75rem;
      colour: #6b7280;
    }

    .deps {
      font-size: 0.6875rem;
      colour: #9ca3af;
    }

    .spec-output {
      margin-top: 0.5rem;
      padding: 0.5rem 0.75rem;
      background: #1e1e1e;
      border-radius: 0.375rem;
      font-family: 'SF Mono', 'Monaco', 'Menlo', 'Consolas', monospace;
      font-size: 0.75rem;
      line-height: 1.5;
      colour: #d4d4d4;
      white-space: pre-wrap;
      word-break: break-all;
      max-height: 12rem;
      overflow-y: auto;
    }

    .spec-error {
      margin-top: 0.375rem;
      font-size: 0.75rem;
      colour: #dc2626;
    }

    .toggle-output {
      font-size: 0.6875rem;
      colour: #6366f1;
      cursor: pointer;
      background: none;
      border: none;
      padding: 0;
      margin-top: 0.375rem;
    }

    .toggle-output:hover {
      text-decoration: underline;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #9ca3af;
      font-size: 0.875rem;
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
  @property({ type: Object }) result: RunAllResult | null = null;

  @state() private expandedOutputs = new Set<string>();

  connectedCallback() {
    super.connectedCallback();
    this.loadResults();
  }

  async loadResults() {
    // Pipeline runner REST endpoints are not yet available.
    // Results can be passed in via the `result` property.
  }

  private toggleOutput(name: string) {
    const next = new Set(this.expandedOutputs);
    if (next.has(name)) {
      next.delete(name);
    } else {
      next.add(name);
    }
    this.expandedOutputs = next;
  }

  private formatDuration(ns: number): string {
    if (ns < 1_000_000) return `${(ns / 1000).toFixed(0)}\u00b5s`;
    if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(0)}ms`;
    return `${(ns / 1_000_000_000).toFixed(2)}s`;
  }

  private resultStatus(r: RunResult): string {
    if (r.skipped) return 'skipped';
    if (r.passed) return 'passed';
    return 'failed';
  }

  render() {
    if (!this.result) {
      return html`
        <div class="info-notice">
          Pipeline runner endpoints are pending. Pass pipeline results via the
          <code>result</code> property, or results will appear here once the REST
          API for pipeline execution is available.
        </div>
        <div class="empty">No pipeline results.</div>
      `;
    }

    const { results, duration, passed, failed, skipped, success } = this.result;

    return html`
      <div class="summary">
        <span class="overall-badge ${success ? 'success' : 'failure'}">
          ${success ? 'Passed' : 'Failed'}
        </span>
        <div class="summary-stat">
          <span class="summary-value passed">${passed}</span>
          <span class="summary-label">Passed</span>
        </div>
        <div class="summary-stat">
          <span class="summary-value failed">${failed}</span>
          <span class="summary-label">Failed</span>
        </div>
        <div class="summary-stat">
          <span class="summary-value skipped">${skipped}</span>
          <span class="summary-label">Skipped</span>
        </div>
        <span class="summary-duration">${this.formatDuration(duration)}</span>
      </div>

      <div class="list">
        ${results.map(
          (r) => html`
            <div class="spec">
              <div class="spec-header">
                <div class="spec-name">
                  <span>${r.name}</span>
                  <span class="result-badge ${this.resultStatus(r)}">${this.resultStatus(r)}</span>
                </div>
                <span class="duration">${this.formatDuration(r.duration)}</span>
              </div>
              <div class="spec-meta">
                ${r.exitCode !== 0 && !r.skipped
                  ? html`<span>exit ${r.exitCode}</span>`
                  : nothing}
              </div>
              ${r.error ? html`<div class="spec-error">${r.error}</div>` : nothing}
              ${r.output
                ? html`
                    <button class="toggle-output" @click=${() => this.toggleOutput(r.name)}>
                      ${this.expandedOutputs.has(r.name) ? 'Hide output' : 'Show output'}
                    </button>
                    ${this.expandedOutputs.has(r.name)
                      ? html`<div class="spec-output">${r.output}</div>`
                      : nothing}
                  `
                : nothing}
            </div>
          `,
        )}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'core-process-runner': ProcessRunner;
  }
}
