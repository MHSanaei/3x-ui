const KEY = 'chunk-reloaded-at';
const LOOP_WINDOW_MS = 10_000;

// Reload at most once per window: a genuinely missing file would otherwise loop.
export function claimChunkReload(
  storage: Pick<Storage, 'getItem' | 'setItem'>,
  now: number,
): boolean {
  try {
    const last = Number(storage.getItem(KEY));
    if (last && now - last < LOOP_WINDOW_MS) return false;
    storage.setItem(KEY, String(now));
    return true;
  } catch {
    return false;
  }
}
