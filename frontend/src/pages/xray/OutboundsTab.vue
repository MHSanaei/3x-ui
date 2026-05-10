<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PlusOutlined,
  CloudOutlined,
  ApiOutlined,
  RetweetOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  VerticalAlignTopOutlined,
  ThunderboltOutlined,
  CheckCircleFilled,
  CloseCircleFilled,
  LoadingOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from '@ant-design/icons-vue';
import { Modal } from 'ant-design-vue';

import { SizeFormatter } from '@/utils';
import { Protocols } from '@/models/outbound.js';
import OutboundFormModal from './OutboundFormModal.vue';

const { t } = useI18n();

// Outbounds tab — list + actions over templateSettings.outbounds.
// Mirrors the legacy outbound table layout (identity / address /
// traffic / test result / test button) plus the row action menu
// (set first / edit / reset traffic / delete). Mobile collapses to
// a card list.

const props = defineProps({
  templateSettings: { type: Object, default: null },
  outboundsTraffic: { type: Array, default: () => [] },
  outboundTestStates: { type: Object, default: () => ({}) },
  inboundTags: { type: Array, default: () => [] },
  isMobile: { type: Boolean, default: false },
});

const inboundTagOptions = computed(() => {
  const out = new Set();
  for (const ib of props.templateSettings?.inbounds || []) {
    if (ib.tag) out.add(ib.tag);
  }
  for (const t of props.inboundTags || []) out.add(t);
  return [...out];
});

const emit = defineEmits(['reset-traffic', 'test', 'show-warp', 'show-nord', 'delete']);

// === Modal state ====================================================
const modalOpen = ref(false);
const editingOutbound = ref(null);
const editingIndex = ref(null);
const existingTags = ref([]);

function openAdd() {
  editingOutbound.value = null;
  editingIndex.value = null;
  existingTags.value = (props.templateSettings?.outbounds || []).map((o) => o.tag);
  modalOpen.value = true;
}
function openEdit(idx) {
  editingOutbound.value = props.templateSettings.outbounds[idx];
  editingIndex.value = idx;
  existingTags.value = (props.templateSettings?.outbounds || [])
    .filter((_, i) => i !== idx)
    .map((o) => o.tag);
  modalOpen.value = true;
}
function onConfirm(outbound) {
  if (editingIndex.value == null) {
    if (!outbound.tag) return;
    props.templateSettings.outbounds.push(outbound);
  } else {
    props.templateSettings.outbounds[editingIndex.value] = outbound;
  }
  modalOpen.value = false;
}

function confirmDelete(idx) {
  Modal.confirm({
    title: `${t('delete')} ${t('pages.xray.Outbounds')} #${idx + 1}?`,
    okText: t('delete'),
    okType: 'danger',
    cancelText: t('cancel'),
    onOk: () => { emit('delete', idx); },
  });
}
function setFirst(idx) {
  const arr = props.templateSettings.outbounds;
  arr.unshift(arr.splice(idx, 1)[0]);
}
function moveUp(idx) {
  if (idx <= 0) return;
  const arr = props.templateSettings.outbounds;
  [arr[idx - 1], arr[idx]] = [arr[idx], arr[idx - 1]];
}
function moveDown(idx) {
  const arr = props.templateSettings.outbounds;
  if (idx >= arr.length - 1) return;
  [arr[idx + 1], arr[idx]] = [arr[idx], arr[idx + 1]];
}

// === Per-row helpers ================================================
function trafficFor(o) {
  const t = props.outboundsTraffic.find((x) => x.tag === o.tag);
  return { up: t?.up || 0, down: t?.down || 0 };
}

