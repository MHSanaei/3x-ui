<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  BarsOutlined,
  ControlOutlined,
  CloudServerOutlined,
  CloudDownloadOutlined,
  CloudUploadOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  AreaChartOutlined,
  GlobalOutlined,
  SwapOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
} from '@ant-design/icons-vue';

const { t } = useI18n();

import { HttpUtil, SizeFormatter, TimeFormatter } from '@/utils';
import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useStatus } from '@/composables/useStatus.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import TextModal from '@/components/TextModal.vue';
import StatusCard from './StatusCard.vue';
import XrayStatusCard from './XrayStatusCard.vue';
import PanelUpdateModal from './PanelUpdateModal.vue';
import LogModal from './LogModal.vue';
import BackupModal from './BackupModal.vue';
import SystemHistoryModal from './SystemHistoryModal.vue';
import XrayLogModal from './XrayLogModal.vue';
import VersionModal from './VersionModal.vue';

const { status, fetched, refresh } = useStatus();
const { isMobile } = useMediaQuery();

// `/panel/setting/defaultSettings` returns ipLimitEnable; the xray
// card hides its log button when access logs are off.
const ipLimitEnable = ref(false);
HttpUtil.post('/panel/setting/defaultSettings').then((msg) => {
  if (msg?.success && msg.obj) ipLimitEnable.value = !!msg.obj.ipLimitEnable;
});

// Panel-update info — fetched once on mount, drives both the badge
// in QuickActions and the contents of PanelUpdateModal.
const panelUpdateInfo = ref({ currentVersion: '', latestVersion: '', updateAvailable: false });
onMounted(() => {
  HttpUtil.get('/panel/api/server/getPanelUpdateInfo').then((msg) => {
    if (msg?.success && msg.obj) panelUpdateInfo.value = msg.obj;
  });
});

const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

// In production, dist.go injects window.__X_UI_CUR_VER__ at serve time.
// In dev, Vite serves the HTML directly so the global is missing — fall
// back to currentVersion from the panel-update API once it answers.
const displayVersion = computed(
  () => panelUpdateInfo.value?.currentVersion || window.__X_UI_CUR_VER__ || '?',
);

// Hide/reveal the public IPv4/IPv6 — same pattern as legacy.
const showIp = ref(false);

// Modal open state.
const logsOpen = ref(false);
const backupOpen = ref(false);
const panelUpdateOpen = ref(false);
const sysHistoryOpen = ref(false);
const xrayLogsOpen = ref(false);
const versionOpen = ref(false);
const configTextOpen = ref(false);
const configText = ref('');

// Page-level loading overlay; modals can request it via @busy.
const loading = ref(false);
const loadingTip = ref(t('loading'));
function setBusy({ busy, tip }) {
  loading.value = busy;
  if (tip) loadingTip.value = tip;
}

// Xray controls
async function stopXray() {
  await HttpUtil.post('/panel/api/server/stopXrayService');
  await refresh();
}
async function restartXray() {
  await HttpUtil.post('/panel/api/server/restartXrayService');
  await refresh();
}

function openSystemHistory() { sysHistoryOpen.value = true; }
function openXrayLogs() { xrayLogsOpen.value = true; }
function openVersionSwitch() { versionOpen.value = true; }

