<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  SettingOutlined,
  SwapOutlined,
  UploadOutlined,
  ClusterOutlined,
  DatabaseOutlined,
  CodeOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import BasicsTab from './BasicsTab.vue';
import RoutingTab from './RoutingTab.vue';
import OutboundsTab from './OutboundsTab.vue';
import BalancersTab from './BalancersTab.vue';
import DnsTab from './DnsTab.vue';
import WarpModal from './WarpModal.vue';
import NordModal from './NordModal.vue';
import { useXraySetting } from './useXraySetting.js';
import { useWebSocket } from '@/composables/useWebSocket.js';

const { t } = useI18n();

const {
  fetched,
  spinning,
  saveDisabled,
  fetchError,
  xraySetting,
  templateSettings,
  outboundTestUrl,
  inboundTags,
  clientReverseTags,
  restartResult,
  outboundsTraffic,
  outboundTestStates,
  fetchAll,
  resetOutboundsTraffic,
  testOutbound,
  saveAll,
  resetToDefault,
  restartXray,
  applyOutboundsEvent,
} = useXraySetting();

// Live outbounds traffic — pushed by xray_traffic_job every ~10s.
useWebSocket({ outbounds: applyOutboundsEvent });

async function onTestOutbound(idx) {
  const outbound = templateSettings.value?.outbounds?.[idx];
  if (outbound) await testOutbound(idx, outbound);
}

function onDeleteOutbound(idx) {
  templateSettings.value.outbounds.splice(idx, 1);
  outboundTestStates.value = Object.fromEntries(
    Object.entries(outboundTestStates.value)
      .filter(([k]) => Number(k) !== idx)
      .map(([k, v]) => [Number(k) > idx ? Number(k) - 1 : Number(k), v]),
  );
}

// === Advanced tab — radio-driven view ==============================
// Mirrors the legacy advanced page: a 4-way radio toggles which slice
// of the xray config the textarea edits — the full config, just the
// inbounds, just the outbounds, or just the routing rules. Each slice
// reads/writes through templateSettings so edits propagate to the
// dirty-poll and structured tabs.
const advSettings = ref('xraySetting');

const advancedText = computed({
  get: () => {
    if (advSettings.value === 'xraySetting') return xraySetting.value;
    const t = templateSettings.value;
    if (!t) return '';
    try {
      switch (advSettings.value) {
        case 'inboundSettings':
          return JSON.stringify(t.inbounds || [], null, 2);
        case 'outboundSettings':
          return JSON.stringify(t.outbounds || [], null, 2);
        case 'routingRuleSettings':
          return JSON.stringify(t.routing?.rules || [], null, 2);
        default:
          return '';
      }
    } catch (_e) {
      return '';
    }
  },
  set: (next) => {
    if (advSettings.value === 'xraySetting') {
      xraySetting.value = next;
      return;
    }
    // Slice edits: parse-then-merge into templateSettings so the
    // structured tabs and the dirty-poll re-stringify it cleanly.
    let parsed;
    try { parsed = JSON.parse(next); } catch (_e) { return; }
    const t = templateSettings.value;
    if (!t) return;
    switch (advSettings.value) {
      case 'inboundSettings':
        t.inbounds = parsed;
        break;
      case 'outboundSettings':
        t.outbounds = parsed;
        break;
      case 'routingRuleSettings':
        if (!t.routing) t.routing = {};
        t.routing.rules = parsed;
        break;
    }
  },
});

// `WarpExist` / `NordExist` derive from the parsed templateSettings —
// the Basics tab gates its WARP / NordVPN domain selectors on whether
// the matching outbound is provisioned, falling back to a "configure"
// button that today just toasts (the modals land in 6-v).
const warpExist = computed(
  () => !!templateSettings.value?.outbounds?.find((o) => o?.tag === 'warp'),
);
const nordExist = computed(
  () => !!templateSettings.value?.outbounds?.find((o) => o?.tag?.startsWith?.('nord-')),
);

// === WARP / NordVPN provisioning modals ============================
const warpOpen = ref(false);
const nordOpen = ref(false);

