<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  SwapOutlined,
  PieChartOutlined,
  HistoryOutlined,
  BarsOutlined,
  TeamOutlined,
} from '@ant-design/icons-vue';

import { HttpUtil, SizeFormatter, RandomUtil } from '@/utils';
import { Inbound } from '@/models/inbound.js';
import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import { useNodeList } from '@/composables/useNodeList.js';
import InboundList from './InboundList.vue';
import InboundFormModal from './InboundFormModal.vue';
import ClientFormModal from './ClientFormModal.vue';
import ClientBulkModal from './ClientBulkModal.vue';
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
// Node list lives on the central panel; the Inbounds page consumes
// the id→node map for the new "Node" column. Fetched once on mount.
const { byId: nodesById } = useNodeList();

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

// === Client modal (single + bulk) =====================================
const clientOpen = ref(false);
const clientMode = ref('add');
const clientDbInbound = ref(null);
const clientIndex = ref(null);

const bulkOpen = ref(false);
const bulkDbInbound = ref(null);

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
    out.push(ib.genInboundLinks(remarkModel.value, hostOverrideFor(ib)));
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
  // We don't keep parsed Inbounds in state right now (the page works
  // off DBInbounds); compute on the fly.
  if (!dbInbound.listen?.startsWith?.('@')) return dbInbound;
  for (const candidate of dbInbounds.value) {
    if (candidate.id === dbInbound.id) continue;
    const parsed = candidate.toInbound();
    if (!parsed.isTcp) continue;
    if (!['trojan', 'vless'].includes(parsed.protocol)) continue;
    const fallbacks = parsed.settings.fallbacks || [];
    if (!fallbacks.find((f) => f.dest === dbInbound.listen)) continue;
    // Build a one-off DBInbound copy with the parent's listen/port +
    // copied stream so the link gen sees the public endpoint.
    const projected = JSON.parse(JSON.stringify(dbInbound));
    projected.listen = candidate.listen;
    projected.port = candidate.port;
    const inheritedStream = parsed.stream;
    const ownInbound = dbInbound.toInbound();
    ownInbound.stream.security = inheritedStream.security;
    ownInbound.stream.tls = inheritedStream.tls;
    ownInbound.stream.externalProxy = inheritedStream.externalProxy;
    projected.streamSettings = ownInbound.stream.toString();
    // Re-wrap so callers get the same DBInbound shape they had.
    return new dbInbound.constructor(projected);
  }
  return dbInbound;
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

function getClientId(protocol, client) {
  switch (protocol) {
    case 'trojan': return client.password;
    case 'shadowsocks': return client.email;
    case 'hysteria': return client.auth;
    default: return client.id;
  }
}

// === Per-client handlers (called from the expand-row table) =========
function onEditClient({ dbInbound, client }) {
  clientMode.value = 'edit';
  clientDbInbound.value = dbInbound;
  clientIndex.value = findClientIndex(dbInbound, client);
  clientOpen.value = true;
}

function onQrcodeClient({ dbInbound, client }) {
  qrDbInbound.value = checkFallback(dbInbound);
  qrClient.value = client || null;
  qrOpen.value = true;
}

function onInfoClient({ dbInbound, client }) {
  infoDbInbound.value = checkFallback(dbInbound);
  infoClientIndex.value = findClientIndex(dbInbound, client);
  infoOpen.value = true;
}

async function onResetTrafficClient({ dbInbound, client }) {
  const msg = await HttpUtil.post(
    `/panel/api/inbounds/${dbInbound.id}/resetClientTraffic/${client.email}`,
  );
  if (msg?.success) await refresh();
}

async function onDeleteClient({ dbInbound, client }) {
  const clientId = getClientId(dbInbound.protocol, client);
  const msg = await HttpUtil.post(`/panel/api/inbounds/${dbInbound.id}/delClient/${clientId}`);
  if (msg?.success) await refresh();
}

async function onDeleteClients({ dbInbound, clients }) {
  for (const client of clients) {
    const clientId = getClientId(dbInbound.protocol, client);
    await HttpUtil.post(`/panel/api/inbounds/${dbInbound.id}/delClient/${clientId}`);
  }
  await refresh();
}

async function onToggleEnableClient({ dbInbound, client, next }) {
  // Mirror legacy: clone the parsed inbound, flip enable on the matching
  // client, and post the whole client back through updateClient. This
  // keeps the wire shape identical to the modal save path.
  const inbound = dbInbound.toInbound();
  const clients = inbound?.clients || [];
  const idx = findClientIndex(dbInbound, client);
  if (idx < 0 || !clients[idx]) return;
  clients[idx].enable = next;
  const clientId = getClientId(dbInbound.protocol, clients[idx]);
  const msg = await HttpUtil.post(`/panel/api/inbounds/updateClient/${clientId}`, {
    id: dbInbound.id,
    settings: `{"clients": [${clients[idx].toString()}]}`,
  });
  if (msg?.success) await refresh();
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

function openAddClient(dbInbound) {
  clientMode.value = 'add';
  clientDbInbound.value = dbInbound;
  clientIndex.value = null;
  clientOpen.value = true;
}

function openAddBulkClient(dbInbound) {
  bulkDbInbound.value = dbInbound;
  bulkOpen.value = true;
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
      const msg = await HttpUtil.post(`/panel/api/inbounds/resetAllTraffics`);
      if (msg?.success) await refresh();
    },
  });
}

