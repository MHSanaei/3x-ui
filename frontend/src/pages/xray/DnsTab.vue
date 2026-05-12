<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Modal } from 'ant-design-vue';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  MenuOutlined,
} from '@ant-design/icons-vue';

import SettingListItem from '@/components/SettingListItem.vue';
import DnsServerModal from './DnsServerModal.vue';
import DnsPresetsModal from './DnsPresetsModal.vue';

const { t } = useI18n();

const props = defineProps({
  templateSettings: { type: Object, default: null },
});

const STRATEGIES = ['UseSystem', 'UseIP', 'UseIPv4', 'UseIPv6'];

const dnsFieldOmit = Object.freeze(Object.create(null));

function dnsValueEmptyForOmit(v) {
  if (v === undefined || v === null) return true;
  if (typeof v === 'string') return v.trim() === '';
  return false;
}

const enableDNS = computed({
  get: () => !!props.templateSettings?.dns,
  set: (next) => {
    if (!props.templateSettings) return;
    if (next) {
      props.templateSettings.dns = {
        tag: 'dns_inbound',
        queryStrategy: 'UseIP',
        disableCache: false,
        disableFallback: false,
        disableFallbackIfMatch: false,
        useSystemHosts: false,
        enableParallelQuery: false,
        serveStale: false,
        serveExpiredTTL: 0,
        hosts: {},
        servers: [],
      };
      props.templateSettings.fakedns = null;
    } else {
      delete props.templateSettings.dns;
      delete props.templateSettings.fakedns;
    }
  },
});

function dnsField(field, fallback) {
  const omitWhenUnset = fallback === dnsFieldOmit;
  return computed({
    get: () => {
      const raw = props.templateSettings?.dns?.[field];
      if (fallback === dnsFieldOmit) return raw ?? '';
      return raw ?? fallback;
    },
    set: (v) => {
      if (!props.templateSettings?.dns) return;
      if (omitWhenUnset) {
        if (dnsValueEmptyForOmit(v)) {
          if (field in props.templateSettings.dns) delete props.templateSettings.dns[field];
        } else {
          props.templateSettings.dns[field] = v;
        }
      } else {
        props.templateSettings.dns[field] = v;
      }
    },
  });
}

const dnsTag = dnsField('tag', 'dns_inbound');
const dnsClientIp = dnsField('clientIp', dnsFieldOmit);
const dnsStrategy = dnsField('queryStrategy', 'UseIP');
const dnsDisableCache = dnsField('disableCache', false);
const dnsDisableFallback = dnsField('disableFallback', false);
const dnsDisableFallbackIfMatch = dnsField('disableFallbackIfMatch', false);
const dnsEnableParallelQuery = dnsField('enableParallelQuery', false);
const dnsUseSystemHosts = dnsField('useSystemHosts', false);
const dnsServeStale = dnsField('serveStale', false);
const dnsServeExpiredTTL = dnsField('serveExpiredTTL', 0);

const hostsList = ref([]);

function hydrateHostsFromBackend() {
  const src = props.templateSettings?.dns?.hosts || {};
  hostsList.value = Object.entries(src).map(([domain, val]) => ({
    domain,
    values: Array.isArray(val) ? [...val] : [String(val)],
  }));
}

function syncHostsToBackend() {
  if (!props.templateSettings?.dns) return;
  const obj = {};
  for (const row of hostsList.value) {
    if (!row.domain) continue;
    const vals = (row.values || []).filter(Boolean);
    if (vals.length === 0) continue;
    obj[row.domain] = vals.length === 1 ? vals[0] : vals;
  }
  if (Object.keys(obj).length > 0) {
    props.templateSettings.dns.hosts = obj;
  } else if ('hosts' in props.templateSettings.dns) {
    delete props.templateSettings.dns.hosts;
  }
}

watch(
  () => !!props.templateSettings?.dns,
  (enabled) => {
    if (enabled) hydrateHostsFromBackend();
    else hostsList.value = [];
  },
  { immediate: true },
);

watch(hostsList, syncHostsToBackend, { deep: true });

function addHost() {
  hostsList.value.push({ domain: '', values: [] });
}
function deleteHost(idx) {
  hostsList.value.splice(idx, 1);
}

const dnsServers = computed(() => {
  const list = props.templateSettings?.dns?.servers || [];
  return list.map((s, idx) => ({ key: idx, server: s }));
});

