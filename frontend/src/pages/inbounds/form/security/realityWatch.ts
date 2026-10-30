// Same limit and shape as the server (internal/web/service/reality_watch.go).
export const REALITY_WATCH_MAX_SPARES = 10;

export function normalizeSpare(target: string): string {
  const t = target.trim().toLowerCase();
  return /:\d+$/.test(t) ? t : `${t}:443`;
}

export function addSpare(spares: string[], target: string, current: string): string[] {
  const t = normalizeSpare(target);
  if (!t || t === ':443' || spares.includes(t) || (current && t === normalizeSpare(current)))
    return spares;
  if (spares.length >= REALITY_WATCH_MAX_SPARES) return spares;
  return [...spares, t];
}
