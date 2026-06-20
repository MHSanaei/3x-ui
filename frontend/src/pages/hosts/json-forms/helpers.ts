// Shared helpers for the host's structured JSON-override editors. Each host
// override is persisted as a JSON string (muxParams / sockoptParams /
// finalMask); these convert between that string and the object the reused
// outbound/sub-JSON forms edit.

export function parseJsonObject(raw: string): Record<string, unknown> {
  if (!raw) return {};
  try {
    const v = JSON.parse(raw);
    return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

// Recursively drop '', null, undefined, and empty arrays/objects so an override
// stays sparse — only the keys the operator actually set are emitted and merged
// into the inbound stream. 0 and false are kept (meaningful sockopt/mux values).
export function pruneEmptyDeep(value: unknown): unknown {
  if (Array.isArray(value)) {
    const arr = value.map(pruneEmptyDeep).filter((v) => v !== undefined);
    return arr.length ? arr : undefined;
  }
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const pv = pruneEmptyDeep(v);
      if (pv !== undefined) out[k] = pv;
    }
    return Object.keys(out).length ? out : undefined;
  }
  if (value === '' || value === null) return undefined;
  return value;
}

// Prune then stringify; an all-empty override serializes to '' (= no override).
export function serializeOverride(value: unknown): string {
  const pruned = pruneEmptyDeep(value);
  return pruned === undefined ? '' : JSON.stringify(pruned);
}

// Build a nested object { a: { b: leaf } } from a form path ['a','b'] so the
// inner form can be seeded with initialValues at the exact path it edits.
export function nestAtPath(path: (string | number)[], leaf: unknown): Record<string, unknown> {
  let acc: unknown = leaf;
  for (let i = path.length - 1; i >= 0; i -= 1) {
    acc = { [path[i]]: acc };
  }
  return acc as Record<string, unknown>;
}
