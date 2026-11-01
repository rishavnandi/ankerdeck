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
  document.addEventListener('htmx:after:swap', filter);
  document.addEventListener('htmx:finally:request', event => {
    const ctx = event.detail.ctx;
    if (!(ctx?.sourceElement instanceof Element) || !ctx.sourceElement.matches('.refresh-button')) return;
    const result = document.querySelector('#refresh-result');
    if (!result) return;
    if (!ctx.response || ctx.response.status >= 400) result.textContent = 'Couldn’t refresh. Please try again.';
    if (ctx.response?.status === 202) {
      document.body.dispatchEvent(new Event('refreshCharts'));
      // Poll promptly while the requested refresh is in flight.
      [5000, 15000, 30000].forEach(delay => setTimeout(() => document.body.dispatchEvent(new Event('refreshCharts')), delay));
    }
    setTimeout(() => { if (result?.isConnected) result.textContent = ''; }, 8000);
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
