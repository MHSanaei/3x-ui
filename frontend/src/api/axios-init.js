import axios from 'axios';
import qs from 'qs';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE']);
const CSRF_TOKEN_PATH = '/csrf-token';

let csrfToken = null;
let csrfFetchPromise = null;
let sessionExpired = false;

function readMetaToken() {
  return document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || null;
}

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
        if (!sessionExpired) {
          sessionExpired = true;
          const basePath = window.X_UI_BASE_PATH || '/';
          window.location.replace(basePath);
        }
        return new Promise(() => { });
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
