<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import {
  KeyOutlined,
  SearchOutlined,
  ExpandOutlined,
  CompressOutlined,
  ApiOutlined,
  SafetyCertificateOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  GlobalOutlined,
  SaveOutlined,
  SettingOutlined,
  WifiOutlined,
  LinkOutlined,
  NodeIndexOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import AppSidebar from '@/components/AppSidebar.vue';
import { sections as allSections } from './endpoints.js';
import EndpointSection from './EndpointSection.vue';
import CodeBlock from './CodeBlock.vue';

const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;
const settingsHref = `${basePath}panel/settings#security`;

const searchQuery = ref('');
const collapsedSections = ref(new Set());
const activeSection = ref('');

const sectionIcons = {
  authentication: SafetyCertificateOutlined,
  inbounds: NodeIndexOutlined,
  server: CloudServerOutlined,
  nodes: ClusterOutlined,
  'custom-geo': GlobalOutlined,
  backup: SaveOutlined,
  settings: SettingOutlined,
  'api-tokens': KeyOutlined,
  'xray-settings': WifiOutlined,
  subscription: LinkOutlined,
  websocket: ApiOutlined,
};

const curlExample = `curl -X GET \\
  -H "Authorization: Bearer YOUR_API_TOKEN" \\
  -H "Accept: application/json" \\
  https://your-panel.example.com/panel/api/inbounds/list`;

const sections = computed(() => {
  const q = searchQuery.value.toLowerCase().trim();
  if (!q) return allSections;
  return allSections
    .map(s => {
      const matching = s.endpoints.filter(e =>
        e.path.toLowerCase().includes(q) ||
        e.summary?.toLowerCase().includes(q) ||
        e.method.toLowerCase().includes(q)
      );
      return { ...s, endpoints: matching };
    })
    .filter(s => s.endpoints.length > 0);
});

const endpointCount = computed(() =>
  allSections.reduce((sum, s) => sum + s.endpoints.length, 0)
);

const visibleEndpoints = computed(() =>
  sections.value.reduce((sum, s) => sum + s.endpoints.length, 0)
);

function isCollapsed(id) {
  return collapsedSections.value.has(id);
}

function toggleSection(id) {
  const s = new Set(collapsedSections.value);
  if (s.has(id)) s.delete(id); else s.add(id);
  collapsedSections.value = s;
}

function expandAll() {
  collapsedSections.value = new Set();
}

function collapseAll() {
  collapsedSections.value = new Set(allSections.map(s => s.id));
}

function scrollToSection(id) {
  const el = document.getElementById(id);
  if (!el) return;
  el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  if (window.location.hash !== `#${id}`) {
    history.replaceState(null, '', `#${id}`);
  }
}

function scrollToHash() {
  const id = window.location.hash.slice(1);
  if (!id) return;
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: 'auto', block: 'start' });
}

let scrollObserver = null;
function onScroll() {
  const toc = document.querySelector('.toc-nav');
  const tocHeight = toc ? toc.offsetHeight : 56;
  let current = '';
  for (const s of sections.value) {
    const el = document.getElementById(s.id);
    if (!el) continue;
    const rect = el.getBoundingClientRect();
    if (rect.top <= tocHeight + 20) {
      current = s.id;
    }
  }
  activeSection.value = current;
}

onMounted(() => {
  scrollObserver = onScroll;
  window.addEventListener('scroll', scrollObserver, { passive: true });
  window.addEventListener('hashchange', scrollToHash);
  requestAnimationFrame(() => {
    scrollToHash();
    onScroll();
  });
});

