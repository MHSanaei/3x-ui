<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  SwapOutlined,
  PieChartOutlined,
  BarsOutlined,
} from '@ant-design/icons-vue';

import { HttpUtil, SizeFormatter, RandomUtil } from '@/utils';
import { Inbound } from '@/models/inbound.js';
import { coerceInboundJsonField } from '@/models/dbinbound.js';
import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import { useNodeList } from '@/composables/useNodeList.js';
import InboundList from './InboundList.vue';
import InboundFormModal from './InboundFormModal.vue';
import InboundInfoModal from './InboundInfoModal.vue';
import QrCodeModal from './QrCodeModal.vue';
import TextModal from '@/components/TextModal.vue';
import PromptModal from '@/components/PromptModal.vue';
import { useInbounds } from './useInbounds.js';
import { useWebSocket } from '@/composables/useWebSocket.js';

const { t } = useI18n();

const {
  fetched,
  dbInbounds,
  clientCount,
  onlineClients,
  totals,
  expireDiff,
  trafficDiff,
  pageSize,
  subSettings,
  tgBotEnable,
  ipLimitEnable,
  remarkModel,
  lastOnlineMap,
  statsVersion,
  refresh,
  fetchDefaultSettings,
  applyTrafficEvent,
  applyClientStatsEvent,
  applyInvalidate,
  applyInboundsEvent,
} = useInbounds();

// Live updates over WebSocket — replaces the old 5s polling loop.
// The backend pushes traffic + per-client deltas every ~10s; we merge
// them into the local refs in-place so counters and online badges
// update without re-fetching the whole list.
useWebSocket({
  traffic: applyTrafficEvent,
  client_stats: applyClientStatsEvent,
  invalidate: applyInvalidate,
  inbounds: applyInboundsEvent,
});
const { isMobile } = useMediaQuery();
const { byId: nodesById, hasActive: hasActiveNode } = useNodeList();
const hasNodeAttachedInbound = computed(() =>
  (dbInbounds.value || []).some((ib) => ib?.nodeId != null),
);
const showNodeInfo = computed(() => hasNodeAttachedInbound.value || hasActiveNode.value);

const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;

onMounted(async () => {
  await fetchDefaultSettings();
  await refresh();
});

// === Add/Edit modal ===================================================
const formOpen = ref(false);
const formMode = ref('add');
const formDbInbound = ref(null);

// === Info / QR-code modals ===========================================
const infoOpen = ref(false);
const infoDbInbound = ref(null);
const infoClientIndex = ref(0);

const qrOpen = ref(false);
const qrDbInbound = ref(null);
const qrClient = ref(null);

// hostOverrideFor returns the node's address for a node-managed inbound,
// or '' when the inbound runs locally. Wired into the QR / Info modals
// and into export-all-links functions so generated share links point at
// the node, not the central panel.
function hostOverrideFor(dbInbound) {
  if (!dbInbound || dbInbound.nodeId == null) return '';
  return nodesById.value.get(dbInbound.nodeId)?.address || '';
}

const infoNodeAddress = computed(() => hostOverrideFor(infoDbInbound.value));
const qrNodeAddress = computed(() => hostOverrideFor(qrDbInbound.value));

// === Shared text + prompt modal state =================================
const textOpen = ref(false);
const textTitle = ref('');
const textContent = ref('');
const textFileName = ref('');

const promptOpen = ref(false);
const promptTitle = ref('');
const promptOkText = ref('OK');
const promptType = ref('textarea');
const promptInitial = ref('');
const promptLoading = ref(false);
let promptHandler = null;

function openText({ title, content, fileName = '' }) {
  textTitle.value = title;
  textContent.value = content;
  textFileName.value = fileName;
  textOpen.value = true;
}

function openPrompt({ title, okText, type = 'textarea', value = '', confirm }) {
  promptTitle.value = title;
  promptOkText.value = okText || 'OK';
  promptType.value = type;
  promptInitial.value = value;
  promptHandler = confirm;
  promptOpen.value = true;
}

async function onPromptConfirm(value) {
  if (!promptHandler) { promptOpen.value = false; return; }
  promptLoading.value = true;
  try {
    const ok = await promptHandler(value);
    if (ok !== false) promptOpen.value = false;
  } finally {
    promptLoading.value = false;
  }
}

