<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  CloudServerOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import NodeList from './NodeList.vue';
import NodeFormModal from './NodeFormModal.vue';
import { useNodes } from './useNodes.js';
import { useWebSocket } from '@/composables/useWebSocket.js';

const { t } = useI18n();

const {
  nodes,
  loading,
  fetched,
  totals,
  applyNodesEvent,
  create,
  update,
  remove,
  setEnable,
  testConnection,
  probe,
} = useNodes();

// Live updates — NodeHeartbeatJob pushes the fresh list every 10s.
useWebSocket({ nodes: applyNodesEvent });

const { isMobile } = useMediaQuery();

const basePath = window.__X_UI_BASE_PATH__ || '';
const requestUri = window.location.pathname;

// === Form modal state =================================================
const formOpen = ref(false);
const formMode = ref('add');
const formNode = ref(null);

function onAdd() {
  formMode.value = 'add';
  formNode.value = null;
  formOpen.value = true;
}

function onEdit(node) {
  formMode.value = 'edit';
  formNode.value = { ...node };
  formOpen.value = true;
}

// Save callback the modal hands its payload to. We hide the create vs.
// update branching here so the modal stays mode-agnostic.
async function onSave(payload) {
  if (formMode.value === 'edit' && formNode.value?.id) {
    return update(formNode.value.id, payload);
  }
  return create(payload);
}

function onDelete(node) {
  Modal.confirm({
    title: t('pages.nodes.deleteConfirmTitle', { name: node.name }),
    content: t('pages.nodes.deleteConfirmContent'),
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await remove(node.id);
      if (msg?.success) message.success(t('pages.nodes.toasts.deleted'));
    },
  });
}

async function onProbe(node) {
  const msg = await probe(node.id);
  if (msg?.success && msg.obj) {
    if (msg.obj.status === 'online') {
      message.success(t('pages.nodes.connectionOk', { ms: msg.obj.latencyMs }));
    } else {
      message.error(msg.obj.error || t('pages.nodes.toasts.probeFailed'));
    }
  }
}

async function onToggleEnable(node, next) {
  await setEnable(node.id, next);
}
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="nodes-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
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
                    <a-col :sm="12" :md="6">
                      <CustomStatistic :title="t('pages.nodes.totalNodes')" :value="String(totals.total)">
                        <template #prefix>
                          <CloudServerOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="12" :md="6">
                      <CustomStatistic :title="t('pages.nodes.onlineNodes')" :value="String(totals.online)">
                        <template #prefix>
                          <CheckCircleOutlined style="color: #52c41a" />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="12" :md="6">
                      <CustomStatistic :title="t('pages.nodes.offlineNodes')" :value="String(totals.offline)">
                        <template #prefix>
                          <CloseCircleOutlined style="color: #ff4d4f" />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :sm="12" :md="6">
                      <CustomStatistic :title="t('pages.nodes.avgLatency')"
                        :value="totals.avgLatency > 0 ? `${totals.avgLatency} ms` : '-'">
                        <template #prefix>
                          <ThunderboltOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

              <!-- Node table -->
              <a-col :span="24">
                <NodeList :nodes="nodes" :loading="loading" :is-mobile="isMobile" @add="onAdd" @edit="onEdit"
                  @delete="onDelete" @probe="onProbe" @toggle-enable="onToggleEnable" />
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <NodeFormModal v-model:open="formOpen" :mode="formMode" :node="formNode" :test-connection="testConnection"
        :save="onSave" />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.nodes-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;

  min-height: 100vh;
  background: var(--bg-page);
}

.nodes-page.is-dark {
  --bg-page: #0a1222;
  --bg-card: #151f31;
}

.nodes-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.nodes-page :deep(.ant-layout),
.nodes-page :deep(.ant-layout-content) {
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
