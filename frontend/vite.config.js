import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'node:path';

// Output goes to web/dist/ at the repo root so the Go binary can embed it
// via embed.FS without reaching outside the web/ tree.
const outDir = path.resolve(__dirname, '../web/dist');

// Build a proxy config that suppresses ECONNREFUSED noise when the Go
// backend isn't running locally. Real errors (timeouts, 5xx, etc.) still
// surface in the Vite log.
function makeBackendProxy(target, patterns) {
  const config = {};
  for (const pattern of patterns) {
    config[pattern] = {
      target,
      changeOrigin: true,
      configure(proxy) {
        proxy.on('error', (err) => {
          if (err.code === 'ECONNREFUSED') return;
          // eslint-disable-next-line no-console
          console.error('[proxy]', err);
        });
      },
    };
  }
  return config;
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir,
    emptyOutDir: true,
    sourcemap: true,
    target: 'es2020',
    // Multiple HTML entries — one per legacy page we migrate.
    // As pages get ported in later phases, add their entrypoints here.
    rollupOptions: {
      input: {
        index: path.resolve(__dirname, 'index.html'),
        login: path.resolve(__dirname, 'login.html'),
      },
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: makeBackendProxy('http://localhost:2053', [
      // Patterns are anchored regex so /login.html and /index.html
      // (which Vite serves itself) are NOT forwarded — only the bare
      // backend paths and their sub-routes.
      '^/(login|logout|getTwoFactorEnable)$',
      '^/(panel|server)(/|$)',
    ]),
  },
});
