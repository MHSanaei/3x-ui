import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js';

// Mirrors deriveSpiderX in internal/sub/service.go byte-for-byte so panel
// links and subscription links agree; returns '' when there is no seed and
// no client key (the caller then omits spx, as the legacy builder did).
// The seed's query is kept: xray reads p, c, t, i and r there as the
// spider's own settings (padding, concurrency, times, interval, return).
export function deriveSpiderX(seed: string, clientKey: string): string {
  if (!seed && !clientKey) return '';
  const path = `/${bytesToHex(sha256(utf8ToBytes(`${seed}|${clientKey}`))).slice(0, 15)}`;
  const at = seed.indexOf('?');
  const query = at === -1 ? '' : seed.slice(at + 1);
  return query ? `${path}?${query}` : path;
}