// Lifted from legacy findOutboundAddress — returns an array of
// "host:port" strings for the protocols that have one, or null when
// the outbound has no externally-visible endpoint (Freedom, Blackhole,
// DNS without an explicit address, etc.).
function outboundAddresses(o) {
  let serverObj;
  switch (o.protocol) {
    case Protocols.VMess:
      serverObj = o.settings?.vnext;
      break;
    case Protocols.VLESS:
      return [`${o.settings?.address || ''}:${o.settings?.port || ''}`];
    case Protocols.HTTP:
    case Protocols.Socks:
    case Protocols.Shadowsocks:
    case Protocols.Trojan:
      serverObj = o.settings?.servers;
      break;
    case Protocols.DNS: {
      const addr = o.settings?.rewriteAddress || o.settings?.address || '';
      const port = o.settings?.rewritePort || o.settings?.port || '';
      return addr || port ? [`${addr}:${port}`] : [];
    }
    case Protocols.Wireguard:
      return (o.settings?.peers || []).map((p) => p.endpoint);
    default:
      return [];
  }
  return serverObj ? serverObj.map((s) => `${s.address}:${s.port}`) : [];
}

function isUntestable(o) {
  return o.protocol === Protocols.Blackhole
    || o.protocol === Protocols.Loopback
    || o.tag === 'blocked';
}
function isTesting(idx) {
  return !!props.outboundTestStates?.[idx]?.testing;
}
function testResult(idx) {
  return props.outboundTestStates?.[idx]?.result || null;
}
function showSecurity(security) {
  return security === 'tls' || security === 'reality';
}

// === Columns ========================================================
// Computed so titles re-render after a locale swap.
const columns = computed(() => [
  { title: '#', key: 'action', align: 'center', width: 70 },
  { title: 'Tag', key: 'identity', align: 'left', width: 220 },
  { title: t('pages.inbounds.address'), key: 'address', align: 'left', width: 230 },
  { title: t('pages.inbounds.traffic'), key: 'traffic', align: 'left', width: 200 },
  { title: t('check'), key: 'testResult', align: 'left', width: 140 },
  { title: t('check'), key: 'test', align: 'center', width: 80 },
]);

const rows = computed(() => {
  if (!props.templateSettings?.outbounds) return [];
  return props.templateSettings.outbounds.map((o, i) => ({ key: i, ...o }));
});
</script>