// === Export helpers — mirror legacy txtModal call sites ==============
function exportInboundLinks(dbInbound) {
  const projected = checkFallback(dbInbound);
  openText({
    title: 'Export inbound links',
    content: projected.genInboundLinks(remarkModel.value, hostOverrideFor(dbInbound)),
    fileName: projected.remark || 'inbound',
  });
}

function exportInboundClipboard(dbInbound) {
  openText({
    title: 'Inbound JSON',
    content: JSON.stringify(dbInbound, null, 2),
  });
}

function exportInboundSubs(dbInbound) {
  const inbound = dbInbound.toInbound();
  const clients = inbound?.clients || [];
  const subLinks = [];
  for (const c of clients) {
    if (c.subId && subSettings.value.subURI) {
      subLinks.push(subSettings.value.subURI + c.subId);
    }
  }
  openText({
    title: 'Export subscription links',
    content: [...new Set(subLinks)].join('\n'),
    fileName: `${dbInbound.remark || 'inbound'}-Subs`,
  });
}

function exportAllLinks() {
  const out = [];
  for (const ib of dbInbounds.value) {
    const projected = checkFallback(ib);
    out.push(projected.genInboundLinks(remarkModel.value, hostOverrideFor(ib)));
  }
  openText({
    title: 'Export all inbound links',
    content: out.join('\r\n'),
    fileName: 'All-Inbounds',
  });
}

function exportAllSubs() {
  const out = [];
  for (const ib of dbInbounds.value) {
    const inbound = ib.toInbound();
    const clients = inbound?.clients || [];
    for (const c of clients) {
      if (c.subId && subSettings.value.subURI) {
        out.push(subSettings.value.subURI + c.subId);
      }
    }
  }
  openText({
    title: 'Export all subscription links',
    content: [...new Set(out)].join('\r\n'),
    fileName: 'All-Inbounds-Subs',
  });
}

function importInbound() {
  openPrompt({
    title: 'Import inbound',
    okText: 'Import',
    type: 'textarea',
    value: '',
    confirm: async (value) => {
      const msg = await HttpUtil.post('/panel/api/inbounds/import', { data: value });
      if (msg?.success) {
        await refresh();
        return true;
      }
      return false;
    },
  });
}

// `checkFallback` mirrors the legacy helper: when an inbound listens
// on a unix-socket fallback (`@<name>`), point the link generator at
// the root inbound that owns the listen address so QRs/links carry
// the externally-reachable host:port and the right TLS state.
function checkFallback(dbInbound) {
  // Path 1: panel-tracked fallback relationship (inbound_fallbacks row).
  // The backend annotates each child inbound with fallbackParent so the
  // child's client-share link advertises the master's reachable endpoint
  // and inherits its TLS / Reality state.
  const parent = dbInbound.fallbackParent;
  if (parent?.masterId) {
    const master = dbInbounds.value.find((ib) => ib.id === parent.masterId);
    if (master) return projectChildThroughMaster(dbInbound, master);
  }
  // Path 2: legacy unix-socket convention (`@vless-ws` etc.) — walk the
  // VLESS/Trojan TCP inbounds and look for one whose settings.fallbacks
  // references this child's listen address.
  if (!dbInbound.listen?.startsWith?.('@')) return dbInbound;
  for (const candidate of dbInbounds.value) {
    if (candidate.id === dbInbound.id) continue;
    const parsed = candidate.toInbound();
    if (!parsed.isTcp) continue;
    if (!['trojan', 'vless'].includes(parsed.protocol)) continue;
    const fallbacks = parsed.settings.fallbacks || [];
    if (!fallbacks.find((f) => f.dest === dbInbound.listen)) continue;
    return projectChildThroughMaster(dbInbound, candidate);
  }
  return dbInbound;
}

// projectChildThroughMaster returns a one-off DBInbound copy whose
// listen/port + TLS/Reality state come from the master, while the
// protocol/transport/clients stay the child's. This is what makes a
// `vless://uuid@server:443?type=ws&path=/vlws&security=tls` link work
// for a child VLESS-WS bound to 127.0.0.1.
function projectChildThroughMaster(child, master) {
  const projected = JSON.parse(JSON.stringify(child));
  projected.listen = master.listen;
  projected.port = master.port;
  const masterStream = master.toInbound().stream;
  const childInbound = child.toInbound();
  childInbound.stream.security = masterStream.security;
  childInbound.stream.tls = masterStream.tls;
  childInbound.stream.reality = masterStream.reality;
  childInbound.stream.externalProxy = masterStream.externalProxy;
  projected.streamSettings = childInbound.stream.toString();
  return new child.constructor(projected);
}

