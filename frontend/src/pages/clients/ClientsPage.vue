<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import {
  PlusOutlined,
  UserOutlined,
  EditOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  QrcodeOutlined,
  RetweetOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import { SizeFormatter, IntlUtil } from '@/utils';
import { useClients } from './useClients.js';
import ClientFormModal from './ClientFormModal.vue';
import ClientInfoModal from './ClientInfoModal.vue';
import ClientQrModal from './ClientQrModal.vue';

const { t } = useI18n();

const {
  clients,
  inbounds,
  onlines,
  loading,
  fetched,
  create,
  update,
  remove,
  attach,
  detach,
  resetTraffic,
} = useClients();

const { isMobile } = useMediaQuery();
const basePath = window.X_UI_BASE_PATH || '';
const requestUri = window.location.pathname;

const formOpen = ref(false);
const formMode = ref('add');
const editingClient = ref(null);
const editingAttachedIds = ref([]);

const infoOpen = ref(false);
const infoClient = ref(null);

const qrOpen = ref(false);
const qrClient = ref(null);

const onlineSet = computed(() => new Set(onlines.value || []));
const inboundsById = computed(() => {
  const out = {};
  for (const ib of inbounds.value) out[ib.id] = ib;
  return out;
});

function isOnline(email) {
  return !!email && onlineSet.value.has(email);
}

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

function onResetTraffic(row) {
  if (!row?.email || !Array.isArray(row.inboundIds) || row.inboundIds.length === 0) {
    message.warning(t('pages.clients.resetNotPossible') || 'Attach this client to an inbound first.');
    return;
  }
  Modal.confirm({
    title: `${t('pages.inbounds.resetTraffic') || 'Reset traffic'} — ${row.email}`,
    content: t('pages.inbounds.resetTrafficContent')
      || 'Counters drop to zero. Quota and expiry stay as-is.',
    okText: t('reset') || 'Reset',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await resetTraffic(row);
      if (msg?.success) message.success(t('pages.clients.toasts.trafficReset') || 'Traffic reset');
    },
  });
}

function onShowInfo(row) {
  infoClient.value = row;
  infoOpen.value = true;
}

function onShowQr(row) {
  qrClient.value = row;
  qrOpen.value = true;
}

async function onSave(payload, meta) {
  if (!meta?.isEdit) {
    return create(payload);
  }
  const id = meta.id;
  const updateMsg = await update(id, payload);
  if (!updateMsg?.success) return updateMsg;
  if (Array.isArray(meta.attach) && meta.attach.length > 0) {
    const r = await attach(id, meta.attach);
    if (!r?.success) return r;
  }
  if (Array.isArray(meta.detach) && meta.detach.length > 0) {
    const r = await detach(id, meta.detach);
    if (!r?.success) return r;
  }
  return updateMsg;
}

function trafficLabel(row) {
  const t0 = row.traffic;
  if (!t0) return '-';
  const used = (t0.up || 0) + (t0.down || 0);
  const total = row.totalGB || 0;
  if (total <= 0) return `${SizeFormatter.sizeFormat(used)} / ∞`;
  return `${SizeFormatter.sizeFormat(used)} / ${SizeFormatter.sizeFormat(total)}`;
}

function remainingLabel(row) {
  const total = row.totalGB || 0;
  if (total <= 0) return '∞';
  const used = (row.traffic?.up || 0) + (row.traffic?.down || 0);
  const r = total - used;
  return r > 0 ? SizeFormatter.sizeFormat(r) : '0';
}

function remainingColor(row) {
  const total = row.totalGB || 0;
  if (total <= 0) return 'purple';
  const used = (row.traffic?.up || 0) + (row.traffic?.down || 0);
  const ratio = used / total;
  if (ratio >= 1) return 'red';
  if (ratio >= 0.85) return 'orange';
  return 'green';
}

function expiryLabel(row) {
  if (!row.expiryTime || row.expiryTime <= 0) return '∞';
  return IntlUtil.formatDate(row.expiryTime);
}

function expiryRelative(row) {
  if (!row.expiryTime || row.expiryTime <= 0) return '';
  return IntlUtil.formatRelativeTime(row.expiryTime);
}

