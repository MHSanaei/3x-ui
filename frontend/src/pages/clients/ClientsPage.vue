<script setup>
import { computed, ref, watch } from 'vue';
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
  RestOutlined,
  MoreOutlined,
  UsergroupAddOutlined,
  SearchOutlined,
  FilterOutlined,
  TeamOutlined,
} from '@ant-design/icons-vue';

import { theme as themeState, antdThemeConfig } from '@/composables/useTheme.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';
import { useWebSocket } from '@/composables/useWebSocket.js';
import AppSidebar from '@/components/AppSidebar.vue';
import CustomStatistic from '@/components/CustomStatistic.vue';
import { ObjectUtil, SizeFormatter, IntlUtil } from '@/utils';
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
  ipLimitEnable,
  tgBotEnable,
  expireDiff,
  trafficDiff,
  create,
  update,
  remove,
  removeMany,
  attach,
  detach,
  resetTraffic,
  resetAllTraffics,
  delDepleted,
  setEnable,
  applyTrafficEvent,
  applyClientStatsEvent,
  applyInvalidate,
} = useClients();

useWebSocket({
  traffic: applyTrafficEvent,
  client_stats: applyClientStatsEvent,
  invalidate: applyInvalidate,
});

const togglingEmail = ref(null);

async function onToggleEnable(row, next) {
  togglingEmail.value = row.email;
  try {
    const msg = await setEnable(row, next);
    if (!msg?.success) {
      message.error(msg?.msg || t('somethingWentWrong'));
    }
  } finally {
    togglingEmail.value = null;
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

function toggleSelect(email, checked) {
  const cur = new Set(selectedRowKeys.value);
  if (checked) cur.add(email);
  else cur.delete(email);
  selectedRowKeys.value = Array.from(cur);
}

function isSelected(email) {
  return selectedRowKeys.value.includes(email);
}

function selectAll(checked) {
  selectedRowKeys.value = checked ? filteredClients.value.map((c) => c.email) : [];
}

const allSelected = computed(
  () => filteredClients.value.length > 0 && selectedRowKeys.value.length === filteredClients.value.length,
);

const someSelected = computed(
  () => selectedRowKeys.value.length > 0 && selectedRowKeys.value.length < filteredClients.value.length,
);

function onBulkAdd() {
  bulkAddOpen.value = true;
}

function onBulkDelete() {
  const emails = [...selectedRowKeys.value];
  if (emails.length === 0) return;
  Modal.confirm({
    title: t('pages.clients.bulkDeleteConfirmTitle', { count: emails.length }),
    content: t('pages.clients.bulkDeleteConfirmContent'),
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const results = await removeMany(emails);
      selectedRowKeys.value = [];
      let ok = 0;
      let failed = 0;
      let firstError = '';
      for (const msg of results) {
        if (msg?.success) ok++;
        else {
          failed++;
          if (!firstError && msg?.msg) firstError = msg.msg;
        }
      }
      if (failed === 0) {
        message.success(t('pages.clients.toasts.bulkDeleted', { count: ok }));
      } else {
        message.warning(firstError
          ? `${t('pages.clients.toasts.bulkDeletedMixed', { ok, failed })} — ${firstError}`
          : t('pages.clients.toasts.bulkDeletedMixed', { ok, failed }));
      }
    },
  });
}

async function onBulkAddSaved() {
  bulkAddOpen.value = false;
}

function onDelDepleted() {
  Modal.confirm({
    title: t('pages.clients.delDepletedConfirmTitle'),
    content: t('pages.clients.delDepletedConfirmContent'),
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await delDepleted();
      if (msg?.success) {
        const deleted = msg.obj?.deleted ?? 0;
        message.success(t('pages.clients.toasts.delDepleted', { count: deleted }));
      }
    },
  });
}

const FILTER_STATE_KEY = 'clientsFilterState';
const savedFilterState = (() => {
  try { return JSON.parse(localStorage.getItem(FILTER_STATE_KEY) || '{}'); }
  catch (_e) { return {}; }
})();
const enableFilter = ref(!!savedFilterState.enableFilter);
const searchKey = ref(savedFilterState.searchKey || '');
const filterBy = ref(savedFilterState.filterBy || '');
const protocolFilter = ref(savedFilterState.protocolFilter || undefined);

