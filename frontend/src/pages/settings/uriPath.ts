export function sanitizePath(input: string): string {
  let out = '';
  for (const ch of String(input ?? '')) {
    const code = ch.charCodeAt(0);
    if (ch === ':' || ch === '*' || ch === ' ' || ch === '\\' || code < 0x20 || code === 0x7f) continue;
    out += ch;
  }
  return out;
}

export function normalizePath(input: string): string {
  let p = input || '/';
  if (!p.startsWith('/')) p = '/' + p;
  if (!p.endsWith('/')) p += '/';
  p = p.replace(/\/+/g, '/');
  return p;
}
