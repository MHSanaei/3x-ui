import path from 'node:path';

import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  test: {
    globals: false,
    projects: [
      {
        extends: true,
        test: {
          name: 'unit',
          include: ['src/test/**/*.test.ts'],
          environment: 'node',
          setupFiles: ['./src/test/setup.ts', './src/test/setup.msw.ts'],
        },
      },
      {
        extends: true,
        test: {
          name: 'components',
          include: ['src/test/**/*.test.tsx'],
          environment: 'jsdom',
          setupFiles: ['./src/test/setup.ts', './src/test/setup.components.ts'],
        },
      },
    ],
  },
});
