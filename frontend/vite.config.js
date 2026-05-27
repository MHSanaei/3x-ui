import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import fs from 'node:fs';
import path from 'node:path';
import { DatabaseSync } from 'node:sqlite';

const outDir = path.resolve(__dirname, '../web/dist');
const BACKEND_TARGET = 'http://localhost:2053';

function resolveDBPath() {
  const envFolder = process.env.XUI_DB_FOLDER;
  if (envFolder) {
    const abs = path.isAbsolute(envFolder)
      ? envFolder
      : path.resolve(__dirname, '..', envFolder);
    return path.join(abs, 'x-ui.db');
  }
  const repoSubDB = path.resolve(__dirname, '..', 'x-ui', 'x-ui.db');
  if (fs.existsSync(repoSubDB)) return repoSubDB;
  const repoDB = path.resolve(__dirname, '..', 'x-ui.db');
  if (fs.existsSync(repoDB)) return repoDB;
  return '/etc/x-ui/x-ui.db';
}

const PANEL_API_PREFIXES = ['panel/api/', 'panel/setting/', 'panel/xray/', 'panel/csrf-token'];

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

function readPanelVersion() {
  try {
    const versionFile = path.resolve(__dirname, '..', 'config', 'version');
    return fs.readFileSync(versionFile, 'utf8').trim();
  } catch (_e) {
    return '';
  }
}

// `apply: 'serve'` keeps the injection out of `vite build` — dist.go
// already injects webBasePath and version at runtime in production.
function injectBasePathPlugin() {
  return {
    name: 'xui-inject-base-path',
    apply: 'serve',
    transformIndexHtml(html) {
      const basePath = refreshBasePath();
      const escaped = basePath.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
      const version = readPanelVersion().replace(/\\/g, '\\\\').replace(/"/g, '\\"');
      const tag = `<script>window.X_UI_BASE_PATH="${escaped}";window.X_UI_CUR_VER="${version}";</script>`;
      return html.replace('</head>', `${tag}</head>`);
    },
  };
}

// es-toolkit's `./compat/*` exports map only declares a CJS condition, so deep
// imports like `es-toolkit/compat/get` resolve to a CJS shim. That shim uses a
// `require_X.Y` pattern that Vite's optimizer and Rolldown both mishandle
// (TypeError: require_isUnsafeProperty is not a function). The ESM build at
// `dist/compat/<category>/<name>.mjs` is fine but only carries a named export,
// while consumers like recharts use default imports — so emit a virtual module
// that re-exports the named symbol as default.
const ES_TOOLKIT_COMPAT_DIRS = ['array', 'function', 'math', 'object', 'predicate', 'string', 'util'];
const ES_TOOLKIT_SHIM_PREFIX = '\0es-toolkit-compat:';

function findEsToolkitCompatMjs(name) {
  for (const sub of ES_TOOLKIT_COMPAT_DIRS) {
    const candidate = path.resolve(__dirname, 'node_modules/es-toolkit/dist/compat', sub, `${name}.mjs`);
    if (fs.existsSync(candidate)) return candidate;
  }
  return null;
}

function esToolkitCompatEsmResolver() {
  return {
    name: 'es-toolkit-compat-esm',
    enforce: 'pre',
    resolveId(id) {
      const m = id.match(/^es-toolkit\/compat\/(.+)$/);
      if (!m) return null;
      if (!findEsToolkitCompatMjs(m[1])) return null;
      return ES_TOOLKIT_SHIM_PREFIX + m[1];
    },
    load(id) {
      if (!id.startsWith(ES_TOOLKIT_SHIM_PREFIX)) return null;
      const name = id.slice(ES_TOOLKIT_SHIM_PREFIX.length);
      const target = findEsToolkitCompatMjs(name);
      if (!target) return null;
      const url = target.replace(/\\/g, '/');
      return `import { ${name} } from ${JSON.stringify(url)};\nexport { ${name} };\nexport default ${name};\n`;
    },
  };
}

function bypassMigratedRoute(req) {
  if (req.method !== 'GET') return undefined;
  const url = req.url.split('?')[0];
  const basePath = refreshBasePath();

  if (url === basePath) return '/login.html';

  if (url.startsWith(basePath)) {
    const stripped = url.slice(basePath.length);
    for (const prefix of PANEL_API_PREFIXES) {
      if (prefix.endsWith('/')) {
        if (stripped.startsWith(prefix)) return undefined;
      } else if (stripped === prefix || stripped.startsWith(prefix + '/')) {
        return undefined;
      }
    }
    if (stripped === 'panel' || stripped === 'panel/' || stripped.startsWith('panel/')) {
      return '/index.html';
    }
  }
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
  plugins: [esToolkitCompatEsmResolver(), react(), injectBasePathPlugin()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  optimizeDeps: {
    rolldownOptions: {
      plugins: [esToolkitCompatEsmResolver()],
    },
  },
  experimental: {
    renderBuiltUrl(filename, { hostType }) {
      if (hostType === 'js') {
        return {
          runtime: `((window.X_UI_BASE_PATH||'/')+${JSON.stringify(filename)})`,
        };
      }
      return undefined;
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
        subpage: path.resolve(__dirname, 'subpage.html'),
      },
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined;
          if (id.includes('/node_modules/antd/')) return 'vendor-antd';
          if (id.includes('/@ant-design/icons/') || id.includes('/@ant-design/icons-svg/')) return 'vendor-icons';
          if (
            id.includes('/node_modules/@rc-component/')
            || id.includes('/node_modules/rc-')
            || id.includes('/@ant-design/cssinjs')
            || id.includes('/@ant-design/colors')
            || id.includes('/@ant-design/fast-color')
            || id.includes('/@ant-design/react-slick')
            || id.includes('/@ctrl/tinycolor')
          ) return 'vendor-antd';
          if (
            id.includes('/node_modules/react-i18next/')
            || id.includes('/node_modules/i18next/')
          ) return 'vendor-i18next';
          if (
            id.includes('/node_modules/react/')
            || id.includes('/node_modules/react-dom/')
            || id.includes('/node_modules/scheduler/')
          ) return 'vendor-react';
          if (
            id.includes('/node_modules/codemirror/')
            || id.includes('/node_modules/@codemirror/')
            || id.includes('/node_modules/@lezer/')
          ) return 'vendor-codemirror';
          if (id.includes('/node_modules/persian-calendar-suite/')) return 'vendor-jalali';
          if (id.includes('/node_modules/otpauth/')) return 'vendor-otpauth';
          if (id.includes('/node_modules/@tanstack/')) return 'vendor-tanstack';
          if (id.includes('/node_modules/react-router')) return 'vendor-router';
          if (
            id.includes('/node_modules/swagger-ui-react/')
            || id.includes('/node_modules/swagger-ui/')
            || id.includes('/node_modules/swagger-client/')
          ) return 'vendor-swagger';
          if (
            id.includes('/node_modules/recharts/')
            || id.includes('/node_modules/victory-vendor/')
            || id.includes('/node_modules/d3-')
          ) return 'vendor-recharts';
          if (id.includes('dayjs')) return 'vendor-dayjs';
          if (id.includes('axios')) return 'vendor-axios';
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