function findClientIndex(dbInbound, client) {
  if (!client) return 0;
  const inbound = dbInbound.toInbound();
  const clients = inbound?.clients || [];
  const idx = clients.findIndex((c) => {
    if (!c) return false;
    switch (dbInbound.protocol) {
      case 'trojan':
      case 'shadowsocks':
        return c.password === client.password && c.email === client.email;
      default:
        return c.id === client.id && c.email === client.email;
    }
  });
  return idx >= 0 ? idx : 0;
}

function onAddInbound() {
  formMode.value = 'add';
  formDbInbound.value = null;
  formOpen.value = true;
}

function openEdit(dbInbound) {
  formMode.value = 'edit';
  formDbInbound.value = dbInbound;
  formOpen.value = true;
}

// Per-row destructive actions go through Modal.confirm (matches legacy).
function confirmDelete(dbInbound) {
  Modal.confirm({
    title: `Delete inbound "${dbInbound.remark}"?`,
    content: 'This removes the inbound and all its clients. This cannot be undone.',
    okText: 'Delete',
    okType: 'danger',
    cancelText: 'Cancel',
    onOk: async () => {
      const msg = await HttpUtil.post(`/panel/api/inbounds/del/${dbInbound.id}`);
      if (msg?.success) await refresh();
    },
  });
}

function confirmResetTraffic(dbInbound) {
  Modal.confirm({
    title: `Reset traffic for "${dbInbound.remark}"?`,
    content: 'Resets up/down counters to 0 for this inbound.',
    okText: 'Reset',
    cancelText: 'Cancel',
    onOk: async () => {
      const msg = await HttpUtil.post(`/panel/api/inbounds/${dbInbound.id}/resetTraffic`);
      if (msg?.success) await refresh();
    },
  });
}

// Clone — adds a new inbound with the same protocol+stream+sniffing
// but a fresh remark/port and an empty client list.
function confirmClone(dbInbound) {
  Modal.confirm({
    title: `Clone inbound "${dbInbound.remark}"?`,
    content: 'Creates a copy with a new port and an empty client list.',
    okText: 'Clone',
    cancelText: 'Cancel',
    onOk: async () => {
      const baseInbound = dbInbound.toInbound();
      let clonedSettings;
      try {
        const raw = coerceInboundJsonField(dbInbound.settings);
        raw.clients = [];
        clonedSettings = JSON.stringify(raw);
      } catch (_e) {
        clonedSettings = Inbound.Settings.getSettings(baseInbound.protocol).toString();
      }
      const data = {
        up: 0,
        down: 0,
        total: 0,
        remark: `${dbInbound.remark} (clone)`,
        enable: false,
        expiryTime: 0,
        listen: '',
        port: RandomUtil.randomInteger(10000, 60000),
        protocol: baseInbound.protocol,
        settings: clonedSettings,
        streamSettings: baseInbound.stream.toString(),
        sniffing: baseInbound.sniffing.toString(),
      };
      const msg = await HttpUtil.post('/panel/api/inbounds/add', data);
      if (msg?.success) await refresh();
    },
  });
}

function onGeneralAction(key) {
  switch (key) {
    case 'import':
      importInbound();
      break;
    case 'export':
      exportAllLinks();
      break;
    case 'subs':
      exportAllSubs();
      break;
    case 'resetInbounds':
      Modal.confirm({
        title: 'Reset all inbound traffic?',
        okText: 'Reset',
        cancelText: 'Cancel',
        onOk: async () => {
          const msg = await HttpUtil.post('/panel/api/inbounds/resetAllTraffics');
          if (msg?.success) await refresh();
        },
      });
      break;
    default:
      message.info(`General action "${key}" — coming in a later 5f subphase`);
  }
}

