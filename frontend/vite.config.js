import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'node:path';

// Output goes to web/dist/ at the repo root so the Go binary can embed it
// via embed.FS without reaching outside the web/ tree.
const outDir = path.resolve(__dirname, '../web/dist');

// In production the Go binary serves /panel/<route> from web/dist/<route>.html.
// In dev the Vue app lives at /index.html, /settings.html, ... while AppSidebar
// links use the production-style /panel/<route> URLs. Map each migrated route
// to its Vite entry so the sidebar works without relying on the Go backend
// for already-ported pages. Unmigrated routes (inbounds, xray) fall through
// to the proxy.
const MIGRATED_ROUTES = {
  '/panel': '/index.html',
  '/panel/': '/index.html',
  '/panel/settings': '/settings.html',
  '/panel/settings/': '/settings.html',
  '/panel/inbounds': '/inbounds.html',
  '/panel/inbounds/': '/inbounds.html',
};

// Build a proxy config that suppresses ECONNREFUSED noise when the Go
// backend isn't running locally. Real errors (timeouts, 5xx, etc.) still
// surface in the Vite log.
function makeBackendProxy(target, patterns) {
  const config = {};
  for (const pattern of patterns) {
    config[pattern] = {
      target,
      changeOrigin: true,
      // Returning a path from bypass tells Vite to serve that file from
      // its own dev server instead of forwarding the request — used here
      // to short-circuit /panel/<route> for pages we've already migrated.
      bypass(req) {
        const url = req.url.split('?')[0];
        if (Object.prototype.hasOwnProperty.call(MIGRATED_ROUTES, url)) {
          return MIGRATED_ROUTES[url];
        }
        return undefined;
      },
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
        settings: path.resolve(__dirname, 'settings.html'),
        inbounds: path.resolve(__dirname, 'inbounds.html'),
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
