<script setup>
import { computed, ref, watch } from 'vue';
import { ApiOutlined, SyncOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { HttpUtil, SizeFormatter, ObjectUtil, Wireguard } from '@/utils';

// Cloudflare WARP provisioning modal. Mirrors the legacy warp_modal:
//   • when no WARP account is registered yet, a single Create button
//     generates a wireguard keypair locally and posts it to
//     /panel/xray/warp/reg to create a Cloudflare device record;
//   • once registered, the modal displays the access_token /
//     device_id / license_key / private_key, lets the user upgrade
//     to WARP+ via /panel/xray/warp/license, fetches the current
//     account config (premium data / quota / usage) via
//     /panel/xray/warp/config, and stages a wireguard outbound
//     ready for adding to templateSettings.outbounds.

const props = defineProps({
  open: { type: Boolean, default: false },
  templateSettings: { type: Object, default: null },
});

const emit = defineEmits(['update:open', 'add-outbound', 'reset-outbound', 'remove-outbound']);

const loading = ref(false);
const warpData = ref(null);
const warpConfig = ref(null);
const warpPlus = ref('');
// Held in memory so the parent's add/reset handlers receive the same
// object the modal computed from getConfig().
const stagedOutbound = ref(null);

const warpOutboundIndex = computed(() => {
  const list = props.templateSettings?.outbounds;
  if (!list) return -1;
  return list.findIndex((o) => o?.tag === 'warp');
});

watch(() => props.open, (next) => {
  if (!next) return;
  warpConfig.value = null;
  stagedOutbound.value = null;
  fetchData();
});

async function fetchData() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/warp/data');
    if (msg?.success) {
      const raw = msg.obj;
      warpData.value = raw && raw.length > 0 ? JSON.parse(raw) : null;
    }
  } finally {
    loading.value = false;
  }
}

async function register() {
  loading.value = true;
  try {
    const keys = Wireguard.generateKeypair();
    const msg = await HttpUtil.post('/panel/xray/warp/reg', keys);
    if (msg?.success) {
      const resp = JSON.parse(msg.obj);
      warpData.value = resp.data;
      warpConfig.value = resp.config;
      collectConfig();
    }
  } finally {
    loading.value = false;
  }
}

async function getConfig() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/warp/config');
    if (msg?.success) {
      warpConfig.value = JSON.parse(msg.obj);
      collectConfig();
    }
  } finally {
    loading.value = false;
  }
}

async function updateLicense() {
  if (warpPlus.value.length < 26) return;
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/warp/license', { license: warpPlus.value });
    if (msg?.success) {
      warpData.value = JSON.parse(msg.obj);
      warpConfig.value = null;
      warpPlus.value = '';
    }
  } finally {
    loading.value = false;
  }
}

async function delConfig() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post('/panel/xray/warp/del');
    if (msg?.success) {
      warpData.value = null;
      warpConfig.value = null;
      stagedOutbound.value = null;
      emit('remove-outbound', 'warp');
      close();
    }
  } finally {
    loading.value = false;
  }
}

// Build the wireguard outbound shape from the WARP account data.
// Keep this here (not on the parent) because the encoding of the
// reserved bytes from `client_id` is WARP-specific.
function collectConfig() {
  const config = warpConfig.value?.config;
  if (!config?.peers?.length) return;
  const peer = config.peers[0];
  stagedOutbound.value = {
    tag: 'warp',
    protocol: 'wireguard',
    settings: {
      mtu: 1420,
      secretKey: warpData.value.private_key,
      address: addressesFor(config.interface?.addresses || {}),
      reserved: reservedFor(warpData.value.client_id),
      domainStrategy: 'ForceIP',
      peers: [{
        publicKey: peer.public_key,
        endpoint: peer.endpoint?.host,
      }],
      noKernelTun: false,
    },
  };
}

function addressesFor(addrs) {
  const out = [];
  if (addrs.v4) out.push(`${addrs.v4}/32`);
  if (addrs.v6) out.push(`${addrs.v6}/128`);
  return out;
}

// WARP encodes its reserved bytes as a base64-decoded triplet pulled
// from `client_id`. We turn those bytes into an int array — same
// algorithm the legacy modal used.
function reservedFor(clientId) {
  if (!clientId) return [];
  const decoded = atob(clientId);
  const out = [];
  for (let i = 0; i < decoded.length; i++) out.push(decoded.charCodeAt(i));
  return out;
}

function addOutbound() {
  if (!stagedOutbound.value) {
    message.warning('Fetch the WARP config first.');
    return;
  }
  emit('add-outbound', stagedOutbound.value);
  close();
}

function resetOutbound() {
  if (!stagedOutbound.value) return;
  emit('reset-outbound', { index: warpOutboundIndex.value, outbound: stagedOutbound.value });
  close();
}

function close() { emit('update:open', false); }

