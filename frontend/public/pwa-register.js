(() => {
  if (!('serviceWorker' in navigator)) return;

  const script = document.currentScript;
  if (!(script instanceof HTMLScriptElement)) return;

  const scriptUrl = new URL(script.src, window.location.href);
  const baseUrl = new URL('./', scriptUrl);
  const workerUrl = new URL('service-worker.js', baseUrl);

  navigator.serviceWorker.register(workerUrl.pathname, {
    scope: baseUrl.pathname,
  }).catch(() => {});
})();
