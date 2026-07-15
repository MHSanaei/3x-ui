const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE']);
const CSRF_TOKEN_PATH = '/csrf-token';

let csrfToken: string | null = null;
let csrfFetchPromise: Promise<string | null> | null = null;
let sessionExpired = false;
let basePathPrefix = '';

export interface HttpResponse {
  ok: boolean;
  status: number;
  statusText: string;
  data: unknown;
}

export class HttpError extends Error {
  status: number;
  response: { status: number; statusText: string; data: unknown };

  constructor(status: number, statusText: string, data: unknown) {
    super(`Request failed with status ${status}`);
    this.name = 'HttpError';
    this.status = status;
    this.response = { status, statusText, data };
  }
}

export interface HttpRequestOptions {
  headers?: Record<string, string> | Headers;
  params?: unknown;
  timeout?: number;
  signal?: AbortSignal;
}

function readMetaToken(): string | null {
  return document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || null;
}

async function fetchCsrfToken(): Promise<string | null> {
  try {
    const res = await fetch(basePathPrefix + CSRF_TOKEN_PATH, {
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

function encodeForm(data: unknown): string {
  if (data == null || typeof data !== 'object') return '';
  const parts: string[] = [];
  const append = (key: string, value: unknown): void => {
    if (value === undefined) return;
    if (value === null) {
      parts.push(`${encodeURIComponent(key)}=`);
      return;
    }
    if (Array.isArray(value)) {
      value.forEach((item) => append(key, item));
      return;
    }
    if (typeof value === 'object') {
      Object.entries(value as Record<string, unknown>).forEach(([k, v]) => append(`${key}[${k}]`, v));
      return;
    }
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
  };
  Object.entries(data as Record<string, unknown>).forEach(([k, v]) => append(k, v));
  return parts.join('&');
}

async function performFetch(
  method: string,
  url: string,
  data: unknown,
  options: HttpRequestOptions,
  csrfOverride?: string,
): Promise<Response> {
  const upper = method.toUpperCase();
  const headers = new Headers(options.headers);
  headers.set('X-Requested-With', 'XMLHttpRequest');

  let body: BodyInit | undefined;
  if (data instanceof FormData) {
    body = data;
    headers.delete('Content-Type');
  } else if (!SAFE_METHODS.has(upper)) {
    const declaredType = (headers.get('Content-Type') || '').toLowerCase();
    if (declaredType.startsWith('application/json')) {
      if (data !== undefined) {
        body = typeof data === 'string' ? data : JSON.stringify(data);
      }
    } else {
      headers.set('Content-Type', 'application/x-www-form-urlencoded; charset=UTF-8');
      body = encodeForm(data);
    }
  }

  if (!SAFE_METHODS.has(upper)) {
    const token = csrfOverride ?? (await ensureCsrfToken());
    if (token) headers.set('X-CSRF-Token', token);
  }

  const query = encodeForm(options.params);
  const fullUrl = basePathPrefix + url + (query ? `?${query}` : '');
  const signal = options.timeout ? AbortSignal.timeout(options.timeout) : options.signal;

  return fetch(fullUrl, { method: upper, headers, body, credentials: 'same-origin', signal });
}

async function parseBody(res: Response): Promise<unknown> {
  if (res.status === 204 || res.status === 205) return '';
  const text = await res.text();
  if (text === '') return '';
  const contentType = (res.headers.get('content-type') || '').toLowerCase();
  if (contentType.includes('application/json') || text[0] === '{' || text[0] === '[') {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }
  return text;
}

export async function httpRequest(
  method: string,
  url: string,
  data?: unknown,
  options: HttpRequestOptions = {},
): Promise<HttpResponse> {
  let res = await performFetch(method, url, data, options);

  if (res.status === 403 && !SAFE_METHODS.has(method.toUpperCase())) {
    csrfToken = null;
    const fresh = await fetchCsrfToken();
    if (fresh) {
      csrfToken = fresh;
      res = await performFetch(method, url, data, options, fresh);
    }
  }

  if (res.status === 401) {
    if (!sessionExpired) {
      sessionExpired = true;
      window.location.replace(window.X_UI_BASE_PATH || basePathPrefix || '/');
    }
    return new Promise<HttpResponse>(() => {});
  }

  const parsed = await parseBody(res);
  if (!res.ok) throw new HttpError(res.status, res.statusText, parsed);
  return { ok: true, status: res.status, statusText: res.statusText, data: parsed };
}

export function setupHttp(): void {
  let basePath: string | null | undefined = window.X_UI_BASE_PATH;
  if (!basePath) {
    const metaTag = document.querySelector('meta[name="base-path"]');
    basePath = metaTag ? metaTag.getAttribute('content') : null;
  }
  basePathPrefix =
    typeof basePath === 'string' && basePath !== '' && basePath !== '/'
      ? basePath.replace(/\/$/, '')
      : '';

  csrfToken = readMetaToken();
}
