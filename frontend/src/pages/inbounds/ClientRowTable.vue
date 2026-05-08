<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  EditOutlined,
  InfoCircleOutlined,
  QrcodeOutlined,
  RetweetOutlined,
  DeleteOutlined,
  EllipsisOutlined,
} from '@ant-design/icons-vue';
import { Modal } from 'ant-design-vue';

import { SizeFormatter, IntlUtil, ColorUtils } from '@/utils';

const { t } = useI18n();

// Per-inbound expand-row table. Rendered inside the inbound list's
// a-table#expandedRowRender slot for any inbound where
// `dbInbound.isMultiUser()` returns true. Mirrors the legacy
// component/aClientTable layout.
//
// The component itself does no API calls — it emits typed events the
// parent routes back to the existing modals/handlers (edit, qr, info,
// reset traffic, delete, toggle-enable).

const props = defineProps({
  dbInbound: { type: Object, required: true },
  isMobile: { type: Boolean, default: false },
  trafficDiff: { type: Number, default: 0 },
  expireDiff: { type: Number, default: 0 },
  onlineClients: { type: Array, default: () => [] },
  lastOnlineMap: { type: Object, default: () => ({}) },
  isDarkTheme: { type: Boolean, default: false },
});

const emit = defineEmits([
  'edit-client',
  'qrcode-client',
  'info-client',
  'reset-traffic-client',
  'delete-client',
  'toggle-enable-client',
]);

// Surface the parsed Inbound so we can read its clients array
// directly. legacy used dbInbound.toInbound().clients via a
// `getInboundClients` helper; the parsed cache is invalidated on
// every refresh by useInbounds.setInbounds.
const inbound = computed(() => props.dbInbound.toInbound());
const clients = computed(() => inbound.value?.clients || []);

// === Per-client stats lookup =======================================
// Mirrors the legacy lazy-built email->stats Map cached on the
// dbInbound; recomputed when the underlying clientStats array is
// replaced by a refresh.
const statsMap = computed(() => {
  const m = new Map();
  for (const cs of (props.dbInbound.clientStats || [])) m.set(cs.email, cs);
  return m;
});
function statsFor(email) {
  return email ? statsMap.value.get(email) : null;
}

function getUp(email) { return statsFor(email)?.up || 0; }
function getDown(email) { return statsFor(email)?.down || 0; }
function getSum(email) { const s = statsFor(email); return s ? s.up + s.down : 0; }
function getRem(email) {
  const s = statsFor(email);
  if (!s) return 0;
  const r = s.total - s.up - s.down;
  return r > 0 ? r : 0;
}
function getAllTime(email) {
  const s = statsFor(email);
  if (!s) return 0;
  // allTime is the cumulative-historical counter; never let it dip
  // below up+down (manual edits / partial migrations can push it under).
  const current = (s.up || 0) + (s.down || 0);
  return s.allTime > current ? s.allTime : current;
}
function isClientDepleted(email) {
  const s = statsFor(email);
  if (!s) return false;
  const total = s.total ?? 0;
  const used = (s.up ?? 0) + (s.down ?? 0);
  if (total > 0 && used >= total) return true;
  const exp = s.expiryTime ?? 0;
  if (exp > 0 && Date.now() >= exp) return true;
  return false;
}
function isClientOnline(email) {
  return !!email && props.onlineClients.includes(email);
}
function lastOnlineLabel(email) {
  const ts = props.lastOnlineMap[email];
  if (!ts) return '-';
  return IntlUtil.formatDate(ts);
}

function statsProgress(email) {
  const s = statsFor(email);
  if (!s) return 0;
  if (s.total === 0) return 100;
  return (100 * (s.down + s.up)) / s.total;
}
function expireProgress(expTime, reset) {
  const now = Date.now();
  const remainedSec = expTime < 0 ? -expTime / 1000 : (expTime - now) / 1000;
  const resetSec = reset * 86400;
  if (remainedSec >= resetSec) return 0;
  return 100 * (1 - remainedSec / resetSec);
}
function clientStatsColor(email) {
  return ColorUtils.clientUsageColor(statsFor(email), props.trafficDiff);
}
function statsExpColor(email) {
  if (!email) return '#7a316f';
  const s = statsFor(email);
  if (!s) return '#7a316f';
  const a = ColorUtils.usageColor(s.down + s.up, props.trafficDiff, s.total);
  const b = ColorUtils.usageColor(Date.now(), props.expireDiff, s.expiryTime);
  if (a === 'red' || b === 'red') return '#cf3c3c';
  if (a === 'orange' || b === 'orange') return '#f37b24';
  if (a === 'green' || b === 'green') return '#008771';
  return '#7a316f';
}

