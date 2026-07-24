import path from 'node:path';
import { fileURLToPath } from 'node:url';

import react from '@vitejs/plugin-react';
import { storybookTest } from '@storybook/addon-vitest/vitest-plugin';
import { playwright } from '@vitest/browser-playwright';
import { defineConfig } from 'vitest/config';

const dirname = typeof __dirname !== 'undefined' ? __dirname : path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(dirname, 'src'),
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
      {
        extends: true,
        optimizeDeps: {
          include: ['aria-query', 'lz-string', 'pretty-format', 'dom-accessibility-api'],
        },
        plugins: [storybookTest({ configDir: path.join(dirname, '.storybook') })],
        test: {
          name: 'storybook',
          browser: {
            enabled: true,
            headless: true,
            provider: playwright({}),
            instances: [{ browser: 'chromium' }],
          },
        },
      },
    ],
  },
});
