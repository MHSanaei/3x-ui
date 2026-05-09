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
// for already-ported pages.
const MIGRATED_ROUTES = {
  '/panel': '/index.html',
  '/panel/': '/index.html',
  '/panel/settings': '/settings.html',
  '/panel/settings/': '/settings.html',
  '/panel/inbounds': '/inbounds.html',
  '/panel/inbounds/': '/inbounds.html',
  '/panel/xray': '/xray.html',
  '/panel/xray/': '/xray.html',
  '/panel/nodes': '/nodes.html',
  '/panel/nodes/': '/nodes.html',
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
      //
      // Only GETs get bypassed: the xray page reuses its page URL
      // (`POST /panel/xray/`) for data, so a method-blind bypass would
      // hand HTML back to fetch calls and break the page in dev.
      bypass(req) {
        if (req.method !== 'GET') return undefined;
        const url = req.url.split('?')[0];
        if (Object.prototype.hasOwnProperty.call(MIGRATED_ROUTES, url)) {
          return MIGRATED_ROUTES[url];
        }
        return undefined;
      },
      configure(proxy) {
        let warned = false;
        proxy.on('error', (err, req) => {
          // Node wraps connection failures in an AggregateError when DNS
          // returns multiple addresses (e.g. ::1 + 127.0.0.1) and all
          // refuse — the code lands on the inner errors, not the outer.
          const codes = new Set();
          if (err && err.code) codes.add(err.code);
          if (err && Array.isArray(err.errors)) {
            for (const inner of err.errors) {
              if (inner && inner.code) codes.add(inner.code);
            }
          }
          const offline = codes.has('ECONNREFUSED') || codes.has('ECONNRESET');
          if (offline) {
            // Print a single friendly hint the first time, then stay quiet.
            if (!warned) {
              warned = true;
              // eslint-disable-next-line no-console
              console.warn(
                `[proxy] backend ${target} is not reachable — start the Go server (e.g. \`go run main.go\`) to forward ${req?.url || 'requests'}.`,
              );
            }
            return;
          }
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
    // ant-design-vue is intentionally bundled as one chunk (its
    // components share internals — splitting it breaks Modal/Form/
    // Select interop). Minified it lands ~1.4MB but gzips to ~410kB,
    // so the actual transfer is fine and caches across every page.
    // Bump the warning past that ceiling so the build stays quiet.
    chunkSizeWarningLimit: 1500,
    // Multiple HTML entries — one per legacy page we migrate.
    // As pages get ported in later phases, add their entrypoints here.
    rollupOptions: {
      input: {
        index: path.resolve(__dirname, 'index.html'),
        login: path.resolve(__dirname, 'login.html'),
        settings: path.resolve(__dirname, 'settings.html'),
        inbounds: path.resolve(__dirname, 'inbounds.html'),
        xray: path.resolve(__dirname, 'xray.html'),
        nodes: path.resolve(__dirname, 'nodes.html'),
        subpage: path.resolve(__dirname, 'subpage.html'),
      },
      output: {
        // Split vendor deps into stable chunks so each page only pulls
        // what it needs and the browser caches them across versions.
        // Without this, ant-design-vue + vue + icons all end up in one
        // 1.6MB blob attached to whichever page consumed them first.
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined;
          if (id.includes('ant-design-vue')) return 'vendor-antd';
          if (id.includes('@ant-design/icons-vue')) return 'vendor-icons';
          if (id.includes('vue-i18n')) return 'vendor-i18n';
          if (
            id.includes('/node_modules/vue/')
            || id.includes('/node_modules/@vue/')
          ) return 'vendor-vue';
          if (id.includes('dayjs')) return 'vendor-dayjs';
          if (id.includes('qrious')) return 'vendor-qrious';
          if (id.includes('axios')) return 'vendor-axios';
          // The persian datepicker pulls in moment + moment-jalaali; bundle
          // the trio together so unrelated pages don't pay the cost.
          if (
            id.includes('vue3-persian-datetime-picker')
            || id.includes('moment-jalaali')
            || id.includes('jalaali-js')
            || id.includes('/node_modules/moment/')
          ) return 'vendor-jalali';
          return 'vendor';
        },
      },
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      ...makeBackendProxy('http://localhost:2053', [
        // Patterns are anchored regex so /login.html and /index.html
        // (which Vite serves itself) are NOT forwarded — only the bare
        // backend paths and their sub-routes.
        '^/(login|logout|getTwoFactorEnable|csrf-token)$',
        '^/(panel|server)(/|$)',
      ]),
      // The panel mounts the live-update WebSocket at /ws (basePath +
      // "/ws"). Vite needs `ws: true` to forward the HTTP Upgrade to the
      // Go backend; without it the dev server would 404 the upgrade and
      // the page falls back to the no-data state.
      '/ws': {
        target: 'ws://localhost:2053',
        ws: true,
        changeOrigin: true,
      },
    },
  },
});
