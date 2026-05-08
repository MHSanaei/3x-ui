<script setup>
import { computed, ref } from 'vue';
import { theme as antdTheme } from 'ant-design-vue';

import { theme as themeState } from '@/composables/useTheme.js';
import AppSidebar from '@/components/AppSidebar.vue';

// Drive AD-Vue 4's built-in dark algorithm from our reactive theme.
const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

// Phase 5c-i ships the page shell only — sidebar, layout, theme.
// Real content (CPU/mem/swap/disk cards, Xray status card, panel
// update modal, logs, custom-geo section) follows in 5c-ii through
// 5c-iv. Loading state is currently a placeholder true so the shell
// renders; it will be wired to the real /server/status fetch later.
const fetched = ref(true);

// In production the Go panel injects basePath + requestUri into the
// served HTML; during `npm run dev` we infer them from window.location.
const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="index-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content class="content-area">
          <a-spin :spinning="!fetched" :delay="200" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <div v-else class="page-body">
              <a-card hoverable>
                <a-space direction="vertical" :size="12" style="width: 100%">
                  <h2 style="margin: 0">Dashboard (vue3-migration shell)</h2>
                  <p style="margin: 0; opacity: 0.7">
                    Phase 5c-i: layout, sidebar, and theme switching wired up.
                    Status cards, xray controls, and custom-geo arrive in
                    follow-up commits.
                  </p>
                </a-space>
              </a-card>
            </div>
          </a-spin>
        </a-layout-content>
      </a-layout>
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.index-page {
  /* Same legacy palette source as the login page. */
  --bg-page: #f0f2f5;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.index-page.is-dark {
  --bg-page: #0a1222;  /* legacy --dark-color-background */
  --bg-card: #151f31;  /* legacy --dark-color-surface-100 */
}

.index-page.is-dark.is-ultra {
  --bg-page: #21242a;  /* legacy ultra --dark-color-background */
  --bg-card: #0c0e12;  /* legacy ultra surface-100 */
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

.page-body :deep(.ant-card) {
  background: var(--bg-card);
}
</style>
