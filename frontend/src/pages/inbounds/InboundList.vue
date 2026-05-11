<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PlusOutlined,
  MenuOutlined,
  SearchOutlined,
  FilterOutlined,
  MoreOutlined,
  EditOutlined,
  QrcodeOutlined,
  UserAddOutlined,
  UsergroupAddOutlined,
  CopyOutlined,
  FileDoneOutlined,
  ExportOutlined,
  ImportOutlined,
  ReloadOutlined,
  RestOutlined,
  RetweetOutlined,
  BlockOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  RightOutlined,
} from '@ant-design/icons-vue';

import { HttpUtil, ObjectUtil, SizeFormatter, IntlUtil, ColorUtils } from '@/utils';
import { DBInbound } from '@/models/dbinbound.js';
import { Inbound } from '@/models/inbound.js';
import InfinityIcon from '@/components/InfinityIcon.vue';
import ClientRowTable from './ClientRowTable.vue';
import { useDatepicker } from '@/composables/useDatepicker.js';

const { datepicker } = useDatepicker();

const { t } = useI18n();

const props = defineProps({
  dbInbounds: { type: Array, required: true },
  clientCount: { type: Object, required: true },
  onlineClients: { type: Array, required: true },
  lastOnlineMap: { type: Object, default: () => ({}) },
  expireDiff: { type: Number, default: 0 },
  trafficDiff: { type: Number, default: 0 },
  pageSize: { type: Number, default: 0 },
  isMobile: { type: Boolean, default: false },
  isDarkTheme: { type: Boolean, default: false },
  subEnable: { type: Boolean, default: false },
  // Map node id -> node row, supplied by the parent page so each
  // inbound row can render its node name without an extra fetch.
  nodesById: { type: Map, default: () => new Map() },
});

const emit = defineEmits([
  'refresh',
  'add-inbound',
  'general-action',
  'row-action',
  // Per-client events surfaced from the expand-row table.
  'edit-client',
  'qrcode-client',
  'info-client',
  'reset-traffic-client',
  'delete-client',
  'toggle-enable-client',
]);

// ============ Toolbar / search & filter =============================
const enableFilter = ref(false);
const searchKey = ref('');
const filterBy = ref('');

// Toggle the filter mode — flip cleans the other input.
function onToggleFilter() {
  if (enableFilter.value) searchKey.value = '';
  else filterBy.value = '';
}

// ============ Search / filter projection =============================
// Mirrors the legacy logic: when searching, keep inbounds that match
// anywhere (deep search); when filtering, keep inbounds that have at
// least one client in the requested bucket and reduce their settings
// to that bucket.
function projectInbound(dbInbound, predicate) {
  const next = new DBInbound(dbInbound);
  let settings;
  try {
    settings = JSON.parse(dbInbound.settings || '{}');
  } catch (_e) {
    settings = {};
  }
  if (!Array.isArray(settings.clients)) return next;
  const filtered = settings.clients.filter(predicate);
  next.settings = Inbound.Settings.fromJson(dbInbound.protocol, { clients: filtered });
  next.invalidateCache();
  return next;
}

const visibleInbounds = computed(() => {
  if (enableFilter.value) {
    if (ObjectUtil.isEmpty(filterBy.value)) return [...props.dbInbounds];
    const out = [];
    for (const dbInbound of props.dbInbounds) {
      const c = props.clientCount[dbInbound.id];
      if (!c || !c[filterBy.value] || c[filterBy.value].length === 0) continue;
      const list = c[filterBy.value];
      out.push(projectInbound(dbInbound, (client) => list.includes(client.email)));
    }
    return out;
  }
  if (ObjectUtil.isEmpty(searchKey.value)) return [...props.dbInbounds];
  const out = [];
  for (const dbInbound of props.dbInbounds) {
    if (!ObjectUtil.deepSearch(dbInbound, searchKey.value)) continue;
    out.push(projectInbound(dbInbound, (client) => ObjectUtil.deepSearch(client, searchKey.value)));
  }
  return out;
});

