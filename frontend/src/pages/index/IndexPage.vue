<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { theme as antdTheme } from 'ant-design-vue';
import { BarsOutlined, CloudServerOutlined, CloudDownloadOutlined } from '@ant-design/icons-vue';

const { t } = useI18n();

import { HttpUtil } from '@/utils';
import { theme as themeState } from '@/composables/useTheme.js';
import { useStatus } from '@/composables/useStatus.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import StatusCard from './StatusCard.vue';
import XrayStatusCard from './XrayStatusCard.vue';
import PanelUpdateModal from './PanelUpdateModal.vue';
import LogModal from './LogModal.vue';
import BackupModal from './BackupModal.vue';
import CpuHistoryModal from './CpuHistoryModal.vue';
import XrayLogModal from './XrayLogModal.vue';
import VersionModal from './VersionModal.vue';

// Drive AD-Vue 4's built-in dark algorithm from our reactive theme.
const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

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

// Modal open state.
const logsOpen = ref(false);
const backupOpen = ref(false);
const panelUpdateOpen = ref(false);
const cpuHistoryOpen = ref(false);
const xrayLogsOpen = ref(false);
const versionOpen = ref(false);

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

function openCpuHistory() { cpuHistoryOpen.value = true; }
function openXrayLogs() { xrayLogsOpen.value = true; }
function openVersionSwitch() { versionOpen.value = true; }
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="index-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content class="content-area">
          <a-spin :spinning="loading || !fetched" :delay="200" :tip="loading ? loadingTip : t('loading')" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
              <a-col :span="24">
                <StatusCard :status="status" :is-mobile="isMobile" @open-cpu-history="openCpuHistory" />
              </a-col>

              <a-col :sm="24" :lg="12">
                <XrayStatusCard
                  :status="status"
                  :is-mobile="isMobile"
                  :ip-limit-enable="ipLimitEnable"
                  @stop-xray="stopXray"
                  @restart-xray="restartXray"
                  @open-xray-logs="openXrayLogs"
                  @open-logs="logsOpen = true"
                  @open-version-switch="openVersionSwitch"
                />
              </a-col>

              <a-col :sm="24" :lg="12">
                <a-card :title="t('menu.link')" hoverable>
                  <template v-if="panelUpdateInfo.updateAvailable" #extra>
                    <a-tooltip :title="`${t('pages.index.updatePanel')}: ${panelUpdateInfo.latestVersion}`">
                      <a-tag color="orange" class="update-tag" @click="panelUpdateOpen = true">
                        <CloudDownloadOutlined />
                        {{ panelUpdateInfo.latestVersion }}
                        <span v-if="!isMobile">{{ t('update') }}</span>
                      </a-tag>
                    </a-tooltip>
                  </template>
                  <template #actions>
                    <a-space class="action" @click="logsOpen = true">
                      <BarsOutlined />
                      <span v-if="!isMobile">{{ t('pages.index.logs') }}</span>
                    </a-space>
                    <a-space class="action" @click="backupOpen = true">
                      <CloudServerOutlined />
                      <span v-if="!isMobile">{{ t('pages.index.backupTitle') }}</span>
                    </a-space>
                    <a-space class="action" @click="panelUpdateOpen = true">
                      <CloudDownloadOutlined />
                      <span v-if="!isMobile">
                        {{ panelUpdateInfo.updateAvailable
                          ? `${t('update')} → ${panelUpdateInfo.latestVersion}`
                          : t('pages.index.panelUpToDate') }}
                      </span>
                    </a-space>
                  </template>
                </a-card>
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <PanelUpdateModal
        v-model:open="panelUpdateOpen"
        :info="panelUpdateInfo"
        @busy="setBusy"
      />
      <LogModal v-model:open="logsOpen" />
      <BackupModal
        v-model:open="backupOpen"
        :base-path="basePath"
        @busy="setBusy"
      />
      <CpuHistoryModal v-model:open="cpuHistoryOpen" :status="status" />
      <XrayLogModal v-model:open="xrayLogsOpen" />
      <VersionModal v-model:open="versionOpen" :status="status" @busy="setBusy" />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.index-page {
  --bg-page: #f0f2f5;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.index-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.index-page.is-dark.is-ultra {
  --bg-page: #21242a;
  --bg-card: #0c0e12;
}

.index-page :deep(.ant-layout),
.index-page :deep(.ant-layout-content) {
  background: transparent;
}

.content-shell { background: transparent; }
.content-area { padding: 24px; }

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
</style>
