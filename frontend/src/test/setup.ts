// Vitest setup. The frontend's Base64 utility (used by link generators)
// reaches for `window.btoa` directly. Node 16+ ships btoa/atob on
// globalThis, so we just alias `window` to `globalThis` instead of
// pulling in jsdom — keeps the test env light and avoids a new dep.

if (typeof globalThis.window === 'undefined') {
  (globalThis as unknown as { window: typeof globalThis }).window = globalThis;
}
