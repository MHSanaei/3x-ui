<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  SettingOutlined,
  AndroidOutlined,
  AppleOutlined,
  DownOutlined,
  CopyOutlined,
} from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { ClipboardManager, IntlUtil, LanguageManager } from '@/utils';
import {
  theme as themeState,
  antdThemeConfig,
  toggleTheme,
  toggleUltra,
  pauseAnimationsUntilLeave,
} from '@/composables/useTheme.js';

const { t } = useI18n();

// Read the view-model Go injects via window.__SUB_PAGE_DATA__. Falls
// back to safe defaults so the page still renders if the global is
// missing (e.g. during local dev without the backend).
const subData = window.__SUB_PAGE_DATA__ || {};

const sId = subData.sId || '';
const enabled = !!subData.enabled;
const download = subData.download || '0';
const upload = subData.upload || '0';
const total = subData.total || '∞';
const used = subData.used || '0';
const remained = subData.remained || '';
const totalByte = Number(subData.totalByte || 0);
const expireMs = Number(subData.expire || 0) * 1000;
const lastOnlineMs = Number(subData.lastOnline || 0);
const subUrl = subData.subUrl || '';
const subJsonUrl = subData.subJsonUrl || '';
const subClashUrl = subData.subClashUrl || '';
const subTitle = subData.subTitle || '';
const links = Array.isArray(subData.links) ? subData.links : [];
// Panel's "Calendar Type" setting; controls whether expiry / lastOnline
// render in Gregorian or Jalali on this standalone subscription page.
const datepicker = subData.datepicker || 'gregorian';

// Derived state ===============================================
const isUnlimited = computed(() => totalByte <= 0 && expireMs === 0);
const isActive = computed(() => {
  if (!enabled) return false;
  if (totalByte > 0) {
    const used = Number(subData.usedByte || 0)
      || (Number(subData.downloadByte || 0) + Number(subData.uploadByte || 0));
    if (used >= totalByte) return false;
  }
  if (expireMs > 0 && Date.now() >= expireMs) return false;
  return true;
});

// Mobile-aware layout — shows app dropdowns full-width below 576px
const isMobile = ref(false);
function updateMobile() { isMobile.value = window.innerWidth < 576; }
onMounted(() => {
  updateMobile();
  window.addEventListener('resize', updateMobile);
});

// Language switcher mirrors the legacy panel: setting the language
// triggers a full-page reload which re-renders with the new locale.
const lang = ref(LanguageManager.getLanguage());
function onLangChange(next) {
  LanguageManager.setLanguage(next);
}

/* Same Light -> Dark -> Ultra Dark -> Light cycle the panel sidebar
 * uses, so the standalone subscription page offers a one-click theme
 * toggle without the popover ceremony. */
function cycleTheme() {
  pauseAnimationsUntilLeave('sub-theme-cycle');
  if (!themeState.isDark) {
    toggleTheme();
    if (themeState.isUltra) toggleUltra();
  } else if (!themeState.isUltra) {
    toggleUltra();
  } else {
    toggleUltra();
    toggleTheme();
  }
}

const QR_SIZE = 240;

// Actions =====================================================
async function copy(value) {
  if (!value) return;
  const ok = await ClipboardManager.copyText(value);
  if (ok) message.success(t('copied'));
}

function open(url) {
  if (!url) return;
  window.open(url, '_blank');
}

// Pretty label per share link — pulls protocol + remark out of the
// URL fragment (most clients put the remark after the # sign).
function linkName(link, idx) {
  if (!link) return `Link ${idx + 1}`;
  const hashIdx = link.indexOf('#');
  if (hashIdx >= 0 && hashIdx + 1 < link.length) {
    try {
      return decodeURIComponent(link.slice(hashIdx + 1));
    } catch (_e) {
      return link.slice(hashIdx + 1);
    }
  }
  const proto = link.split('://')[0];
  return `${proto.toUpperCase()} ${idx + 1}`;
}

// iOS deep links — taken verbatim from the legacy subpage. Each
// client expects the sub URL in a slightly different param name.
const shadowrocketUrl = computed(() => {
  if (!subUrl) return '';
  const separator = subUrl.includes('?') ? '&' : '?';
  const rawUrl = subUrl + separator + 'flag=shadowrocket';
  const base64Url = encodeURIComponent(btoa(rawUrl));
  const remark = encodeURIComponent(subTitle || sId || 'Subscription');
  return `shadowrocket://add/sub/${base64Url}?remark=${remark}`;
});
const v2boxUrl = computed(() => `v2box://install-sub?url=${encodeURIComponent(subUrl)}&name=${encodeURIComponent(sId)}`);
const streisandUrl = computed(() => `streisand://import/${encodeURIComponent(subUrl)}`);
const v2raytunUrl = computed(() => subUrl);
const npvtunUrl = computed(() => subUrl);
const happUrl = computed(() => `happ://add/${subUrl}`);