function showWarp() { warpOpen.value = true; }
function showNord() { nordOpen.value = true; }

function ensureOutbounds() {
  if (!templateSettings.value) return null;
  if (!Array.isArray(templateSettings.value.outbounds)) {
    templateSettings.value.outbounds = [];
  }
  return templateSettings.value.outbounds;
}

function onAddOutbound(outbound) {
  const list = ensureOutbounds();
  if (list) list.push(outbound);
}
function onResetOutbound({ index, outbound, oldTag, newTag }) {
  const list = ensureOutbounds();
  if (!list || index < 0) return;
  list[index] = outbound;
  // Tag rename across routing rules — preserves Nord's
  // server-switch flow without dangling references.
  if (oldTag && newTag && oldTag !== newTag) {
    const rules = templateSettings.value?.routing?.rules || [];
    for (const r of rules) {
      if (r?.outboundTag === oldTag) r.outboundTag = newTag;
    }
  }
}
function onRemoveOutboundByTag(tag) {
  const list = ensureOutbounds();
  if (!list) return;
  const idx = list.findIndex((o) => o?.tag === tag);
  if (idx >= 0) list.splice(idx, 1);
}
function onRemoveOutboundByIndex(index) {
  const list = ensureOutbounds();
  if (list && index >= 0) list.splice(index, 1);
}
function onRemoveRoutingRules({ prefix }) {
  const rules = templateSettings.value?.routing?.rules;
  if (!Array.isArray(rules)) return;
  templateSettings.value.routing.rules = rules.filter(
    (r) => !r?.outboundTag?.startsWith?.(prefix),
  );
}

// `message` is used by some of the in-progress UX flows (kept around
// because future provisioning errors will surface through it).
void message;
const { isMobile } = useMediaQuery();

const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

// See SettingsPage scrollTarget — wrap so `document` is in scope.
function scrollTarget() {
  return document.getElementById('content-layout');
}