watch([enableFilter, searchKey, filterBy, protocolFilter], () => {
  localStorage.setItem(FILTER_STATE_KEY, JSON.stringify({
    enableFilter: enableFilter.value,
    searchKey: searchKey.value,
    filterBy: filterBy.value,
    protocolFilter: protocolFilter.value,
  }));
});

function onToggleFilter() {
  if (enableFilter.value) searchKey.value = '';
  else filterBy.value = '';
}

const protocolOptions = computed(() => {
  const values = new Set((inbounds.value || []).map((i) => i.protocol).filter(Boolean));
  return [...values].sort();
});

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

function clientBucket(row) {
  if (!row) return null;
  const traffic = row.traffic || {};
  const used = (traffic.up || 0) + (traffic.down || 0);
  const total = row.totalGB || 0;
  const now = Date.now();
  const expired = row.expiryTime > 0 && row.expiryTime <= now;
  const exhausted = total > 0 && used >= total;
  if (expired || exhausted) return 'depleted';
  if (!row.enable) return 'deactive';
  const nearExpiry = row.expiryTime > 0 && row.expiryTime - now < (expireDiff.value || 0);
  const nearLimit = total > 0 && total - used < (trafficDiff.value || 0);
  if (nearExpiry || nearLimit) return 'expiring';
  return 'active';
}

function bucketTagColor(bucket) {
  switch (bucket) {
    case 'depleted': return 'red';
    case 'expiring': return 'orange';
    case 'deactive': return 'default';
    case 'active': return 'green';
    default: return 'default';
  }
}

function clientMatchesProtocol(row, protocol) {
  if (!protocol) return true;
  const ids = Array.isArray(row.inboundIds) ? row.inboundIds : [];
  for (const id of ids) {
    const ib = inboundsById.value[id];
    if (ib && ib.protocol === protocol) return true;
  }
  return false;
}

const filteredClients = computed(() => {
  let rows = clients.value || [];
  if (enableFilter.value) {
    if (filterBy.value === 'online') {
      rows = rows.filter((r) => r.enable && isOnline(r.email));
    } else if (filterBy.value) {
      rows = rows.filter((r) => clientBucket(r) === filterBy.value);
    }
  } else if (!ObjectUtil.isEmpty(searchKey.value)) {
    rows = rows.filter((r) => ObjectUtil.deepSearch(r, searchKey.value));
  }
  if (protocolFilter.value) {
    rows = rows.filter((r) => clientMatchesProtocol(r, protocolFilter.value));
  }
  return rows;
});

const summary = computed(() => {
  const rows = clients.value || [];
  const deactive = [];
  const depleted = [];
  const expiring = [];
  const online = [];
  let active = 0;
  for (const row of rows) {
    const bucket = clientBucket(row);
    if (bucket === 'deactive') deactive.push(row.email);
    else if (bucket === 'depleted') depleted.push(row.email);
    else if (bucket === 'expiring') expiring.push(row.email);
    else if (bucket === 'active') active++;
    if (row.enable && isOnline(row.email)) online.push(row.email);
  }
  return { total: rows.length, active, deactive, depleted, expiring, online };
});

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
    title: t('pages.clients.deleteConfirmTitle', { email: row.email }),
    content: t('pages.clients.deleteConfirmContent'),
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await remove(row.email);
      if (msg?.success) message.success(t('pages.clients.toasts.deleted'));
    },
  });
}