// ============ Columns =================================================
// `key`-driven so we can render via the body-cell slot below. AD-Vue 4's
// `responsive` array still works on column defs. Computed so column
// labels react to live locale switches.
const desktopColumns = computed(() => {
  const cols = [
    { title: 'ID', dataIndex: 'id', key: 'id', align: 'right', width: 30, responsive: ['xs'] },
    { title: t('pages.inbounds.operate'), key: 'action', align: 'center', width: 30 },
    { title: t('pages.inbounds.enable'), key: 'enable', align: 'center', width: 35 },
    { title: t('pages.inbounds.remark'), dataIndex: 'remark', key: 'remark', align: 'center', width: 60 },
  ];
  if (props.nodesById.size > 0) {
    cols.push({ title: t('pages.inbounds.node'), key: 'node', align: 'center', width: 60 });
  }
  cols.push(
    { title: t('pages.inbounds.port'), dataIndex: 'port', key: 'port', align: 'center', width: 40 },
    { title: t('pages.inbounds.protocol'), key: 'protocol', align: 'left', width: 130 },
    { title: t('clients'), key: 'clients', align: 'left', width: 50 },
    { title: t('pages.inbounds.traffic'), key: 'traffic', align: 'center', width: 90 },
    { title: t('pages.inbounds.allTimeTraffic'), key: 'allTimeInbound', align: 'center', width: 95 },
    { title: t('pages.inbounds.expireDate'), key: 'expiryTime', align: 'center', width: 40 },
  );
  return cols;
});
const columns = computed(() => desktopColumns.value);

