import { Tag, listTags, createTag, updateTag, deleteTag } from './api.js';
import { tagTextColor } from './util.js';

export async function renderTagManager(main: HTMLElement): Promise<void> {
  main.innerHTML = `
    <div class="tag-manager">
      <a href="#catalog" class="back-link">← Back to catalog</a>
      <h1>Manage Tags</h1>
      <div class="new-tag-form">
        <input type="text" id="new-tag-name" placeholder="Tag name">
        <input type="color" id="new-tag-color" value="#6b7280">
        <button id="create-tag-btn">Create Tag</button>
      </div>
      <div id="tag-list" class="tag-list">Loading...</div>
    </div>
  `;

  loadTags();

  document.getElementById('create-tag-btn')!.addEventListener('click', async () => {
    const name = (document.getElementById('new-tag-name') as HTMLInputElement).value.trim();
    const color = (document.getElementById('new-tag-color') as HTMLInputElement).value;
    if (!name) return;
    try {
      await createTag(name, color);
      (document.getElementById('new-tag-name') as HTMLInputElement).value = '';
      loadTags();
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });
}

async function loadTags(): Promise<void> {
  const list = document.getElementById('tag-list')!;
  try {
    const tags = await listTags();
    if (tags.length === 0) {
      list.innerHTML = '<p class="empty">No tags yet.</p>';
      return;
    }
    list.innerHTML = tags.map(t => tagRow(t)).join('');

    list.querySelectorAll('.tag-edit-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = parseInt((btn as HTMLElement).dataset.id!);
        const row = document.getElementById('tag-row-' + id)!;
        const tag = tags.find(t => t.id === id)!;
        row.innerHTML = editForm(tag);
        bindEditEvents(id);
      });
    });

    list.querySelectorAll('.tag-delete-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const id = parseInt((btn as HTMLElement).dataset.id!);
        if (!confirm('Delete this tag? It will be removed from all repositories.')) return;
        try {
          await deleteTag(id);
          loadTags();
        } catch (e: any) {
          alert('Error: ' + e.message);
        }
      });
    });
  } catch (e: any) {
    list.innerHTML = `<p class="error">${e.message}</p>`;
  }
}

function tagRow(t: Tag): string {
  return `
    <div class="tag-row" id="tag-row-${t.id}">
      <span class="tag-chip" style="background:${esc(t.color)};color:${tagTextColor(t.color)}">${esc(t.name)}</span>
      <button class="tag-edit-btn" data-id="${t.id}">Edit</button>
      <button class="tag-delete-btn" data-id="${t.id}">Delete</button>
    </div>
  `;
}

function editForm(t: Tag): string {
  return `
    <input type="text" id="edit-name-${t.id}" value="${esc(t.name)}">
    <input type="color" id="edit-color-${t.id}" value="${esc(t.color)}">
    <button class="tag-save-btn" data-id="${t.id}">Save</button>
    <button class="tag-cancel-btn" data-id="${t.id}">Cancel</button>
  `;
}

function bindEditEvents(id: number): void {
  document.querySelector('.tag-save-btn')?.addEventListener('click', async () => {
    const name = (document.getElementById('edit-name-' + id) as HTMLInputElement).value.trim();
    const color = (document.getElementById('edit-color-' + id) as HTMLInputElement).value;
    if (!name) return;
    try {
      await updateTag(id, name, color);
      loadTags();
    } catch (e: any) {
      alert('Error: ' + e.message);
    }
  });

  document.querySelector('.tag-cancel-btn')?.addEventListener('click', () => {
    loadTags();
  });
}

function esc(s: string): string {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}
