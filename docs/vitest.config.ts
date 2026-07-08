import { defineConfig } from 'vitest/config';

// Pure logic in `lib/xray/*` is tested in a Node environment (no React/DOM),
// so unit tests run fast without jsdom. WebCrypto (X25519) is available on
// Node 22+ via globalThis.crypto.
export default defineConfig({
  test: {
    environment: 'node',
    include: ['lib/**/*.test.ts'],
  },
});
