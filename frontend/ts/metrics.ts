import { getMetrics } from './api.js';

export async function renderMetrics(el: HTMLElement): Promise<void> {
  try {
    const m = await getMetrics();
    let nextUpdateHtml = '';
    if (m.next_update_time) {
      const d = new Date(m.next_update_time);
      nextUpdateHtml = `<div class="metric"><span class="metric-val">${d.toLocaleString()}</span> next update</div>`;
    }
    el.innerHTML = `
      <div class="metrics">
        <div class="metric"><span class="metric-val">${m.total_repos}</span> repos</div>
        <div class="metric${m.need_attention > 0 ? ' warn' : ''}"><span class="metric-val">${m.need_attention}</span> need attention</div>
        <div class="metric"><span class="metric-val">${m.total_size_gb.toFixed(2)}</span> GB</div>
        ${nextUpdateHtml}
      </div>
    `;
  } catch {
    el.innerHTML = '';
  }
}
