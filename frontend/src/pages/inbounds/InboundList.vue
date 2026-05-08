<script setup>
import { computed, ref, watch } from 'vue';
import {
  PlusOutlined,
  MenuOutlined,
  SyncOutlined,
  DownOutlined,
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
} from '@ant-design/icons-vue';

import { HttpUtil, ObjectUtil, SizeFormatter, IntlUtil, ColorUtils } from '@/utils';
import { DBInbound } from '@/models/dbinbound.js';
import { Inbound } from '@/models/inbound.js';

const props = defineProps({
  dbInbounds: { type: Array, required: true },
  clientCount: { type: Object, required: true },
  onlineClients: { type: Array, required: true },
  refreshing: { type: Boolean, default: false },
  expireDiff: { type: Number, default: 0 },
  trafficDiff: { type: Number, default: 0 },
  pageSize: { type: Number, default: 0 },
  isMobile: { type: Boolean, default: false },
  subEnable: { type: Boolean, default: false },
});

const emit = defineEmits([
  'refresh',
  'add-inbound',
  'general-action',
  'row-action',
]);

// ============ Toolbar / search & filter =============================
const enableFilter = ref(false);
const searchKey = ref('');
const filterBy = ref('');

// Auto-refresh — same defaults as legacy (5s, opt-in via switch).
const isRefreshEnabled = ref(localStorage.getItem('isRefreshEnabled') === 'true');
const refreshIntervalMs = ref(Number(localStorage.getItem('refreshInterval')) || 5000);