// Theme classes for the page wrapper.
const themeClass = computed(() => ({
  'is-dark': themeState.isDark,
  'is-ultra': themeState.isUltra,
}));

</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="subscription-page" :class="themeClass">
      <a-layout-content class="content">
        <a-row type="flex" justify="center">
          <a-col :xs="24" :sm="22" :md="18" :lg="14" :xl="12">
            <a-card hoverable class="subscription-card">
              <template #title>
                <a-space>
                  <span>{{ t('subscription.title') }}</span>
                  <a-tag>{{ sId }}</a-tag>
                </a-space>
              </template>
              <template #extra>
                <a-space :size="8" align="center">
                  <button type="button" class="theme-cycle" :aria-label="t('menu.theme')" :title="t('menu.theme')"
                    @click="cycleTheme">
                    <svg v-if="!themeState.isDark" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                      stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                      <circle cx="12" cy="12" r="4" />
                      <path
                        d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
                    </svg>
                    <svg v-else-if="!themeState.isUltra" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                      stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
                    </svg>
                    <svg v-else viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.5"
                      stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
                      <path fill="none" d="M19 3l0.7 1.4 1.4 0.7-1.4 0.7L19 7.2l-0.7-1.4-1.4-0.7 1.4-0.7z" />
                    </svg>
                  </button>

                  <a-popover :title="t('pages.settings.language')" placement="bottomRight" trigger="click">
                    <template #content>
                      <a-space direction="vertical" :size="10" class="settings-popover">
                        <a-select v-model:value="lang" class="lang-select" @change="onLangChange">
                          <a-select-option v-for="l in LanguageManager.supportedLanguages" :key="l.value"
                            :value="l.value">
                            <span :aria-label="l.name">{{ l.icon }}</span>
                            &nbsp;&nbsp;<span>{{ l.name }}</span>
                          </a-select-option>
                        </a-select>
                      </a-space>
                    </template>
                    <a-button shape="circle">
                      <template #icon>
                        <SettingOutlined />
                      </template>
                    </a-button>
                  </a-popover>
                </a-space>
              </template>

              <!-- ============== QR codes ============== -->
              <a-row :gutter="[8, 8]" justify="center" class="qr-row">
                <a-col :xs="24" :sm="subJsonUrl || subClashUrl ? 12 : 24" class="qr-col">
                  <div class="qr-box">
                    <a-tag color="purple" class="qr-tag">{{ t('pages.settings.subSettings') }}</a-tag>
                    <a-qrcode class="qr-code" :value="subUrl" :size="QR_SIZE" type="svg" :bordered="false"
                      color="#000000" bg-color="#ffffff" :title="t('copy')" @click="copy(subUrl)" />
                  </div>
                </a-col>
                <a-col v-if="subJsonUrl" :xs="24" :sm="12" class="qr-col">
                  <div class="qr-box">
                    <a-tag color="purple" class="qr-tag">
                      {{ t('pages.settings.subSettings') }} JSON
                    </a-tag>
                    <a-qrcode class="qr-code" :value="subJsonUrl" :size="QR_SIZE" type="svg" :bordered="false"
                      color="#000000" bg-color="#ffffff" :title="t('copy')" @click="copy(subJsonUrl)" />
                  </div>
                </a-col>
                <a-col v-if="subClashUrl" :xs="24" :sm="12" class="qr-col">
                  <div class="qr-box">
                    <a-tag color="purple" class="qr-tag">Clash / Mihomo</a-tag>
                    <a-qrcode class="qr-code" :value="subClashUrl" :size="QR_SIZE" type="svg" :bordered="false"
                      color="#000000" bg-color="#ffffff" :title="t('copy')" @click="copy(subClashUrl)" />
                  </div>
                </a-col>
              </a-row>

              <!-- ============== Subscription details ============== -->
              <a-descriptions bordered :column="1" size="small" class="info-table">
                <a-descriptions-item :label="t('subscription.subId')">{{ sId }}</a-descriptions-item>
                <a-descriptions-item :label="t('subscription.status')">
                  <a-tag v-if="!enabled" color="red">{{ t('subscription.inactive') }}</a-tag>
                  <a-tag v-else-if="isUnlimited" color="purple">{{ t('subscription.unlimited') }}</a-tag>
                  <a-tag v-else :color="isActive ? 'green' : 'red'">
                    {{ isActive ? t('subscription.active') : t('subscription.inactive') }}
                  </a-tag>
                </a-descriptions-item>
                <a-descriptions-item :label="t('subscription.downloaded')">{{ download }}</a-descriptions-item>
                <a-descriptions-item :label="t('subscription.uploaded')">{{ upload }}</a-descriptions-item>
                <a-descriptions-item :label="t('usage')">{{ used }}</a-descriptions-item>
                <a-descriptions-item :label="t('subscription.totalQuota')">{{ total }}</a-descriptions-item>
                <a-descriptions-item v-if="totalByte > 0" :label="t('remained')">
                  {{ remained }}
                </a-descriptions-item>
                <a-descriptions-item :label="t('lastOnline')">
                  <template v-if="lastOnlineMs > 0">{{ IntlUtil.formatDate(lastOnlineMs, datepicker) }}</template>
                  <template v-else>-</template>
                </a-descriptions-item>
                <a-descriptions-item :label="t('subscription.expiry')">
                  <template v-if="expireMs === 0">{{ t('subscription.noExpiry') }}</template>
                  <template v-else>{{ IntlUtil.formatDate(expireMs, datepicker) }}</template>
                </a-descriptions-item>
              </a-descriptions>

              <!-- ============== Individual links ============== -->
              <div v-if="links.length" class="links-section">
                <div v-for="(link, idx) in links" :key="link" class="link-row" @click="copy(link)">
                  <a-tag color="purple" class="link-tag">{{ linkName(link, idx) }}</a-tag>
                  <div class="link-box">
                    <CopyOutlined class="link-copy-icon" />
                    {{ link }}
                  </div>
                </div>
              </div>

              <!-- ============== App dropdowns ============== -->
              <a-row :gutter="[8, 8]" justify="center" class="apps-row">
                <a-col :xs="24" :sm="12" class="app-col">
                  <a-dropdown :trigger="['click']">
                    <a-button :block="isMobile" size="large" type="primary">
                      <AndroidOutlined /> Android
                      <DownOutlined />
                    </a-button>
                    <template #overlay>
                      <a-menu>
                        <a-menu-item key="android-v2box"
                          @click="open(`v2box://install-sub?url=${encodeURIComponent(subUrl)}&name=${encodeURIComponent(sId)}`)">V2Box</a-menu-item>
                        <a-menu-item key="android-v2rayng"
                          @click="open(`v2rayng://install-config?url=${encodeURIComponent(subUrl)}`)">V2RayNG</a-menu-item>
                        <a-menu-item key="android-singbox" @click="copy(subUrl)">Sing-box</a-menu-item>
                        <a-menu-item key="android-v2raytun" @click="copy(subUrl)">V2RayTun</a-menu-item>
                        <a-menu-item key="android-npvtunnel" @click="copy(subUrl)">NPV Tunnel</a-menu-item>
                        <a-menu-item key="android-happ" @click="open(`happ://add/${subUrl}`)">Happ</a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown>
                </a-col>
                <a-col :xs="24" :sm="12" class="app-col">
                  <a-dropdown :trigger="['click']">
                    <a-button :block="isMobile" size="large" type="primary">
                      <AppleOutlined /> iOS
                      <DownOutlined />
                    </a-button>
                    <template #overlay>
                      <a-menu>
                        <a-menu-item key="ios-shadowrocket" @click="open(shadowrocketUrl)">Shadowrocket</a-menu-item>
                        <a-menu-item key="ios-v2box" @click="open(v2boxUrl)">V2Box</a-menu-item>
                        <a-menu-item key="ios-streisand" @click="open(streisandUrl)">Streisand</a-menu-item>
                        <a-menu-item key="ios-v2raytun" @click="copy(v2raytunUrl)">V2RayTun</a-menu-item>
                        <a-menu-item key="ios-npvtunnel" @click="copy(npvtunUrl)">NPV Tunnel</a-menu-item>
                        <a-menu-item key="ios-happ" @click="open(happUrl)">Happ</a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown>
                </a-col>
              </a-row>
            </a-card>
          </a-col>
        </a-row>
      </a-layout-content>
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.subscription-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;
  min-height: 100vh;
  background: var(--bg-page);
}