// Mobile expansion state — replaces a-table's expandable() since the
// mobile branch renders a hand-rolled card list rather than a table.
const expandedIds = ref(new Set());
function toggleExpanded(id) {
  const next = new Set(expandedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  expandedIds.value = next;
}
function isExpanded(id) {
  return expandedIds.value.has(id);
}

// ============ Pagination ============================================
function paginationFor(rows) {
  const size = props.pageSize > 0 ? props.pageSize : rows.length || 1;
  return {
    pageSize: size,
    showSizeChanger: false,
    hideOnSinglePage: true,
  };
}

// ============ Per-row enable switch =================================
async function onSwitchEnable(dbInbound, next) {
  const previous = dbInbound.enable;
  dbInbound.enable = next; // optimistic
  try {
    const formData = new FormData();
    formData.append('enable', String(next));
    const msg = await HttpUtil.post(`/panel/api/inbounds/setEnable/${dbInbound.id}`, formData);
    if (!msg?.success) dbInbound.enable = previous;
  } catch (_e) {
    dbInbound.enable = previous;
  }
}

// ============ Helpers shared with the templates =====================
// Whether to show the "Switch xray" / qrcode menu entry — same predicate
// as legacy: SS single-user inbounds and WireGuard inbounds expose
// inbound-wide QR codes.
function showQrCodeMenu(dbInbound) {
  if (dbInbound.isWireguard) return true;
  if (dbInbound.isSS) {
    try {
      return !dbInbound.toInbound().isSSMultiUser;
    } catch (_e) {
      return false;
    }
  }
  return false;
}
</script>

<template>
  <a-card hoverable>
    <template #title>
      <a-space direction="horizontal">
        <a-button type="primary" @click="emit('add-inbound')">
          <template #icon>
            <PlusOutlined />
          </template>
          <template v-if="!isMobile">{{ t('pages.inbounds.addInbound') }}</template>
        </a-button>
        <a-dropdown :trigger="['click']">
          <a-button type="primary">
            <template #icon>
              <MenuOutlined />
            </template>
            <template v-if="!isMobile">{{ t('pages.inbounds.generalActions') }}</template>
          </a-button>
          <template #overlay>
            <a-menu @click="(a) => emit('general-action', a.key)">
              <a-menu-item key="import">
                <ImportOutlined /> {{ t('pages.inbounds.importInbound') }}
              </a-menu-item>
              <a-menu-item key="export">
                <ExportOutlined /> {{ t('pages.inbounds.export') }}
              </a-menu-item>
              <a-menu-item v-if="subEnable" key="subs">
                <ExportOutlined /> {{ t('pages.inbounds.export') }} — {{ t('pages.settings.subSettings') }}
              </a-menu-item>
              <a-menu-item key="resetInbounds">
                <ReloadOutlined /> {{ t('pages.inbounds.resetAllTraffic') }}
              </a-menu-item>
              <a-menu-item key="resetClients">
                <FileDoneOutlined /> {{ t('pages.inbounds.resetAllClientTraffics') }}
              </a-menu-item>
              <a-menu-item key="delDepletedClients" class="danger-item">
                <RestOutlined /> {{ t('pages.inbounds.delDepletedClients') }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-space>
    </template>

    <a-space direction="vertical" :style="{ width: '100%' }">
      <!-- Search / filter toolbar -->
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
      </div>

      <!-- ====================== Mobile: card list ======================= -->
      <div v-if="isMobile" class="inbound-cards">
        <div v-if="visibleInbounds.length === 0" class="card-empty">—</div>

        <div v-for="record in visibleInbounds" :key="record.id" class="inbound-card">
          <!-- Header: chevron (multi-user only) + remark + enable + actions -->
          <div class="card-head" @click="record.isMultiUser() && toggleExpanded(record.id)">
            <RightOutlined v-if="record.isMultiUser()" class="card-expand"
              :class="{ 'is-expanded': isExpanded(record.id) }" />
            <span class="card-id">#{{ record.id }}</span>
            <span class="tag-name">{{ record.remark }}</span>
            <div class="card-actions" @click.stop>
              <a-switch :checked="record.enable" size="small" @change="(next) => onSwitchEnable(record, next)" />
              <a-dropdown :trigger="['click']" placement="bottomRight">
                <MoreOutlined class="row-action-trigger" @click.prevent />
                <template #overlay>
                  <a-menu @click="(a) => emit('row-action', { key: a.key, dbInbound: record })">
                    <a-menu-item key="edit">
                      <EditOutlined /> {{ t('edit') }}
                    </a-menu-item>
                    <a-menu-item v-if="showQrCodeMenu(record)" key="qrcode">
                      <QrcodeOutlined /> {{ t('qrCode') }}
                    </a-menu-item>
                    <template v-if="record.isMultiUser()">
                      <a-menu-item key="addClient">
                        <UserAddOutlined /> {{ t('pages.client.add') }}
                      </a-menu-item>
                      <a-menu-item key="addBulkClient">
                        <UsergroupAddOutlined /> {{ t('pages.client.bulk') }}
                      </a-menu-item>
                      <a-menu-item key="copyClients">
                        <CopyOutlined /> {{ t('pages.client.copyFromInbound') }}
                      </a-menu-item>
                      <a-menu-item key="resetClients">
                        <FileDoneOutlined /> {{ t('pages.inbounds.resetInboundClientTraffics') }}
                      </a-menu-item>
                      <a-menu-item key="export">
                        <ExportOutlined /> {{ t('pages.inbounds.export') }}
                      </a-menu-item>
                      <a-menu-item v-if="subEnable" key="subs">
                        <ExportOutlined /> {{ t('pages.inbounds.export') }} — {{ t('pages.settings.subSettings') }}
                      </a-menu-item>
                      <a-menu-item key="delDepletedClients" class="danger-item">
                        <RestOutlined /> {{ t('pages.inbounds.delDepletedClients') }}
                      </a-menu-item>
                    </template>
                    <template v-else>
                      <a-menu-item key="showInfo">
                        <InfoCircleOutlined /> {{ t('info') }}
                      </a-menu-item>
                    </template>
                    <a-menu-item key="clipboard">
                      <CopyOutlined /> {{ t('pages.inbounds.exportInbound') }}
                    </a-menu-item>
                    <a-menu-item key="resetTraffic">
                      <RetweetOutlined /> {{ t('pages.inbounds.resetTraffic') }}
                    </a-menu-item>
                    <a-menu-item key="clone">
                      <BlockOutlined /> {{ t('pages.inbounds.clone') }}
                    </a-menu-item>
                    <a-menu-item key="delete" class="danger-item">
                      <DeleteOutlined /> {{ t('delete') }}
                    </a-menu-item>
                  </a-menu>
                </template>
              </a-dropdown>
            </div>
          </div>

          <!-- 2-column labelled stat grid: protocol/port/node + traffic/clients/expiry -->
          <div class="card-stats">
            <div class="stat-row">
              <span class="stat-label">{{ t('pages.inbounds.protocol') }}</span>
              <a-tag color="purple">{{ record.protocol }}</a-tag>
              <template v-if="record.isVMess || record.isVLess || record.isTrojan || record.isSS">
                <a-tag color="green">{{ record.toInbound().stream.network }}</a-tag>
                <a-tag v-if="record.toInbound().stream.isTls" color="blue">TLS</a-tag>
                <a-tag v-if="record.toInbound().stream.isReality" color="blue">Reality</a-tag>
              </template>
            </div>
            <div class="stat-row">
              <span class="stat-label">{{ t('pages.inbounds.port') }}</span>
              <a-tag>{{ record.port }}</a-tag>
            </div>
            <div v-if="nodesById.size > 0" class="stat-row">
              <span class="stat-label">{{ t('pages.inbounds.node') }}</span>
              <a-tag v-if="record.nodeId == null" color="default">
                {{ t('pages.inbounds.localPanel') }}
              </a-tag>
              <a-tag v-else-if="nodesById.get(record.nodeId)"
                :color="nodesById.get(record.nodeId).status === 'online' ? 'blue' : 'red'">
                {{ nodesById.get(record.nodeId).name }}
              </a-tag>
              <a-tag v-else color="orange">#{{ record.nodeId }}</a-tag>
            </div>
            <div class="stat-row">
              <span class="stat-label">{{ t('pages.inbounds.traffic') }}</span>
              <a-tag :color="ColorUtils.usageColor(record.up + record.down, trafficDiff, record.total)">
                {{ SizeFormatter.sizeFormat(record.up + record.down) }} /
                <template v-if="record.total > 0">{{ SizeFormatter.sizeFormat(record.total) }}</template>
                <InfinityIcon v-else />
              </a-tag>
            </div>
            <div class="stat-row">
              <span class="stat-label">{{ t('pages.inbounds.allTimeTraffic') }}</span>
              <a-tag>{{ SizeFormatter.sizeFormat(record.allTime || 0) }}</a-tag>
            </div>
            <div v-if="clientCount[record.id]" class="stat-row">
              <span class="stat-label">{{ t('clients') }}</span>
              <a-tag color="green">{{ clientCount[record.id].clients }}</a-tag>
              <a-tag v-if="clientCount[record.id].online.length" color="blue">
                {{ clientCount[record.id].online.length }} {{ t('online') }}
              </a-tag>
              <a-tag v-if="clientCount[record.id].depleted.length" color="red">
                {{ clientCount[record.id].depleted.length }} {{ t('depleted') }}
              </a-tag>
              <a-tag v-if="clientCount[record.id].expiring.length" color="orange">
                {{ clientCount[record.id].expiring.length }} {{ t('depletingSoon') }}
              </a-tag>
            </div>
            <div class="stat-row">
              <span class="stat-label">{{ t('pages.inbounds.expireDate') }}</span>
              <a-tag v-if="record.expiryTime > 0"
                :color="ColorUtils.usageColor(Date.now(), expireDiff, record._expiryTime)">
                {{ IntlUtil.formatRelativeTime(record.expiryTime) }}
              </a-tag>
              <a-tag v-else color="purple">
                <InfinityIcon />
              </a-tag>
            </div>
          </div>

          <!-- Expanded client list (multi-user only) -->
          <div v-if="record.isMultiUser() && isExpanded(record.id)" class="card-clients">
            <ClientRowTable :db-inbound="record" :is-mobile="true" :traffic-diff="trafficDiff" :expire-diff="expireDiff"
              :online-clients="onlineClients" :last-online-map="lastOnlineMap" :is-dark-theme="isDarkTheme"
              @edit-client="(p) => emit('edit-client', p)" @qrcode-client="(p) => emit('qrcode-client', p)"
              @info-client="(p) => emit('info-client', p)"
              @reset-traffic-client="(p) => emit('reset-traffic-client', p)"
              @delete-client="(p) => emit('delete-client', p)"
              @toggle-enable-client="(p) => emit('toggle-enable-client', p)" />
          </div>
        </div>
      </div>

      <!-- ====================== Desktop: a-table ======================== -->
      <a-table v-else :columns="columns" :data-source="visibleInbounds" :row-key="(r) => r.id"
        :pagination="paginationFor(visibleInbounds)" :scroll="{ x: 1000 }" :style="{ marginTop: '10px' }" size="small"
        :row-class-name="(r) => (r.isMultiUser() ? '' : 'hide-expand-icon')">
        <!-- Per-inbound client list, expanded by clicking the row's
             default expand chevron. Hidden via row-class-name for
             non-multi-user inbounds (matches legacy behavior). -->
        <template #expandedRowRender="{ record }">
          <ClientRowTable v-if="record.isMultiUser()" :db-inbound="record" :is-mobile="isMobile"
            :traffic-diff="trafficDiff" :expire-diff="expireDiff" :online-clients="onlineClients"
            :last-online-map="lastOnlineMap" :is-dark-theme="isDarkTheme" @edit-client="(p) => emit('edit-client', p)"
            @qrcode-client="(p) => emit('qrcode-client', p)" @info-client="(p) => emit('info-client', p)"
            @reset-traffic-client="(p) => emit('reset-traffic-client', p)"
            @delete-client="(p) => emit('delete-client', p)"
            @toggle-enable-client="(p) => emit('toggle-enable-client', p)" />
        </template>

        <template #bodyCell="{ column, record }">
          <!-- ============== Action dropdown ============== -->
          <template v-if="column.key === 'action'">
            <a-dropdown :trigger="['click']">
              <MoreOutlined class="row-action-trigger" @click.prevent />
              <template #overlay>
                <a-menu @click="(a) => emit('row-action', { key: a.key, dbInbound: record })">
                  <a-menu-item key="edit">
                    <EditOutlined /> {{ t('edit') }}
                  </a-menu-item>
                  <a-menu-item v-if="showQrCodeMenu(record)" key="qrcode">
                    <QrcodeOutlined /> {{ t('qrCode') }}
                  </a-menu-item>
                  <template v-if="record.isMultiUser()">
                    <a-menu-item key="addClient">
                      <UserAddOutlined /> {{ t('pages.client.add') }}
                    </a-menu-item>
                    <a-menu-item key="addBulkClient">
                      <UsergroupAddOutlined /> {{ t('pages.client.bulk') }}
                    </a-menu-item>
                    <a-menu-item key="copyClients">
                      <CopyOutlined /> {{ t('pages.client.copyFromInbound') }}
                    </a-menu-item>
                    <a-menu-item key="resetClients">
                      <FileDoneOutlined /> {{ t('pages.inbounds.resetInboundClientTraffics') }}
                    </a-menu-item>
                    <a-menu-item key="export">
                      <ExportOutlined /> {{ t('pages.inbounds.export') }}
                    </a-menu-item>
                    <a-menu-item v-if="subEnable" key="subs">
                      <ExportOutlined /> {{ t('pages.inbounds.export') }} — {{ t('pages.settings.subSettings') }}
                    </a-menu-item>
                    <a-menu-item key="delDepletedClients" class="danger-item">
                      <RestOutlined /> {{ t('pages.inbounds.delDepletedClients') }}
                    </a-menu-item>
                  </template>
                  <template v-else>
                    <a-menu-item key="showInfo">
                      <InfoCircleOutlined /> {{ t('info') }}
                    </a-menu-item>
                  </template>
                  <a-menu-item key="clipboard">
                    <CopyOutlined /> {{ t('pages.inbounds.exportInbound') }}
                  </a-menu-item>
                  <a-menu-item key="resetTraffic">
                    <RetweetOutlined /> {{ t('pages.inbounds.resetTraffic') }}
                  </a-menu-item>
                  <a-menu-item key="clone">
                    <BlockOutlined /> {{ t('pages.inbounds.clone') }}
                  </a-menu-item>
                  <a-menu-item key="delete" class="danger-item">
                    <DeleteOutlined /> {{ t('delete') }}
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </template>

          <!-- ============== Enable switch (desktop) ============== -->
          <template v-else-if="column.key === 'enable'">
            <a-switch :checked="record.enable" @change="(next) => onSwitchEnable(record, next)" />
          </template>

          <!-- ============== Node deployment tag ============== -->
          <template v-else-if="column.key === 'node'">
            <template v-if="record.nodeId == null">
              <a-tag color="default">{{ t('pages.inbounds.localPanel') }}</a-tag>
            </template>
            <template v-else-if="nodesById.get(record.nodeId)">
              <a-tag :color="nodesById.get(record.nodeId).status === 'online' ? 'blue' : 'red'">
                {{ nodesById.get(record.nodeId).name }}
              </a-tag>
            </template>
            <template v-else>
              <!-- Node row was deleted but inbound still references it. -->
              <a-tag color="orange">node #{{ record.nodeId }}</a-tag>
            </template>
          </template>

          <!-- ============== Protocol tags ============== -->
          <template v-else-if="column.key === 'protocol'">
            <div class="protocol-tags">
              <a-tag color="purple">{{ record.protocol }}</a-tag>
              <template v-if="record.isVMess || record.isVLess || record.isTrojan || record.isSS">
                <a-tag color="green">{{ record.toInbound().stream.network }}</a-tag>
                <a-tag v-if="record.toInbound().stream.isTls" color="blue">TLS</a-tag>
                <a-tag v-if="record.toInbound().stream.isReality" color="blue">Reality</a-tag>
              </template>
            </div>
          </template>

          <!-- ============== Clients tag + popovers ============== -->
          <template v-else-if="column.key === 'clients'">
            <template v-if="clientCount[record.id]">
              <a-tag color="green" style="margin: 0">{{ clientCount[record.id].clients }}</a-tag>
              <a-popover v-if="clientCount[record.id].deactive.length" :title="t('disabled')">
                <template #content>
                  <div v-for="email in clientCount[record.id].deactive" :key="email">{{ email }}</div>
                </template>
                <a-tag style="margin: 0; padding: 0 2px">{{ clientCount[record.id].deactive.length }}</a-tag>
              </a-popover>
              <a-popover v-if="clientCount[record.id].depleted.length" :title="t('depleted')">
                <template #content>
                  <div v-for="email in clientCount[record.id].depleted" :key="email">{{ email }}</div>
                </template>
                <a-tag color="red" style="margin: 0; padding: 0 2px">{{ clientCount[record.id].depleted.length
                }}</a-tag>
              </a-popover>
              <a-popover v-if="clientCount[record.id].expiring.length" :title="t('depletingSoon')">
                <template #content>
                  <div v-for="email in clientCount[record.id].expiring" :key="email">{{ email }}</div>
                </template>
                <a-tag color="orange" style="margin: 0; padding: 0 2px">{{ clientCount[record.id].expiring.length
                }}</a-tag>
              </a-popover>
              <a-popover v-if="clientCount[record.id].online.length" :title="t('online')">
                <template #content>
                  <div v-for="email in clientCount[record.id].online" :key="email">{{ email }}</div>
                </template>
                <a-tag color="blue" style="margin: 0; padding: 0 2px">{{ clientCount[record.id].online.length }}</a-tag>
              </a-popover>
            </template>
          </template>

          <!-- ============== Traffic ============== -->
          <template v-else-if="column.key === 'traffic'">
            <a-popover>
              <template #content>
                <table cellpadding="2">
                  <tbody>
                    <tr>
                      <td>↑ {{ SizeFormatter.sizeFormat(record.up) }}</td>
                      <td>↓ {{ SizeFormatter.sizeFormat(record.down) }}</td>
                    </tr>
                    <tr v-if="record.total > 0 && record.up + record.down < record.total">
                      <td>{{ t('remained') }}</td>
                      <td>{{ SizeFormatter.sizeFormat(record.total - record.up - record.down) }}</td>
                    </tr>
                  </tbody>
                </table>
              </template>
              <a-tag :color="ColorUtils.usageColor(record.up + record.down, trafficDiff, record.total)">
                {{ SizeFormatter.sizeFormat(record.up + record.down) }} /
                <template v-if="record.total > 0">{{ SizeFormatter.sizeFormat(record.total) }}</template>
                <InfinityIcon v-else />
              </a-tag>
            </a-popover>
          </template>

          <!-- ============== All-time inbound traffic ============== -->
          <template v-else-if="column.key === 'allTimeInbound'">
            <a-tag>{{ SizeFormatter.sizeFormat(record.allTime || 0) }}</a-tag>
          </template>

          <!-- ============== Expiry ============== -->
          <template v-else-if="column.key === 'expiryTime'">
            <a-popover v-if="record.expiryTime > 0">
              <template #content>{{ IntlUtil.formatDate(record.expiryTime, datepicker) }}</template>
              <a-tag :color="ColorUtils.usageColor(Date.now(), expireDiff, record._expiryTime)" style="min-width: 50px">
                {{ IntlUtil.formatRelativeTime(record.expiryTime) }}
              </a-tag>
            </a-popover>
            <a-tag v-else color="purple">
              <InfinityIcon />
            </a-tag>
          </template>

        </template>
      </a-table>
    </a-space>
  </a-card>
</template>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-bar.mobile {
  display: block;
}

.filter-bar.mobile>* {
  margin-bottom: 4px;
}

.protocol-tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}

.row-action-trigger {
  font-size: 20px;
  cursor: pointer;
}

.danger-item {
  color: #ff4d4f;
}

/* Hide the expand chevron on rows whose inbound has no client list
 * (HTTP/Mixed/Tunnel/WireGuard single-config). */
:deep(.hide-expand-icon .ant-table-row-expand-icon) {
  visibility: hidden;
}

/* Push the expand chevron away from the table's left edge so it has
 * a little breathing room instead of being flush against the corner. */
:deep(.ant-table-tbody .ant-table-cell-with-append) {
  padding-left: 12px;
}

:deep(.ant-table-row-expand-icon) {
  margin-inline-end: 10px;
  margin-inline-start: 4px;
}

/* Round the table's outer corners — AD-Vue gives .ant-table the radius
 * token, but the inner header strip and footer touch the edges, so clip
 * them here. */
:deep(.ant-table) {
  border-radius: 8px;
  overflow: hidden;
}

:deep(.ant-table-container) {
  border-radius: 8px;
  overflow: hidden;
}

:deep(.ant-table-thead > tr:first-child > *:first-child) {
  border-start-start-radius: 8px;
}

:deep(.ant-table-thead > tr:first-child > *:last-child) {
  border-start-end-radius: 8px;
}

:deep(.ant-table-tbody > tr:last-child > *:first-child) {
  border-end-start-radius: 8px;
}

:deep(.ant-table-tbody > tr:last-child > *:last-child) {
  border-end-end-radius: 8px;
}

/* ===== Mobile card list ===========================================
 * <768px renders inbounds as a vertical stack of cards via the
 * v-if="isMobile" branch above; the desktop <a-table> isn't mounted
 * so the legacy table-cell tightening rules went away. */
.inbound-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 4px;
}

