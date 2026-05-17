<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import { PlusOutlined, UserOutlined } from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import { SizeFormatter, IntlUtil } from '@/utils';
import { useClients } from './useClients.js';
import ClientFormModal from './ClientFormModal.vue';

const { t } = useI18n();

const {
  clients,
  inbounds,
  loading,
  fetched,
  create,
  update,
  remove,
} = useClients();

const { isMobile } = useMediaQuery();
const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;

const formOpen = ref(false);
const formMode = ref('add');
const editingClient = ref(null);
const editingAttachedIds = ref([]);

const inboundsById = computed(() => {
  const out = {};
  for (const ib of inbounds.value) out[ib.id] = ib;
  return out;
});

function inboundLabel(id) {
  const ib = inboundsById.value[id];
  if (!ib) return `#${id}`;
  return ib.remark ? `${ib.remark} (${ib.protocol}:${ib.port})` : `${ib.protocol}:${ib.port}`;
}

function onAdd() {
  formMode.value = 'add';
  editingClient.value = null;
  editingAttachedIds.value = [];
  formOpen.value = true;
}

function onEdit(row) {
  formMode.value = 'edit';
  editingClient.value = { ...row };
  editingAttachedIds.value = Array.isArray(row.inboundIds) ? [...row.inboundIds] : [];
  formOpen.value = true;
}

function onDelete(row) {
  Modal.confirm({
    title: t('pages.clients.deleteConfirmTitle', { email: row.email }) || `Delete ${row.email}?`,
    content: t('pages.clients.deleteConfirmContent')
      || 'This removes the client from every attached inbound and drops its traffic record.',
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await remove(row.id);
      if (msg?.success) message.success(t('pages.clients.toasts.deleted') || 'Client deleted');
    },
  });
}

async function onSave(payload, meta) {
  if (meta?.isEdit) {
    return update(meta.id, payload);
  }
  return create(payload);
}

function trafficLabel(row) {
  const t0 = row.traffic;
  if (!t0) return '-';
  const used = (t0.up || 0) + (t0.down || 0);
  const total = row.totalGB || 0;
  if (total <= 0) return `${SizeFormatter.sizeFormat(used)} / ∞`;
  return `${SizeFormatter.sizeFormat(used)} / ${SizeFormatter.sizeFormat(total)}`;
}

function expiryLabel(row) {
  if (!row.expiryTime || row.expiryTime <= 0) return '-';
  return IntlUtil.formatDate(row.expiryTime);
}

const columns = computed(() => [
  { title: t('pages.inbounds.client.email') || 'Email', dataIndex: 'email', key: 'email' },
  { title: 'subId', dataIndex: 'subId', key: 'subId' },
  { title: t('pages.clients.attachedInbounds') || 'Attached inbounds', key: 'inboundIds' },
  { title: t('pages.inbounds.traffic') || 'Traffic', key: 'traffic' },
  { title: t('pages.inbounds.client.expiryTime') || 'Expiry', key: 'expiryTime' },
  { title: t('enable'), key: 'enable', width: 90 },
  { title: t('actions') || 'Actions', key: 'actions', width: 160 },
]);
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="clients-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="!fetched" :delay="200" tip="Loading…" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, isMobile ? 8 : 12]">
              <a-col :span="24">
                <a-card size="small" :title="t('menu.clients') || 'Clients'">
                  <template #extra>
                    <a-button type="primary" @click="onAdd">
                      <template #icon>
                        <PlusOutlined />
                      </template>
                      {{ t('add') }}
                    </a-button>
                  </template>

                  <a-table :columns="columns" :data-source="clients" :loading="loading" row-key="id" :pagination="{ pageSize: 20, showSizeChanger: true, pageSizeOptions: ['10','20','50','100'] }" size="small">
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'inboundIds'">
                        <a-tag v-for="id in record.inboundIds" :key="id" color="blue" style="margin: 2px">
                          {{ inboundLabel(id) }}
                        </a-tag>
                        <span v-if="!record.inboundIds || record.inboundIds.length === 0" style="color: rgba(0,0,0,0.45)">—</span>
                      </template>
                      <template v-else-if="column.key === 'traffic'">
                        {{ trafficLabel(record) }}
                      </template>
                      <template v-else-if="column.key === 'expiryTime'">
                        {{ expiryLabel(record) }}
                      </template>
                      <template v-else-if="column.key === 'enable'">
                        <a-tag :color="record.enable ? 'green' : 'default'">
                          {{ record.enable ? t('enable') : t('disable') }}
                        </a-tag>
                      </template>
                      <template v-else-if="column.key === 'actions'">
                        <a-space>
                          <a-button size="small" @click="onEdit(record)">{{ t('edit') }}</a-button>
                          <a-button size="small" danger @click="onDelete(record)">{{ t('delete') }}</a-button>
                        </a-space>
                      </template>
                    </template>

                    <template #emptyText>
                      <div style="padding: 32px 0; color: rgba(0, 0, 0, 0.45); text-align: center">
                        <UserOutlined style="font-size: 32px; margin-bottom: 8px" />
                        <div>{{ t('pages.clients.empty') || 'No clients yet.' }}</div>
                      </div>
                    </template>
                  </a-table>
                </a-card>
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <ClientFormModal v-model:open="formOpen" :mode="formMode" :client="editingClient"
        :attached-ids="editingAttachedIds" :inbounds="inbounds" :save="onSave" />
    </a-layout>
  </a-config-provider>
</template>

<style scoped>
.clients-page {
  --bg-page: #e6e8ec;
  --bg-card: #ffffff;
  min-height: 100vh;
  background: var(--bg-page);
}

.clients-page.is-dark {
  --bg-page: #1e1e1e;
  --bg-card: #252526;
}

.clients-page.is-dark.is-ultra {
  --bg-page: #050505;
  --bg-card: #0c0e12;
}

.clients-page :deep(.ant-layout),
.clients-page :deep(.ant-layout-content) {
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
</style>
