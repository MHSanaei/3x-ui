// Client-side validation for xray-core #6258 sessionIDTable / sessionIDLength.
// xray-core also enforces a room-size minimum (sum(table^k for k in
// from..to) >= 2<<30) server-side; we deliberately skip replicating that
// big-int check and only catch the cheap, obvious mistakes here.

// xray-core requires the charset table to be ASCII-only.
export function validateSessionIDTable(_rule: unknown, value: unknown): Promise<void> {
  const str = typeof value === 'string' ? value : '';
  if (str === '') return Promise.resolve();
  // eslint-disable-next-line no-control-regex
  if (/[^\x00-\x7f]/.test(str)) {
    return Promise.reject(new Error('sessionIDTable must contain only ASCII characters'));
  }
  return Promise.resolve();
}

// A dash-range like "8-16" or a single "8". The lower bound must be > 0
// (xray rejects sessionIDLength.from <= 0 when a table is set).
export function validateSessionIDLength(_rule: unknown, value: unknown): Promise<void> {
  const str = typeof value === 'string' ? value.trim() : '';
  if (str === '') return Promise.resolve();
  if (!/^\d+(?:-\d+)?$/.test(str)) {
    return Promise.reject(new Error('Use a length or range, e.g. 8 or 8-16'));
  }
  const parts = str.split('-');
  const from = Number(parts[0]);
  if (!Number.isFinite(from) || from <= 0) {
    return Promise.reject(new Error('sessionIDLength minimum must be greater than 0'));
  }
  if (parts.length === 2 && Number(parts[1]) < from) {
    return Promise.reject(new Error('sessionIDLength range upper bound must be ≥ lower bound'));
  }
  return Promise.resolve();
}
