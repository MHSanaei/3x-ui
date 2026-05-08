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
  fetchOutboundsTraffic,
  resetOutboundsTraffic,
  testOutbound,
  saveAll,
  restartXray,
} = useXraySetting();

async function onTestOutbound(idx) {
  const outbound = templateSettings.value?.outbounds?.[idx];
  if (outbound) await testOutbound(idx, outbound);
}

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
    <a-layout
      class="xray-page"
      :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }"
    >
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="spinning || !fetched" :delay="200" tip="Loading…" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-result
              v-else-if="fetchError"
              status="error"
              :title="t('somethingWentWrong')"
              :sub-title="fetchError"
            >
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
                        <a-alert
                          type="warning"
                          show-icon
                          :message="t('pages.settings.infoDesc')"
                        />
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
                      <BasicsTab
                        :template-settings="templateSettings"
                        :outbound-test-url="outboundTestUrl"
                        :warp-exist="warpExist"
                        :nord-exist="nordExist"
                        @update:outbound-test-url="(v) => (outboundTestUrl = v)"
                        @show-warp="showWarp"
                        @show-nord="showNord"
                      />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-routing" class="tab-pane">
                      <template #tab>
                        <SwapOutlined /> <span>{{ t('pages.xray.Routings') }}</span>
                      </template>
                      <RoutingTab
                        :template-settings="templateSettings"
                        :inbound-tags="inboundTags"
                        :client-reverse-tags="clientReverseTags"
                        :is-mobile="isMobile"
                      />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-outbound" class="tab-pane">
                      <template #tab>
                        <UploadOutlined /> <span>{{ t('pages.xray.Outbounds') }}</span>
                      </template>
                      <OutboundsTab
                        :template-settings="templateSettings"
                        :outbounds-traffic="outboundsTraffic"
                        :outbound-test-states="outboundTestStates"
                        :is-mobile="isMobile"
                        @refresh-traffic="fetchOutboundsTraffic"
                        @reset-traffic="resetOutboundsTraffic"
                        @test="onTestOutbound"
                        @show-warp="showWarp"
                        @show-nord="showNord"
                      />
                    </a-tab-pane>

                    <a-tab-pane key="tpl-balancer" class="tab-pane">
                      <template #tab>
                        <ClusterOutlined /> <span>{{ t('pages.xray.Balancers') }}</span>
                      </template>
                      <BalancersTab :template-settings="templateSettings" />
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
                      <a-form layout="vertical">
                        <a-form-item label="xraySetting (full JSON)">
                          <a-textarea
                            v-model:value="xraySetting"
                            :auto-size="{ minRows: 18, maxRows: 40 }"
                            spellcheck="false"
                            class="json-editor"
                          />
                        </a-form-item>
                      </a-form>
                    </a-tab-pane>
                  </a-tabs>
                </a-col>
              </a-row>
            </template>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <WarpModal
        v-model:open="warpOpen"
        :template-settings="templateSettings"
        @add-outbound="onAddOutbound"
        @reset-outbound="onResetOutbound"
        @remove-outbound="onRemoveOutboundByTag"
      />
      <NordModal
        v-model:open="nordOpen"
        :template-settings="templateSettings"
        @add-outbound="onAddOutbound"
        @reset-outbound="onResetOutbound"
        @remove-outbound="onRemoveOutboundByIndex"
        @remove-routing-rules="onRemoveRoutingRules"
      />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.xray-page {
  --bg-page: #f0f2f5;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.xray-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.xray-page.is-dark.is-ultra {
  --bg-page: #21242a;
  --bg-card: #0c0e12;
}

.xray-page :deep(.ant-layout),
.xray-page :deep(.ant-layout-content) {
  background: transparent;
}

.content-shell { background: transparent; }
.content-area { padding: 24px; }

.loading-spacer { min-height: calc(100vh - 120px); }

.header-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}
.header-actions { padding: 4px; }
.header-info {
  display: flex;
  justify-content: flex-end;
}

.tab-pane { padding-top: 20px; }

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
