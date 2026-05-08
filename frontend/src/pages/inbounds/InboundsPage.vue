<script setup>
import { computed, onMounted } from 'vue';
import { theme as antdTheme } from 'ant-design-vue';
import {
  SwapOutlined,
  PieChartOutlined,
  HistoryOutlined,
  BarsOutlined,
  TeamOutlined,
  SyncOutlined,
} from '@ant-design/icons-vue';

import { SizeFormatter } from '@/utils';
import { theme as themeState } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import { useInbounds } from './useInbounds.js';

const antdThemeConfig = computed(() => ({
  algorithm: themeState.isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
}));

const {
  fetched,
  refreshing,
  dbInbounds,
  totals,
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

              <!-- Inbound list (basic columns; row actions/modals come in later subphases) -->
              <a-col :span="24">
                <a-card hoverable>
                  <template #title>
                    <span>Inbounds</span>
                  </template>
                  <template #extra>
                    <a-button :loading="refreshing" @click="refresh">
                      <template #icon><SyncOutlined /></template>
                    </a-button>
                  </template>

                  <a-table
                    :columns="[
                      { title: 'Enable', dataIndex: 'enable', key: 'enable', width: 80 },
                      { title: 'Remark', dataIndex: 'remark', key: 'remark' },
                      { title: 'Port', dataIndex: 'port', key: 'port', width: 100 },
                      { title: 'Protocol', dataIndex: 'protocol', key: 'protocol', width: 130 },
                      { title: 'Traffic', key: 'traffic', width: 200 },
                      { title: 'Expiry', key: 'expiry', width: 140 },
                    ]"
                    :data-source="dbInbounds"
                    :row-key="(r) => r.id"
                    :pagination="false"
                    size="small"
                  >
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'enable'">
                        <a-switch :checked="record.enable" disabled />
                      </template>
                      <template v-else-if="column.key === 'protocol'">
                        <a-tag color="purple">{{ record.protocol }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'traffic'">
                        <a-tag>
                          {{ SizeFormatter.sizeFormat(record.up + record.down) }}
                          <template v-if="record.total > 0">
                            / {{ SizeFormatter.sizeFormat(record.total) }}
                          </template>
                          <template v-else>/ ∞</template>
                        </a-tag>
                      </template>
                      <template v-else-if="column.key === 'expiry'">
                        <a-tag v-if="record.expiryTime > 0">
                          {{ new Date(record.expiryTime).toLocaleDateString() }}
                        </a-tag>
                        <a-tag v-else color="purple">∞</a-tag>
                      </template>
                    </template>
                  </a-table>
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