.subscription-page.is-dark {
  --bg-page: #1e1e1e;
  --bg-card: #252526;
}

.subscription-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.subscription-page :deep(.ant-layout),
.subscription-page :deep(.ant-layout-content) {
  background: transparent;
}

.content {
  padding: 24px 12px;
}

.subscription-card {
  margin-top: 8px;
}

/* QR section */
.qr-row {
  margin-bottom: 12px;
}

.qr-col {
  display: flex;
  justify-content: center;
}

.qr-box {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  width: 240px;
}

.qr-tag {
  width: 100%;
  text-align: center;
  margin: 0;
}

.qr-code {
  cursor: pointer;
  padding: 0 !important;
  background: #fff;
  border-radius: 4px;
}

/* Description list spacing + visible borders. AD-Vue's default
 * descriptions border is rgba(5,5,5,0.06) which disappears against
 * the white card in light theme. AD-Vue puts the horizontal divider
 * on <tr> with border-collapse:collapse — browsers treat <tr>
 * borders inconsistently in collapse mode, so paint the divider on
 * each cell's bottom edge instead. */
.info-table {
  margin-top: 12px;
}

.info-table :deep(.ant-descriptions-view),
.info-table :deep(.ant-descriptions-view) table,
.info-table :deep(.ant-descriptions-view) th,
.info-table :deep(.ant-descriptions-view) td {
  border-color: rgba(0, 0, 0, 0.18) !important;
}

