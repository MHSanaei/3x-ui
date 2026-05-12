<script setup>
import { computed, ref, watch } from 'vue';
import { LoginOutlined, SaveOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { HttpUtil } from '@/utils';

// NordVPN provisioning modal — mirrors the legacy nord_modal.
//
// Login routes:
//   • access token (NordVPN account) → /panel/xray/nord/reg
//   • manual private key (existing wireguard key from NordLynx) →
//     /panel/xray/nord/setKey
// Once authenticated, the country / city / server selectors fetch
// from /panel/xray/nord/{countries,servers}, and the user can stage
// a wireguard outbound (tag `nord-<hostname>`) for the parent's
// outbound list.

const props = defineProps({
  open: { type: Boolean, default: false },
  templateSettings: { type: Object, default: null },
});

const emit = defineEmits([
  'update:open',
  'add-outbound',
  'reset-outbound',
  'remove-outbound',
  // Routing rules referencing the deleted nord-* outbound need the
  // parent to clean them up — we emit, the parent purges.
  'remove-routing-rules',
]);

const loading = ref(false);
const nordData = ref(null);
const token = ref('');
const manualKey = ref('');

const countries = ref([]);
const cities = ref([]);
const servers = ref([]);
const countryId = ref(null);
const cityId = ref(null);
const serverId = ref(null);

const nordOutboundIndex = computed(() => {
  const list = props.templateSettings?.outbounds;
  if (!list) return -1;
  return list.findIndex((o) => o?.tag?.startsWith?.('nord-'));
});

const filteredServers = computed(() => {
  if (!cityId.value) return servers.value;
  return servers.value.filter((s) => s.cityId === cityId.value);
});

watch(() => props.open, (next) => {
  if (next) fetchData();
});

watch(() => filteredServers.value, (list) => {
  // Auto-select the first server in the visible list (lowest load
  // because servers were sorted ascending by load on fetch).
  serverId.value = list.length > 0 ? list[0].id : null;
});

// === API actions ====================================================
async function fetchData() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/nord/data');
    if (msg?.success) {
      nordData.value = msg.obj ? JSON.parse(msg.obj) : null;
      if (nordData.value) await fetchCountries();
    }
  } finally {
    loading.value = false;
  }
}

async function login() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/nord/reg', { token: token.value });
    if (msg?.success) {
      nordData.value = JSON.parse(msg.obj);
      await fetchCountries();
    }
  } finally {
    loading.value = false;
  }
}

async function saveKey() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/nord/setKey', { key: manualKey.value });
    if (msg?.success) {
      nordData.value = JSON.parse(msg.obj);
      await fetchCountries();
    }
  } finally {
    loading.value = false;
  }
}

async function logout() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/nord/del');
    if (msg?.success) {
      // Clean up the staged outbound + matching routing rules first
      // so a re-login doesn't carry stale references.
      emit('remove-outbound', nordOutboundIndex.value);
      emit('remove-routing-rules', { prefix: 'nord-' });
      nordData.value = null;
      token.value = '';
      manualKey.value = '';
      countries.value = [];
      cities.value = [];
      servers.value = [];
      countryId.value = null;
      cityId.value = null;
      serverId.value = null;
    }
  } finally {
    loading.value = false;
  }
}

async function fetchCountries() {
  const msg = await HttpUtil.post('/panel/xray/nord/countries');
  if (msg?.success) countries.value = JSON.parse(msg.obj);
}

async function fetchServers() {
  if (!countryId.value) return;
  loading.value = true;
  servers.value = [];
  cities.value = [];
  serverId.value = null;
  cityId.value = null;
  try {
    const msg = await HttpUtil.post('/panel/xray/nord/servers', { countryId: countryId.value });
    if (!msg?.success) return;
    const data = JSON.parse(msg.obj);
    const locations = data.locations || [];
    const locToCity = {};
    const citiesMap = new Map();
    for (const loc of locations) {
      if (loc.country?.city) {
        citiesMap.set(loc.country.city.id, loc.country.city);
        locToCity[loc.id] = loc.country.city;
      }
    }
    cities.value = Array.from(citiesMap.values()).sort((a, b) => a.name.localeCompare(b.name));

    servers.value = (data.servers || [])
      .map((s) => {
        const firstLocId = (s.location_ids || [])[0];
        const city = locToCity[firstLocId];
        return { ...s, cityId: city?.id || null, cityName: city?.name || 'Unknown' };
      })
      .sort((a, b) => a.load - b.load);

    if (servers.value.length === 0) {
      message.warning('No servers found for the selected country');
    }
  } finally {
    loading.value = false;
  }
}

// === Outbound staging ==============================================
// NordVPN exposes its WireGuard public key via a "technologies"
// array entry with id 35; the legacy modal pulls the key from the
// metadata field of that entry. Same here.
function buildNordOutbound() {
  const server = servers.value.find((s) => s.id === serverId.value);
  if (!server) return null;
  const tech = server.technologies?.find((t) => t.id === 35);
  const publicKey = tech?.metadata?.find((m) => m.name === 'public_key')?.value;
  if (!publicKey) {
    message.error('Selected server does not advertise a NordLynx public key.');
    return null;
  }
  return {
    tag: `nord-${server.hostname}`,
    protocol: 'wireguard',
    settings: {
      secretKey: nordData.value.private_key,
      address: ['10.5.0.2/32'],
      peers: [{ publicKey, endpoint: `${server.station}:51820` }],
      noKernelTun: false,
    },
  };
}

function addOutbound() {
  const ob = buildNordOutbound();
  if (!ob) return;
  emit('add-outbound', ob);
  message.success('NordVPN outbound added');
  close();
}