function onRowAction({ key, dbInbound }) {
  switch (key) {
    case 'edit':
      openEdit(dbInbound);
      break;
    case 'showInfo':
      infoDbInbound.value = checkFallback(dbInbound);
      infoClientIndex.value = findClientIndex(dbInbound, null);
      infoOpen.value = true;
      break;
    case 'qrcode':
      qrDbInbound.value = checkFallback(dbInbound);
      qrClient.value = null;
      qrOpen.value = true;
      break;
    case 'export':
      exportInboundLinks(dbInbound);
      break;
    case 'subs':
      exportInboundSubs(dbInbound);
      break;
    case 'clipboard':
      exportInboundClipboard(dbInbound);
      break;
    case 'delete':
      confirmDelete(dbInbound);
      break;
    case 'resetTraffic':
      confirmResetTraffic(dbInbound);
      break;
    case 'clone':
      confirmClone(dbInbound);
      break;
    default:
      message.info(`Action "${key}" — coming in a later 5f subphase`);
  }
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="inbounds-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="!fetched" :delay="200" tip="Loading…" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, 12]">
              <!-- Summary statistics card -->
              <a-col :span="24">
                <a-card size="small" hoverable class="summary-card">
                  <a-row :gutter="[16, 12]">
                    <a-col :xs="12" :sm="12" :md="8">
                      <CustomStatistic :title="t('pages.inbounds.totalDownUp')"
                        :value="`${SizeFormatter.sizeFormat(totals.up)} / ${SizeFormatter.sizeFormat(totals.down)}`">
                        <template #prefix>
                          <SwapOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="12" :sm="12" :md="8">
                      <CustomStatistic :title="t('pages.inbounds.totalUsage')"
                        :value="SizeFormatter.sizeFormat(totals.up + totals.down)">
                        <template #prefix>
                          <PieChartOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="24" :sm="24" :md="8">
                      <CustomStatistic :title="t('pages.inbounds.inboundCount')" :value="String(dbInbounds.length)">
                        <template #prefix>
                          <BarsOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <!-- Inbound list — toolbar, search/filter, columns, row actions -->
              <a-col :span="24">
                <InboundList :db-inbounds="dbInbounds" :client-count="clientCount" :online-clients="onlineClients"
                  :last-online-map="lastOnlineMap" :is-dark-theme="themeState.isDark" :expire-diff="expireDiff"
                  :traffic-diff="trafficDiff" :page-size="pageSize" :is-mobile="isMobile"
                  :sub-enable="subSettings.enable" :nodes-by-id="nodesById" :has-active-node="showNodeInfo"
                  :stats-version="statsVersion" @refresh="refresh" @add-inbound="onAddInbound"
                  @general-action="onGeneralAction" @row-action="onRowAction" />
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <InboundFormModal v-model:open="formOpen" :mode="formMode" :db-inbound="formDbInbound" :db-inbounds="dbInbounds"
        @saved="refresh" />
      <InboundInfoModal v-model:open="infoOpen" :db-inbound="infoDbInbound" :client-index="infoClientIndex"
        :remark-model="remarkModel" :expire-diff="expireDiff" :traffic-diff="trafficDiff"
        :ip-limit-enable="ipLimitEnable" :tg-bot-enable="tgBotEnable" :sub-settings="subSettings"
        :last-online-map="lastOnlineMap" :node-address="infoNodeAddress" />
      <QrCodeModal v-model:open="qrOpen" :db-inbound="qrDbInbound" :client="qrClient" :remark-model="remarkModel"
        :node-address="qrNodeAddress" :sub-settings="subSettings" />

      <TextModal v-model:open="textOpen" :title="textTitle" :content="textContent" :file-name="textFileName" />
      <PromptModal v-model:open="promptOpen" :title="promptTitle" :ok-text="promptOkText" :type="promptType"
        :initial-value="promptInitial" :loading="promptLoading" @confirm="onPromptConfirm" />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.inbounds-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.inbounds-page.is-dark {
  --bg-page: #1e1e1e;
  --bg-card: #252526;
}

.inbounds-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.inbounds-page :deep(.ant-layout),
.inbounds-page :deep(.ant-layout-content) {
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
    padding: 8px;
  }
}

.loading-spacer {
  min-height: calc(100vh - 120px);
}

.summary-card {
  padding: 16px;
}

@media (max-width: 768px) {
  .summary-card {
    padding: 8px;
  }
}
</style>