onBeforeUnmount(() => {
  if (scrollObserver) {
    window.removeEventListener('scroll', scrollObserver);
  }
  window.removeEventListener('hashchange', scrollToHash);
});
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="api-docs-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content class="content-area">
          <div class="docs-wrapper">
            <header class="docs-header">
              <h1 class="docs-title">API Documentation</h1>
              <p class="docs-lead">
                The 3x-ui panel exposes a REST API under <code>/panel/api/</code>. Authenticate with the panel session
                cookie, or with the <code>Authorization: Bearer &lt;token&gt;</code> header below. Every endpoint
                returns a uniform <code>{ success, msg, obj }</code> envelope unless otherwise noted.
              </p>

            </header>

            <a-card class="token-card" size="small">
              <div class="token-card-head">
                <div class="token-card-title">
                  <KeyOutlined />
                  <span>API Tokens</span>
                </div>
                <a-button type="primary" size="small" :href="settingsHref">
                  Manage tokens
                </a-button>
              </div>
              <p class="token-hint">
                Create, enable, or revoke named Bearer tokens in
                <a :href="settingsHref">Settings → Security</a>. Send each request as
                <code>Authorization: Bearer &lt;token&gt;</code>. Token-authenticated callers skip CSRF and don't
                need a session cookie. Deleting a token revokes it immediately — running bots will need a new one.
              </p>
            </a-card>

            <a-card class="curl-card" size="small" title="Quick example">
              <CodeBlock :code="curlExample" lang="text" />
            </a-card>

            <div class="toolbar">
              <a-input-search
                v-model:value="searchQuery"
                placeholder="Search endpoints by path, method, or description…"
                allow-clear
                class="search-bar"
              >
                <template #prefix><SearchOutlined /></template>
              </a-input-search>
              <span class="match-count" v-if="searchQuery">
                {{ visibleEndpoints }} / {{ endpointCount }} endpoints
              </span>
              <a-space size="small">
                <a-button size="small" @click="expandAll">
                  <template #icon><ExpandOutlined /></template>
                  Expand all
                </a-button>
                <a-button size="small" @click="collapseAll">
                  <template #icon><CompressOutlined /></template>
                  Collapse all
                </a-button>
              </a-space>
            </div>

            <nav class="toc-nav">
              <span class="toc-label">On this page:</span>
              <div class="toc-links">
                <a
                  v-for="s in sections"
                  :key="s.id"
                  class="toc-link"
                  :class="{ active: activeSection === s.id }"
                  :href="`#${s.id}`"
                  @click.prevent="scrollToSection(s.id)"
                >
                  <component :is="sectionIcons[s.id]" class="toc-icon" />
                  <span class="toc-text">{{ s.title }}</span>
                  <span class="toc-badge">{{ s.endpoints.length }}</span>
                </a>
              </div>
            </nav>

            <EndpointSection
              v-for="s in sections"
              :key="s.id"
              :section="s"
              :icon="sectionIcons[s.id]"
              :collapsed="isCollapsed(s.id)"
              @toggle="toggleSection(s.id)"
            />
          </div>
        </a-layout-content>
      </a-layout>
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.api-docs-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;
  min-height: 100vh;
  background: var(--bg-page);
}

.api-docs-page.is-dark {
  --bg-page: #1e1e1e;
  --bg-card: #252526;
}

.api-docs-page.is-dark.is-ultra {
  --bg-page: #000;
  --bg-card: #0a0a0a;
}

.content-shell {
  background: var(--bg-page);
}

.content-area {
  padding: 24px;
  max-width: 100%;
}

@media (max-width: 768px) {
  .content-area {
    padding: 16px 12px 12px;
    padding-top: 64px;
  }
}

.docs-wrapper {
  max-width: 1100px;
  margin: 0 auto;
}

.docs-header {
  margin-bottom: 20px;
  padding: 24px;
  background: var(--bg-card);
  border: 1px solid rgba(128, 128, 128, 0.12);
  border-radius: 10px;
}

.docs-title {
  font-size: 28px;
  font-weight: 800;
  margin: 0 0 8px;
  color: rgba(0, 0, 0, 0.88);
  letter-spacing: -0.3px;
}

.docs-lead {
  margin: 0;
  color: rgba(0, 0, 0, 0.65);
  line-height: 1.65;
  font-size: 14px;
}

.docs-lead code,
.token-hint code {
  background: rgba(128, 128, 128, 0.12);
  padding: 1px 6px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
}

.token-card,
.curl-card {
  margin-bottom: 16px;
}

.token-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 10px;
  min-height: 32px;
}

.token-card-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 14px;
}

.token-hint {
  margin: 10px 0 0;
  color: rgba(0, 0, 0, 0.55);
  font-size: 12.5px;
  line-height: 1.55;
}