let timer = null;
function startAutoRefresh() {
  stopAutoRefresh();
  timer = setInterval(() => emit('refresh'), refreshIntervalMs.value);
}
function stopAutoRefresh() {
  if (timer != null) {
    clearInterval(timer);
    timer = null;
  }
}
watch(isRefreshEnabled, (next) => {
  localStorage.setItem('isRefreshEnabled', String(next));
  if (next) startAutoRefresh();
  else stopAutoRefresh();
}, { immediate: true });
watch(refreshIntervalMs, (next) => {
  localStorage.setItem('refreshInterval', String(next));
  if (isRefreshEnabled.value) startAutoRefresh();
});

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
  let settings = {};
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
// `responsive` array still works on column defs.
const desktopColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', align: 'right', width: 30, responsive: ['xs'] },
  { title: 'Action', key: 'action', align: 'center', width: 30 },
  { title: 'Enable', key: 'enable', align: 'center', width: 35 },
  { title: 'Remark', dataIndex: 'remark', key: 'remark', align: 'center', width: 60 },
  { title: 'Port', dataIndex: 'port', key: 'port', align: 'center', width: 40 },
  { title: 'Protocol', key: 'protocol', align: 'left', width: 70 },
  { title: 'Clients', key: 'clients', align: 'left', width: 50 },
  { title: 'Traffic', key: 'traffic', align: 'center', width: 90 },
  { title: 'All-time', key: 'allTimeInbound', align: 'center', width: 60 },
  { title: 'Expiry', key: 'expiryTime', align: 'center', width: 40 },
];
const mobileColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', align: 'right', width: 10, responsive: ['s'] },
  { title: 'Action', key: 'action', align: 'center', width: 25 },
  { title: 'Remark', dataIndex: 'remark', key: 'remark', align: 'left', width: 70 },
  { title: 'Info', key: 'info', align: 'center', width: 10 },
];
const columns = computed(() => (props.isMobile ? mobileColumns : desktopColumns));

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
function isClientOnline(email) {
  return props.onlineClients.includes(email);
}

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
          <template #icon><PlusOutlined /></template>
          <template v-if="!isMobile">Add inbound</template>
        </a-button>
        <a-dropdown :trigger="['click']">
          <a-button type="primary">
            <template #icon><MenuOutlined /></template>
            <template v-if="!isMobile">General actions</template>
          </a-button>
          <template #overlay>
            <a-menu @click="(a) => emit('general-action', a.key)">
              <a-menu-item key="import">
                <ImportOutlined /> Import inbound
              </a-menu-item>
              <a-menu-item key="export">
                <ExportOutlined /> Export
              </a-menu-item>
              <a-menu-item v-if="subEnable" key="subs">
                <ExportOutlined /> Export — Subscription
              </a-menu-item>
              <a-menu-item key="resetInbounds">
                <ReloadOutlined /> Reset all traffic
              </a-menu-item>
              <a-menu-item key="resetClients">
                <FileDoneOutlined /> Reset all client traffic
              </a-menu-item>
              <a-menu-item key="delDepletedClients" class="danger-item">
                <RestOutlined /> Delete depleted clients
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-space>
    </template>

    <template #extra>
      <a-button-group>
        <a-button :loading="refreshing" @click="emit('refresh')">
          <template #icon><SyncOutlined /></template>
        </a-button>
        <a-popover placement="bottomRight" trigger="click">
          <template #title>
            <div class="auto-refresh-title">
              <a-switch v-model:checked="isRefreshEnabled" size="small" />
              <span>Auto refresh</span>
            </div>
          </template>
          <template #content>
            <a-space direction="vertical">
              <span>Auto-refresh interval</span>
              <a-select
                v-model:value="refreshIntervalMs"
                :disabled="!isRefreshEnabled"
                :style="{ width: '100%' }"
              >
                <a-select-option v-for="key in [5, 10, 30, 60]" :key="key" :value="key * 1000">
                  {{ key }}s
                </a-select-option>
              </a-select>
            </a-space>
          </template>
          <a-button>
            <template #icon><DownOutlined /></template>
          </a-button>
        </a-popover>
      </a-button-group>
    </template>

    <a-space direction="vertical" :style="{ width: '100%' }">
      <!-- Search / filter toolbar -->
      <div :class="isMobile ? 'filter-bar mobile' : 'filter-bar'">
        <a-switch v-model:checked="enableFilter" @change="onToggleFilter">
          <template #checkedChildren><SearchOutlined /></template>
          <template #unCheckedChildren><FilterOutlined /></template>
        </a-switch>
        <a-input
          v-if="!enableFilter"
          v-model:value="searchKey"
          placeholder="Search"
          autofocus
          :size="isMobile ? 'small' : 'middle'"
          :style="{ maxWidth: '300px' }"
        />
        <a-radio-group
          v-if="enableFilter"
          v-model:value="filterBy"
          button-style="solid"
          :size="isMobile ? 'small' : 'middle'"
        >
          <a-radio-button value="">None</a-radio-button>
          <a-radio-button value="active">Active</a-radio-button>
          <a-radio-button value="deactive">Disabled</a-radio-button>
          <a-radio-button value="depleted">Depleted</a-radio-button>
          <a-radio-button value="expiring">Depleting</a-radio-button>
          <a-radio-button value="online">Online</a-radio-button>
        </a-radio-group>
      </div>

      <a-table
        :columns="columns"
        :data-source="visibleInbounds"
        :row-key="(r) => r.id"
        :pagination="paginationFor(visibleInbounds)"
        :scroll="isMobile ? {} : { x: 1000 }"
        :style="{ marginTop: '10px' }"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <!-- ============== Action dropdown ============== -->
          <template v-if="column.key === 'action'">
            <a-dropdown :trigger="['click']">
              <MoreOutlined class="row-action-trigger" @click.prevent />
              <template #overlay>
                <a-menu @click="(a) => emit('row-action', { key: a.key, dbInbound: record })">
                  <a-menu-item key="edit"><EditOutlined /> Edit</a-menu-item>
                  <a-menu-item v-if="showQrCodeMenu(record)" key="qrcode">
                    <QrcodeOutlined /> QR code
                  </a-menu-item>
                  <template v-if="record.isMultiUser()">
                    <a-menu-item key="addClient"><UserAddOutlined /> Add client</a-menu-item>
                    <a-menu-item key="addBulkClient"><UsergroupAddOutlined /> Add bulk clients</a-menu-item>
                    <a-menu-item key="copyClients"><CopyOutlined /> Copy clients from inbound</a-menu-item>
                    <a-menu-item key="resetClients"><FileDoneOutlined /> Reset client traffic</a-menu-item>
                    <a-menu-item key="export"><ExportOutlined /> Export</a-menu-item>
                    <a-menu-item v-if="subEnable" key="subs">
                      <ExportOutlined /> Export — Subscription
                    </a-menu-item>
                    <a-menu-item key="delDepletedClients" class="danger-item">
                      <RestOutlined /> Delete depleted clients
                    </a-menu-item>
                  </template>
                  <template v-else>
                    <a-menu-item key="showInfo"><InfoCircleOutlined /> Info</a-menu-item>
                  </template>
                  <a-menu-item key="clipboard"><CopyOutlined /> Export inbound</a-menu-item>
                  <a-menu-item key="resetTraffic"><RetweetOutlined /> Reset traffic</a-menu-item>
                  <a-menu-item key="clone"><BlockOutlined /> Clone</a-menu-item>
                  <a-menu-item key="delete" class="danger-item">
                    <DeleteOutlined /> Delete
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </template>

          <!-- ============== Enable switch (desktop) ============== -->
          <template v-else-if="column.key === 'enable'">
            <a-switch
              :checked="record.enable"
              @change="(next) => onSwitchEnable(record, next)"
            />
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
              <a-popover v-if="clientCount[record.id].deactive.length" title="Disabled">
                <template #content>
                  <div v-for="email in clientCount[record.id].deactive" :key="email">{{ email }}</div>
                </template>
                <a-tag style="margin: 0; padding: 0 2px">{{ clientCount[record.id].deactive.length }}</a-tag>
              </a-popover>
              <a-popover v-if="clientCount[record.id].depleted.length" title="Depleted">
                <template #content>
                  <div v-for="email in clientCount[record.id].depleted" :key="email">{{ email }}</div>
                </template>
                <a-tag color="red" style="margin: 0; padding: 0 2px">{{ clientCount[record.id].depleted.length }}</a-tag>
              </a-popover>
              <a-popover v-if="clientCount[record.id].expiring.length" title="Depleting soon">
                <template #content>
                  <div v-for="email in clientCount[record.id].expiring" :key="email">{{ email }}</div>
                </template>
                <a-tag color="orange" style="margin: 0; padding: 0 2px">{{ clientCount[record.id].expiring.length }}</a-tag>
              </a-popover>
              <a-popover v-if="clientCount[record.id].online.length" title="Online">
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
                      <td>Remaining</td>
                      <td>{{ SizeFormatter.sizeFormat(record.total - record.up - record.down) }}</td>
                    </tr>
                  </tbody>
                </table>
              </template>
              <a-tag :color="ColorUtils.usageColor(record.up + record.down, trafficDiff, record.total)">
                {{ SizeFormatter.sizeFormat(record.up + record.down) }} /
                <template v-if="record.total > 0">{{ SizeFormatter.sizeFormat(record.total) }}</template>
                <template v-else>∞</template>
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
              <template #content>{{ IntlUtil.formatDate(record.expiryTime) }}</template>
              <a-tag
                :color="ColorUtils.usageColor(Date.now(), expireDiff, record._expiryTime)"
                style="min-width: 50px"
              >
                {{ IntlUtil.formatRelativeTime(record.expiryTime) }}
              </a-tag>
            </a-popover>
            <a-tag v-else color="purple">∞</a-tag>
          </template>

          <!-- ============== Mobile info popover ============== -->
          <template v-else-if="column.key === 'info'">
            <a-popover placement="bottomRight" trigger="click">
              <template #content>
                <table cellpadding="2">
                  <tbody>
                    <tr>
                      <td>Protocol</td>
                      <td><a-tag color="purple">{{ record.protocol }}</a-tag></td>
                    </tr>
                    <tr>
                      <td>Port</td>
                      <td><a-tag>{{ record.port }}</a-tag></td>
                    </tr>
                    <tr v-if="clientCount[record.id]">
                      <td>Clients</td>
                      <td><a-tag color="blue">{{ clientCount[record.id].clients }}</a-tag></td>
                    </tr>
                    <tr>
                      <td>Traffic</td>
                      <td>
                        <a-tag>
                          {{ SizeFormatter.sizeFormat(record.up + record.down) }} /
                          <template v-if="record.total > 0">{{ SizeFormatter.sizeFormat(record.total) }}</template>
                          <template v-else>∞</template>
                        </a-tag>
                      </td>
                    </tr>
                    <tr>
                      <td>Expiry</td>
                      <td>
                        <a-tag v-if="record.expiryTime > 0">{{ IntlUtil.formatRelativeTime(record.expiryTime) }}</a-tag>
                        <a-tag v-else color="purple">∞</a-tag>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </template>
              <InfoCircleOutlined class="row-info-trigger" />
            </a-popover>
          </template>
        </template>
      </a-table>
    </a-space>
  </a-card>
</template>

<style scoped>
.auto-refresh-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.filter-bar.mobile {
  display: block;
}
.filter-bar.mobile > * {
  margin-bottom: 4px;
}

.protocol-tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}

.row-action-trigger,
.row-info-trigger {
  font-size: 20px;
  cursor: pointer;
}

.danger-item {
  color: #ff4d4f;
}
</style>
