<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons-vue';

import SettingListItem from '@/components/SettingListItem.vue';
import DnsServerModal from './DnsServerModal.vue';

const { t } = useI18n();

// Structured DNS editor — mirrors web/html/settings/xray/dns.html.
// Master enable switch + general DNS options + per-server table with
// add/edit/delete (modal flow), plus a Fake DNS table. Both lists
// flow through templateSettings.dns / .fakedns reactively so the
// useXraySetting composable picks every edit up via its deep watch.

const props = defineProps({
  templateSettings: { type: Object, default: null },
});

const STRATEGIES = ['UseSystem', 'UseIP', 'UseIPv4', 'UseIPv6'];

// ============== Master toggle ==============
const enableDNS = computed({
  get: () => !!props.templateSettings?.dns,
  set: (next) => {
    if (!props.templateSettings) return;
    if (next) {
      props.templateSettings.dns = {
        tag: 'dns_inbound',
        clientIp: '',
        queryStrategy: 'UseIP',
        disableCache: false,
        disableFallback: false,
        disableFallbackIfMatch: false,
        useSystemHosts: false,
        enableParallelQuery: false,
        servers: [],
      };
      props.templateSettings.fakedns = null;
    } else {
      delete props.templateSettings.dns;
      delete props.templateSettings.fakedns;
    }
  },
});

// ============== Field bridges ==============
function dnsField(field, fallback) {
  return computed({
    get: () => props.templateSettings?.dns?.[field] ?? fallback,
    set: (v) => {
      if (props.templateSettings?.dns) props.templateSettings.dns[field] = v;
    },
  });
}

const dnsTag = dnsField('tag', 'dns_inbound');
const dnsClientIp = dnsField('clientIp', '');
const dnsStrategy = dnsField('queryStrategy', 'UseIP');
const dnsDisableCache = dnsField('disableCache', false);
const dnsDisableFallback = dnsField('disableFallback', false);
const dnsDisableFallbackIfMatch = dnsField('disableFallbackIfMatch', false);
const dnsEnableParallelQuery = dnsField('enableParallelQuery', false);
const dnsUseSystemHosts = dnsField('useSystemHosts', false);

// ============== DNS server table ==============
const dnsServers = computed(() => {
  const list = props.templateSettings?.dns?.servers || [];
  return list.map((s, idx) => ({ key: idx, server: s }));
});

const dnsColumns = computed(() => [
  { title: '#', key: 'action', align: 'center', width: 60 },
  { title: t('pages.inbounds.address'), key: 'address', align: 'left' },
  { title: t('pages.xray.dns.domains'), key: 'domains', align: 'left' },
  { title: t('pages.xray.dns.expectIPs'), key: 'expectIPs', align: 'left' },
]);

function addrFor(server) {
  return typeof server === 'string' ? server : server?.address || '';
}
function domainsFor(server) {
  return typeof server === 'object' ? (server.domains || []).join(',') : '';
}
function expectIPsFor(server) {
  return typeof server === 'object' ? (server.expectIPs || []).join(',') : '';
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
      </template>
    </a-collapse-panel>

    <!-- ============== DNS servers ============== -->
    <a-collapse-panel v-if="enableDNS" key="2" header="DNS">
      <a-empty v-if="dnsServers.length === 0" :description="t('emptyDnsDesc')">
        <a-button type="primary" @click="openAddServer">
          <template #icon><PlusOutlined /></template>
          {{ t('pages.xray.dns.add') }}
        </a-button>
      </a-empty>

      <template v-else>
        <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
          <a-button type="primary" @click="openAddServer">
            <template #icon><PlusOutlined /></template>
            {{ t('pages.xray.dns.add') }}
          </a-button>
          <a-table
            :columns="dnsColumns"
            :data-source="dnsServers"
            :row-key="(r) => r.key"
            :pagination="false"
            size="small"
            bordered
          >
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
              <template v-else-if="column.key === 'expectIPs'">
                <span class="muted">{{ expectIPsFor(record.server) }}</span>
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
          <template #icon><PlusOutlined /></template>
          {{ t('pages.xray.fakedns.add') }}
        </a-button>
      </a-empty>

      <template v-else>
        <a-space direction="vertical" size="middle" :style="{ width: '100%' }">
          <a-button type="primary" @click="addFakedns">
            <template #icon><PlusOutlined /></template>
            {{ t('pages.xray.fakedns.add') }}
          </a-button>
          <a-table
            :columns="fakednsColumns"
            :data-source="fakeDnsList"
            :row-key="(r) => r.key"
            :pagination="false"
            size="small"
            bordered
          >
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
                <a-input
                  :value="record.ipPool"
                  size="small"
                  @change="(e) => updateFakednsField(index, 'ipPool', e.target.value)"
                />
              </template>
              <template v-else-if="column.key === 'poolSize'">
                <a-input-number
                  :value="record.poolSize"
                  :min="1"
                  size="small"
                  @change="(v) => updateFakednsField(index, 'poolSize', v)"
                />
              </template>
            </template>
          </a-table>
        </a-space>
      </template>
    </a-collapse-panel>
  </a-collapse>

  <DnsServerModal
    v-model:open="serverModalOpen"
    :server="editingServer"
    :is-edit="editingIndex != null"
    @confirm="onServerConfirm"
  />
</template>

<style scoped>
.row-index {
  font-weight: 500;
  opacity: 0.7;
}
.muted { opacity: 0.7; word-break: break-all; }
.danger { color: #ff4d4f; }
</style>