function resetOutbound() {
  if (nordOutboundIndex.value === -1) return;
  const ob = buildNordOutbound();
  if (!ob) return;
  // Tag rename across routing.rules is the parent's job — pass
  // both old and new tag in the payload.
  const oldTag = props.templateSettings.outbounds[nordOutboundIndex.value]?.tag;
  emit('reset-outbound', {
    index: nordOutboundIndex.value,
    outbound: ob,
    oldTag,
    newTag: ob.tag,
  });
  message.success('NordVPN outbound updated');
  close();
}

function close() { emit('update:open', false); }

function loadColor(load) {
  if (load < 30) return 'green';
  if (load < 70) return 'orange';
  return 'red';
}
</script>

<template>
  <a-modal :open="open" title="NordVPN NordLynx" :footer="null" :closable="true" :mask-closable="true" @cancel="close">
    <!-- WARP / NordVPN provisioning forms keep technical wire labels in
         English on purpose: they map directly to API field names users
         look up in vendor docs. Only the primary action buttons +
         dialog headers translate. -->
    <!-- Not authenticated → tabbed login (token or manual key) -->
    <template v-if="nordData == null">
      <a-tabs default-active-key="token">
        <a-tab-pane key="token" tab="Access token">
          <a-form :colon="false" :label-col="{ md: { span: 6 } }" :wrapper-col="{ md: { span: 18 } }" class="mt-20">
            <a-form-item label="Access token">
              <a-input v-model:value="token" placeholder="Access token" />
              <a-button type="primary" class="mt-10" :loading="loading" @click="login">
                <template #icon>
                  <LoginOutlined />
                </template>
                Login
              </a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
        <a-tab-pane key="key" tab="Private key">
          <a-form :colon="false" :label-col="{ md: { span: 6 } }" :wrapper-col="{ md: { span: 18 } }" class="mt-20">
            <a-form-item label="Private key">
              <a-input v-model:value="manualKey" placeholder="Private key" />
              <a-button type="primary" class="mt-10" :loading="loading" @click="saveKey">
                <template #icon>
                  <SaveOutlined />
                </template>
                Save
              </a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
      </a-tabs>
    </template>

    <!-- Authenticated → server picker + outbound controls -->
    <template v-else>
      <table class="nord-data-table">
        <tbody>
          <tr v-if="nordData.token" class="row-odd">
            <td>Access token</td>
            <td>{{ nordData.token }}</td>
          </tr>
          <tr>
            <td>Private key</td>
            <td>{{ nordData.private_key }}</td>
          </tr>
        </tbody>
      </table>

      <a-button :loading="loading" type="primary" danger class="mt-8" @click="logout">Logout</a-button>

      <a-divider class="zero-margin">Settings</a-divider>

      <a-form :colon="false" :label-col="{ md: { span: 6 } }" :wrapper-col="{ md: { span: 18 } }" class="mt-10">
        <a-form-item label="Country">
          <a-select v-model:value="countryId" show-search option-filter-prop="label" @change="fetchServers">
            <a-select-option v-for="c in countries" :key="c.id" :value="c.id" :label="c.name">
              {{ c.name }} ({{ c.code }})
            </a-select-option>
          </a-select>
        </a-form-item>

        <a-form-item v-if="cities.length > 0" label="City">
          <a-select v-model:value="cityId" show-search option-filter-prop="label">
            <a-select-option :value="null" label="All cities">All cities</a-select-option>
            <a-select-option v-for="c in cities" :key="c.id" :value="c.id" :label="c.name">{{ c.name
            }}</a-select-option>
          </a-select>
        </a-form-item>

        <a-form-item v-if="filteredServers.length > 0" label="Server">
          <a-select v-model:value="serverId" show-search option-filter-prop="label">
            <a-select-option v-for="s in filteredServers" :key="s.id" :value="s.id"
              :label="`${s.cityName} ${s.name} ${s.hostname}`">
              <span class="server-row">
                <span class="server-name">{{ s.cityName }} - {{ s.name }}</span>
                <a-tag :color="loadColor(s.load)" class="server-load-tag">{{ s.load }}%</a-tag>
              </span>
            </a-select-option>
          </a-select>
        </a-form-item>
      </a-form>

      <a-divider class="my-10">Outbound status</a-divider>

      <template v-if="nordOutboundIndex >= 0">
        <a-tag color="green">Enabled</a-tag>
        <a-button type="primary" danger :loading="loading" class="ml-8" @click="resetOutbound">
          Reset
        </a-button>
      </template>
      <template v-else>
        <a-tag color="orange">Disabled</a-tag>
        <a-button type="primary" class="ml-8" :disabled="!serverId" :loading="loading" @click="addOutbound">Add
          outbound</a-button>
      </template>
    </template>
  </a-modal>
</template>

<style scoped>
.nord-data-table {
  margin: 5px 0;
  width: 100%;
  border-collapse: collapse;
}

.nord-data-table td {
  padding: 4px 8px;
  word-break: break-all;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.nord-data-table td:first-child {
  font-family: inherit;
  font-weight: 500;
  white-space: nowrap;
  width: 130px;
}

.row-odd {
  background: rgba(0, 0, 0, 0.03);
}

:global(body.dark) .row-odd {
  background: rgba(255, 255, 255, 0.04);
}

.zero-margin {
  margin: 0;
}

.mt-8 {
  margin-top: 8px;
}

.mt-10 {
  margin-top: 10px;
}

.mt-20 {
  margin-top: 20px;
}

.my-10 {
  margin: 10px 0;
}

.ml-8 {
  margin-left: 8px;
}

.server-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.server-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}

.server-load-tag {
  margin-right: 0;
  flex-shrink: 0;
}
</style>
