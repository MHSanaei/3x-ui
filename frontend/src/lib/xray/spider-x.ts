import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js';

// Mirrors deriveSpiderX in internal/sub/service.go byte-for-byte so panel
// links and subscription links agree; returns '' when there is no seed and
// no client key (the caller then omits spx, as the legacy builder did).
export function deriveSpiderX(seed: string, clientKey: string): string {
  if (!seed && !clientKey) return '';
  return `/${bytesToHex(sha256(utf8ToBytes(`${seed}|${clientKey}`))).slice(0, 15)}`;
}
