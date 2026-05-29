import path from 'node:path';

import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  test: {
    include: ['src/test/**/*.test.ts'],
    environment: 'node',
    globals: false,
    setupFiles: ['./src/test/setup.ts'],
  },
});
