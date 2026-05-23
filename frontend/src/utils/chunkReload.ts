// After a panel upgrade the embedded dist/ ships with new hashed chunk
// filenames, so an SPA that was loaded before the upgrade still holds
// references to chunks that no longer exist on the server. The first
// time a lazy import 404s we force a full reload so the browser picks
// up the new index.html and its new chunk references.
if (typeof window !== 'undefined') {
  const RELOAD_FLAG = '__xuiChunkReloadOnce';
  window.addEventListener('vite:preloadError', (event) => {
    event.preventDefault();
    if (sessionStorage.getItem(RELOAD_FLAG) === '1') return;
    sessionStorage.setItem(RELOAD_FLAG, '1');
    window.location.reload();
  });
  window.addEventListener('load', () => {
    sessionStorage.removeItem(RELOAD_FLAG);
  });
}
