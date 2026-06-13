import { listRepos, deleteRepo, updateRepo, updateAllRepos, RepoSummary } from './api.js';
import { renderMetrics } from './metrics.js';
import { tagTextColor } from './util.js';

let sortField = getCookie('sort') || 'added_date';
let sortOrder = getCookie('order') || 'desc';
let searchTerm = '';

function parseHashSearch(): string {
  const m = location.hash.match(/\?search=(.*)/);
  return m ? decodeURIComponent(m[1]) : '';
}

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'));
  return match ? decodeURIComponent(match[1]) : null;
}

function setCookie(name: string, value: string, days: number = 365): void {
  const d = new Date();
  d.setTime(d.getTime() + days * 24 * 60 * 60 * 1000);
  document.cookie = name + '=' + encodeURIComponent(value) + ';expires=' + d.toUTCString() + ';path=/;SameSite=Lax';
}

export async function renderCatalog(main: HTMLElement): Promise<void> {
  searchTerm = parseHashSearch();
  main.innerHTML = `
    <div class="catalog">
      <div id="metrics-bar"></div>
      <div class="catalog-controls">
        <input type="text" id="search-input" placeholder="Search repos..." value="${esc(searchTerm)}">
        <select id="sort-select">
          <option value="added_date" ${sortField === 'added_date' ? 'selected' : ''}>Date Added</option>
          <option value="name" ${sortField === 'name' ? 'selected' : ''}>Name</option>
          <option value="last_updated" ${sortField === 'last_updated' ? 'selected' : ''}>Last Updated</option>
        </select>
        <button id="sort-order-btn">${sortOrder === 'desc' ? '↓ Desc' : '↑ Asc'}</button>
        <button id="add-repo-btn">+ Add Repo</button>
        <button id="update-all-btn">Update All</button>
        <button id="manage-tags-btn">Manage Tags</button>
      </div>
      <div id="repo-list" class="repo-list">Loading...</div>
    </div>
  `;

  renderMetrics(document.getElementById('metrics-bar')!);

  document.getElementById('search-input')!.addEventListener('input', (e) => {
    searchTerm = (e.target as HTMLInputElement).value.trim();
    updateHashSearch();
    loadRepos();
  });

  document.getElementById('sort-select')!.addEventListener('change', (e) => {
    sortField = (e.target as HTMLSelectElement).value;
    setCookie('sort', sortField);
    loadRepos();
  });

  document.getElementById('sort-order-btn')!.addEventListener('click', () => {
    sortOrder = sortOrder === 'desc' ? 'asc' : 'desc';
    setCookie('order', sortOrder);
    (document.getElementById('sort-order-btn') as HTMLButtonElement).textContent =
      sortOrder === 'desc' ? '↓ Desc' : '↑ Asc';
    loadRepos();
  });

  document.getElementById('add-repo-btn')!.addEventListener('click', showAddDialog);
  document.getElementById('update-all-btn')!.addEventListener('click', async () => {
    try {
      await updateAllRepos();
      alert('Update triggered. Check back in a moment.');
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });
  document.getElementById('manage-tags-btn')!.addEventListener('click', () => {
    location.hash = '#tags';
  });

  loadRepos();
}

async function loadRepos(): Promise<void> {
  const list = document.getElementById('repo-list')!;
  list.innerHTML = 'Loading...';

  try {
    const repos = await listRepos({
      sort: sortField,
      order: sortOrder,
      search: searchTerm || '',
      limit: '100',
    });

    if (repos.length === 0) {
      list.innerHTML = '<p class="empty">No repositories found. <a href="#" id="add-first">Add one</a>.</p>';
      document.getElementById('add-first')?.addEventListener('click', (e) => { e.preventDefault(); showAddDialog(); });
      return;
    }

    list.innerHTML = repos.map(r => repoCard(r)).join('');

    list.querySelectorAll('.repo-card').forEach(card => {
      card.addEventListener('click', (e) => {
        const target = e.target as HTMLElement;
        if (target.closest('button')) return;
        location.hash = '#repo/' + (card as HTMLElement).dataset.id;
      });
    });

    list.querySelectorAll('.delete-btn').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const id = parseInt((btn as HTMLElement).dataset.id!);
        if (!confirm('Delete this repository? Its local copy will be removed and it will be removed from the catalog.')) return;
        try {
          await deleteRepo(id);
          loadRepos();
        } catch (e: any) {
          alert('Error: ' + e.message);
        }
      });
    });

    list.querySelectorAll('.update-btn').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const id = parseInt((btn as HTMLElement).dataset.id!);
        try {
          await updateRepo(id);
          alert('Update triggered.');
        } catch (e: any) {
          alert('Error: ' + e.message);
        }
      });
    });

  } catch (e: any) {
    list.innerHTML = `<p class="error">Failed to load: ${esc(e.message)}</p>`;
  }
}

