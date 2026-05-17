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
  ControlOutlined,
  DownOutlined,
  MoreOutlined,
  UsergroupAddOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import AppSidebar from '@/components/AppSidebar.vue';
import { SizeFormatter, IntlUtil } from '@/utils';
import { useClients } from './useClients.js';
import ClientFormModal from './ClientFormModal.vue';
import ClientInfoModal from './ClientInfoModal.vue';
import ClientQrModal from './ClientQrModal.vue';
import ClientBulkAddModal from './ClientBulkAddModal.vue';

const { t } = useI18n();

const {
  clients,
  inbounds,
  onlines,
  loading,
  fetched,
  subSettings,
  create,
  update,
  remove,
  attach,
  detach,
  resetTraffic,
  resetAllTraffics,
  setEnable,
} = useClients();

const togglingId = ref(null);

async function onToggleEnable(row, next) {
  togglingId.value = row.id;
  try {
    const msg = await setEnable(row, next);
    if (!msg?.success) {
      message.error(msg?.msg || t('somethingWentWrong'));
    }
  } finally {
    togglingId.value = null;
  }
}

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

const bulkAddOpen = ref(false);
const selectedRowKeys = ref([]);

const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys) => { selectedRowKeys.value = keys; },
}));

function toggleSelect(id, checked) {
  const cur = new Set(selectedRowKeys.value);
  if (checked) cur.add(id);
  else cur.delete(id);
  selectedRowKeys.value = Array.from(cur);
}

function isSelected(id) {
  return selectedRowKeys.value.includes(id);
}

function selectAll(checked) {
  selectedRowKeys.value = checked ? clients.value.map((c) => c.id) : [];
}

const allSelected = computed(
  () => clients.value.length > 0 && selectedRowKeys.value.length === clients.value.length,
);

const someSelected = computed(
  () => selectedRowKeys.value.length > 0 && selectedRowKeys.value.length < clients.value.length,
);

function onBulkAdd() {
  bulkAddOpen.value = true;
}

function onBulkDelete() {
  const ids = [...selectedRowKeys.value];
  if (ids.length === 0) return;
  Modal.confirm({
    title: t('pages.clients.bulkDeleteConfirmTitle', { count: ids.length })
      || `Delete ${ids.length} clients?`,
    content: t('pages.clients.bulkDeleteConfirmContent')
      || 'Each client is removed from every attached inbound and its traffic record is dropped. This cannot be undone.',
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      let ok = 0;
      let failed = 0;
      for (const id of ids) {
        const msg = await remove(id);
        if (msg?.success) ok++;
        else failed++;
      }
      selectedRowKeys.value = [];
      if (failed === 0) {
        message.success(t('pages.clients.toasts.bulkDeleted', { count: ok }) || `${ok} clients deleted`);
      } else {
        message.warning(`${ok} deleted, ${failed} failed`);
      }
    },
  });
}

async function onBulkAddSaved() {
  bulkAddOpen.value = false;
}

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