.inbound-card {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 10px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.02);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

:global(body.dark) .inbound-card {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
}

.card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.card-id {
  font-size: 11px;
  opacity: 0.6;
}

.tag-name {
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
  gap: 8px;
  flex-shrink: 0;
}

.card-expand {
  font-size: 12px;
  opacity: 0.6;
  transition: transform 150ms ease;
  flex-shrink: 0;
}

.card-expand.is-expanded {
  transform: rotate(90deg);
}

.card-stats {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.stat-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.stat-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.6;
  min-width: 96px;
  flex-shrink: 0;
}

.card-stats :deep(.ant-tag) {
  margin: 0;
}

.card-clients {
  margin-top: 4px;
  padding-top: 8px;
  border-top: 1px solid rgba(128, 128, 128, 0.15);
}

.card-empty {
  text-align: center;
  opacity: 0.4;
  padding: 20px 0;
}

@media (max-width: 768px) {
  :deep(.ant-card-head) {
    padding: 0 12px;
    min-height: 44px;
  }

  :deep(.ant-card-head-title),
  :deep(.ant-card-extra) {
    padding: 8px 0;
  }

  :deep(.ant-card-body) {
    padding: 8px;
  }

  .filter-bar.mobile {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .filter-bar.mobile>* {
    margin-bottom: 0;
  }

  .row-action-trigger {
    font-size: 22px;
    padding: 4px;
  }
}
</style>
