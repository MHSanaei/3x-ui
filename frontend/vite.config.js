import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import fs from 'node:fs';
import path from 'node:path';
import { DatabaseSync } from 'node:sqlite';

const outDir = path.resolve(import.meta.dirname, '../internal/web/dist');
const BACKEND_TARGET = process.env.BACKEND_TARGET || 'http://localhost:2053';

function resolveDBPath() {
  const envFolder = process.env.XUI_DB_FOLDER;
  if (envFolder) {
    const abs = path.isAbsolute(envFolder)
      ? envFolder
      : path.resolve(import.meta.dirname, '..', envFolder);
    return path.join(abs, 'x-ui.db');
  }
  const repoSubDB = path.resolve(import.meta.dirname, '..', 'x-ui', 'x-ui.db');
  if (fs.existsSync(repoSubDB)) return repoSubDB;
  const repoDB = path.resolve(import.meta.dirname, '..', 'x-ui.db');
  if (fs.existsSync(repoDB)) return repoDB;
  return '/etc/x-ui/x-ui.db';
}

const PANEL_API_PREFIXES = ['panel/api/', 'panel/csrf-token'];

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
    const versionFile = path.resolve(import.meta.dirname, '..', 'config', 'version');
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

// Cloudflare Rocket Loader rewrites script tags and runs bundles through its
// own loader, breaking ES-module semantics; data-cfasync="false" opts out.
function rocketLoaderOptOutPlugin() {
  return {
    name: 'xui-rocket-loader-opt-out',
    apply: 'build',
    transformIndexHtml(html) {
      return html.replaceAll('<script ', '<script data-cfasync="false" ');
    },
  };
}

function bypassMigratedRoute(req) {
  if (req.method !== 'GET') return undefined;
  const url = req.url.split('?')[0];

  if (
    url.startsWith('/src/') ||
    url.startsWith('/@') ||
    url.startsWith('/node_modules/') ||
    url.startsWith('/assets/') ||
    url === '/favicon.ico' ||
    url.endsWith('.tsx') ||
    url.endsWith('.ts') ||
    url.endsWith('.js') ||
    url.endsWith('.mjs') ||
    url.endsWith('.css') ||
    url.endsWith('.svg') ||
    url.endsWith('.png') ||
    url.endsWith('.woff2')
  ) {
    return req.url;
  }

  const basePath = refreshBasePath();

  if (url === basePath || url === basePath.slice(0, -1)) return '/login.html';

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
    secure: false,
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

// Deps only reachable through swagger-ui-react (verified via `npm ls`). The
// catch-all `vendor` chunk would otherwise eager-load them on first paint,
// although the api-docs page is the only lazy route importing them.
const SWAGGER_ONLY_DEPS = [
  '@babel/runtime-corejs3',
  '@scarf/scarf',
  '@swagger-api/apidom-',
  '@swaggerexpert/',
  'base64-js',
  'buffer',
  'classnames',
  'css.escape',
  'deep-extend',
  'dompurify',
  'fast-json-patch',
  'highlight.js',
  'highlightjs-vue',
  'ieee754',
  'immutable',
  'js-file-download',
  'js-yaml',
  'lodash',
  'lowlight',
  'neotraverse',
  'node-abort-controller',
  'openapi-path-templating',
  'openapi-server-url-templating',
  'prismjs',
  'prop-types',
  'ramda',
  'ramda-adjunct',
  'randexp',
  'react-copy-to-clipboard',
  'react-debounce-input',
  'react-immutable-proptypes',
  'react-immutable-pure-component',
  'react-inspector',
  'react-redux',
  'react-syntax-highlighter',
  'redux',
  'redux-immutable',
  'remarkable',
  'reselect',
  'serialize-error',
  'sha.js',
  'url-parse',
  'xml',
  'xml-but-prettier',
  'zenscroll',
];

export default defineConfig({
  plugins: [react(), injectBasePathPlugin(), rocketLoaderOptOutPlugin()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, 'src'),
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
    // Everything in outDir is embedded into the Go binary via embed.FS, so
    // production sourcemaps (~18MB across 112 files, 72% of dist) ship inside
    // every release build. Nothing consumes them there; `npm run dev` serves
    // its own maps regardless of this setting. To debug a minified bundle
    // (including the XUI_DEBUG serve-from-disk path), build once with
    // XUI_SOURCEMAP=true — no tracked-file edit to accidentally commit.
    sourcemap: process.env.XUI_SOURCEMAP === 'true',
    target: 'es2020',
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      input: {
        index: path.resolve(import.meta.dirname, 'index.html'),
        login: path.resolve(import.meta.dirname, 'login.html'),
        subpage: path.resolve(import.meta.dirname, 'subpage.html'),
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
            || SWAGGER_ONLY_DEPS.some((dep) => id.includes(`/node_modules/${dep}/`))
          ) return 'vendor-swagger';
          if (id.includes('/node_modules/uplot/')) return 'vendor-uplot';
          if (id.includes('dayjs')) return 'vendor-dayjs';
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