function confirmDelDepleted(dbInboundId) {
  Modal.confirm({
    title: 'Delete depleted clients?',
    content: 'Removes every client whose traffic is exhausted or whose expiry has passed.',
    okText: 'Delete',
    okType: 'danger',
    cancelText: 'Cancel',
    onOk: async () => {
      const msg = await HttpUtil.post(`/panel/api/inbounds/delDepletedClients/${dbInboundId}`);
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
        settings: Inbound.Settings.getSettings(baseInbound.protocol).toString(),
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
    case 'resetClients':
      Modal.confirm({
        title: 'Reset all client traffic across all inbounds?',
        okText: 'Reset',
        cancelText: 'Cancel',
        onOk: async () => {
          const msg = await HttpUtil.post('/panel/api/inbounds/resetAllClientTraffics/-1');
          if (msg?.success) await refresh();
        },
      });
      break;
    case 'delDepletedClients':
      confirmDelDepleted(-1);
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
    case 'addClient':
      openAddClient(dbInbound);
      break;
    case 'addBulkClient':
      openAddBulkClient(dbInbound);
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
    case 'copyClients':
      // Copy-clients-from-inbound is a tiny dedicated modal in legacy
      // (lets you tick clients to copy across inbounds). Defer to a
      // future commit — surface a friendly message for now.
      message.info('Copy clients across inbounds — coming soon');
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
    case 'resetClients':
      Modal.confirm({
        title: `Reset client traffic on "${dbInbound.remark}"?`,
        okText: 'Reset',
        cancelText: 'Cancel',
        onOk: async () => {
          const msg = await HttpUtil.post(`/panel/api/inbounds/resetAllClientTraffics/${dbInbound.id}`);
          if (msg?.success) await refresh();
        },
      });
      break;
    case 'delDepletedClients':
      confirmDelDepleted(dbInbound.id);
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
                    <a-col :xs="12" :sm="12" :md="5">
                      <CustomStatistic :title="t('pages.inbounds.totalDownUp')"
                        :value="`${SizeFormatter.sizeFormat(totals.up)} / ${SizeFormatter.sizeFormat(totals.down)}`">
                        <template #prefix>
                          <SwapOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="12" :sm="12" :md="5">
                      <CustomStatistic :title="t('pages.inbounds.totalUsage')"
                        :value="SizeFormatter.sizeFormat(totals.up + totals.down)">
                        <template #prefix>
                          <PieChartOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="12" :sm="12" :md="5">
                      <CustomStatistic :title="t('pages.inbounds.allTimeTrafficUsage')"
                        :value="SizeFormatter.sizeFormat(totals.allTime)">
                        <template #prefix>
                          <HistoryOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="12" :sm="12" :md="5">
                      <CustomStatistic :title="t('pages.inbounds.inboundCount')" :value="String(dbInbounds.length)">
                        <template #prefix>
                          <BarsOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="24" :sm="24" :md="4">
                      <CustomStatistic :title="t('clients')" value=" ">
                        <template #prefix>
                          <a-space direction="horizontal">
                            <TeamOutlined />
                            <a-tag color="green">{{ totals.clients }}</a-tag>
                            <a-popover v-if="totals.deactive.length" :title="t('disabled')">
                              <template #content>
                                <div class="client-email-list">
                                  <div v-for="email in totals.deactive" :key="email">{{ email }}</div>
                                </div>
                              </template>
                              <a-tag>{{ totals.deactive.length }}</a-tag>
                            </a-popover>
                            <a-popover v-if="totals.depleted.length" :title="t('depleted')">
                              <template #content>
                                <div class="client-email-list">
                                  <div v-for="email in totals.depleted" :key="email">{{ email }}</div>
                                </div>
                              </template>
                              <a-tag color="red">{{ totals.depleted.length }}</a-tag>
                            </a-popover>
                            <a-popover v-if="totals.expiring.length" :title="t('depletingSoon')">
                              <template #content>
                                <div class="client-email-list">
                                  <div v-for="email in totals.expiring" :key="email">{{ email }}</div>
                                </div>
                              </template>
                              <a-tag color="orange">{{ totals.expiring.length }}</a-tag>
                            </a-popover>
                            <a-popover v-if="totals.online.length" :title="t('online')">
                              <template #content>
                                <div class="client-email-list">
                                  <div v-for="email in totals.online" :key="email">{{ email }}</div>
                                </div>
                              </template>
                              <a-tag color="blue">{{ totals.online.length }}</a-tag>
                            </a-popover>
                          </a-space>
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
                  :sub-enable="subSettings.enable" :nodes-by-id="nodesById" @refresh="refresh"
                  @add-inbound="onAddInbound" @general-action="onGeneralAction" @row-action="onRowAction"
                  @edit-client="onEditClient" @qrcode-client="onQrcodeClient" @info-client="onInfoClient"
                  @reset-traffic-client="onResetTrafficClient" @delete-client="onDeleteClient"
                  @delete-clients="onDeleteClients" @toggle-enable-client="onToggleEnableClient" />
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <InboundFormModal v-model:open="formOpen" :mode="formMode" :db-inbound="formDbInbound" @saved="refresh" />
      <ClientFormModal v-model:open="clientOpen" :mode="clientMode" :db-inbound="clientDbInbound"
        :client-index="clientIndex" :sub-enable="subSettings.enable" :tg-bot-enable="tgBotEnable"
        :ip-limit-enable="ipLimitEnable" :traffic-diff="trafficDiff" @saved="refresh" />
      <ClientBulkModal v-model:open="bulkOpen" :db-inbound="bulkDbInbound" :sub-enable="subSettings.enable"
        :tg-bot-enable="tgBotEnable" :ip-limit-enable="ipLimitEnable" @saved="refresh" />
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

<style>
/* AD-Vue popovers teleport their content to <body>, so scoped styles
   don't reach them — this block has to be unscoped. */
.client-email-list {
  max-height: 280px;
  min-width: 160px;
  overflow-y: auto;
  padding-right: 4px;
}

.client-email-list > div {
  padding: 2px 0;
  font-size: 12px;
  white-space: nowrap;
}
</style>