// Legacy "Config" action — fetch the rendered xray config and show
// it as JSON in the shared TextModal (same UX as main).
async function openConfig() {
  loading.value = true;
  try {
    const msg = await HttpUtil.get('/panel/api/server/getConfigJson');
    if (!msg?.success) return;
    configText.value = JSON.stringify(msg.obj, null, 2);
    configTextOpen.value = true;
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="index-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content class="content-area">
          <a-spin :spinning="loading || !fetched" :delay="200" :tip="loading ? loadingTip : t('loading')" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, 12]">
              <a-col :span="24">
                <StatusCard :status="status" :is-mobile="isMobile" />
              </a-col>

              <a-col :xs="24" :lg="12">
                <XrayStatusCard :status="status" :is-mobile="isMobile" :ip-limit-enable="ipLimitEnable"
                  @stop-xray="stopXray" @restart-xray="restartXray" @open-xray-logs="openXrayLogs"
                  @open-logs="logsOpen = true" @open-version-switch="openVersionSwitch" />
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('menu.link')" hoverable>
                  <template #actions>
                    <a-space class="action" @click="logsOpen = true">
                      <BarsOutlined />
                      <span v-if="!isMobile">{{ t('pages.index.logs') }}</span>
                    </a-space>
                    <a-space class="action" @click="openConfig">
                      <ControlOutlined />
                      <span v-if="!isMobile">{{ t('pages.index.config') }}</span>
                    </a-space>
                    <a-space class="action" @click="backupOpen = true">
                      <CloudServerOutlined />
                      <span v-if="!isMobile">{{ t('pages.index.backupTitle') }}</span>
                    </a-space>
                  </template>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card title="3X-UI" hoverable>
                  <template v-if="panelUpdateInfo.updateAvailable" #extra>
                    <a-tooltip :title="`${t('pages.index.updatePanel')}: ${panelUpdateInfo.latestVersion}`">
                      <a-tag color="orange" class="update-tag" @click="panelUpdateOpen = true">
                        <CloudDownloadOutlined />
                        {{ panelUpdateInfo.latestVersion }}
                        <span v-if="!isMobile">{{ t('pages.index.updatePanel') }}</span>
                      </a-tag>
                    </a-tooltip>
                  </template>
                  <div class="link-tags">
                    <a href="https://github.com/MHSanaei/3x-ui/releases" target="_blank" rel="noopener noreferrer">
                      <a-tag color="green">v{{ displayVersion }}</a-tag>
                    </a>
                    <a href="https://t.me/XrayUI" target="_blank" rel="noopener noreferrer">
                      <a-tag color="green">@XrayUI</a-tag>
                    </a>
                    <a href="https://github.com/MHSanaei/3x-ui/wiki" target="_blank" rel="noopener noreferrer">
                      <a-tag color="purple">{{ t('pages.index.documentation') }}</a-tag>
                    </a>
                  </div>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('pages.index.operationHours')" hoverable>
                  <a-tag :color="status.xray.color">
                    Xray: {{ TimeFormatter.formatSecond(status.appStats.uptime) }}
                  </a-tag>
                  <a-tag color="green">OS: {{ TimeFormatter.formatSecond(status.uptime) }}</a-tag>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('pages.index.systemLoad')" hoverable>
                  <template #extra>
                    <a-tag color="blue" class="history-tag" @click="openSystemHistory">
                      <AreaChartOutlined />
                      {{ t('pages.index.systemHistoryTitle') }}
                    </a-tag>
                  </template>
                  <a-tooltip :title="t('pages.index.systemLoadDesc')">
                    <a-tag color="green">
                      {{ status.loads[0] }} | {{ status.loads[1] }} | {{ status.loads[2] }}
                    </a-tag>
                  </a-tooltip>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('usage')" hoverable>
                  <a-tag color="green">
                    {{ t('pages.index.memory') }}: {{ SizeFormatter.sizeFormat(status.appStats.mem) }}
                  </a-tag>
                  <a-tag color="green">
                    {{ t('pages.index.threads') }}: {{ status.appStats.threads }}
                  </a-tag>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('pages.index.overallSpeed')" hoverable>
                  <a-row :gutter="isMobile ? [8, 8] : 0">
                    <a-col :span="12">
                      <CustomStatistic :title="t('pages.index.upload')"
                        :value="SizeFormatter.sizeFormat(status.netIO.up)">
                        <template #prefix>
                          <ArrowUpOutlined />
                        </template>
                        <template #suffix>/s</template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :span="12">
                      <CustomStatistic :title="t('pages.index.download')"
                        :value="SizeFormatter.sizeFormat(status.netIO.down)">
                        <template #prefix>
                          <ArrowDownOutlined />
                        </template>
                        <template #suffix>/s</template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('pages.index.totalData')" hoverable>
                  <a-row :gutter="isMobile ? [8, 8] : 0">
                    <a-col :span="12">
                      <CustomStatistic :title="t('pages.index.sent')"
                        :value="SizeFormatter.sizeFormat(status.netTraffic.sent)">
                        <template #prefix>
                          <CloudUploadOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :span="12">
                      <CustomStatistic :title="t('pages.index.received')"
                        :value="SizeFormatter.sizeFormat(status.netTraffic.recv)">
                        <template #prefix>
                          <CloudDownloadOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('pages.index.ipAddresses')" hoverable>
                  <template #extra>
                    <a-tooltip :title="t('pages.index.toggleIpVisibility')" :placement="isMobile ? 'topRight' : 'top'">
                      <component :is="showIp ? EyeOutlined : EyeInvisibleOutlined" class="ip-toggle-icon"
                        @click="showIp = !showIp" />
                    </a-tooltip>
                  </template>
                  <a-row :class="showIp ? 'ip-visible' : 'ip-hidden'" :gutter="isMobile ? [8, 8] : 0">
                    <a-col :span="isMobile ? 24 : 12">
                      <CustomStatistic title="IPv4" :value="status.publicIP.ipv4">
                        <template #prefix>
                          <GlobalOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :span="isMobile ? 24 : 12">
                      <CustomStatistic title="IPv6" :value="status.publicIP.ipv6">
                        <template #prefix>
                          <GlobalOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <a-col :xs="24" :lg="12">
                <a-card :title="t('pages.index.connectionCount')" hoverable>
                  <a-row :gutter="isMobile ? [8, 8] : 0">
                    <a-col :span="12">
                      <CustomStatistic title="TCP" :value="status.tcpCount">
                        <template #prefix>
                          <SwapOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :span="12">
                      <CustomStatistic title="UDP" :value="status.udpCount">
                        <template #prefix>
                          <SwapOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <PanelUpdateModal v-model:open="panelUpdateOpen" :info="panelUpdateInfo" @busy="setBusy" />
      <LogModal v-model:open="logsOpen" />
      <BackupModal v-model:open="backupOpen" :base-path="basePath" @busy="setBusy" />
      <SystemHistoryModal v-model:open="sysHistoryOpen" :status="status" />
      <XrayLogModal v-model:open="xrayLogsOpen" />
      <VersionModal v-model:open="versionOpen" :status="status" @busy="setBusy" />
      <TextModal v-model:open="configTextOpen" :title="t('pages.index.config')" :content="configText"
        file-name="config.json" />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.index-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.index-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.index-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.index-page :deep(.ant-layout),
.index-page :deep(.ant-layout-content) {
  background: transparent;
}

.content-shell {
  background: transparent;
}

.content-area {
  padding: 24px;
}

@media (max-width: 768px) {
  .content-area {
    padding: 12px;
    padding-top: 64px;
  }
}

.loading-spacer {
  min-height: calc(100vh - 120px);
}

.action {
  cursor: pointer;
  justify-content: center;
}

.update-tag {
  cursor: pointer;
  margin: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.history-tag {
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-inline-end: 0;
}

.link-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.link-tags a {
  display: inline-flex;
}

.link-tags :deep(.ant-tag) {
  margin-inline-end: 0;
}

.ip-toggle-icon {
  cursor: pointer;
  font-size: 16px;
}

.ip-hidden :deep(.ant-statistic-content-value) {
  filter: blur(6px);
  transition: filter 0.2s ease;
}

.ip-visible :deep(.ant-statistic-content-value) {
  filter: none;
}
</style>
