import { getRepo, getReadme, updateRepo, deleteRepo, recloneRepo } from './api.js';
import { Tag, getRepoTags, setRepoTags, listTags, createTag } from './api.js';
import { tagTextColor } from './util.js';

export async function renderRepoDetail(main: HTMLElement, id: number): Promise<void> {
  main.innerHTML = '<p>Loading...</p>';

  try {
    const repo = await getRepo(id);
    const [readme, tags, allTags] = await Promise.all([
      getReadme(id).catch(() => 'No README found.'),
      getRepoTags(id),
      listTags(),
    ]);

    main.innerHTML = buildDetailHTML(repo, readme, tags, allTags);
    bindDetailEvents(id, tags, allTags);
  } catch (e: any) {
    main.innerHTML = `<p class="error">Failed to load: ${escHtml(e.message)}</p>`;
  }
}

function buildDetailHTML(repo: any, readme: string, repoTagList: Tag[], allTags: Tag[]): string {
  const attn = repo.needs_attention
    ? `<div class="attention-notice">⚠ This repo needs attention: ${escHtml(repo.attention_reason || 'Unknown')}</div>`
    : '';

  const tagChips = repoTagList.map(t =>
    `<span class="tag-chip" style="background:${escHtml(t.color)};color:${tagTextColor(t.color)}">${escHtml(t.name)}</span>`
  ).join('');

  const unusedTags = allTags.filter(at => !repoTagList.some(rt => rt.id === at.id));

  const tagOptions = unusedTags.map(t =>
    `<option value="t:${t.id}">${escHtml(t.name)}</option>`
  ).join('');

  const hasCreateOption = unusedTags.length > 0 || repoTagList.length === 0;

  return `
    <div class="repo-detail">
      <a href="#catalog" class="back-link">← Back to catalog</a>
      <div class="detail-header">
        <h1>${escHtml(repo.owner)}/<strong>${escHtml(repo.name)}</strong></h1>
        <div class="detail-links">
          <a href="${escHtml(repo.github_url)}" target="_blank">View on GitHub ↗</a>
          <button id="detail-update-btn" class="detail-action-btn">Update</button>
          <button id="detail-reclone-btn" class="detail-action-btn">Re-Clone</button>
          <button id="detail-delete-btn" class="detail-action-btn danger">Remove</button>
        </div>
      </div>
      ${attn}
      ${repo.description ? `<p class="detail-desc">${escHtml(repo.description)}</p>` : ''}
      <div class="detail-meta">
        ${repo.license ? `<span>License: ${escHtml(repo.license)}</span>` : ''}
        <span>Default branch: ${escHtml(repo.default_branch)}</span>
        ${repo.last_updated ? `<span>Last updated: ${new Date(repo.last_updated).toLocaleDateString()}</span>` : ''}
        <span>Added: ${new Date(repo.added_date).toLocaleDateString()}</span>
        <span>Size: ${(repo.total_size_bytes / 1024 / 1024).toFixed(1)} MB</span>
      </div>
      <div class="detail-tags">
        <strong>Tags:</strong> ${tagChips || '<em>none</em>'}
        <span class="add-tag-row">
          <select id="tag-select">
            <option value="">Add tag...</option>
            ${tagOptions}
            ${hasCreateOption ? '<option value="__new__">Create new tag...</option>' : ''}
          </select>
          <span id="new-tag-fields" style="display:none">
            <input type="text" id="new-tag-name" placeholder="Name" size="10">
            <input type="color" id="new-tag-color" value="#6b7280">
          </span>
          <button id="add-tag-btn" class="add-btn">Add</button>
        </span>
      </div>
      <hr>
      <div class="readme">${readme}</div>
    </div>
  `;
}

function bindDetailEvents(repoId: number, currentTags: Tag[], allTags: Tag[]): void {
  document.getElementById('detail-update-btn')?.addEventListener('click', async () => {
    try {
      await updateRepo(repoId);
      alert('Update triggered.');
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });

  document.getElementById('detail-reclone-btn')?.addEventListener('click', async () => {
    if (!confirm('This will delete the local copy and re-clone from GitHub. Continue?')) return;
    try {
      await recloneRepo(repoId);
      alert('Re-clone triggered. Refresh to see updated data.');
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });

  document.getElementById('detail-delete-btn')?.addEventListener('click', async () => {
    if (!confirm('Delete this repository? Its local copy will be removed and it will be removed from the catalog.')) return;
    try {
      await deleteRepo(repoId);
      alert('Repository deleted.');
      location.hash = '#catalog';
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });

  document.getElementById('tag-select')?.addEventListener('change', () => {
    const sel = document.getElementById('tag-select') as HTMLSelectElement;
    const fields = document.getElementById('new-tag-fields')!;
    fields.style.display = sel.value === '__new__' ? '' : 'none';
  });

  document.getElementById('add-tag-btn')?.addEventListener('click', async () => {
    const sel = document.getElementById('tag-select') as HTMLSelectElement;
    if (!sel.value) return;

    try {
      if (sel.value === '__new__') {
        const name = (document.getElementById('new-tag-name') as HTMLInputElement).value.trim();
        if (!name) return;
        const color = (document.getElementById('new-tag-color') as HTMLInputElement).value;
        const tag = await createTag(name, color);
        await setRepoTags(repoId, [...currentTags.map(t => t.id), tag.id]);
      } else {
        const tagId = parseInt(sel.value.slice(2));
        await setRepoTags(repoId, [...currentTags.map(t => t.id), tagId]);
      }
      location.reload();
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });
}

function escHtml(s: string): string {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}
