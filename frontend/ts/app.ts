import { renderCatalog } from './catalog.js';
import { renderRepoDetail } from './repo.js';
import { renderTagManager } from './tags.js';

const main = document.getElementById('main')!;

function route(): void {
  const hash = location.hash.slice(1) || 'catalog';
  const page = hash.split('?')[0];

  if (page === 'catalog') {
    renderCatalog(main);
  } else if (page === 'tags') {
    renderTagManager(main);
  } else if (page.startsWith('repo/')) {
    const id = parseInt(page.split('/')[1]);
    if (isNaN(id)) {
      location.hash = '#catalog';
      return;
    }
    renderRepoDetail(main, id);
  } else {
    location.hash = '#catalog';
  }
}

window.addEventListener('hashchange', route);
route();