function onResetTraffic(row) {
  if (!row?.email || !Array.isArray(row.inboundIds) || row.inboundIds.length === 0) {
    message.warning(t('pages.clients.resetNotPossible'));
    return;
  }
  Modal.confirm({
    title: `${t('pages.inbounds.resetTraffic')} — ${row.email}`,
    content: t('pages.inbounds.resetTrafficContent'),
    okText: t('reset'),
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await resetTraffic(row);
      if (msg?.success) message.success(t('pages.clients.toasts.trafficReset'));
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
    title: t('pages.clients.resetAllTrafficsTitle'),
    content: t('pages.clients.resetAllTrafficsContent'),
    okText: t('reset'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: async () => {
      const msg = await resetAllTraffics();
      if (msg?.success) message.success(t('pages.clients.toasts.allTrafficsReset'));
    },
  });
}

async function onSave(payload, meta) {
  if (!meta?.isEdit) {
    return create(payload);
  }
  const updateMsg = await update(meta.email, payload);
  if (!updateMsg?.success) return updateMsg;
  if (Array.isArray(meta.attach) && meta.attach.length > 0) {
    const r = await attach(meta.email, meta.attach);
    if (!r?.success) return r;
  }
  if (Array.isArray(meta.detach) && meta.detach.length > 0) {
    const r = await detach(meta.email, meta.detach);
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
  if (!row.expiryTime) return '∞';
  if (row.expiryTime < 0) {
    const days = Math.round(row.expiryTime / -86400000);
    return `${t('pages.clients.delayedStart')}: ${days}d`;
  }
  return IntlUtil.formatDate(row.expiryTime);
}

function expiryRelative(row) {
  if (!row.expiryTime) return '';
  if (row.expiryTime < 0) {
    const days = Math.round(row.expiryTime / -86400000);
    return `${days}d`;
  }
  return IntlUtil.formatRelativeTime(row.expiryTime);
}

function expiryColor(row) {
  if (!row.expiryTime) return 'purple';
  if (row.expiryTime < 0) return 'blue';
  const now = Date.now();
  if (row.expiryTime <= now) return 'red';
  if (row.expiryTime - now < 86400 * 1000 * 3) return 'orange';
  return 'green';
}

const sortState = ref({ column: null, order: null });
const paginationState = ref({ current: 1, pageSize: 20 });

function sortableCol(col, key) {
  return {
    ...col,
    sorter: true,
    showSorterTooltip: false,
    sortOrder: sortState.value.column === key ? sortState.value.order : null,
    sortDirections: ['ascend', 'descend'],
  };
}

const sortFns = {
  enable: (a, b) => Number(a.enable) - Number(b.enable),
  email: (a, b) => (a.email || '').localeCompare(b.email || ''),
  inboundIds: (a, b) => (a.inboundIds?.length || 0) - (b.inboundIds?.length || 0),
  traffic: (a, b) => {
    const ua = (a.traffic?.up || 0) + (a.traffic?.down || 0);
    const ub = (b.traffic?.up || 0) + (b.traffic?.down || 0);
    return ua - ub;
  },
  remaining: (a, b) => {
    const ra = a.totalGB > 0 ? a.totalGB - ((a.traffic?.up || 0) + (a.traffic?.down || 0)) : Infinity;
    const rb = b.totalGB > 0 ? b.totalGB - ((b.traffic?.up || 0) + (b.traffic?.down || 0)) : Infinity;
    return ra - rb;
  },
  expiryTime: (a, b) => {
    const ea = a.expiryTime > 0 ? a.expiryTime : Infinity;
    const eb = b.expiryTime > 0 ? b.expiryTime : Infinity;
    return ea - eb;
  },
};

const sortedClients = computed(() => {
  const { column, order } = sortState.value;
  const rows = filteredClients.value;
  if (!column || !order) return rows;
  const fn = sortFns[column];
  if (!fn) return rows;
  const sorted = [...rows].sort(fn);
  return order === 'descend' ? sorted.reverse() : sorted;
});

function onTableChange(pag, _filters, sorter) {
  if (pag) {
    paginationState.value = {
      current: pag.current || 1,
      pageSize: pag.pageSize || paginationState.value.pageSize,
    };
  }
  sortState.value = {
    column: sorter?.columnKey || sorter?.field || null,
    order: sorter?.order || null,
  };
}

const tablePagination = computed(() => ({
  current: paginationState.value.current,
  pageSize: paginationState.value.pageSize,
  total: sortedClients.value.length,
  showSizeChanger: sortedClients.value.length > 10,
  pageSizeOptions: ['10', '20', '50', '100'],
  hideOnSinglePage: sortedClients.value.length <= paginationState.value.pageSize,
}));

const columns = computed(() => [
  { title: t('pages.clients.actions'), key: 'actions', width: 200 },
  sortableCol({ title: t('pages.clients.enabled'), key: 'enable', width: 80 }, 'enable'),
  { title: t('pages.clients.online'), key: 'online', width: 90 },
  sortableCol({ title: t('pages.clients.client'), key: 'email' }, 'email'),
  sortableCol({ title: t('pages.clients.attachedInbounds'), key: 'inboundIds' }, 'inboundIds'),
  sortableCol({ title: t('pages.clients.traffic'), key: 'traffic' }, 'traffic'),
  sortableCol({ title: t('pages.clients.remaining'), key: 'remaining', width: 130 }, 'remaining'),
  sortableCol({ title: t('pages.clients.duration'), key: 'expiryTime' }, 'expiryTime'),
]);
</script>

<template>
  <a-config-provider :theme="antdThemeConfig">
    <a-layout class="clients-page" :class="{ 'is-dark': themeState.isDark, 'is-ultra': themeState.isUltra }">
      <AppSidebar :base-path="basePath" :request-uri="requestUri" />

      <a-layout class="content-shell">
        <a-layout-content id="content-layout" class="content-area">
          <a-spin :spinning="!fetched" :delay="200" :tip="t('loading')" size="large">
            <div v-if="!fetched" class="loading-spacer" />

            <a-row v-else :gutter="[isMobile ? 8 : 16, isMobile ? 8 : 12]">
              <a-col :span="24">
                <a-card size="small" hoverable class="summary-card">
                  <a-row :gutter="[16, 12]">
                    <a-col :xs="12" :sm="8" :md="4">
                      <CustomStatistic :title="t('clients')" :value="String(summary.total)">
                        <template #prefix>
                          <TeamOutlined />
                        </template>
                      </CustomStatistic>
                    </a-col>
                    <a-col :xs="12" :sm="8" :md="4">
                      <a-popover :title="t('online')" :open="summary.online.length ? undefined : false">
                        <template #content>
                          <div class="client-email-list">
                            <div v-for="email in summary.online" :key="email">{{ email }}</div>
                          </div>
                        </template>
                        <CustomStatistic :title="t('online')" :value="String(summary.online.length)">
                          <template #prefix>
                            <span class="dot dot-blue" />
                          </template>
                        </CustomStatistic>
                      </a-popover>
                    </a-col>
                    <a-col :xs="12" :sm="8" :md="4">
                      <a-popover :title="t('depleted')" :open="summary.depleted.length ? undefined : false">
                        <template #content>
                          <div class="client-email-list">
                            <div v-for="email in summary.depleted" :key="email">{{ email }}</div>
                          </div>
                        </template>
                        <CustomStatistic :title="t('depleted')" :value="String(summary.depleted.length)">
                          <template #prefix>
                            <span class="dot dot-red" />
                          </template>
                        </CustomStatistic>
                      </a-popover>
                    </a-col>
                    <a-col :xs="12" :sm="8" :md="4">
                      <a-popover :title="t('depletingSoon')" :open="summary.expiring.length ? undefined : false">
                        <template #content>
                          <div class="client-email-list">
                            <div v-for="email in summary.expiring" :key="email">{{ email }}</div>
                          </div>
                        </template>
                        <CustomStatistic :title="t('depletingSoon')" :value="String(summary.expiring.length)">
                          <template #prefix>
                            <span class="dot dot-orange" />
                          </template>
                        </CustomStatistic>
                      </a-popover>
                    </a-col>
                    <a-col :xs="12" :sm="8" :md="4">
                      <a-popover :title="t('disabled')" :open="summary.deactive.length ? undefined : false">
                        <template #content>
                          <div class="client-email-list">
                            <div v-for="email in summary.deactive" :key="email">{{ email }}</div>
                          </div>
                        </template>
                        <CustomStatistic :title="t('disabled')" :value="String(summary.deactive.length)">
                          <template #prefix>
                            <span class="dot dot-gray" />
                          </template>
                        </CustomStatistic>
                      </a-popover>
                    </a-col>
                    <a-col :xs="12" :sm="8" :md="4">
                      <CustomStatistic :title="t('subscription.active')" :value="String(summary.active)">
                        <template #prefix>
                          <span class="dot dot-green" />
                        </template>
                      </CustomStatistic>
                    </a-col>
                  </a-row>
                </a-card>
              </a-col>

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
                        <template v-if="!isMobile">{{ t('pages.clients.bulk') }}</template>
                      </a-button>
                      <a-button v-if="selectedRowKeys.length > 0" danger size="small" @click="onBulkDelete">
                        <template #icon>
                          <DeleteOutlined />
                        </template>
                        {{ t('pages.clients.deleteSelected', { count: selectedRowKeys.length }) }}
                      </a-button>
                      <a-button size="small" @click="onResetAllTraffics">
                        <template #icon>
                          <RetweetOutlined />
                        </template>
                        <template v-if="!isMobile">{{ t('pages.clients.resetAllTraffics') }}</template>
                      </a-button>
                      <a-button size="small" danger @click="onDelDepleted">
                        <template #icon>
                          <RestOutlined />
                        </template>
                        <template v-if="!isMobile">{{ t('pages.clients.delDepleted') }}</template>
                      </a-button>
                    </div>
                  </template>

                  <div :class="isMobile ? 'filter-bar mobile' : 'filter-bar'">
                    <a-switch v-model:checked="enableFilter" @change="onToggleFilter">
                      <template #checkedChildren>
                        <SearchOutlined />
                      </template>
                      <template #unCheckedChildren>
                        <FilterOutlined />
                      </template>
                    </a-switch>
                    <a-input v-if="!enableFilter" v-model:value="searchKey" :placeholder="t('search')" autofocus
                      :size="isMobile ? 'small' : 'middle'" :style="{ maxWidth: '300px' }" />
                    <a-radio-group v-if="enableFilter" v-model:value="filterBy" button-style="solid"
                      :size="isMobile ? 'small' : 'middle'">
                      <a-radio-button value="">{{ t('none') }}</a-radio-button>
                      <a-radio-button value="active">{{ t('subscription.active') }}</a-radio-button>
                      <a-radio-button value="deactive">{{ t('disabled') }}</a-radio-button>
                      <a-radio-button value="depleted">{{ t('depleted') }}</a-radio-button>
                      <a-radio-button value="expiring">{{ t('depletingSoon') }}</a-radio-button>
                      <a-radio-button value="online">{{ t('online') }}</a-radio-button>
                    </a-radio-group>
                    <a-select v-model:value="protocolFilter" allow-clear :placeholder="t('pages.inbounds.protocol')"
                      :size="isMobile ? 'small' : 'middle'" :style="{ width: '150px' }">
                      <a-select-option v-for="protocol in protocolOptions" :key="protocol" :value="protocol">
                        {{ protocol }}
                      </a-select-option>
                    </a-select>
                  </div>

                  <a-table v-if="!isMobile" :columns="columns" :data-source="sortedClients" :loading="loading" row-key="email"
                    :row-selection="rowSelection" :pagination="tablePagination" size="small" @change="onTableChange">
                    <template #bodyCell="{ column, record }">
                      <template v-if="column.key === 'email'">
                        <div class="email-cell">
                          <span class="email">{{ record.email }}</span>
                          <span v-if="record.subId" class="sub" :title="record.subId">{{ record.subId }}</span>
                        </div>
                      </template>
                      <template v-else-if="column.key === 'online'">
                        <a-tag v-if="clientBucket(record) === 'depleted'" color="red">
                          {{ t('depleted') }}
                        </a-tag>
                        <a-tag v-else-if="record.enable && isOnline(record.email)" color="green">
                          {{ t('pages.clients.online') }}
                        </a-tag>
                        <a-tag v-else-if="!record.enable">{{ t('disabled') }}</a-tag>
                        <a-tag v-else-if="clientBucket(record) === 'expiring'" color="orange">
                          {{ t('depletingSoon') }}
                        </a-tag>
                        <a-tag v-else>{{ t('pages.clients.offline') }}</a-tag>
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
                            {{ record.expiryTime ? expiryRelative(record) : '∞' }}
                          </a-tag>
                        </a-tooltip>
                      </template>
                      <template v-else-if="column.key === 'enable'">
                        <a-switch :checked="record.enable" size="small" :loading="togglingEmail === record.email"
                          @change="(next) => onToggleEnable(record, next)" />
                      </template>
                      <template v-else-if="column.key === 'actions'">
                        <a-space :size="4">
                          <a-tooltip :title="t('pages.clients.qrCode')">
                            <a-button size="small" type="text" @click="onShowQr(record)">
                              <QrcodeOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.clients.moreInformation')">
                            <a-button size="small" type="text" @click="onShowInfo(record)">
                              <InfoCircleOutlined />
                            </a-button>
                          </a-tooltip>
                          <a-tooltip :title="t('pages.inbounds.resetTraffic')">
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
                      <div class="clients-empty">
                        <UserOutlined style="font-size: 32px; margin-bottom: 8px" />
                        <div>{{ t('pages.clients.empty') }}</div>
                      </div>
                    </template>
                  </a-table>

                  <a-spin v-else :spinning="loading">
                    <div class="client-cards">
                      <div v-if="filteredClients.length > 0" class="card-bulk-bar">
                        <a-checkbox :checked="allSelected" :indeterminate="someSelected"
                          @change="(e) => selectAll(e.target.checked)">
                          {{ t('pages.clients.selectAll') }}
                        </a-checkbox>
                        <span v-if="selectedRowKeys.length > 0" class="bulk-count">
                          {{ selectedRowKeys.length }}
                        </span>
                      </div>

                      <div v-if="filteredClients.length === 0" class="card-empty">
                        <UserOutlined style="font-size: 28px; opacity: 0.5" />
                        <div>{{ t('pages.clients.empty') }}</div>
                      </div>

                      <div v-for="row in filteredClients" :key="row.email" class="client-card"
                        :class="{ 'is-selected': isSelected(row.email) }">
                        <div class="card-head">
                          <a-checkbox :checked="isSelected(row.email)"
                            @change="(e) => toggleSelect(row.email, e.target.checked)" />
                          <a-badge :color="bucketTagColor(clientBucket(row))" />
                          <span class="tag-name">{{ row.email }}</span>
                          <a-tag v-if="clientBucket(row) === 'depleted'" color="red" class="status-tag">
                            {{ t('depleted') }}
                          </a-tag>
                          <a-tag v-else-if="clientBucket(row) === 'expiring'" color="orange" class="status-tag">
                            {{ t('depletingSoon') }}
                          </a-tag>
                          <div class="card-actions" @click.stop>
                            <a-tooltip :title="t('pages.clients.moreInformation')">
                              <InfoCircleOutlined class="row-action-trigger" @click="onShowInfo(row)" />
                            </a-tooltip>
                            <a-switch :checked="row.enable" size="small" :loading="togglingEmail === row.email"
                              @change="(next) => onToggleEnable(row, next)" />
                            <a-dropdown :trigger="['click']" placement="bottomRight">
                              <MoreOutlined class="row-action-trigger" @click.prevent />
                              <template #overlay>
                                <a-menu>
                                  <a-menu-item key="qr" @click="onShowQr(row)">
                                    <QrcodeOutlined /> {{ t('pages.clients.qrCode') }}
                                  </a-menu-item>
                                  <a-menu-item key="reset" @click="onResetTraffic(row)">
                                    <RetweetOutlined /> {{ t('pages.inbounds.resetTraffic') }}
                                  </a-menu-item>
                                  <a-menu-item key="edit" @click="onEdit(row)">
                                    <EditOutlined /> {{ t('edit') }}
                                  </a-menu-item>
                                  <a-menu-item key="delete" class="danger-item" @click="onDelete(row)">
                                    <DeleteOutlined /> {{ t('delete') }}
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
        :attached-ids="editingAttachedIds" :inbounds="inbounds" :ip-limit-enable="ipLimitEnable"
        :tg-bot-enable="tgBotEnable" :save="onSave" />
      <ClientInfoModal v-model:open="infoOpen" :client="infoClient" :inbounds-by-id="inboundsById"
        :is-online="infoClient ? isOnline(infoClient.email) : false" :sub-settings="subSettings" />
      <ClientQrModal v-model:open="qrOpen" :client="qrClient" :sub-settings="subSettings" />
      <ClientBulkAddModal v-model:open="bulkAddOpen" :inbounds="inbounds" :ip-limit-enable="ipLimitEnable"
        @saved="onBulkAddSaved" />
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

.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.filter-bar.mobile {
  gap: 6px;
  margin-bottom: 8px;
}

.filter-bar.mobile > * {
  flex: 0 0 auto;
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

.dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 4px;
  vertical-align: middle;
}

.dot-green { background: #52c41a; }
.dot-blue { background: #1677ff; }
.dot-red { background: #ff4d4f; }
.dot-orange { background: #fa8c16; }
.dot-gray { background: rgba(128, 128, 128, 0.6); }

.status-tag {
  margin: 0 0 0 4px;
  font-size: 11px;
  padding: 0 6px;
  line-height: 18px;
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

.clients-empty {
  padding: 32px 0;
  text-align: center;
  opacity: 0.55;
}

.danger-item {
  color: #ff4d4f;
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