.code-block {
  background: rgba(128, 128, 128, 0.08);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 6px;
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.55;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.search-bar {
  flex: 1;
  min-width: 200px;
  max-width: 480px;
}

.match-count {
  font-size: 12px;
  color: rgba(0, 0, 0, 0.5);
  white-space: nowrap;
}

.toc-nav {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 8px 12px;
  padding: 12px 16px;
  background: var(--bg-card);
  border: 1px solid rgba(128, 128, 128, 0.12);
  border-radius: 8px;
  margin-bottom: 16px;
}

.toc-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.6px;
  color: rgba(0, 0, 0, 0.5);
  padding-top: 3px;
  flex-shrink: 0;
}

.toc-links {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.toc-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 12.5px;
  color: rgba(0, 0, 0, 0.65);
  background: rgba(128, 128, 128, 0.06);
  border: 1px solid transparent;
  text-decoration: none;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.toc-link:hover {
  background: rgba(22, 119, 255, 0.08);
  color: #1677ff;
  border-color: rgba(22, 119, 255, 0.2);
}

.toc-link.active {
  background: rgba(22, 119, 255, 0.12);
  color: #1677ff;
  border-color: rgba(22, 119, 255, 0.3);
  font-weight: 600;
}

.toc-icon {
  font-size: 13px;
  opacity: 0.8;
}

.toc-text {
  font-size: 12.5px;
}

.toc-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  font-size: 10.5px;
  font-weight: 700;
  background: rgba(22, 119, 255, 0.12);
  color: #1677ff;
  line-height: 1;
}

.toc-link.active .toc-badge {
  background: #1677ff;
  color: #fff;
}
</style>

<style>
body.dark .docs-title {
  color: rgba(255, 255, 255, 0.92);
}

html[data-theme='ultra-dark'] .docs-title {
  color: rgba(255, 255, 255, 0.95);
}

body.dark .docs-header {
  background: #252526;
  border-color: rgba(255, 255, 255, 0.08);
}

html[data-theme='ultra-dark'] .docs-header {
  background: #0a0a0a;
  border-color: rgba(255, 255, 255, 0.06);
}

body.dark .docs-lead,
body.dark .token-hint {
  color: rgba(255, 255, 255, 0.7);
}

html[data-theme='ultra-dark'] .docs-lead,
html[data-theme='ultra-dark'] .token-hint {
  color: rgba(255, 255, 255, 0.75);
}

body.dark .docs-lead code,
body.dark .token-hint code {
  background: rgba(255, 255, 255, 0.1);
}

html[data-theme='ultra-dark'] .docs-lead code,
html[data-theme='ultra-dark'] .token-hint code {
  background: rgba(255, 255, 255, 0.12);
}

body.dark .code-block {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.88);
}

html[data-theme='ultra-dark'] .code-block {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.08);
}

body.dark .toc-nav {
  background: #252526;
  border-color: rgba(255, 255, 255, 0.08);
}

html[data-theme='ultra-dark'] .toc-nav {
  background: #0a0a0a;
  border-color: rgba(255, 255, 255, 0.06);
}

body.dark .toc-label {
  color: rgba(255, 255, 255, 0.55);
}

html[data-theme='ultra-dark'] .toc-label {
  color: rgba(255, 255, 255, 0.6);
}

body.dark .toc-link {
  color: rgba(255, 255, 255, 0.65);
  background: rgba(255, 255, 255, 0.06);
}

html[data-theme='ultra-dark'] .toc-link {
  background: rgba(255, 255, 255, 0.04);
}

body.dark .toc-link:hover {
  background: rgba(88, 166, 255, 0.12);
  color: #58a6ff;
  border-color: rgba(88, 166, 255, 0.25);
}

body.dark .toc-link.active {
  background: rgba(88, 166, 255, 0.15);
  color: #58a6ff;
  border-color: rgba(88, 166, 255, 0.35);
}

body.dark .toc-badge {
  background: rgba(88, 166, 255, 0.15);
  color: #58a6ff;
}

body.dark .toc-link.active .toc-badge {
  background: #58a6ff;
  color: #0d1117;
}
</style>
