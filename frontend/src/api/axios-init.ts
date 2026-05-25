import axios from 'axios';
import type { AxiosError, AxiosResponse, InternalAxiosRequestConfig } from 'axios';
import qs from 'qs';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE']);
const CSRF_TOKEN_PATH = '/csrf-token';

let csrfToken: string | null = null;
let csrfFetchPromise: Promise<string | null> | null = null;
let sessionExpired = false;

type CsrfAwareConfig = InternalAxiosRequestConfig & { __csrfRetried?: boolean };

function readMetaToken(): string | null {
  return document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || null;
}

async function fetchCsrfToken(): Promise<string | null> {
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
    const json = (await res.json()) as { success?: boolean; obj?: unknown } | null;
    return json?.success && typeof json.obj === 'string' ? json.obj : null;
  } catch {
    return null;
  }
}

async function ensureCsrfToken(): Promise<string | null> {
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

export function setupAxios(): void {
  axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
  axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

  let basePath: string | null | undefined = window.X_UI_BASE_PATH;
  if (!basePath) {
    const metaTag = document.querySelector('meta[name="base-path"]');
    basePath = metaTag ? metaTag.getAttribute('content') : null;
  }
  if (typeof basePath === 'string' && basePath !== '' && basePath !== '/') {
    axios.defaults.baseURL = basePath;
  }

  csrfToken = readMetaToken();

  axios.interceptors.request.use(
    async (config: InternalAxiosRequestConfig) => {
      const method = (config.method || 'get').toUpperCase();
      if (!SAFE_METHODS.has(method)) {
        const token = await ensureCsrfToken();
        if (token) config.headers.set('X-CSRF-Token', token);
      }
      if (config.data instanceof FormData) {
        config.headers.set('Content-Type', 'multipart/form-data');
      } else {
        const declaredType = String(config.headers.get('Content-Type') || config.headers.get('content-type') || '');
        if (declaredType.toLowerCase().startsWith('application/json')) {
          if (config.data !== undefined && typeof config.data !== 'string') {
            config.data = JSON.stringify(config.data);
          }
        } else {
          config.data = qs.stringify(config.data, { arrayFormat: 'repeat' });
        }
      }
      return config;
    },
    (error: unknown) => Promise.reject(error),
  );

  axios.interceptors.response.use(
    (response: AxiosResponse) => response,
    async (error: AxiosError) => {
      const status = error.response?.status;
      if (status === 401) {
        if (!sessionExpired) {
          sessionExpired = true;
          const basePath = window.X_UI_BASE_PATH || '/';
          window.location.replace(basePath);
        }
        return new Promise(() => {});
      }
      const cfg = error.config as CsrfAwareConfig | undefined;
      if (status === 403 && cfg && !cfg.__csrfRetried) {
        csrfToken = null;
        cfg.__csrfRetried = true;
        const token = await ensureCsrfToken();
        if (token) {
          cfg.headers.set('X-CSRF-Token', token);
          const declaredType = String(cfg.headers.get('Content-Type') || cfg.headers.get('content-type') || '');
          if (typeof cfg.data === 'string') {
            if (declaredType.toLowerCase().startsWith('application/json')) {
              try { cfg.data = JSON.parse(cfg.data); } catch {}
            } else {
              cfg.data = qs.parse(cfg.data);
            }
          }
          return axios(cfg);
        }
      }
      return Promise.reject(error);
    },
  );
}
