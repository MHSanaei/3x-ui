import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import fs from 'node:fs';
import path from 'node:path';
import { DatabaseSync } from 'node:sqlite';

const outDir = path.resolve(__dirname, '../web/dist');
const BACKEND_TARGET = 'http://localhost:2053';

function resolveDBPath() {
  const envFolder = process.env.XUI_DB_FOLDER;
  if (envFolder) return path.join(envFolder, 'x-ui.db');
  const repoDB = path.resolve(__dirname, '..', 'x-ui.db');
  if (fs.existsSync(repoDB)) return repoDB;
  return '/etc/x-ui/x-ui.db';
}

const BASE_MIGRATED_ROUTES = {
  'panel': '/index.html',
  'panel/': '/index.html',
  'panel/settings': '/settings.html',
  'panel/settings/': '/settings.html',
  'panel/inbounds': '/inbounds.html',
  'panel/inbounds/': '/inbounds.html',
  'panel/xray': '/xray.html',
  'panel/xray/': '/xray.html',
  'panel/nodes': '/nodes.html',
  'panel/nodes/': '/nodes.html',
};

let cachedBasePath = '/';

function readBasePathFromDB() {
  const dbPath = resolveDBPath();
  let db;
  try {
    db = new DatabaseSync(dbPath, { readOnly: true });
  } catch (_e) {
    return '/';
  }
  try {
    const row = db.prepare('SELECT value FROM settings WHERE key = ?').get('webBasePath');
    let value = row && typeof row.value === 'string' ? row.value : '/';
    if (!value.startsWith('/')) value = '/' + value;
    if (!value.endsWith('/')) value += '/';
    return value;
  } catch (_e) {
    return '/';
  } finally {
    db.close();
  }
}

function refreshBasePath() {
  cachedBasePath = readBasePathFromDB();
  return cachedBasePath;
}

// `apply: 'serve'` keeps the injection out of `vite build` — dist.go
// already injects __X_UI_BASE_PATH__ at runtime in production.
function injectBasePathPlugin() {
  return {
    name: 'xui-inject-base-path',
    apply: 'serve',
    transformIndexHtml(html) {
      const basePath = refreshBasePath();
      const escaped = basePath.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
      const tag = `<script>window.__X_UI_BASE_PATH__="${escaped}";</script>`;
      return html.replace('</head>', `${tag}</head>`);
    },
  };
}

function bypassMigratedRoute(req) {
  if (req.method !== 'GET') return undefined;
  const url = req.url.split('?')[0];

  for (const [key, value] of Object.entries(BASE_MIGRATED_ROUTES)) {
    if (url === '/' + key) return value;
  }

  const m = url.match(/^\/[^/]+\/(.+)$/);
  if (m) {
    const stripped = m[1];
    if (stripped in BASE_MIGRATED_ROUTES) return BASE_MIGRATED_ROUTES[stripped];
  }

  if (url === '/' || /^\/[^/]+\/$/.test(url)) return '/login.html';

  return undefined;
}

function rewriteToBackend(p) {
  if (cachedBasePath === '/' || p.startsWith(cachedBasePath)) return p;
  return cachedBasePath + p.replace(/^\//, '');
}

function makeBackendProxy(target) {
  return {
    target,
    changeOrigin: true,
    rewrite: rewriteToBackend,
    bypass: bypassMigratedRoute,
    configure(proxy) {
      let warned = false;
      proxy.on('error', (err, req) => {
        const codes = new Set();
        if (err && err.code) codes.add(err.code);
        if (err && Array.isArray(err.errors)) {
          for (const inner of err.errors) {
            if (inner && inner.code) codes.add(inner.code);
          }
        }
        const offline = codes.has('ECONNREFUSED') || codes.has('ECONNRESET');
        if (offline) {
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

export default defineConfig({
  plugins: [vue(), injectBasePathPlugin()],
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
    chunkSizeWarningLimit: 1500,
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
      '^/(?:[^/]+/)?(login|logout|getTwoFactorEnable|csrf-token|panel|server)(?:/|$)': makeBackendProxy(BACKEND_TARGET),
      '^/$': makeBackendProxy(BACKEND_TARGET),
      '^/[^/]+/$': makeBackendProxy(BACKEND_TARGET),
      '^/(?:[^/]+/)?ws$': {
        target: 'ws://localhost:2053',
        ws: true,
        changeOrigin: true,
        rewrite: rewriteToBackend,
      },
    },
  },
});