const dnsColumns = computed(() => [
  { title: '#', key: 'action', align: 'center', width: 60 },
  { title: t('pages.inbounds.address'), key: 'address', align: 'left' },
  { title: t('pages.xray.dns.domains'), key: 'domains', align: 'left' },
  { title: t('pages.xray.dns.expectIPs'), key: 'expectedIPs', align: 'left' },
]);

function addrFor(server) {
  return typeof server === 'string' ? server : server?.address || '';
}
function domainsFor(server) {
  return typeof server === 'object' ? (server.domains || []).join(',') : '';
}
function expectedIPsFor(server) {
  if (typeof server !== 'object' || !server) return '';
  const list = server.expectedIPs || server.expectIPs || [];
  return Array.isArray(list) ? list.join(',') : '';
}

// ============== Server modal ==============
const serverModalOpen = ref(false);
const editingServer = ref(null);
const editingIndex = ref(null);

function openAddServer() {
  editingServer.value = null;
  editingIndex.value = null;
  serverModalOpen.value = true;
}
function openEditServer(idx) {
  editingServer.value = props.templateSettings.dns.servers[idx];
  editingIndex.value = idx;
  serverModalOpen.value = true;
}
function onServerConfirm(value) {
  if (!props.templateSettings?.dns) return;
  if (!Array.isArray(props.templateSettings.dns.servers)) {
    props.templateSettings.dns.servers = [];
  }
  if (editingIndex.value == null) {
    props.templateSettings.dns.servers.push(value);
  } else {
    props.templateSettings.dns.servers[editingIndex.value] = value;
  }
  serverModalOpen.value = false;
}
function deleteServer(idx) {
  props.templateSettings.dns.servers.splice(idx, 1);
}
function clearAllServers() {
  if (!props.templateSettings?.dns) return;
  Modal.confirm({
    title: t('pages.xray.dns.clearAllTitle'),
    content: t('pages.xray.dns.clearAllConfirm'),
    okText: t('delete'),
    okButtonProps: { danger: true },
    cancelText: t('cancel'),
    onOk() {
      props.templateSettings.dns.servers = [];
    },
  });
}

const presetsModalOpen = ref(false);
function openPresets() { presetsModalOpen.value = true; }
function onPresetInstall(serverList) {
  if (!props.templateSettings?.dns) return;
  props.templateSettings.dns.servers = serverList;
  presetsModalOpen.value = false;
}

// ============== Fake DNS table ==============
const DEFAULT_FAKEDNS = () => ({ ipPool: '198.18.0.0/15', poolSize: 65535 });

const fakeDnsList = computed(() => {
  const list = Array.isArray(props.templateSettings?.fakedns)
    ? props.templateSettings.fakedns
    : [];
  return list.map((entry, idx) => ({ key: idx, ...entry }));
});

const fakednsColumns = computed(() => [
  { title: '#', key: 'action', align: 'center', width: 60 },
  { title: 'IP pool', dataIndex: 'ipPool', key: 'ipPool', align: 'left' },
  { title: 'Pool size', dataIndex: 'poolSize', key: 'poolSize', align: 'right', width: 120 },
]);

function addFakedns() {
  if (!props.templateSettings) return;
  if (!Array.isArray(props.templateSettings.fakedns)) {
    props.templateSettings.fakedns = [];
  }
  props.templateSettings.fakedns.push(DEFAULT_FAKEDNS());
}
function deleteFakedns(idx) {
  props.templateSettings.fakedns.splice(idx, 1);
  if (props.templateSettings.fakedns.length === 0) {
    props.templateSettings.fakedns = null;
  }
}
function updateFakednsField(idx, field, value) {
  if (!props.templateSettings.fakedns?.[idx]) return;
  props.templateSettings.fakedns[idx] = {
    ...props.templateSettings.fakedns[idx],
    [field]: value,
  };
}
</script>