// === Helpers ========================================================
const isRemovable = computed(() => clients.value.length > 1);

function totalGbDisplay(client) {
  if (!client.totalGB || client.totalGB <= 0) return '∞';
  // The model class exposes ._totalGB as bytes->GB for the form, but
  // the table shows a coarser rounding. Match legacy: tail with 'GB'.
  return `${Math.round((client.totalGB / 1073741824) * 100) / 100} GB`;
}

function statusBadgeColor(client) {
  if (!client.enable) return props.isDarkTheme ? '#2c3950' : '#bcbcbc';
  return statsExpColor(client.email);
}

// === Action confirms (mounted on the row, not a modal) ==============
function confirmReset(client) {
  Modal.confirm({
    title: `${t('pages.inbounds.resetTraffic')} — ${client.email}`,
    content: t('pages.inbounds.resetTrafficContent'),
    okText: t('reset'),
    cancelText: t('cancel'),
    onOk: () => emit('reset-traffic-client', { dbInbound: props.dbInbound, client }),
  });
}
function confirmDelete(client) {
  Modal.confirm({
    title: `${t('pages.inbounds.deleteClient')} — ${client.email}`,
    content: t('pages.inbounds.deleteClientContent'),
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: () => emit('delete-client', { dbInbound: props.dbInbound, client }),
  });
}

// === Columns ========================================================
// Two layouts: desktop has icon-row actions across; mobile collapses
// the per-row actions into a single dropdown + an info popover.
// Computed so column titles re-render after a locale swap.
const desktopColumns = computed(() => [
  { title: t('pages.settings.actions'), key: 'actions', width: 140 },
  { title: t('enable'), key: 'enable', width: 60 },
  { title: t('online'), key: 'online', width: 80 },
  { title: t('pages.inbounds.client'), key: 'client', width: 160 },
  { title: t('pages.inbounds.traffic'), key: 'traffic', align: 'center', width: 200 },
  { title: t('pages.inbounds.allTimeTraffic'), key: 'allTime', align: 'center', width: 110 },
  { title: t('pages.inbounds.expireDate'), key: 'expiryTime', align: 'center', width: 180 },
]);
const mobileColumns = computed(() => [
  { title: t('pages.settings.actions'), key: 'actionMenu', align: 'center', width: 10 },
  { title: t('pages.inbounds.client'), key: 'client', align: 'left', width: 90 },
  { title: t('info'), key: 'info', align: 'center', width: 10 },
]);

const columns = computed(() => (props.isMobile ? mobileColumns.value : desktopColumns.value));
</script>