function onResetAllTraffics() {
  Modal.confirm({
    title: t('pages.clients.resetAllTrafficsTitle') || 'Reset all client traffic?',
    content: t('pages.clients.resetAllTrafficsContent')
      || 'Every client’s up/down counter drops to zero. Quotas and expiry are not affected.',
    okText: t('reset') || 'Reset',
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await resetAllTraffics();
      if (msg?.success) message.success(t('pages.clients.toasts.allTrafficsReset') || 'All client traffic reset');
    },
  });
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
  { title: t('pages.clients.actions') || 'Actions', key: 'actions', width: 200 },
  { title: t('pages.clients.enabled') || 'Enabled', key: 'enable', width: 80 },
  { title: t('pages.clients.online') || 'Online', key: 'online', width: 90 },
  { title: t('pages.clients.client') || 'Client', key: 'email' },
  { title: t('pages.clients.attachedInbounds') || 'Attached inbounds', key: 'inboundIds' },
  { title: t('pages.clients.traffic') || 'Traffic', key: 'traffic' },
  { title: t('pages.clients.remaining') || 'Remaining', key: 'remaining', width: 130 },
  { title: t('pages.clients.duration') || 'Duration', key: 'expiryTime' },
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
                <a-card size="small">
                  <template #title>
                    <div class="card-toolbar">
                      <a-button type="primary" size="small" @click="onAdd">
                        <template #icon>
                          <PlusOutlined />
                        </template>
                        <template v-if="!isMobile">{{ t('pages.clients.addClients') }}</template>
                      </a-button>
                      <a-button size="small" @click="onBulkAdd">
                        <template #icon>
                          <UsergroupAddOutlined />
                        </template>
                        <template v-if="!isMobile">{{ t('pages.clients.bulk') || 'Add Bulk' }}</template>
                      </a-button>
                      <a-button v-if="selectedRowKeys.length > 0" danger size="small" @click="onBulkDelete">
                        <template #icon>
                          <DeleteOutlined />
                        </template>
                        {{ t('pages.clients.deleteSelected', { count: selectedRowKeys.length })
                          || `Delete (${selectedRowKeys.length})` }}
                      </a-button>
                      <a-dropdown :trigger="['click']">
                        <a-button size="small">
                          <ControlOutlined />
                          <span v-if="!isMobile">{{ t('pages.clients.general') }}</span>
                          <DownOutlined />
                        </a-button>
                        <template #overlay>
                          <a-menu>
                            <a-menu-item key="resetAllTraffics" @click="onResetAllTraffics">
                              <RetweetOutlined />
                              <span style="margin-left: 6px">
                                {{ t('pages.clients.resetAllTraffics') }}
                              </span>
                            </a-menu-item>
                          </a-menu>
                        </template>
                      </a-dropdown>
                    </div>
                  </template>

                  <a-table v-if="!isMobile" :columns="columns" :data-source="clients" :loading="loading" row-key="id"
                    :row-selection="rowSelection"
                    :pagination="{ pageSize: 20, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'] }"
                    size="small">
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'email'">
                        <div class="email-cell">
                          <span class="email">{{ record.email }}</span>
                          <span v-if="record.subId" class="sub" :title="record.subId">{{ record.subId }}</span>
                        </div>
                      </template>
                      <template v-else-if="column.key === 'online'">
                        <a-tag v-if="record.enable && isOnline(record.email)" color="green">{{ t('pages.clients.online')
                          || 'Online'
                        }}</a-tag>
                        <a-tag v-else>{{ t('pages.clients.offline') || 'Offline' }}</a-tag>
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
                        <a-switch :checked="record.enable" size="small" :loading="togglingId === record.id"
                          @change="(next) => onToggleEnable(record, next)" />
                      </template>
                      <template v-else-if="column.key === 'actions'">
                        <a-space :size="4">
                          <a-tooltip :title="t('pages.clients.qrCode') || 'QR Code'">
                            <a-button size="small" type="text" @click="onShowQr(record)">
                              <QrcodeOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.clients.moreInformation') || 'More Information'">
                            <a-button size="small" type="text" @click="onShowInfo(record)">
                              <InfoCircleOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.inbounds.resetTraffic') || 'Reset traffic'">
                            <a-button size="small" type="text" @click="onResetTraffic(record)">
                              <RetweetOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.clients.edit') || 'Edit'">
                            <a-button size="small" type="text" @click="onEdit(record)">
                              <EditOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.clients.delete') || 'Delete'">
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

                  <a-spin v-else :spinning="loading">
                    <div class="client-cards">
                      <div v-if="clients.length > 0" class="card-bulk-bar">
                        <a-checkbox :checked="allSelected" :indeterminate="someSelected"
                          @change="(e) => selectAll(e.target.checked)">
                          {{ t('pages.clients.selectAll') || 'Select all' }}
                        </a-checkbox>
                        <span v-if="selectedRowKeys.length > 0" class="bulk-count">
                          {{ selectedRowKeys.length }}
                        </span>
                      </div>

                      <div v-if="clients.length === 0" class="card-empty">
                        <UserOutlined style="font-size: 28px; opacity: 0.5" />
                        <div>{{ t('pages.clients.empty') || 'No clients yet.' }}</div>
                      </div>

                      <div v-for="row in clients" :key="row.id" class="client-card"
                        :class="{ 'is-selected': isSelected(row.id) }">
                        <div class="card-head">
                          <a-checkbox :checked="isSelected(row.id)"
                            @change="(e) => toggleSelect(row.id, e.target.checked)" />
                          <a-badge :color="row.enable && isOnline(row.email) ? 'green' : (row.enable ? 'default' : 'red')" />
                          <span class="tag-name">{{ row.email }}</span>
                          <div class="card-actions" @click.stop>
                            <a-tooltip :title="t('pages.clients.moreInformation') || 'Info'">
                              <InfoCircleOutlined class="row-action-trigger" @click="onShowInfo(row)" />
                            </a-tooltip>
                            <a-switch :checked="row.enable" size="small" :loading="togglingId === row.id"
                              @change="(next) => onToggleEnable(row, next)" />
                            <a-dropdown :trigger="['click']" placement="bottomRight">
                              <MoreOutlined class="row-action-trigger" @click.prevent />
                              <template #overlay>
                                <a-menu>
                                  <a-menu-item key="qr" @click="onShowQr(row)">
                                    <QrcodeOutlined /> {{ t('pages.clients.qrCode') || 'QR Code' }}
                                  </a-menu-item>
                                  <a-menu-item key="reset" @click="onResetTraffic(row)">
                                    <RetweetOutlined /> {{ t('pages.inbounds.resetTraffic') || 'Reset traffic' }}
                                  </a-menu-item>
                                  <a-menu-item key="edit" @click="onEdit(row)">
                                    <EditOutlined /> {{ t('pages.clients.edit') || 'Edit' }}
                                  </a-menu-item>
                                  <a-menu-item key="delete" class="danger-item" @click="onDelete(row)">
                                    <DeleteOutlined /> {{ t('pages.clients.delete') || 'Delete' }}
                                  </a-menu-item>
                                </a-menu>
                              </template>
                            </a-dropdown>
                          </div>
                        </div>
                      </div>
                    </div>
                  </a-spin>
                </a-card>
              </a-col>
            </a-row>
          </a-spin>
        </a-layout-content>
      </a-layout>

      <ClientFormModal v-model:open="formOpen" :mode="formMode" :client="editingClient"
        :attached-ids="editingAttachedIds" :inbounds="inbounds" :save="onSave" />
      <ClientInfoModal v-model:open="infoOpen" :client="infoClient" :inbounds-by-id="inboundsById"
        :is-online="infoClient ? isOnline(infoClient.email) : false" :sub-settings="subSettings" />
      <ClientQrModal v-model:open="qrOpen" :client="qrClient" :sub-settings="subSettings" />
      <ClientBulkAddModal v-model:open="bulkAddOpen" :inbounds="inbounds" @saved="onBulkAddSaved" />
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

.card-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.card-title {
  font-weight: 600;
  margin-right: 4px;
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

.client-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 4px;
}

.card-bulk-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 4px 8px;
}

.bulk-count {
  font-size: 12px;
  background: rgba(22, 119, 255, 0.12);
  color: var(--ant-color-primary, #1677ff);
  padding: 1px 8px;
  border-radius: 10px;
}

.client-card {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 10px;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.02);
}

.client-card.is-selected {
  border-color: var(--ant-color-primary, #1677ff);
  background: rgba(22, 119, 255, 0.06);
}

:global(body.dark) .client-card {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
}

.card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  user-select: none;
}

.card-head .tag-name {
  font-weight: 600;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.row-action-trigger {
  font-size: 18px;
  cursor: pointer;
  opacity: 0.75;
  transition: opacity 120ms ease;
}

.row-action-trigger:hover {
  opacity: 1;
}

.card-empty {
  text-align: center;
  padding: 40px 16px;
  opacity: 0.55;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.danger-item {
  color: #ff4d4f;
}
</style>
