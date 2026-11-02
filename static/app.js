(() => {
  const search = document.querySelector('#game-search');
  const rating = document.querySelector('#rating-filter');
  const groups = {recommended: ['great', 'good'], playable: ['playable'], issues: ['issues', 'broken'], unknown: ['none', 'untested', 'pending', 'error', '']};
  const filter = () => {
    if (!search || !rating) return;
    const query = search.value.trim().toLocaleLowerCase();
    let visible = 0;
    const rows = document.querySelectorAll('#list-results .game-row');
    rows.forEach(row => {
      row.hidden = !(row.dataset.title.toLocaleLowerCase().includes(query) && (rating.value === 'all' || groups[rating.value].includes(row.dataset.status)));
      if (!row.hidden) visible++;
    });
    const count = document.querySelector('[data-results-count]');
    if (count) count.textContent = query || rating.value !== 'all' ? `${visible} of ${rows.length} games` : `${rows.length} games`;
    const empty = document.querySelector('.filter-empty');
    if (empty) empty.hidden = visible > 0 || rows.length === 0;
  };
  search?.addEventListener('input', filter);
  rating?.addEventListener('change', filter);
  document.addEventListener('click', event => {
    if (event.target.closest('[data-clear-filters]')) { search.value = ''; rating.value = 'all'; filter(); search.focus(); }
  });
  // Replace failed cover images with a quiet title initial.
  document.addEventListener('error', event => {
    const img = event.target;
    if (!(img instanceof HTMLImageElement) || !img.closest('.thumb')) return;
    const fallback = document.createElement('span');
    fallback.className = 'thumb-fallback';
    fallback.textContent = img.closest('.game-row')?.querySelector('.game-title')?.textContent.trim().charAt(0) || 'A';
    img.replaceWith(fallback);
  }, true);
  filter();
})();