<template>
  <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
    <!-- Toolbar -->
    <a-row :gutter="[12, 12]" align="middle" justify="space-between">
      <a-col :xs="24" :sm="14">
        <a-space size="small">
          <a-button type="primary" @click="openAdd">
            <template #icon>
              <PlusOutlined />
            </template>
            <span v-if="!isMobile">{{ t('pages.xray.Outbounds') }}</span>
          </a-button>
          <a-button type="primary" @click="emit('show-warp')">
            <template #icon>
              <CloudOutlined />
            </template>
            WARP
          </a-button>
          <a-button type="primary" @click="emit('show-nord')">
            <template #icon>
              <ApiOutlined />
            </template>
            NordVPN
          </a-button>
        </a-space>
      </a-col>
      <a-col :xs="24" :sm="10" class="toolbar-right">
        <a-popconfirm placement="topRight" :ok-text="t('reset')" :cancel-text="t('cancel')"
          :title="t('pages.inbounds.resetAllTrafficContent')" @confirm="emit('reset-traffic', '-alltags-')">
          <a-button>
            <template #icon>
              <RetweetOutlined />
            </template>
          </a-button>
        </a-popconfirm>
      </a-col>
    </a-row>

    <!-- Mobile: card list -->
    <template v-if="isMobile">
      <div v-if="rows.length === 0" class="card-empty">—</div>
      <div v-for="(record, index) in rows" :key="record.key" class="outbound-card">
        <div class="card-head">
          <div class="card-identity">
            <span class="card-num">{{ index + 1 }}</span>
            <a-tooltip :title="record.tag">
              <span class="tag-name">{{ record.tag }}</span>
            </a-tooltip>
            <a-tag color="green">{{ record.protocol }}</a-tag>
            <template
              v-if="[Protocols.VMess, Protocols.VLESS, Protocols.Trojan, Protocols.Shadowsocks].includes(record.protocol)">
              <a-tag>{{ record.streamSettings?.network }}</a-tag>
              <a-tag v-if="showSecurity(record.streamSettings?.security)" color="purple">
                {{ record.streamSettings.security }}
              </a-tag>
            </template>
          </div>
          <a-dropdown :trigger="['click']">
            <a-button shape="circle" size="small">
              <MoreOutlined />
            </a-button>
            <template #overlay>
              <a-menu>
                <a-menu-item v-if="index > 0" @click="setFirst(index)">
                  <VerticalAlignTopOutlined />
                </a-menu-item>
                <a-menu-item @click="openEdit(index)">
                  <EditOutlined /> {{ t('edit') }}
                </a-menu-item>
                <a-menu-item @click="emit('reset-traffic', record.tag || '')">
                  <RetweetOutlined /> {{ t('pages.inbounds.resetTraffic') }}
                </a-menu-item>
                <a-menu-item class="danger" @click="confirmDelete(index)">
                  <DeleteOutlined /> {{ t('delete') }}
                </a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>
        <div v-if="outboundAddresses(record).length > 0" class="address-list">
          <a-tooltip v-for="addr in outboundAddresses(record)" :key="addr" :title="addr">
            <span class="address-pill">{{ addr }}</span>
          </a-tooltip>
        </div>
        <div class="card-foot">
          <span class="traffic-up">↑ {{ SizeFormatter.sizeFormat(trafficFor(record).up) }}</span>
          <span class="traffic-sep" />
          <span class="traffic-down">↓ {{ SizeFormatter.sizeFormat(trafficFor(record).down) }}</span>
          <span class="card-test">
            <span v-if="testResult(index)" :class="testResult(index).success ? 'pill-ok' : 'pill-fail'">
              <CheckCircleFilled v-if="testResult(index).success" />
              <CloseCircleFilled v-else />
              <span v-if="testResult(index).success">{{ testResult(index).delay }}&nbsp;ms</span>
              <span v-else>failed</span>
            </span>
            <LoadingOutlined v-else-if="isTesting(index)" />
            <a-button type="primary" shape="circle" size="small" :loading="isTesting(index)"
              :disabled="isUntestable(record) || isTesting(index)" @click="emit('test', index)">
              <template #icon>
                <ThunderboltOutlined />
              </template>
            </a-button>
          </span>
        </div>
      </div>
    </template>

    <!-- Desktop: table -->
    <a-table v-else :columns="columns" :data-source="rows" :row-key="(r) => r.key" :pagination="false" size="small">
      <template #bodyCell="{ column, record, index }">
        <template v-if="column.key === 'action'">
          <div class="action-cell">
            <span class="row-index">{{ index + 1 }}</span>
            <a-dropdown :trigger="['click']">
              <a-button shape="circle" size="small">
                <MoreOutlined />
              </a-button>
              <template #overlay>
                <a-menu>
                  <a-menu-item v-if="index > 0" @click="setFirst(index)">
                    <VerticalAlignTopOutlined /> Move to top
                  </a-menu-item>
                  <a-menu-item @click="openEdit(index)">
                    <EditOutlined /> Edit
                  </a-menu-item>
                  <a-menu-item :disabled="index === 0" @click="moveUp(index)">
                    <ArrowUpOutlined />
                  </a-menu-item>
                  <a-menu-item :disabled="index === rows.length - 1" @click="moveDown(index)">
                    <ArrowDownOutlined />
                  </a-menu-item>
                  <a-menu-item @click="emit('reset-traffic', record.tag || '')">
                    <RetweetOutlined /> Reset traffic
                  </a-menu-item>
                  <a-menu-item class="danger" @click="confirmDelete(index)">
                    <DeleteOutlined /> Delete
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </div>
        </template>

        <template v-else-if="column.key === 'identity'">
          <div class="identity-cell">
            <a-tooltip :title="record.tag">
              <span class="tag-name">{{ record.tag }}</span>
            </a-tooltip>
            <div class="protocol-line">
              <a-tag color="green">{{ record.protocol }}</a-tag>
              <template
                v-if="[Protocols.VMess, Protocols.VLESS, Protocols.Trojan, Protocols.Shadowsocks].includes(record.protocol)">
                <a-tag>{{ record.streamSettings?.network }}</a-tag>
                <a-tag v-if="showSecurity(record.streamSettings?.security)" color="purple">
                  {{ record.streamSettings.security }}
                </a-tag>
              </template>
            </div>
          </div>
        </template>

        <template v-else-if="column.key === 'address'">
          <div class="address-list">
            <a-tooltip v-for="addr in outboundAddresses(record)" :key="addr" :title="addr">
              <span class="address-pill">{{ addr }}</span>
            </a-tooltip>
            <span v-if="outboundAddresses(record).length === 0" class="empty">—</span>
          </div>
        </template>

        <template v-else-if="column.key === 'traffic'">
          <span class="traffic-up">↑ {{ SizeFormatter.sizeFormat(trafficFor(record).up) }}</span>
          <span class="traffic-sep" />
          <span class="traffic-down">↓ {{ SizeFormatter.sizeFormat(trafficFor(record).down) }}</span>
        </template>

        <template v-else-if="column.key === 'testResult'">
          <span v-if="testResult(index)" :class="testResult(index).success ? 'pill-ok' : 'pill-fail'">
            <CheckCircleFilled v-if="testResult(index).success" />
            <CloseCircleFilled v-else />
            <span v-if="testResult(index).success">{{ testResult(index).delay }}&nbsp;ms</span>
            <a-tooltip v-else :title="testResult(index).error">
              <span>failed</span>
            </a-tooltip>
          </span>
          <LoadingOutlined v-else-if="isTesting(index)" />
          <span v-else class="empty">—</span>
        </template>

        <template v-else-if="column.key === 'test'">
          <a-tooltip :title="t('check')">
            <a-button type="primary" shape="circle" :loading="isTesting(index)"
              :disabled="isUntestable(record) || isTesting(index)" @click="emit('test', index)">
              <template #icon>
                <ThunderboltOutlined />
              </template>
            </a-button>
          </a-tooltip>
        </template>
      </template>
    </a-table>

    <OutboundFormModal v-model:open="modalOpen" :outbound="editingOutbound" :existing-tags="existingTags"
      :inbound-tags="inboundTagOptions" @confirm="onConfirm" />
  </a-space>