function expiryColor(row) {
  if (!row.expiryTime || row.expiryTime <= 0) return 'purple';
  const now = Date.now();
  if (row.expiryTime <= now) return 'red';
  if (row.expiryTime - now < 86400 * 1000 * 3) return 'orange';
  return 'green';
}

const columns = computed(() => [
  { title: t('pages.inbounds.client.email') || 'Email', key: 'email' },
  { title: t('online') || 'Online', key: 'online', width: 90 },
  { title: t('pages.clients.attachedInbounds') || 'Attached inbounds', key: 'inboundIds' },
  { title: t('pages.inbounds.traffic') || 'Traffic', key: 'traffic' },
  { title: t('remained') || 'Remaining', key: 'remaining', width: 130 },
  { title: t('pages.inbounds.expireDate') || 'Expiry', key: 'expiryTime' },
  { title: t('enable') || 'Enable', key: 'enable', width: 90 },
  { title: t('actions') || 'Actions', key: 'actions', width: 220 },
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

                  <a-table :columns="columns" :data-source="clients" :loading="loading" row-key="id"
                    :pagination="{ pageSize: 20, showSizeChanger: true, pageSizeOptions: ['10','20','50','100'] }"
                    size="small">
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'email'">
                        <div class="email-cell">
                          <span class="email">{{ record.email }}</span>
                          <span v-if="record.subId" class="sub" :title="record.subId">{{ record.subId }}</span>
                        </div>
                      </template>
                      <template v-else-if="column.key === 'online'">
                        <a-tag v-if="record.enable && isOnline(record.email)" color="green">{{ t('online') || 'Online' }}</a-tag>
                        <a-tag v-else>{{ t('offline') || 'Offline' }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'inboundIds'">
                        <a-tag v-for="id in record.inboundIds" :key="id" color="blue" style="margin: 2px">
                          {{ inboundLabel(id) }}
                        </a-tag>
                        <span v-if="!record.inboundIds || record.inboundIds.length === 0"
                          style="color: rgba(0,0,0,0.45)">—</span>
                      </template>
                      <template v-else-if="column.key === 'traffic'">
                        {{ trafficLabel(record) }}
                      </template>
                      <template v-else-if="column.key === 'remaining'">
                        <a-tag :color="remainingColor(record)">{{ remainingLabel(record) }}</a-tag>
                      </template>
                      <template v-else-if="column.key === 'expiryTime'">
                        <a-tooltip :title="expiryLabel(record)">
                          <a-tag :color="expiryColor(record)">
                            {{ record.expiryTime > 0 ? expiryRelative(record) : '∞' }}
                          </a-tag>
                        </a-tooltip>
                      </template>
                      <template v-else-if="column.key === 'enable'">
                        <a-tag :color="record.enable ? 'green' : 'default'">
                          {{ record.enable ? t('enable') : t('disable') }}
                        </a-tag>
                      </template>
                      <template v-else-if="column.key === 'actions'">
                        <a-space :size="4">
                          <a-tooltip :title="t('qrCode') || 'QR Code'">
                            <a-button size="small" type="text" @click="onShowQr(record)">
                              <QrcodeOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('info') || 'Info'">
                            <a-button size="small" type="text" @click="onShowInfo(record)">
                              <InfoCircleOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.inbounds.resetTraffic') || 'Reset traffic'">
                            <a-button size="small" type="text" @click="onResetTraffic(record)">
                              <RetweetOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('edit')">
                            <a-button size="small" type="text" @click="onEdit(record)">
                              <EditOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('delete')">
                            <a-button size="small" type="text" danger @click="onDelete(record)">
                              <DeleteOutlined />
                            </a-button>
                          </a-tooltip>
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
      <ClientInfoModal v-model:open="infoOpen" :client="infoClient" :inbounds-by-id="inboundsById"
        :is-online="infoClient ? isOnline(infoClient.email) : false" />
      <ClientQrModal v-model:open="qrOpen" :client="qrClient" />
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

.email-cell {
  display: flex;
  flex-direction: column;
}

.email {
  font-weight: 500;
}

.sub {
  font-size: 11px;
  opacity: 0.55;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 220px;
}
</style>
