<script setup>
import { computed, onMounted, ref } from 'vue';
import { theme as antdTheme, Modal, message } from 'ant-design-vue';
import {
  SwapOutlined,
  PieChartOutlined,
  HistoryOutlined,
  BarsOutlined,
  TeamOutlined,
} from '@ant-design/icons-vue';

import { HttpUtil, SizeFormatter, RandomUtil } from '@/utils';
import { Inbound } from '@/models/inbound.js';
import { theme as themeState } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import InboundList from './InboundList.vue';
import InboundFormModal from './InboundFormModal.vue';
import { useInbounds } from './useInbounds.js';

const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

const {
  fetched,
  refreshing,
  dbInbounds,
  clientCount,
  onlineClients,
  totals,
  expireDiff,
  trafficDiff,
  pageSize,
  subSettings,
  refresh,
  fetchDefaultSettings,
} = useInbounds();
const { isMobile } = useMediaQuery();

const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

onMounted(async () => {
  await fetchDefaultSettings();
  await refresh();
});

// === Add/Edit modal ===================================================
const formOpen = ref(false);
const formMode = ref('add');
const formDbInbound = ref(null);

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
    <a-layout
      class="inbounds-page"
      :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }"
    >
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="!fetched" :delay="200" tip="Loading…" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, isMobile ? 0 : 12]">
              <!-- Summary statistics card -->
              <a-col :span="24">
                <a-card size="small" hoverable class="summary-card">
                  <a-row :gutter="[16, 12]">
                    <a-col :sm="12" :md="5">
                      <CustomStatistic
                        title="Total ↑ / ↓"
                        :value="`${SizeFormatter.sizeFormat(totals.up)} / ${SizeFormatter.sizeFormat(totals.down)}`"
                      >
                        <template #prefix><SwapOutlined /></template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="12" :md="5">
                      <CustomStatistic
                        title="Total usage"
                        :value="SizeFormatter.sizeFormat(totals.up + totals.down)"
                      >
                        <template #prefix><PieChartOutlined /></template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="12" :md="5">
                      <CustomStatistic
                        title="All-time traffic"
                        :value="SizeFormatter.sizeFormat(totals.allTime)"
                      >
                        <template #prefix><HistoryOutlined /></template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="12" :md="5">
                      <CustomStatistic title="Inbounds" :value="String(dbInbounds.length)">
                        <template #prefix><BarsOutlined /></template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="24" :md="4">
                      <CustomStatistic title="Clients" value=" ">
                        <template #prefix>
                          <a-space direction="horizontal">
                            <TeamOutlined />
                            <a-tag color="green">{{ totals.clients }}</a-tag>
                            <a-tag v-if="totals.deactive.length">{{ totals.deactive.length }}</a-tag>
                            <a-tag v-if="totals.depleted.length" color="red">{{ totals.depleted.length }}</a-tag>
                            <a-tag v-if="totals.expiring.length" color="orange">{{ totals.expiring.length }}</a-tag>
                          </a-space>
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <!-- Inbound list — toolbar, search/filter, columns, row actions -->
              <a-col :span="24">
                <InboundList
                  :db-inbounds="dbInbounds"
                  :client-count="clientCount"
                  :online-clients="onlineClients"
                  :refreshing="refreshing"
                  :expire-diff="expireDiff"
                  :traffic-diff="trafficDiff"
                  :page-size="pageSize"
                  :is-mobile="isMobile"
                  :sub-enable="subSettings.enable"
                  @refresh="refresh"
                  @add-inbound="onAddInbound"
                  @general-action="onGeneralAction"
                  @row-action="onRowAction"
                />
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <InboundFormModal
        v-model:open="formOpen"
        :mode="formMode"
        :db-inbound="formDbInbound"
        @saved="refresh"
      />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.inbounds-page {
  --bg-page: #f0f2f5;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.inbounds-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.inbounds-page.is-dark.is-ultra {
  --bg-page: #21242a;
  --bg-card: #0c0e12;
}

.inbounds-page :deep(.ant-layout),
.inbounds-page :deep(.ant-layout-content) {
  background: transparent;
}

.content-shell { background: transparent; }
.content-area { padding: 24px; }

.loading-spacer { min-height: calc(100vh - 120px); }

.summary-card {
  padding: 16px;
}
</style>