.info-table :deep(tbody > tr > th),
.info-table :deep(tbody > tr > td) {
  border-bottom: 1px solid rgba(0, 0, 0, 0.18) !important;
}

.info-table :deep(tbody > tr:last-child > th),
.info-table :deep(tbody > tr:last-child > td) {
  border-bottom: none !important;
}

.is-dark .info-table :deep(.ant-descriptions-view),
.is-dark .info-table :deep(.ant-descriptions-view) table,
.is-dark .info-table :deep(.ant-descriptions-view) th,
.is-dark .info-table :deep(.ant-descriptions-view) td {
  border-color: rgba(255, 255, 255, 0.18) !important;
}

.is-dark .info-table :deep(tbody > tr > th),
.is-dark .info-table :deep(tbody > tr > td) {
  border-bottom: 1px solid rgba(255, 255, 255, 0.18) !important;
}

.is-dark .info-table :deep(tbody > tr:last-child > th),
.is-dark .info-table :deep(tbody > tr:last-child > td) {
  border-bottom: none !important;
}

/* Share links */
.links-section {
  margin-top: 16px;
}

.link-row {
  position: relative;
  margin-bottom: 16px;
  text-align: center;
}

.link-tag {
  margin-bottom: -10px;
  position: relative;
  z-index: 2;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.link-box {
  cursor: pointer;
  border-radius: 12px;
  padding: 22px 18px 14px;
  margin-top: -10px;
  word-break: break-all;
  font-size: 13px;
  line-height: 1.5;
  text-align: left;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  transition: background 120ms ease, border-color 120ms ease;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.08);
  background: rgba(0, 0, 0, 0.03);
  border: 1px solid rgba(0, 0, 0, 0.08);
}

.link-box:hover {
  background: rgba(0, 0, 0, 0.05);
  border-color: rgba(0, 0, 0, 0.14);
}

.link-copy-icon {
  margin-right: 6px;
  opacity: 0.6;
}

.is-dark .link-box {
  background: rgba(0, 0, 0, 0.2);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.85);
}

.is-dark .link-box:hover {
  background: rgba(0, 0, 0, 0.3);
  border-color: rgba(255, 255, 255, 0.2);
}

/* App dropdown row */
.apps-row {
  margin-top: 24px;
}

.app-col {
  text-align: center;
}

.settings-popover {
  min-width: 220px;
}

.theme-cycle {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: var(--bg-card);
  color: rgba(0, 0, 0, 0.65);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: background-color 0.2s, transform 0.15s, color 0.2s;
}

.theme-cycle:hover,
.theme-cycle:focus-visible {
  background-color: rgba(64, 150, 255, 0.1);
  color: #4096ff;
  transform: scale(1.05);
  outline: none;
}

.theme-cycle svg {
  width: 16px;
  height: 16px;
}

.is-dark .theme-cycle {
  border-color: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.85);
}

.is-dark .theme-cycle:hover,
.is-dark .theme-cycle:focus-visible {
  background-color: rgba(64, 150, 255, 0.1);
  color: #4096ff;
}

.lang-select {
  width: 100%;
}
</style>