function repoCard(r: RepoSummary): string {
  const tags = r.tags.map(t =>
    `<span class="tag-chip" style="background:${esc(t.color)};color:${tagTextColor(t.color)}">${esc(t.name)}</span>`
  ).join('');
  const topics = r.topics.map(t => `<span class="topic-chip">${esc(t)}</span>`).join('');
  const lang = r.languages.slice(0, 3).map(l =>
    `<span class="lang-item">${esc(l.language_name)} ${l.percentage.toFixed(0)}%</span>`
  ).join(' ');
  const attn = r.needs_attention ? '<span class="attention-badge">!</span>' : '';
  const release = r.latest_release
    ? `<span class="release-tag">${esc(r.latest_release.tag_name)}</span>`
    : '';
  const lic = r.license ? `<span class="license">${esc(r.license)}</span>` : '';

  return `
    <div class="repo-card" data-id="${r.id}">
      <div class="repo-card-header">
        <span class="repo-name">${esc(r.owner)}/${attn}<strong>${esc(r.name)}</strong></span>
        <a href="${esc(r.github_url)}" target="_blank" class="gh-link" onclick="event.stopPropagation()">↗</a>
      </div>
      ${r.description ? `<p class="repo-desc">${esc(r.description)}</p>` : ''}
      <div class="repo-meta">
        ${lic} ${release} ${lang}
      </div>
      <div class="repo-chips">${topics}</div>
      <div class="repo-tags">${tags}</div>
      <div class="repo-actions">
        <button class="update-btn" data-id="${r.id}">Update</button>
        <button class="delete-btn" data-id="${r.id}">Remove</button>
      </div>
    </div>
  `;
}

function showAddDialog(): void {
  const html = `
    <div class="dialog-overlay" id="add-dialog">
      <div class="dialog">
        <h2>Add Repository</h2>
        <label>GitHub URL <input type="text" id="add-url" placeholder="https://github.com/owner/name"></label>
        <div class="dialog-actions">
          <button id="add-cancel">Cancel</button>
          <button id="add-confirm">Add</button>
        </div>
        <p id="add-error" class="error" style="display:none"></p>
      </div>
    </div>
  `;
  const overlay = document.createElement('div');
  overlay.innerHTML = html;
  document.body.appendChild(overlay.firstElementChild!);

  document.getElementById('add-cancel')!.addEventListener('click', () => {
    document.getElementById('add-dialog')!.remove();
  });

  document.getElementById('add-confirm')!.addEventListener('click', async () => {
    const url = (document.getElementById('add-url') as HTMLInputElement).value.trim();
    if (!url) return;

    const match = url.match(/github\.com\/([^\/]+)\/([^\/]+?)(?:\.git)?$/);
    if (!match) {
      document.getElementById('add-error')!.textContent = 'Invalid GitHub URL';
      document.getElementById('add-error')!.style.display = 'block';
      return;
    }

    try {
      const { addRepo } = await import('./api.js');
      await addRepo({ name: match[2], owner: match[1], github_url: url });
      document.getElementById('add-dialog')!.remove();
      loadRepos();
      renderMetrics(document.getElementById('metrics-bar')!);
    } catch (e: any) {
      document.getElementById('add-error')!.textContent = e.message;
      document.getElementById('add-error')!.style.display = 'block';
    }
  });
}

function esc(s: string): string {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function updateHashSearch(): void {
  if (searchTerm) {
    const base = location.hash.replace(/\?.*/, '');
    history.replaceState(null, '', base + '?search=' + encodeURIComponent(searchTerm));
  } else {
    history.replaceState(null, '', location.hash.replace(/\?.*/, ''));
  }
}