function confirmRestart() {
  Modal.confirm({
    title: 'Restart xray?',
    content: 'Reloads the xray service with the saved configuration.',
    okText: 'Restart',
    cancelText: 'Cancel',
    onOk: () => restartXray(),
  });
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="xray-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="spinning || !fetched" :delay="200" tip="Loading…" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-result v-else-if="fetchError" status="error" :title="t('somethingWentWrong')" :sub-title="fetchError">
              <template #extra>
                <a-button type="primary" @click="fetchAll">{{ t('check') }}</a-button>
              </template>
            </a-result>

            <template v-else>
              <a-row :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
                <!-- Save / Restart bar -->
                <a-col :span="24">
                  <a-card hoverable>
                    <a-row class="header-row">
                      <a-col :xs="24" :sm="14" class="header-actions">
                        <a-space direction="horizontal">
                          <a-button type="primary" :disabled="saveDisabled" @click="saveAll">
                            {{ t('pages.xray.save') }}
                          </a-button>
                          <a-button type="primary" danger :disabled="!saveDisabled" @click="confirmRestart">
                            {{ t('pages.xray.restart') }}
                          </a-button>
                          <a-popover v-if="restartResult" placement="rightTop">
                            <template #title>Xray restart output</template>
                            <template #content>
                              <pre class="restart-result">{{ restartResult }}</pre>
                            </template>
                            <QuestionCircleOutlined class="restart-icon" />
                          </a-popover>
                        </a-space>
                      </a-col>
                      <a-col :xs="24" :sm="10" class="header-info">
                        <a-back-top :target="scrollTarget" :visibility-height="200" />
                        <a-alert type="warning" show-icon :message="t('pages.settings.infoDesc')" />
                      </a-col>
                    </a-row>
                  </a-card>
                </a-col>

                <!-- Tabs -->
                <a-col :span="24">
                  <a-tabs default-active-key="tpl-basic">
                    <a-tab-pane key="tpl-basic" class="tab-pane">
                      <template #tab>
                        <SettingOutlined /> <span>{{ t('pages.xray.basicTemplate') }}</span>
                      </template>
                      <BasicsTab :template-settings="templateSettings" :outbound-test-url="outboundTestUrl"
                        :warp-exist="warpExist" :nord-exist="nordExist"
                        @update:outbound-test-url="(v) => (outboundTestUrl = v)" @show-warp="showWarp"
                        @show-nord="showNord" @reset-default="resetToDefault" />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-routing" class="tab-pane">
                      <template #tab>
                        <SwapOutlined /> <span>{{ t('pages.xray.Routings') }}</span>
                      </template>
                      <RoutingTab :template-settings="templateSettings" :inbound-tags="inboundTags"
                        :client-reverse-tags="clientReverseTags" :is-mobile="isMobile" />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-outbound" class="tab-pane">
                      <template #tab>
                        <UploadOutlined /> <span>{{ t('pages.xray.Outbounds') }}</span>
                      </template>
                      <OutboundsTab :template-settings="templateSettings" :outbounds-traffic="outboundsTraffic"
                        :outbound-test-states="outboundTestStates" :inbound-tags="inboundTags" :is-mobile="isMobile"
                        @reset-traffic="resetOutboundsTraffic" @test="onTestOutbound" @delete="onDeleteOutbound"
                        @show-warp="showWarp" @show-nord="showNord" />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-balancer" class="tab-pane">
                      <template #tab>
                        <ClusterOutlined /> <span>{{ t('pages.xray.Balancers') }}</span>
                      </template>
                      <BalancersTab :template-settings="templateSettings" :client-reverse-tags="clientReverseTags" />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-dns" class="tab-pane">
                      <template #tab>
                        <DatabaseOutlined /> <span>DNS</span>
                      </template>
                      <DnsTab :template-settings="templateSettings" />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-advanced" class="tab-pane">
                      <template #tab>
                        <CodeOutlined /> <span>{{ t('pages.xray.advancedTemplate') }}</span>
                      </template>
                      <a-list-item-meta :title="t('pages.xray.Template')" :description="t('pages.xray.TemplateDesc')" />
                      <a-radio-group v-model:value="advSettings" button-style="solid"
                        :size="isMobile ? 'small' : 'middle'" :style="{ margin: '12px 0' }">
                        <a-radio-button value="xraySetting">{{ t('pages.xray.completeTemplate') }}</a-radio-button>
                        <a-radio-button value="inboundSettings">{{ t('pages.xray.Inbounds') }}</a-radio-button>
                        <a-radio-button value="outboundSettings">{{ t('pages.xray.Outbounds') }}</a-radio-button>
                        <a-radio-button value="routingRuleSettings">{{ t('pages.xray.Routings') }}</a-radio-button>
                      </a-radio-group>
                      <a-textarea v-model:value="advancedText" :auto-size="{ minRows: 18, maxRows: 40 }"
                        spellcheck="false" class="json-editor" />
                    </a-tab-pane>
                  </a-tabs>
                </a-col>
              </a-row>
            </template>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <WarpModal v-model:open="warpOpen" :template-settings="templateSettings" @add-outbound="onAddOutbound"
        @reset-outbound="onResetOutbound" @remove-outbound="onRemoveOutboundByTag" />
      <NordModal v-model:open="nordOpen" :template-settings="templateSettings" @add-outbound="onAddOutbound"
        @reset-outbound="onResetOutbound" @remove-outbound="onRemoveOutboundByIndex"
        @remove-routing-rules="onRemoveRoutingRules" />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.xray-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.xray-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.xray-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.xray-page :deep(.ant-layout),
.xray-page :deep(.ant-layout-content) {
  background: transparent;
}

.content-shell {
  background: transparent;
}

.content-area {
  padding: 24px;
}

.loading-spacer {
  min-height: calc(100vh - 120px);
}

.header-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}

.header-actions {
  padding: 4px;
}

.header-info {
  display: flex;
  justify-content: flex-end;
}

.tab-pane {
  padding-top: 20px;
}

.restart-icon {
  font-size: 16px;
  cursor: pointer;
  color: var(--ant-primary-color, #1890ff);
}

.restart-result {
  max-width: 480px;
  white-space: pre-wrap;
  font-size: 12px;
  margin: 0;
}

.json-editor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}
</style>
