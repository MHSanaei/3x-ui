<script setup>
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  KeyOutlined,
  ReloadOutlined,
  CopyOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  SearchOutlined,
  ExpandOutlined,
  CompressOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import AppSidebar from '@/components/AppSidebar.vue';
import { HttpUtil, ClipboardManager } from '@/utils/index.js';
import { sections as allSections } from './endpoints.js';
import EndpointSection from './EndpointSection.vue';
import CodeBlock from './CodeBlock.vue';

const { t } = useI18n();

const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;

const apiToken = ref('');
const tokenLoading = ref(false);
const tokenRotating = ref(false);
const tokenVisible = ref(false);

const searchQuery = ref('');
const collapsedSections = ref(new Set());

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

const visibleSections = computed(() =>
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

async function loadApiToken() {
  tokenLoading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/setting/getApiToken');
    if (msg?.success) apiToken.value = msg.obj || '';
  } finally {
    tokenLoading.value = false;
  }
}

function regenerateApiToken() {
  Modal.confirm({
    title: t('pages.nodes.regenerateConfirm'),
    okText: t('confirm'),
    cancelText: t('cancel'),
    okType: 'danger',
    onOk: async () => {
      tokenRotating.value = true;
      try {
        const msg = await HttpUtil.post('/panel/setting/regenerateApiToken');
        if (msg?.success) {
          apiToken.value = msg.obj || '';
          message.success(t('success'));
        }
      } finally {
        tokenRotating.value = false;
      }
    },
  });
}

async function copyApiToken() {
  if (!apiToken.value) return;
  const ok = await ClipboardManager.copyText(apiToken.value);
  if (ok) message.success(t('success'));
}

function scrollToSection(id) {
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

onMounted(() => {
  loadApiToken();
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
                  <span>API Token</span>
                </div>
                <a-space size="small" wrap>
                  <a-button size="small" @click="tokenVisible = !tokenVisible">
                    <template #icon>
                      <EyeInvisibleOutlined v-if="tokenVisible" />
                      <EyeOutlined v-else />
                    </template>
                    {{ tokenVisible ? 'Hide' : 'Show' }}
                  </a-button>
                  <a-button size="small" :disabled="!apiToken" @click="copyApiToken">
                    <template #icon>
                      <CopyOutlined />
                    </template>
                    Copy
                  </a-button>
                  <a-button size="small" danger :loading="tokenRotating" @click="regenerateApiToken">
                    <template #icon>
                      <ReloadOutlined />
                    </template>
                    Regenerate
                  </a-button>
                </a-space>
              </div>
              <a-spin :spinning="tokenLoading" size="small">
                <pre
                  class="token-value">{{ tokenVisible ? (apiToken || '—') : (apiToken ? '••••••••••••••••••••••••••••' : '—') }}</pre>
              </a-spin>
              <p class="token-hint">
                Send it on every request as <code>Authorization: Bearer &lt;token&gt;</code>. Token-authenticated
                callers skip CSRF and don't need a session cookie. Regenerating rotates the secret immediately —
                running bots will need the new value.
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
                {{ visibleSections }} / {{ endpointCount }} endpoints
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
              <a v-for="s in sections" :key="s.id" class="toc-link" :href="`#${s.id}`"
                @click.prevent="scrollToSection(s.id)">
                {{ s.title }} ({{ s.endpoints.length }})
              </a>
            </nav>

            <EndpointSection
              v-for="s in sections"
              :key="s.id"
              :section="s"
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
  margin-bottom: 18px;
}

.docs-title {
  font-size: 26px;
  font-weight: 700;
  margin: 0 0 8px;
  color: rgba(0, 0, 0, 0.88);
}

.docs-lead {
  margin: 0;
  color: rgba(0, 0, 0, 0.65);
  line-height: 1.6;
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
  margin-bottom: 8px;
}

.token-card-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 14px;
}

.token-value {
  background: rgba(128, 128, 128, 0.08);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 6px;
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  margin: 0;
  word-break: break-all;
  white-space: pre-wrap;
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
  align-items: center;
  gap: 8px 14px;
  padding: 12px 16px;
  background: rgba(128, 128, 128, 0.08);
  border-radius: 6px;
  margin-bottom: 16px;
}

.toc-label {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: rgba(0, 0, 0, 0.5);
}

.toc-link {
  color: #1677ff;
  text-decoration: none;
  cursor: pointer;
  font-size: 13px;
}

.toc-link:hover {
  color: #4096ff;
  text-decoration: underline;
}
</style>

<style>
body.dark .docs-title {
  color: rgba(255, 255, 255, 0.92);
}

body.dark .docs-lead,
body.dark .token-hint {
  color: rgba(255, 255, 255, 0.7);
}

body.dark .docs-lead code,
body.dark .token-hint code {
  background: rgba(255, 255, 255, 0.1);
}

body.dark .token-value,
body.dark .code-block {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.88);
}

body.dark .toc-nav {
  background: rgba(255, 255, 255, 0.04);
}

body.dark .toc-label {
  color: rgba(255, 255, 255, 0.55);
}
</style>
