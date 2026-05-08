<script setup>
import { computed } from 'vue';
import { theme as antdTheme } from 'ant-design-vue';

import { theme as themeState } from '@/composables/useTheme.js';
import { useStatus } from '@/composables/useStatus.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import StatusCard from './StatusCard.vue';

// Drive AD-Vue 4's built-in dark algorithm from our reactive theme.
const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

const { status, fetched } = useStatus();
const { isMobile } = useMediaQuery();

// In production the Go panel injects basePath + requestUri into the
// served HTML; during `npm run dev` we infer them from window.location.
const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

function onOpenCpuHistory() {
  // CPU-history modal is part of Phase 5c-iv. Leaving the emit wired
  // so the button isn't dead-clickable; no-op until then.
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="index-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content class="content-area">
          <a-spin :spinning="!fetched" :delay="200" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
              <a-col :span="24">
                <StatusCard :status="status" :is-mobile="isMobile" @open-cpu-history="onOpenCpuHistory" />
              </a-col>
              <a-col :span="24">
                <a-card hoverable>
                  <a-space direction="vertical" :size="8" style="width: 100%">
                    <h3 style="margin: 0">Dashboard scaffold</h3>
                    <p style="margin: 0; opacity: 0.7">
                      Phase 5c-ii adds the live status cards above (CPU / memory / swap / disk).
                      Xray status, panel update modal, logs, and the custom-geo section
                      arrive in 5c-iii through 5c-v.
                    </p>
                  </a-space>
                </a-card>
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>
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

.content-shell {
  background: transparent;
}

.content-area {
  padding: 24px;
}

.loading-spacer {
  min-height: calc(100vh - 120px);
}
</style>