<template>
  <a-collapse default-active-key="1">
    <!-- ============== General DNS settings ============== -->
    <a-collapse-panel key="1" :header="t('pages.xray.generalConfigs')">
      <SettingListItem paddings="small">
        <template #title>{{ t('pages.xray.dns.enable') }}</template>
        <template #description>{{ t('pages.xray.dns.enableDesc') }}</template>
        <template #control>
          <a-switch v-model:checked="enableDNS" />
        </template>
      </SettingListItem>

      <template v-if="enableDNS">
        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.tag') }}</template>
          <template #description>{{ t('pages.xray.dns.tagDesc') }}</template>
          <template #control>
            <a-input v-model:value="dnsTag" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.clientIp') }}</template>
          <template #description>{{ t('pages.xray.dns.clientIpDesc') }}</template>
          <template #control>
            <a-input v-model:value="dnsClientIp" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.strategy') }}</template>
          <template #description>{{ t('pages.xray.dns.strategyDesc') }}</template>
          <template #control>
            <a-select v-model:value="dnsStrategy" :style="{ width: '100%' }">
              <a-select-option v-for="s in STRATEGIES" :key="s" :value="s">{{ s }}</a-select-option>
            </a-select>
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.disableCache') }}</template>
          <template #description>{{ t('pages.xray.dns.disableCacheDesc') }}</template>
          <template #control>
            <a-switch v-model:checked="dnsDisableCache" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.disableFallback') }}</template>
          <template #description>{{ t('pages.xray.dns.disableFallbackDesc') }}</template>
          <template #control>
            <a-switch v-model:checked="dnsDisableFallback" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.disableFallbackIfMatch') }}</template>
          <template #description>{{ t('pages.xray.dns.disableFallbackIfMatchDesc') }}</template>
          <template #control>
            <a-switch v-model:checked="dnsDisableFallbackIfMatch" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.enableParallelQuery') }}</template>
          <template #description>{{ t('pages.xray.dns.enableParallelQueryDesc') }}</template>
          <template #control>
            <a-switch v-model:checked="dnsEnableParallelQuery" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.useSystemHosts') }}</template>
          <template #description>{{ t('pages.xray.dns.useSystemHostsDesc') }}</template>
          <template #control>
            <a-switch v-model:checked="dnsUseSystemHosts" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.serveStale') }}</template>
          <template #description>{{ t('pages.xray.dns.serveStaleDesc') }}</template>
          <template #control>
            <a-switch v-model:checked="dnsServeStale" />
          </template>
        </SettingListItem>

        <SettingListItem paddings="small">
          <template #title>{{ t('pages.xray.dns.serveExpiredTTL') }}</template>
          <template #description>{{ t('pages.xray.dns.serveExpiredTTLDesc') }}</template>
          <template #control>
            <a-input-number v-model:value="dnsServeExpiredTTL" :min="0" :step="60" :style="{ width: '100%' }" />
          </template>
        </SettingListItem>
      </template>
    </a-collapse-panel>

    <!-- ============== Hosts ============== -->
    <a-collapse-panel v-if="enableDNS" key="hosts" :header="t('pages.xray.dns.hosts')">
      <a-empty v-if="hostsList.length === 0" :description="t('pages.xray.dns.hostsEmpty')">
        <a-button type="primary" @click="addHost">
          <template #icon>
            <PlusOutlined />
          </template>
          {{ t('pages.xray.dns.hostsAdd') }}
        </a-button>
      </a-empty>

      <template v-else>
        <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
          <a-button type="primary" @click="addHost">
            <template #icon>
              <PlusOutlined />
            </template>
            {{ t('pages.xray.dns.hostsAdd') }}
          </a-button>
          <div v-for="(row, idx) in hostsList" :key="`h${idx}`" class="hosts-row">
            <a-input v-model:value="row.domain" :placeholder="t('pages.xray.dns.hostsDomain')"
              :style="{ flex: '1 1 220px' }" />
            <a-select v-model:value="row.values" mode="tags" :placeholder="t('pages.xray.dns.hostsValues')"
              :style="{ flex: '2 1 320px' }" :token-separators="[',', ' ']" />
            <a-button danger @click="deleteHost(idx)">
              <template #icon>
                <DeleteOutlined />
              </template>
            </a-button>
          </div>
        </a-space>
      </template>
    </a-collapse-panel>

    <!-- ============== DNS servers ============== -->
    <a-collapse-panel v-if="enableDNS" key="2" header="DNS">
      <a-empty v-if="dnsServers.length === 0" :description="t('emptyDnsDesc')">
        <a-space>
          <a-button type="primary" @click="openAddServer">
            <template #icon>
              <PlusOutlined />
            </template>
            {{ t('pages.xray.dns.add') }}
          </a-button>
          <a-button @click="openPresets">
            <template #icon>
              <MenuOutlined />
            </template>
            {{ t('pages.xray.dns.usePreset') }}
          </a-button>
        </a-space>
      </a-empty>

      <template v-else>
        <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
          <a-space wrap>
            <a-button type="primary" @click="openAddServer">
              <template #icon>
                <PlusOutlined />
              </template>
              {{ t('pages.xray.dns.add') }}
            </a-button>
            <a-button @click="openPresets">
              <template #icon>
                <MenuOutlined />
              </template>
              {{ t('pages.xray.dns.usePreset') }}
            </a-button>
            <a-button danger @click="clearAllServers">
              <template #icon>
                <DeleteOutlined />
              </template>
              {{ t('pages.xray.dns.clearAll') }}
            </a-button>
          </a-space>
          <a-table :columns="dnsColumns" :data-source="dnsServers" :row-key="(r) => r.key" :pagination="false"
            size="small" bordered>
            <template #bodyCell="{ column, record, index }">
              <template v-if="column.key === 'action'">
                <a-space :size="6">
                  <span class="row-index">{{ index + 1 }}</span>
                  <a-dropdown :trigger="['click']">
                    <a-button shape="circle" size="small">
                      <MoreOutlined />
                    </a-button>
                    <template #overlay>
                      <a-menu>
                        <a-menu-item @click="openEditServer(index)">
                          <EditOutlined /> {{ t('edit') }}
                        </a-menu-item>
                        <a-menu-item class="danger" @click="deleteServer(index)">
                          <DeleteOutlined /> {{ t('delete') }}
                        </a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown>
                </a-space>
              </template>
              <template v-else-if="column.key === 'address'">
                {{ addrFor(record.server) }}
              </template>
              <template v-else-if="column.key === 'domains'">
                <span class="muted">{{ domainsFor(record.server) }}</span>
              </template>
              <template v-else-if="column.key === 'expectedIPs'">
                <span class="muted">{{ expectedIPsFor(record.server) }}</span>
              </template>
            </template>
          </a-table>
        </a-space>
      </template>
    </a-collapse-panel>

    <!-- ============== Fake DNS ============== -->
    <a-collapse-panel v-if="enableDNS" key="3" header="Fake DNS">
      <a-empty v-if="fakeDnsList.length === 0" :description="t('emptyFakeDnsDesc')">
        <a-button type="primary" @click="addFakedns">
          <template #icon>
            <PlusOutlined />
          </template>
          {{ t('pages.xray.fakedns.add') }}
        </a-button>
      </a-empty>

      <template v-else>
        <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
          <a-button type="primary" @click="addFakedns">
            <template #icon>
              <PlusOutlined />
            </template>
            {{ t('pages.xray.fakedns.add') }}
          </a-button>
          <a-table :columns="fakednsColumns" :data-source="fakeDnsList" :row-key="(r) => r.key" :pagination="false"
            size="small" bordered>
            <template #bodyCell="{ column, record, index }">
              <template v-if="column.key === 'action'">
                <a-space :size="6">
                  <span class="row-index">{{ index + 1 }}</span>
                  <a-button shape="circle" size="small" danger @click="deleteFakedns(index)">
                    <DeleteOutlined />
                  </a-button>
                </a-space>
              </template>
              <template v-else-if="column.key === 'ipPool'">
                <a-input :value="record.ipPool" size="small"
                  @change="(e) => updateFakednsField(index, 'ipPool', e.target.value)" />
              </template>
              <template v-else-if="column.key === 'poolSize'">
                <a-input-number :value="record.poolSize" :min="1" size="small"
                  @change="(v) => updateFakednsField(index, 'poolSize', v)" />
              </template>
            </template>
          </a-table>
        </a-space>
      </template>
    </a-collapse-panel>
  </a-collapse>

  <DnsServerModal v-model:open="serverModalOpen" :server="editingServer" :is-edit="editingIndex != null"
    @confirm="onServerConfirm" />
  <DnsPresetsModal v-model:open="presetsModalOpen" @install="onPresetInstall" />
</template>

<style scoped>
.row-index {
  font-weight: 500;
  opacity: 0.7;
}

.muted {
  opacity: 0.7;
  word-break: break-all;
}

.danger {
  color: #ff4d4f;
}

.hosts-row {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
</style>