const hasWarp = computed(() => !ObjectUtil.isEmpty(warpData.value));
const hasConfig = computed(() => !ObjectUtil.isEmpty(warpConfig.value));
</script>

<template>
  <a-modal :open="open" title="Cloudflare WARP" :footer="null" :closable="true" :mask-closable="true" @cancel="close">
    <!-- WARP / NordVPN provisioning forms keep technical wire labels in
         English on purpose: they map directly to API field names users
         look up in vendor docs. Only the primary action buttons +
         dialog headers translate. -->
    <!-- Not registered yet → single Create CTA -->
    <template v-if="!hasWarp">
      <a-button type="primary" :loading="loading" @click="register">
        <template #icon>
          <ApiOutlined />
        </template>
        Create WARP account
      </a-button>
    </template>

    <!-- Registered → account display + license + config + outbound controls -->
    <template v-else>
      <table class="warp-data-table">
        <tbody>
          <tr class="row-odd">
            <td>Access token</td>
            <td>{{ warpData.access_token }}</td>
          </tr>
          <tr>
            <td>Device ID</td>
            <td>{{ warpData.device_id }}</td>
          </tr>
          <tr class="row-odd">
            <td>License key</td>
            <td>{{ warpData.license_key }}</td>
          </tr>
          <tr>
            <td>Private key</td>
            <td>{{ warpData.private_key }}</td>
          </tr>
        </tbody>
      </table>

      <a-button :loading="loading" type="primary" danger class="mt-8" @click="delConfig">
        <template #icon>
          <DeleteOutlined />
        </template>
        Delete account
      </a-button>

      <a-divider class="zero-margin">Settings</a-divider>

      <a-collapse class="my-10">
        <a-collapse-panel header="WARP / WARP+ license key">
          <a-form :colon="false" :label-col="{ md: { span: 6 } }" :wrapper-col="{ md: { span: 14 } }">
            <a-form-item label="Key">
              <a-input v-model:value="warpPlus" placeholder="26-char WARP+ key" />
              <a-button type="primary" class="mt-8" :disabled="warpPlus.length < 26" :loading="loading"
                @click="updateLicense">Update</a-button>
            </a-form-item>
          </a-form>
        </a-collapse-panel>
      </a-collapse>

      <a-divider class="zero-margin">Account info</a-divider>
      <a-button class="my-8" :loading="loading" type="primary" @click="getConfig">
        <template #icon>
          <SyncOutlined />
        </template>
        Refresh
      </a-button>

      <template v-if="hasConfig">
        <table class="warp-data-table">
          <tbody>
            <tr class="row-odd">
              <td>Device name</td>
              <td>{{ warpConfig.name }}</td>
            </tr>
            <tr>
              <td>Device model</td>
              <td>{{ warpConfig.model }}</td>
            </tr>
            <tr class="row-odd">
              <td>Device enabled</td>
              <td>{{ warpConfig.enabled }}</td>
            </tr>
            <template v-if="warpConfig.account">
              <tr>
                <td>Account type</td>
                <td>{{ warpConfig.account.account_type }}</td>
              </tr>
              <tr class="row-odd">
                <td>Role</td>
                <td>{{ warpConfig.account.role }}</td>
              </tr>
              <tr>
                <td>WARP+ data</td>
                <td>{{ SizeFormatter.sizeFormat(warpConfig.account.premium_data) }}</td>
              </tr>
              <tr class="row-odd">
                <td>Quota</td>
                <td>{{ SizeFormatter.sizeFormat(warpConfig.account.quota) }}</td>
              </tr>
              <tr v-if="warpConfig.account.usage">
                <td>Usage</td>
                <td>{{ SizeFormatter.sizeFormat(warpConfig.account.usage) }}</td>
              </tr>
            </template>
          </tbody>
        </table>

        <a-divider class="my-10">Outbound status</a-divider>
        <template v-if="warpOutboundIndex >= 0">
          <a-tag color="green">Enabled</a-tag>
          <a-button type="primary" danger :loading="loading" class="ml-8" @click="resetOutbound">
            Reset
          </a-button>
        </template>
        <template v-else>
          <a-tag color="orange">Disabled</a-tag>
          <a-button type="primary" :loading="loading" class="ml-8" @click="addOutbound">
            <template #icon>
              <PlusOutlined />
            </template>
            Add outbound
          </a-button>
        </template>
      </template>
    </template>
  </a-modal>
</template>

<style scoped>
.warp-data-table {
  margin: 5px 0;
  width: 100%;
  border-collapse: collapse;
}

.warp-data-table td {
  padding: 4px 8px;
  word-break: break-all;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.warp-data-table td:first-child {
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

.my-8 {
  margin: 8px 0;
}

.mt-8 {
  margin-top: 8px;
}

.my-10 {
  margin: 10px 0;
}

.ml-8 {
  margin-left: 8px;
}
</style>