<template>
  <a-table
    :columns="columns"
    :data-source="clients"
    :row-key="(c) => c.email || c.id || c.password"
    :pagination="false"
    :scroll="isMobile ? {} : { x: 'max-content' }"
    size="small"
    class="client-row-table"
  >
    <template #bodyCell="{ column, record }">
      <!-- ============== Desktop action icons ============== -->
      <template v-if="column.key === 'actions'">
        <a-space :size="6">
          <a-tooltip v-if="dbInbound.hasLink()" :title="t('qrCode')">
            <QrcodeOutlined
              class="row-icon"
              @click="emit('qrcode-client', { dbInbound, client: record })"
            />
          </a-tooltip>
          <a-tooltip :title="t('edit')">
            <EditOutlined
              class="row-icon"
              @click="emit('edit-client', { dbInbound, client: record })"
            />
          </a-tooltip>
          <a-tooltip :title="t('info')">
            <InfoCircleOutlined
              class="row-icon"
              @click="emit('info-client', { dbInbound, client: record })"
            />
          </a-tooltip>
          <a-tooltip v-if="record.email" :title="t('pages.inbounds.resetTraffic')">
            <RetweetOutlined class="row-icon" @click="confirmReset(record)" />
          </a-tooltip>
          <a-tooltip v-if="isRemovable" :title="t('delete')">
            <DeleteOutlined class="row-icon danger" @click="confirmDelete(record)" />
          </a-tooltip>
        </a-space>
      </template>

      <!-- ============== Enable switch ============== -->
      <template v-else-if="column.key === 'enable'">
        <a-switch
          :checked="record.enable"
          @change="(next) => emit('toggle-enable-client', { dbInbound, client: record, next })"
        />
      </template>

      <!-- ============== Online tag ============== -->
      <template v-else-if="column.key === 'online'">
        <a-popover>
          <template #content>{{ t('lastOnline') }}: {{ lastOnlineLabel(record.email) }}</template>
          <a-tag v-if="record.enable && isClientOnline(record.email)" color="green">{{ t('online') }}</a-tag>
          <a-tag v-else>{{ t('offline') }}</a-tag>
        </a-popover>
      </template>

      <!-- ============== Client identity (status dot + email + comment) ============== -->
      <template v-else-if="column.key === 'client'">
        <a-space :size="2" class="client-id-cell" :style="{ flexWrap: 'nowrap' }">
          <a-tooltip>
            <template #title>
              <template v-if="isClientDepleted(record.email)">{{ t('depleted') }}</template>
              <template v-else-if="!record.enable">{{ t('disabled') }}</template>
              <template v-else-if="isClientOnline(record.email)">{{ t('online') }}</template>
              <template v-else>{{ t('offline') }}</template>
            </template>
            <a-badge :color="statusBadgeColor(record)" />
          </a-tooltip>
          <a-space direction="vertical" :size="2" class="client-id-stack">
            <a-tooltip :title="record.email">
              <span class="client-email">{{ record.email }}</span>
            </a-tooltip>
            <span v-if="record.comment && record.comment.trim()" class="client-comment">
              {{ record.comment.length > 50 ? record.comment.substring(0, 47) + '…' : record.comment }}
            </span>
          </a-space>
        </a-space>
      </template>

      <!-- ============== Traffic with progress bar ============== -->
      <template v-else-if="column.key === 'traffic'">
        <a-popover>
          <template v-if="record.email" #content>
            <table cellpadding="2">
              <tbody>
                <tr>
                  <td>↑ {{ SizeFormatter.sizeFormat(getUp(record.email)) }}</td>
                  <td>↓ {{ SizeFormatter.sizeFormat(getDown(record.email)) }}</td>
                </tr>
                <tr v-if="record.totalGB > 0">
                  <td>{{ t('remained') }}</td>
                  <td>{{ SizeFormatter.sizeFormat(getRem(record.email)) }}</td>
                </tr>
              </tbody>
            </table>
          </template>
          <div class="traffic-cell">
            <div class="traffic-text">{{ SizeFormatter.sizeFormat(getSum(record.email)) }}</div>
            <div class="traffic-bar" v-if="!record.enable">
              <a-progress
                :stroke-color="isDarkTheme ? 'rgb(72,84,105)' : '#bcbcbc'"
                :show-info="false"
                :percent="statsProgress(record.email)"
              />
            </div>
            <div class="traffic-bar" v-else-if="record.totalGB > 0">
              <a-progress
                :stroke-color="clientStatsColor(record.email)"
                :show-info="false"
                :status="isClientDepleted(record.email) ? 'exception' : ''"
                :percent="statsProgress(record.email)"
              />
            </div>
            <div class="traffic-bar infinite" v-else>
              <a-progress :show-info="false" :percent="100" />
            </div>
            <div class="traffic-text">{{ totalGbDisplay(record) }}</div>
          </div>
        </a-popover>
      </template>

      <!-- ============== All-time ============== -->
      <template v-else-if="column.key === 'allTime'">
        <a-tag>{{ SizeFormatter.sizeFormat(getAllTime(record.email)) }}</a-tag>
      </template>

      <!-- ============== Expiry ============== -->
      <template v-else-if="column.key === 'expiryTime'">
        <template v-if="record.expiryTime !== 0 && record.reset > 0">
          <a-popover>
            <template #content>
              <span v-if="record.expiryTime < 0">{{ t('pages.client.delayedStart') }}</span>
              <span v-else>{{ IntlUtil.formatDate(record.expiryTime) }}</span>
            </template>
            <div class="traffic-cell">
              <div class="traffic-text">{{ IntlUtil.formatRelativeTime(record.expiryTime) }}</div>
              <div class="traffic-bar infinite">
                <a-progress
                  :show-info="false"
                  :status="isClientDepleted(record.email) ? 'exception' : ''"
                  :percent="expireProgress(record.expiryTime, record.reset)"
                />
              </div>
              <div class="traffic-text">{{ record.reset }}d</div>
            </div>
          </a-popover>
        </template>
        <template v-else>
          <a-popover v-if="record.expiryTime !== 0">
            <template #content>
              <span v-if="record.expiryTime < 0">{{ t('pages.client.delayedStart') }}</span>
              <span v-else>{{ IntlUtil.formatDate(record.expiryTime) }}</span>
            </template>
            <a-tag
              :style="{ minWidth: '50px', border: 'none' }"
              :color="ColorUtils.userExpiryColor(expireDiff, record, isDarkTheme)"
            >
              {{ IntlUtil.formatRelativeTime(record.expiryTime) }}
            </a-tag>
          </a-popover>
          <a-tag
            v-else
            :color="ColorUtils.userExpiryColor(expireDiff, record, isDarkTheme)"
            :style="{ border: 'none' }"
          >
            ∞
          </a-tag>
        </template>
      </template>

      <!-- ============== Mobile-only action menu ============== -->
      <template v-else-if="column.key === 'actionMenu'">
        <a-dropdown :trigger="['click']">
          <EllipsisOutlined class="row-icon" @click.prevent />
          <template #overlay>
            <a-menu>
              <a-menu-item
                v-if="dbInbound.hasLink()"
                @click="emit('qrcode-client', { dbInbound, client: record })"
              ><QrcodeOutlined /> {{ t('qrCode') }}</a-menu-item>
              <a-menu-item @click="emit('edit-client', { dbInbound, client: record })">
                <EditOutlined /> {{ t('edit') }}
              </a-menu-item>
              <a-menu-item @click="emit('info-client', { dbInbound, client: record })">
                <InfoCircleOutlined /> {{ t('info') }}
              </a-menu-item>
              <a-menu-item v-if="record.email" @click="confirmReset(record)">
                <RetweetOutlined /> {{ t('pages.inbounds.resetTraffic') }}
              </a-menu-item>
              <a-menu-item v-if="isRemovable" @click="confirmDelete(record)">
                <DeleteOutlined /> <span class="danger">{{ t('delete') }}</span>
              </a-menu-item>
              <a-menu-item>
                <a-switch
                  size="small"
                  :checked="record.enable"
                  @change="(next) => emit('toggle-enable-client', { dbInbound, client: record, next })"
                />
                {{ t('enable') }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </template>

      <!-- ============== Mobile info popover ============== -->
      <template v-else-if="column.key === 'info'">
        <a-popover :placement="isMobile ? 'bottomLeft' : 'bottomRight'" trigger="click">
          <template #content>
            <table cellpadding="2">
              <tbody>
                <tr>
                  <td colspan="2" class="text-center">{{ t('pages.inbounds.traffic') }}</td>
                </tr>
                <tr>
                  <td class="num-cell">
                    {{ SizeFormatter.sizeFormat(getSum(record.email)) }}
                  </td>
                  <td class="num-cell">{{ totalGbDisplay(record) }}</td>
                </tr>
                <tr>
                  <td colspan="2" class="text-center">
                    <a-divider style="margin: 0" />
                    {{ t('pages.inbounds.expireDate') }}
                  </td>
                </tr>
                <tr>
                  <td colspan="2" class="text-center">
                    <a-tag v-if="record.expiryTime > 0">
                      {{ IntlUtil.formatRelativeTime(record.expiryTime) }}
                    </a-tag>
                    <a-tag v-else-if="record.expiryTime < 0" color="green">
                      {{ -record.expiryTime / 86400000 }}d ({{ t('pages.client.delayedStart') }})
                    </a-tag>
                    <a-tag v-else color="purple">∞</a-tag>
                  </td>
                </tr>
              </tbody>
            </table>
          </template>
          <a-button shape="round" size="small">
            <InfoCircleOutlined />
          </a-button>
        </a-popover>
      </template>
    </template>
  </a-table>
</template>

<style scoped>
.client-row-table {
  margin: -10px 22px -21px;
}
:deep(.client-row-table .ant-table-tbody > tr > td) {
  padding-top: 6px;
  padding-bottom: 6px;
}

.row-icon {
  font-size: 18px;
  cursor: pointer;
  padding: 0 4px;
}
.row-icon.danger,
.danger {
  color: #ff4d4f;
}

.client-id-cell {
  display: inline-flex;
  align-items: center;
  min-width: 0;
}
.client-id-stack {
  min-width: 0;
  overflow: hidden;
}
.client-email {
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
  display: inline-block;
}
.client-comment {
  font-size: 11px;
  opacity: 0.7;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
  display: inline-block;
}

.traffic-cell {
  display: grid;
  grid-template-columns: minmax(60px, auto) 1fr minmax(50px, auto);
  align-items: center;
  gap: 6px;
  min-width: 180px;
}
.traffic-text {
  font-size: 12px;
  white-space: nowrap;
}
.traffic-bar {
  min-width: 40px;
}
.traffic-bar.infinite :deep(.ant-progress-inner) {
  background: rgba(122, 49, 111, 0.15);
}

.text-center { text-align: center; }
.num-cell { text-align: right; font-size: 12px; padding: 2px 6px; }
</style>
