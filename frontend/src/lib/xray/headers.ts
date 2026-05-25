// Pure helpers for header-shape conversion between the panel's internal
// HeaderEntry[] form and Xray's V2-style header map. Extracted from
// XrayCommonClass.toHeaders / .toV2Headers so callers can stop relying on
// the class hierarchy. Behavior is byte-equivalent to the legacy methods —
// the shadow tests in src/test/headers.test.ts pin that.

export interface HeaderEntry {
  name: string;
  value: string;
}

export type V2HeaderMap = Record<string, string | string[]>;

// Expand a V2-style header map into the panel's flat HeaderEntry[]. A
// header whose value is an array yields one entry per item, preserving
// order; a string value yields a single entry. Non-object inputs (null,
// undefined, primitives) yield [].
export function toHeaders(v2Headers: unknown): HeaderEntry[] {
  const out: HeaderEntry[] = [];
  if (!v2Headers || typeof v2Headers !== 'object') return out;
  const map = v2Headers as Record<string, unknown>;
  for (const key of Object.keys(map)) {
    const values = map[key];
    if (typeof values === 'string') {
      out.push({ name: key, value: values });
    } else if (Array.isArray(values)) {
      for (const v of values) {
        if (typeof v === 'string') out.push({ name: key, value: v });
      }
    }
  }
  return out;
}

// Case-insensitive lookup against a wire-shape header map. The legacy
// `Inbound.getHeader(obj, name)` iterated `obj.headers` as a HeaderEntry[];
// this version reads the Record map our Zod schemas store. For repeated
// header names (string[] in TCP/WS-style maps) the first value wins —
// matches the legacy iteration order. Returns '' when missing, mirroring
// the legacy fallback so link-generator call sites stay simple.
export function getHeaderValue(
  headers: Readonly<Record<string, string | string[]>> | undefined | null,
  name: string,
): string {
  if (!headers || typeof headers !== 'object') return '';
  const lower = name.toLowerCase();
  for (const key of Object.keys(headers)) {
    if (key.toLowerCase() !== lower) continue;
    const value = headers[key];
    if (typeof value === 'string') return value;
    if (Array.isArray(value)) return value[0] ?? '';
  }
  return '';
}

// Collapse a HeaderEntry[] back into a V2-style header map. When `arr` is
// true (the default — matches Xray's TCP/WS/HTTP request/response shape),
// duplicate header names accumulate into a string[]. When false (used for
// WS/HTTPUpgrade/xHTTP top-level headers, sockopt portMap, etc.), the
// last value wins. Entries with empty name or value are skipped — same as
// the legacy ObjectUtil.isEmpty() filter.
export function toV2Headers(headers: HeaderEntry[], arr: boolean = true): V2HeaderMap {
  const out: V2HeaderMap = {};
  for (const { name, value } of headers) {
    if (name == null || name === '' || value == null || value === '') continue;
    if (!(name in out)) {
      out[name] = arr ? [value] : value;
      continue;
    }
    const existing = out[name];
    if (arr && Array.isArray(existing)) {
      existing.push(value);
    } else {
      out[name] = value;
    }
  }
  return out;
}