</template>

<style scoped>
.toolbar-right {
  display: flex;
  justify-content: flex-end;
}

.card-empty {
  text-align: center;
  opacity: 0.4;
  padding: 16px 0;
}

.outbound-card {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.card-identity {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.card-num {
  font-weight: 500;
  opacity: 0.7;
  min-width: 18px;
  text-align: right;
}

.tag-name {
  font-weight: 500;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
}

.protocol-line {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 2px;
}

.address-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.address-pill {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.05);
}

:global(body.dark) .address-pill {
  background: rgba(255, 255, 255, 0.06);
}

.action-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.row-index {
  font-weight: 500;
  opacity: 0.7;
  min-width: 18px;
  text-align: right;
}

.identity-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.card-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.card-test {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.traffic-up {
  color: #008771;
  font-size: 12px;
}

.traffic-down {
  color: #3c89e8;
  font-size: 12px;
}

.traffic-sep {
  display: inline-block;
  width: 4px;
}

.pill-ok,
.pill-fail {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 8px;
  border-radius: 12px;
  font-size: 12px;
}

.pill-ok {
  color: #008771;
  background: rgba(0, 135, 113, 0.12);
}

.pill-fail {
  color: #e04141;
  background: rgba(224, 65, 65, 0.12);
}

.empty {
  opacity: 0.4;
}

.danger {
  color: #ff4d4f;
}
</style>
