let lastLocalInvalidateAt = 0;

export function markLocalInvalidate(): void {
  lastLocalInvalidateAt = Date.now();
}

export function isRecentLocalInvalidate(windowMs = 1500): boolean {
  return Date.now() - lastLocalInvalidateAt < windowMs;
}
