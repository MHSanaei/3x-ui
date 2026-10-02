import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js';

// Mirrors deriveSpiderX in internal/sub/service.go byte-for-byte, seed query included (#6693);
// '' with neither seed nor client key, so the caller omits spx as the legacy builder did.
export function deriveSpiderX(seed: string, clientKey: string): string {
  if (!seed && !clientKey) return '';
  const path = `/${bytesToHex(sha256(utf8ToBytes(`${seed}|${clientKey}`))).slice(0, 15)}`;
  const at = seed.indexOf('?');
  const query = at === -1 ? '' : seed.slice(at + 1);
  return query ? `${path}?${query}` : path;
}
