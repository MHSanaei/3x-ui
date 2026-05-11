<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal } from 'ant-design-vue';
import {
  SettingOutlined,
  SafetyOutlined,
  MessageOutlined,
  CloudServerOutlined,
  CodeOutlined,
} from '@ant-design/icons-vue';

import { HttpUtil, PromiseUtil } from '@/utils';
import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import { useAllSetting } from './useAllSetting.js';
import GeneralTab from './GeneralTab.vue';
import SecurityTab from './SecurityTab.vue';
import TelegramTab from './TelegramTab.vue';
import SubscriptionGeneralTab from './SubscriptionGeneralTab.vue';
import SubscriptionFormatsTab from './SubscriptionFormatsTab.vue';

const { t } = useI18n();

const { fetched, spinning, saveDisabled, allSetting, saveAll } = useAllSetting();
const { isMobile } = useMediaQuery();

const mustChangeCredentials = window.X_UI_MUST_CHANGE_CREDENTIALS === true
const activeTab = ref(mustChangeCredentials ? '2' : '1')

const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;

// AD-Vue 4's <a-back-top> calls `target()` after mount to find the
// scrolled element. Inline-arrow `() => document.getElementById(...)`
// in the template threw "Cannot read properties of undefined (reading
// 'getElementById')" because of how Vue 3 evaluates the expression
// outside the script-setup scope — wrap in a regular function so
// `document` resolves to the window global at call time.
function scrollTarget() {
  return document.getElementById('content-layout');
}

// `entry*` mirrors the URL the user opened the panel with so the page
// can rebuild it after a restart that may change host/port/scheme.
const entryHost = ref('');
const entryPort = ref('');
const entryIsIP = ref(false);

function isIp(h) {
  if (typeof h !== 'string') return false;
  // IPv4: four dot-separated octets 0-255.
  const v4 = h.split('.');
  if (v4.length === 4 && v4.every((p) => /^\d{1,3}$/.test(p) && Number(p) <= 255)) return true;
  // IPv6: hex groups, optional single :: compression.
  if (!h.includes(':') || h.includes(':::')) return false;
  const parts = h.split('::');
  if (parts.length > 2) return false;
  const split = (s) => (s ? s.split(':').filter(Boolean) : []);
  const head = split(parts[0]);
  const tail = split(parts[1]);
  const valid = (seg) => /^[0-9a-fA-F]{1,4}$/.test(seg);
  if (![...head, ...tail].every(valid)) return false;
  const groups = head.length + tail.length;
  return parts.length === 2 ? groups < 8 : groups === 8;
}

onMounted(() => {
  entryHost.value = window.location.hostname;
  entryPort.value = window.location.port;
  entryIsIP.value = isIp(entryHost.value);
});

