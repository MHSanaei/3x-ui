import axios from 'axios';
import qs from 'qs';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE']);
// Public CSRF endpoint — works pre-login (the panel-scoped
// /panel/csrf-token sits behind checkLogin and would 401 a fresh
// login page that hasn't authenticated yet).
const CSRF_TOKEN_PATH = '/csrf-token';

// Cached session CSRF token. The legacy panel injects it via a
// <meta name="csrf-token"> tag rendered by Go; the new SPA pages
// fetch it once from /panel/csrf-token instead. Module-level so
// every axios POST sees the latest value.
let csrfToken = null;
let csrfFetchPromise = null;

function readMetaToken() {
  return document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || null;
}

// Fetch the token via a bare fetch() (not axios) so the call doesn't
// recurse through this same interceptor.
async function fetchCsrfToken() {
  try {
    const basePath = window.X_UI_BASE_PATH;
    const url = (typeof basePath === 'string' && basePath !== '' && basePath !== '/'
      ? basePath.replace(/\/$/, '') + CSRF_TOKEN_PATH
      : CSRF_TOKEN_PATH);
    const res = await fetch(url, {
      method: 'GET',
      credentials: 'same-origin',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    if (!res.ok) return null;
    const json = await res.json();
    return json?.success && typeof json.obj === 'string' ? json.obj : null;
  } catch (_e) {
    return null;
  }
}

async function ensureCsrfToken() {
  if (csrfToken) return csrfToken;
  const meta = readMetaToken();
  if (meta) {
    csrfToken = meta;
    return csrfToken;
  }
  if (!csrfFetchPromise) csrfFetchPromise = fetchCsrfToken();
  const fetched = await csrfFetchPromise;
  csrfFetchPromise = null;
  if (fetched) csrfToken = fetched;
  return csrfToken;
}

// Apply the panel's axios defaults + interceptors. Call once at app
// startup before any HTTP call goes out.
export function setupAxios() {
  axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
  axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

  const basePath = window.X_UI_BASE_PATH;
  if (typeof basePath === 'string' && basePath !== '' && basePath !== '/') {
    axios.defaults.baseURL = basePath;
  }

  // Seed the cache from the meta tag if a server-rendered page injected
  // one — saves a round trip on legacy templates that still embed it.
  csrfToken = readMetaToken();

  axios.interceptors.request.use(
    async (config) => {
      config.headers = config.headers || {};
      const method = (config.method || 'get').toUpperCase();
      if (!SAFE_METHODS.has(method)) {
        const token = await ensureCsrfToken();
        if (token) config.headers['X-CSRF-Token'] = token;
      }
      if (config.data instanceof FormData) {
        config.headers['Content-Type'] = 'multipart/form-data';
      } else {
        config.data = qs.stringify(config.data, { arrayFormat: 'repeat' });
      }
      return config;
    },
    (error) => Promise.reject(error),
  );

  axios.interceptors.response.use(
    (response) => response,
    async (error) => {
      const status = error.response?.status;
      if (status === 401) {
        // 401 → session is gone. In production, the panel routes
        // are gated by Go's checkLogin which redirects to base_path
        // serving the login page; a reload is enough. In dev, Vite
        // serves /index.html directly at "/", so a reload would put
        // the user right back on the dashboard and the interceptor
        // would loop. Navigate to the dev login entry instead.
        if (import.meta.env.DEV) {
          const basePath = window.X_UI_BASE_PATH || '/';
          window.location.href = `${basePath}login.html`;
        } else {
          window.location.reload();
        }
        return Promise.reject(error);
      }
      // 403 with a stale/missing CSRF token: drop the cache, re-fetch, retry once.
      const cfg = error.config;
      if (status === 403 && cfg && !cfg.__csrfRetried) {
        csrfToken = null;
        cfg.__csrfRetried = true;
        const token = await ensureCsrfToken();
        if (token) {
          cfg.headers = cfg.headers || {};
          cfg.headers['X-CSRF-Token'] = token;
          // axios re-stringifies on retry, so unwind our qs.stringify before
          // letting the same request flow through the interceptor again.
          if (typeof cfg.data === 'string') cfg.data = qs.parse(cfg.data);
          return axios(cfg);
        }
      }
      return Promise.reject(error);
    },
  );
}
