// Pure builders for 3x-ui panel API requests. The panel exposes every endpoint
// under /panel/api/* and authenticates with `Authorization: Bearer <token>`
// (reference/api/authentication). Emits a ready cURL command and a fetch()
// snippet. No React/DOM imports.

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface ApiRequestInput {
  baseUrl: string;
  token: string;
  path: string; // e.g. /panel/api/inbounds/list
  method: HttpMethod;
  body?: string; // JSON string, for POST/PUT
}

export function normalizeBase(baseUrl: string): string {
  return baseUrl.trim().replace(/\/+$/, '');
}

export function joinUrl(baseUrl: string, path: string): string {
  const base = normalizeBase(baseUrl);
  const p = path.trim().startsWith('/') ? path.trim() : `/${path.trim()}`;
  return `${base}${p}`;
}

function hasBody(i: ApiRequestInput): boolean {
  return (i.method === 'POST' || i.method === 'PUT') && !!i.body && i.body.trim().length > 0;
}

export function buildCurl(i: ApiRequestInput): string {
  const url = joinUrl(i.baseUrl, i.path);
  const lines = [`curl -X ${i.method} '${url}'`, `  -H 'Authorization: Bearer ${i.token}'`];
  if (hasBody(i)) {
    lines.push(`  -H 'Content-Type: application/json'`);
    lines.push(`  --data '${i.body!.trim()}'`);
  }
  return lines.join(' \\\n');
}

export function buildFetchSnippet(i: ApiRequestInput): string {
  const url = joinUrl(i.baseUrl, i.path);
  const headers = [`'Authorization': 'Bearer ${i.token}'`];
  if (hasBody(i)) headers.push(`'Content-Type': 'application/json'`);

  const opts = [`method: '${i.method}'`, `headers: { ${headers.join(', ')} }`];
  if (hasBody(i)) opts.push(`body: JSON.stringify(${i.body!.trim()})`);

  return `await fetch('${url}', {\n  ${opts.join(',\n  ')},\n});`;
}