// Rebuild the URL after a restart — host/port/scheme may have changed
// (cert toggled on, port edited, base path edited).
function rebuildUrlAfterRestart() {
  const { webDomain, webPort, webBasePath, webCertFile, webKeyFile } = allSetting;
  const newProtocol = (webCertFile || webKeyFile) ? 'https:' : 'http:';

  let base = webBasePath ? webBasePath.replace(/^\//, '') : '';
  if (base && !base.endsWith('/')) base += '/';

  if (!entryIsIP.value) {
    const url = new URL(window.location.href);
    url.pathname = `/${base}panel/settings`;
    url.protocol = newProtocol;
    return url.toString();
  }

  let finalHost = entryHost.value;
  let finalPort = entryPort.value || '';
  if (webDomain && isIp(webDomain)) finalHost = webDomain;
  if (webPort && Number(webPort) !== Number(entryPort.value)) finalPort = String(webPort);

  const url = new URL(`${newProtocol}//${finalHost}`);
  if (finalPort) url.port = finalPort;
  url.pathname = `/${base}panel/settings`;
  return url.toString();
}

function restartPanel() {
  Modal.confirm({
    title: t('pages.settings.restartPanel'),
    content: t('pages.settings.restartPanelDesc'),
    okText: t('pages.settings.restartPanel'),
    okButtonProps: { danger: true },
    cancelText: t('cancel'),
    async onOk() {
      spinning.value = true;
      try {
        const msg = await HttpUtil.post('/panel/setting/restartPanel');
        if (!msg?.success) return;
        await PromiseUtil.sleep(5000);
        window.location.replace(rebuildUrlAfterRestart());
      } finally {
        spinning.value = false;
      }
    },
  });
}

const securityChecklist = computed(() => {
  const segs = window.location.pathname.split('/').length < 4;
  const out = []
  if (mustChangeCredentials) {
    out.push({
      label: 'Default credentials',
      ok: false,
      action: 'Change the default admin/admin credentials in Authentication settings.',
    })
  }
  out.push(
    {
      label: 'TLS',
      ok: window.location.protocol === 'https:',
      action: 'Set certificate and key paths, then restart.',
    },
    {
      label: 'Base path',
      ok: !(segs && allSetting.webBasePath === '/'),
      action: 'Change the panel URL path from "/".',
    },
    {
      label: 'Panel port',
      ok: allSetting.webPort !== 2053,
      action: 'Use a non-default listening port.',
    },
    {
      label: 'Two-factor authentication',
      ok: allSetting.twoFactorEnable && allSetting.hasTwoFactorToken,
      action: 'Enable 2FA in Security.',
    },
    {
      label: 'API token',
      ok: allSetting.hasApiToken,
      action: 'Generate or rotate the API token in Security.',
    },
  )
  if (allSetting.subEnable) {
    let subPath = allSetting.subPath;
    if (allSetting.subURI) {
      try { subPath = new URL(allSetting.subURI).pathname; } catch (_e) { }
    }
    out.push({
      label: 'Subscription path',
      ok: subPath !== '/sub/',
      action: 'Change the default subscription path.',
    });
  }
  if (allSetting.subJsonEnable) {
    let p = allSetting.subJsonPath;
    if (allSetting.subJsonURI) {
      try { p = new URL(allSetting.subJsonURI).pathname; } catch (_e) { }
    }
    out.push({
      label: 'JSON subscription path',
      ok: p !== '/json/',
      action: 'Change the default JSON subscription path.',
    });
  }
  return out;
});
const hasSecurityGaps = computed(() => securityChecklist.value.some((item) => !item.ok));

const alertVisible = ref(true);
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="settings-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="spinning || !fetched" :delay="200" tip="Loading…" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <template v-else>
              <a-alert
                v-if="mustChangeCredentials"
                type="error"
                show-icon
                banner
                message="Change your default admin credentials to unlock the panel"
                description="All other panel sections are blocked until you set a unique username and password in the Authentication tab."
                class="conf-alert"
              />

              <a-alert v-if="hasSecurityGaps && alertVisible" type="error" show-icon closable class="conf-alert"
                @close="alertVisible = false">
                <template #message>Security posture checklist</template>
                <template #description>
                  <a-list size="small" :data-source="securityChecklist">
                    <template #renderItem="{ item }">
                      <a-list-item class="checklist-item">
                        <a-space :size="8" wrap>
                          <a-tag :color="item.ok ? 'green' : 'red'">{{ item.ok ? 'OK' : 'Action' }}</a-tag>
                          <strong>{{ item.label }}</strong>
                          <span>{{ item.ok ? 'Configured' : item.action }}</span>
                        </a-space>
                      </a-list-item>
                    </template>
                  </a-list>
                </template>
              </a-alert>

              <a-row :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
                <a-col :span="24">
                  <a-card hoverable>
                    <a-row class="header-row">
                      <a-col :xs="24" :sm="10" class="header-actions">
                        <a-space direction="horizontal">
                          <a-button type="primary" :disabled="saveDisabled" @click="saveAll">
                            {{ t('pages.settings.save') }}
                          </a-button>
                          <a-button type="primary" danger :disabled="!saveDisabled" @click="restartPanel">
                            {{ t('pages.settings.restartPanel') }}
                          </a-button>
                        </a-space>
                      </a-col>
                      <a-col :xs="24" :sm="14" class="header-info">
                        <a-back-top :target="scrollTarget" :visibility-height="200" />
                        <a-alert type="warning" show-icon :message="t('pages.settings.infoDesc')" />
                      </a-col>
                    </a-row>
                  </a-card>
                </a-col>

                <a-col :span="24">
                  <a-tabs v-model:activeKey="activeTab">
                    <a-tab-pane key="1" class="tab-pane">
                      <template #tab>
                        <SettingOutlined />
                        <span>{{ t('pages.settings.panelSettings') }}</span>
                      </template>
                      <GeneralTab :all-setting="allSetting" />
                    </a-tab-pane>
                    <a-tab-pane key="2" class="tab-pane">
                      <template #tab>
                        <SafetyOutlined />
                        <span>{{ t('pages.settings.securitySettings') }}</span>
                      </template>
                      <SecurityTab :all-setting="allSetting" />
                    </a-tab-pane>
                    <a-tab-pane key="3" class="tab-pane">
                      <template #tab>
                        <MessageOutlined />
                        <span>{{ t('pages.settings.TGBotSettings') }}</span>
                      </template>
                      <TelegramTab :all-setting="allSetting" />
                    </a-tab-pane>
                    <a-tab-pane key="4" class="tab-pane">
                      <template #tab>
                        <CloudServerOutlined />
                        <span>{{ t('pages.settings.subSettings') }}</span>
                      </template>
                      <SubscriptionGeneralTab :all-setting="allSetting" />
                    </a-tab-pane>
                    <a-tab-pane v-if="allSetting.subJsonEnable || allSetting.subClashEnable" key="5" class="tab-pane">
                      <template #tab>
                        <CodeOutlined />
                        <span>{{ t('pages.settings.subSettings') }} (Formats)</span>
                      </template>
                      <SubscriptionFormatsTab :all-setting="allSetting" />
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
.settings-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.settings-page.is-dark {
  --bg-page: #1e1e1e;
  --bg-card: #252526;
}

.settings-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.settings-page :deep(.ant-layout),
.settings-page :deep(.ant-layout-content) {
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

.conf-alert {
  margin-bottom: 10px;
}

.checklist-item {
  padding-left: 0 !important;
  padding-right: 0 !important;
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
</style>
