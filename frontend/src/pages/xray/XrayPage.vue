<script setup>
import { computed } from 'vue';
import { theme as antdTheme, Modal } from 'ant-design-vue';
import {
  SettingOutlined,
  SwapOutlined,
  UploadOutlined,
  ClusterOutlined,
  DatabaseOutlined,
  CodeOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import { message } from 'ant-design-vue';
import AppSidebar from '@/components/AppSidebar.vue';
import BasicsTab from './BasicsTab.vue';
import RoutingTab from './RoutingTab.vue';
import OutboundsTab from './OutboundsTab.vue';
import BalancersTab from './BalancersTab.vue';
import DnsTab from './DnsTab.vue';
import { useXraySetting } from './useXraySetting.js';

// Phase 6-i: scaffold + advanced JSON tab. Other tabs (Basics, Routing,
// Outbounds, Balancers, DNS) land in subsequent 6-ii…vi commits — they
// each need their own tree of structured forms or a dedicated modal.
// For now they show an a-empty placeholder so the navigation is
// stable and users can still edit the full config via the Advanced
// (JSON) tab.

const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

const {
  fetched,
  spinning,
  saveDisabled,
  xraySetting,
  templateSettings,
  outboundTestUrl,
  inboundTags,
  clientReverseTags,
  restartResult,
  outboundsTraffic,
  outboundTestStates,
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

function showWarp() { message.info('WARP outbound modal — coming in 6-v'); }
function showNord() { message.info('NordVPN outbound modal — coming in 6-v'); }
const { isMobile } = useMediaQuery();

const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

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

            <template v-else>
              <a-row :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
                <!-- Save / Restart bar -->
                <a-col :span="24">
                  <a-card hoverable>
                    <a-row class="header-row">
                      <a-col :xs="24" :sm="14" class="header-actions">
                        <a-space direction="horizontal">
                          <a-button type="primary" :disabled="saveDisabled" @click="saveAll">
                            Save
                          </a-button>
                          <a-button type="primary" danger :disabled="!saveDisabled" @click="confirmRestart">
                            Restart xray
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
                        <a-back-top :target="() => document.getElementById('content-layout')" :visibility-height="200" />
                        <a-alert
                          type="warning"
                          show-icon
                          message="Save before restarting — unsaved changes are dropped on restart."
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
                        <SettingOutlined /> <span>Basic template</span>
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
                        <SwapOutlined /> <span>Routing</span>
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
                        <UploadOutlined /> <span>Outbounds</span>
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
                        <ClusterOutlined /> <span>Balancers</span>
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
                        <CodeOutlined /> <span>Advanced (JSON)</span>
                      </template>
                      <a-form layout="vertical">
                        <a-form-item label="Outbound test URL">
                          <a-input
                            v-model:value="outboundTestUrl"
                            placeholder="https://www.google.com/generate_204"
                          />
                        </a-form-item>
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
