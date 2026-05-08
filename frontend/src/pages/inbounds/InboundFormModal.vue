<script setup>
import { computed, ref, watch } from 'vue';
import dayjs from 'dayjs';
import { message } from 'ant-design-vue';
import { SyncOutlined, PlusOutlined, MinusOutlined, DeleteOutlined } from '@ant-design/icons-vue';

import {
  HttpUtil,
  RandomUtil,
  NumberFormatter,
  SizeFormatter,
  Wireguard,
} from '@/utils';
import {
  Inbound,
  Protocols,
  SSMethods,
  USERS_SECURITY,
  TLS_FLOW_CONTROL,
  SNIFFING_OPTION,
} from '@/models/inbound.js';
import { DBInbound } from '@/models/dbinbound.js';

// Phase 5f-iii-b: structured per-protocol/per-transport forms instead
// of raw JSON textareas. Edits a deeply-reactive Inbound + DBInbound
// pair so the existing model helpers (.toString(), .canEnableTls(),
// genAllLinks(), addPeer(), etc.) keep working unchanged. The
// "Advanced" tab still exposes the full streamSettings JSON for
// transport variants (KCP/XHTTP/sockopt/finalmask) we don't yet have
// dedicated UI for.

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add', validator: (v) => ['add', 'edit'].includes(v) },
  dbInbound: { type: Object, default: null },
});

const emit = defineEmits(['update:open', 'saved']);

const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'];
const PROTOCOLS = Object.values(Protocols);
const SECURITY_OPTIONS = Object.values(USERS_SECURITY);
const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

// === Reactive state ================================================
// Cloned on every open so cancelling the modal doesn't mutate the row.
const inbound = ref(null);
const dbForm = ref(null);
const saving = ref(false);
const advancedJson = ref({ stream: '', sniffing: '' });

// Derived helpers — each is a computed off `inbound` so flips of
// protocol / network / security re-render the right blocks.
const protocol = computed(() => inbound.value?.protocol);
const network = computed({
  get: () => inbound.value?.stream?.network,
  set: (v) => onNetworkChange(v),
});
const security = computed({
  get: () => inbound.value?.stream?.security,
  set: (v) => { if (inbound.value?.stream) inbound.value.stream.security = v; },
});

const isMultiUser = computed(() => {
  if (!inbound.value) return false;
  switch (inbound.value.protocol) {
    case Protocols.VMESS:
    case Protocols.VLESS:
    case Protocols.TROJAN:
    case Protocols.HYSTERIA:
      return true;
    case Protocols.SHADOWSOCKS:
      return !!inbound.value.isSSMultiUser;
    default:
      return false;
  }
});

const clientsArray = computed(() => {
  if (!inbound.value) return [];
  switch (inbound.value.protocol) {
    case Protocols.VMESS: return inbound.value.settings.vmesses || [];
    case Protocols.VLESS: return inbound.value.settings.vlesses || [];
    case Protocols.TROJAN: return inbound.value.settings.trojans || [];
    case Protocols.SHADOWSOCKS: return inbound.value.settings.shadowsockses || [];
    case Protocols.HYSTERIA: return inbound.value.settings.hysterias || [];
    default: return [];
  }
});

const firstClient = computed(() => clientsArray.value[0] || null);
const canEnableStream = computed(() => inbound.value?.canEnableStream?.() === true);
const canEnableTls = computed(() => inbound.value?.canEnableTls?.() === true);
const canEnableReality = computed(() => inbound.value?.canEnableReality?.() === true);
const canEnableTlsFlow = computed(() => inbound.value?.canEnableTlsFlow?.() === true);

// Date / GB bridges (legacy used moment via _expiryTime; we go direct).
const expiryDate = computed({
  get: () => (dbForm.value?.expiryTime > 0 ? dayjs(dbForm.value.expiryTime) : null),
  set: (next) => { if (dbForm.value) dbForm.value.expiryTime = next ? next.valueOf() : 0; },
});
const totalGB = computed({
  get: () => (dbForm.value?.total ? Math.round((dbForm.value.total / SizeFormatter.ONE_GB) * 100) / 100 : 0),
  set: (gb) => { if (dbForm.value) dbForm.value.total = NumberFormatter.toFixed((gb || 0) * SizeFormatter.ONE_GB, 0); },
});

// Client total/expiry bridges (only relevant in add mode for new clients)
const clientExpiryDate = computed({
  get: () => (firstClient.value?.expiryTime > 0 ? dayjs(firstClient.value.expiryTime) : null),
  set: (next) => { if (firstClient.value) firstClient.value.expiryTime = next ? next.valueOf() : 0; },
});
const clientTotalGB = computed({
  get: () => firstClient.value?._totalGB ?? 0,
  set: (gb) => { if (firstClient.value) firstClient.value._totalGB = gb || 0; },
});

// === Open / state management =======================================
function loadFromDbInbound(dbIn) {
  // Round-trip through Inbound.fromJson so subsequent edits get the
  // structured class hierarchy (StreamSettings, TLS, Reality, etc.).
  const parsed = Inbound.fromJson(dbIn.toInbound().toJson());
  inbound.value = parsed;
  // DBForm carries the persisted-fields the parsed Inbound doesn't:
  // remark, enable, total, expiryTime, trafficReset, etc.
  dbForm.value = new DBInbound(dbIn);
  primeAdvancedJson();
}

function makeFreshInbound(proto) {
  const ib = new Inbound();
  ib.protocol = proto;
  ib.settings = Inbound.Settings.getSettings(proto);
  ib.port = RandomUtil.randomInteger(10000, 60000);
  return ib;
}

function freshDbForm() {
  const next = new DBInbound();
  next.enable = true;
  next.remark = '';
  next.total = 0;
  next.expiryTime = 0;
  next.trafficReset = 'never';
  return next;
}

function primeAdvancedJson() {
  if (!inbound.value) return;
  advancedJson.value.stream = JSON.stringify(JSON.parse(inbound.value.stream.toString()), null, 2);
  advancedJson.value.sniffing = JSON.stringify(JSON.parse(inbound.value.sniffing.toString()), null, 2);
}

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.mode === 'edit' && props.dbInbound) {
    loadFromDbInbound(props.dbInbound);
  } else {
    inbound.value = makeFreshInbound(Protocols.VMESS);
    dbForm.value = freshDbForm();
    primeAdvancedJson();
  }
});

// In add mode, switching protocol restamps settings + re-syncs port.
function onProtocolChange(next) {
  if (props.mode === 'edit' || !inbound.value) return;
  inbound.value.protocol = next;
  inbound.value.settings = Inbound.Settings.getSettings(next);
  primeAdvancedJson();
}

function onNetworkChange(next) {
  if (!inbound.value?.stream) return;
  inbound.value.stream.network = next;
  // Mirror legacy streamNetworkChange: clear flow when TLS/Reality
  // become unavailable; reset finalmask.udp when not KCP.
  if (!inbound.value.canEnableTls()) inbound.value.stream.security = 'none';
  if (!inbound.value.canEnableReality()) inbound.value.reality = false;
  if (
    inbound.value.protocol === Protocols.VLESS
    && !inbound.value.canEnableTlsFlow()
    && Array.isArray(inbound.value.settings.vlesses)
  ) {
    inbound.value.settings.vlesses.forEach((c) => { c.flow = ''; });
  }
  if (next !== 'kcp' && inbound.value.stream.finalmask) {
    inbound.value.stream.finalmask.udp = [];
  }
}

// === Random helpers wired to the form's sync icons ==================
function randomEmail(target) {
  if (target) target.email = RandomUtil.randomLowerAndNum(9);
}
function randomUuid(target) {
  if (target) target.id = RandomUtil.randomUUID();
}
function randomPasswordSeq(target, len = 10) {
  if (target) target.password = RandomUtil.randomSeq(len);
}
function randomSSPassword(target) {
  if (target) target.password = RandomUtil.randomShadowsocksPassword(inbound.value.settings.method);
}
function randomAuth(target) {
  if (target) target.auth = RandomUtil.randomSeq(10);
}
function randomSubId(target) {
  if (target) target.subId = RandomUtil.randomLowerAndNum(16);
}
function regenWgKeypair(target) {
  const kp = Wireguard.generateKeypair();
  target.publicKey = kp.publicKey;
  target.privateKey = kp.privateKey;
}
function regenInboundWg() {
  const kp = Wireguard.generateKeypair();
  inbound.value.settings.pubKey = kp.publicKey;
  inbound.value.settings.secretKey = kp.privateKey;
}

// === Reality keygen via existing API =================================
async function genRealityKeypair() {
  saving.value = true;
  try {
    const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
    if (msg?.success) {
      inbound.value.stream.reality.privateKey = msg.obj.privateKey;
      inbound.value.stream.reality.settings.publicKey = msg.obj.publicKey;
    }
  } finally {
    saving.value = false;
  }
}

// === SS method change tracks legacy semantics =========================
function onSSMethodChange() {
  inbound.value.settings.password = RandomUtil.randomShadowsocksPassword(inbound.value.settings.method);
  if (inbound.value.isSSMultiUser) {
    if (inbound.value.settings.shadowsockses.length === 0) {
      inbound.value.settings.shadowsockses = [new Inbound.ShadowsocksSettings.Shadowsocks()];
    }
    inbound.value.settings.shadowsockses.forEach((c) => {
      c.method = inbound.value.isSS2022 ? '' : inbound.value.settings.method;
      c.password = RandomUtil.randomShadowsocksPassword(inbound.value.settings.method);
    });
  } else {
    inbound.value.settings.shadowsockses = [];
  }
}

// === Submit ==========================================================
function close() {
  emit('update:open', false);
}

async function submit() {
  if (!inbound.value || !dbForm.value) return;
  saving.value = true;
  try {
    // Sniffing tab is structured; stream stays JSON for unsupported
    // transports — both go to wire as serialized JSON.
    let streamSettings;
    let sniffing;
    try {
      streamSettings = canEnableStream.value
        ? JSON.stringify(JSON.parse(advancedJson.value.stream))
        : (inbound.value.stream?.sockopt
          ? JSON.stringify({ sockopt: inbound.value.stream.sockopt.toJson() })
          : '');
    } catch (e) { message.error(`Stream JSON invalid: ${e.message}`); return; }
    try {
      sniffing = JSON.stringify(JSON.parse(advancedJson.value.sniffing || inbound.value.sniffing.toString()));
    } catch (e) { message.error(`Sniffing JSON invalid: ${e.message}`); return; }

    // The structured form mutates `inbound.stream` directly when the
    // user edits TCP/WS/gRPC/HTTPUpgrade fields, but if they touched
    // the Advanced JSON tab their edits live there. Keep the JSON tab
    // authoritative — it was populated from the live model on open
    // and watch handlers below sync in either direction.
    const payload = {
      up: dbForm.value.up || 0,
      down: dbForm.value.down || 0,
      total: dbForm.value.total,
      remark: dbForm.value.remark,
      enable: dbForm.value.enable,
      expiryTime: dbForm.value.expiryTime,
      trafficReset: dbForm.value.trafficReset,
      lastTrafficResetTime: dbForm.value.lastTrafficResetTime || 0,
      listen: inbound.value.listen,
      port: inbound.value.port,
      protocol: inbound.value.protocol,
      settings: inbound.value.settings.toString(),
      streamSettings: streamSettings,
      sniffing: sniffing,
    };

    const url = props.mode === 'edit'
      ? `/panel/api/inbounds/update/${props.dbInbound.id}`
      : '/panel/api/inbounds/add';
    const msg = await HttpUtil.post(url, payload);
    if (msg?.success) {
      emit('saved');
      close();
    }
  } finally {
    saving.value = false;
  }
}

const title = computed(() => (props.mode === 'edit' ? 'Edit inbound' : 'Add inbound'));
const okText = computed(() => (props.mode === 'edit' ? 'Update' : 'Create'));

// Whenever the structured stream form mutates the model, refresh the
// Advanced JSON tab so it reflects the latest state. Use a deep watch
// on the parsed JSON of the stream.
watch(
  () => inbound.value && JSON.stringify(inbound.value.stream?.toJson?.() || {}),
  (next) => {
    if (next) {
      try {
        advancedJson.value.stream = JSON.stringify(JSON.parse(inbound.value.stream.toString()), null, 2);
      } catch (_e) { /* leave as is */ }
    }
  },
);
</script>

<template>
  <a-modal
    :open="open"
    :title="title"
    :ok-text="okText"
    cancel-text="Close"
    :confirm-loading="saving"
    :mask-closable="false"
    width="780px"
    @ok="submit"
    @cancel="close"
  >
    <a-tabs v-if="inbound && dbForm" default-active-key="basic">
      <!-- ============================== BASICS ============================== -->
      <a-tab-pane key="basic" tab="Basics">
        <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
          <a-form-item label="Enable">
            <a-switch v-model:checked="dbForm.enable" />
          </a-form-item>
          <a-form-item label="Remark">
            <a-input v-model:value="dbForm.remark" />
          </a-form-item>
          <a-form-item label="Protocol">
            <a-select :value="protocol" :disabled="mode === 'edit'" @change="onProtocolChange">
              <a-select-option v-for="p in PROTOCOLS" :key="p" :value="p">{{ p }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="Listen IP">
            <a-input v-model:value="inbound.listen" placeholder="(blank = all interfaces)" />
          </a-form-item>
          <a-form-item label="Port">
            <a-input-number v-model:value="inbound.port" :min="1" :max="65535" />
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip title="0 means no limit">Total traffic (GB)</a-tooltip>
            </template>
            <a-input-number v-model:value="totalGB" :min="0" :step="0.1" />
          </a-form-item>
          <a-form-item label="Traffic reset">
            <a-select v-model:value="dbForm.trafficReset">
              <a-select-option v-for="r in TRAFFIC_RESETS" :key="r" :value="r">{{ r }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip title="Leave blank to never expire">Expiry date</a-tooltip>
            </template>
            <a-date-picker
              v-model:value="expiryDate"
              :show-time="{ format: 'HH:mm:ss' }"
              format="YYYY-MM-DD HH:mm:ss"
              :style="{ width: '100%' }"
            />
          </a-form-item>
        </a-form>
      </a-tab-pane>

      <!-- ============================== PROTOCOL ============================== -->
      <a-tab-pane key="protocol" tab="Protocol">
        <!-- Multi-user inbounds: in add mode embed the first client form,
             in edit mode show a count summary. -->
        <template v-if="isMultiUser">
          <a-collapse v-if="mode === 'add' && firstClient" default-active-key="0">
            <a-collapse-panel key="0" header="Client">
              <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
                <a-form-item label="Enable">
                  <a-switch v-model:checked="firstClient.enable" />
                </a-form-item>
                <a-form-item>
                  <template #label>
                    <a-tooltip title="Friendly identifier">
                      Email <SyncOutlined class="random-icon" @click="randomEmail(firstClient)" />
                    </a-tooltip>
                  </template>
                  <a-input v-model:value="firstClient.email" />
                </a-form-item>

                <a-form-item v-if="protocol === Protocols.VMESS || protocol === Protocols.VLESS">
                  <template #label>
                    <a-tooltip title="Reset to a fresh UUID">
                      ID <SyncOutlined class="random-icon" @click="randomUuid(firstClient)" />
                    </a-tooltip>
                  </template>
                  <a-input v-model:value="firstClient.id" />
                </a-form-item>

                <a-form-item v-if="protocol === Protocols.VMESS" label="Security">
                  <a-select v-model:value="firstClient.security">
                    <a-select-option v-for="k in SECURITY_OPTIONS" :key="k" :value="k">{{ k }}</a-select-option>
                  </a-select>
                </a-form-item>

                <a-form-item v-if="protocol === Protocols.TROJAN || protocol === Protocols.SHADOWSOCKS">
                  <template #label>
                    <a-tooltip title="Reset to a fresh random value">
                      Password
                      <SyncOutlined
                        v-if="protocol === Protocols.SHADOWSOCKS"
                        class="random-icon"
                        @click="randomSSPassword(firstClient)"
                      />
                      <SyncOutlined
                        v-else
                        class="random-icon"
                        @click="randomPasswordSeq(firstClient)"
                      />
                    </a-tooltip>
                  </template>
                  <a-input v-model:value="firstClient.password" />
                </a-form-item>

                <a-form-item v-if="protocol === Protocols.HYSTERIA">
                  <template #label>
                    <a-tooltip title="Reset"><span>Auth password</span> <SyncOutlined class="random-icon" @click="randomAuth(firstClient)" /></a-tooltip>
                  </template>
                  <a-input v-model:value="firstClient.auth" />
                </a-form-item>

                <a-form-item v-if="canEnableTlsFlow" label="Flow">
                  <a-select v-model:value="firstClient.flow">
                    <a-select-option value="">none</a-select-option>
                    <a-select-option v-for="k in FLOW_OPTIONS" :key="k" :value="k">{{ k }}</a-select-option>
                  </a-select>
                </a-form-item>

                <a-form-item label="Subscription">
                  <a-input v-model:value="firstClient.subId">
                    <template #addonAfter>
                      <SyncOutlined class="random-icon" @click="randomSubId(firstClient)" />
                    </template>
                  </a-input>
                </a-form-item>

                <a-form-item label="Comment">
                  <a-input v-model:value="firstClient.comment" />
                </a-form-item>

                <a-form-item label="Total traffic (GB)">
                  <a-input-number v-model:value="clientTotalGB" :min="0" :step="0.1" />
                </a-form-item>

                <a-form-item label="Expiry">
                  <a-date-picker
                    v-model:value="clientExpiryDate"
                    :show-time="{ format: 'HH:mm:ss' }"
                    format="YYYY-MM-DD HH:mm:ss"
                    :style="{ width: '100%' }"
                  />
                </a-form-item>
              </a-form>
            </a-collapse-panel>
          </a-collapse>

          <a-collapse v-else>
            <a-collapse-panel
              key="summary"
              :header="`Clients: ${clientsArray.length}`"
            >
              <table class="client-summary">
                <thead>
                  <tr>
                    <th>Email</th>
                    <th>{{ protocol === Protocols.TROJAN || protocol === Protocols.SHADOWSOCKS ? 'Password' : (protocol === Protocols.HYSTERIA ? 'Auth' : 'ID') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(c, idx) in clientsArray" :key="idx">
                    <td>{{ c.email }}</td>
                    <td>{{ c.id || c.password || c.auth }}</td>
                  </tr>
                </tbody>
              </table>
            </a-collapse-panel>
          </a-collapse>
        </template>

        <!-- VLess decryption / encryption -->
        <a-form
          v-if="protocol === Protocols.VLESS"
          :colon="false"
          :label-col="{ md: { span: 8 } }"
          :wrapper-col="{ md: { span: 14 } }"
          class="mt-12"
        >
          <a-form-item label="Decryption">
            <a-input v-model:value="inbound.settings.decryption" />
          </a-form-item>
          <a-form-item label="Encryption">
            <a-input v-model:value="inbound.settings.encryption" />
          </a-form-item>
        </a-form>

        <!-- Shadowsocks shared fields (method/network/ivCheck) -->
        <a-form
          v-if="protocol === Protocols.SHADOWSOCKS"
          :colon="false"
          :label-col="{ md: { span: 8 } }"
          :wrapper-col="{ md: { span: 14 } }"
          class="mt-12"
        >
          <a-form-item label="Encryption method">
            <a-select v-model:value="inbound.settings.method" @change="onSSMethodChange">
              <a-select-option v-for="(m, k) in SSMethods" :key="k" :value="m">{{ k }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item v-if="inbound.isSS2022">
            <template #label>
              Password
              <SyncOutlined class="random-icon" @click="randomSSPassword(inbound.settings)" />
            </template>
            <a-input v-model:value="inbound.settings.password" />
          </a-form-item>
          <a-form-item label="Network">
            <a-select v-model:value="inbound.settings.network" :style="{ width: '120px' }">
              <a-select-option value="tcp,udp">TCP, UDP</a-select-option>
              <a-select-option value="tcp">TCP</a-select-option>
              <a-select-option value="udp">UDP</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="ivCheck">
            <a-switch v-model:checked="inbound.settings.ivCheck" />
          </a-form-item>
        </a-form>

        <!-- HTTP / Mixed accounts -->
        <a-form
          v-if="protocol === Protocols.HTTP || protocol === Protocols.MIXED"
          :colon="false"
          :label-col="{ md: { span: 8 } }"
          :wrapper-col="{ md: { span: 14 } }"
          class="mt-12"
        >
          <a-form-item label="Accounts">
            <a-button
              size="small"
              @click="protocol === Protocols.HTTP
                ? inbound.settings.addAccount(new Inbound.HttpSettings.HttpAccount())
                : inbound.settings.addAccount(new Inbound.MixedSettings.MixedAccount())"
            >
              <template #icon><PlusOutlined /></template>
              Add
            </a-button>
          </a-form-item>
          <a-form-item :wrapper-col="{ span: 24 }">
            <a-input-group v-for="(account, idx) in inbound.settings.accounts" :key="idx" compact class="mb-8">
              <a-input :style="{ width: '45%' }" v-model:value="account.user" placeholder="Username">
                <template #addonBefore>{{ idx + 1 }}</template>
              </a-input>
              <a-input :style="{ width: '45%' }" v-model:value="account.pass" placeholder="Password" />
              <a-button @click="inbound.settings.delAccount(idx)">
                <template #icon><MinusOutlined /></template>
              </a-button>
            </a-input-group>
          </a-form-item>
          <a-form-item v-if="protocol === Protocols.HTTP" label="Allow transparent">
            <a-switch v-model:checked="inbound.settings.allowTransparent" />
          </a-form-item>
          <template v-if="protocol === Protocols.MIXED">
            <a-form-item label="Auth">
              <a-select v-model:value="inbound.settings.auth">
                <a-select-option value="noauth">noauth</a-select-option>
                <a-select-option value="password">password</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="UDP">
              <a-switch v-model:checked="inbound.settings.udp" />
            </a-form-item>
            <a-form-item v-if="inbound.settings.udp" label="UDP IP">
              <a-input v-model:value="inbound.settings.ip" />
            </a-form-item>
          </template>
        </a-form>

        <!-- Tunnel -->
        <a-form
          v-if="protocol === Protocols.TUNNEL"
          :colon="false"
          :label-col="{ md: { span: 8 } }"
          :wrapper-col="{ md: { span: 14 } }"
          class="mt-12"
        >
          <a-form-item label="Address">
            <a-input v-model:value="inbound.settings.address" />
          </a-form-item>
          <a-form-item label="Destination port">
            <a-input-number v-model:value="inbound.settings.port" :min="1" :max="65535" />
          </a-form-item>
          <a-form-item label="Network">
            <a-select v-model:value="inbound.settings.network">
              <a-select-option value="tcp,udp">TCP, UDP</a-select-option>
              <a-select-option value="tcp">TCP</a-select-option>
              <a-select-option value="udp">UDP</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="Follow redirect">
            <a-switch v-model:checked="inbound.settings.followRedirect" />
          </a-form-item>
        </a-form>

        <!-- WireGuard -->
        <a-form
          v-if="protocol === Protocols.WIREGUARD"
          :colon="false"
          :label-col="{ md: { span: 8 } }"
          :wrapper-col="{ md: { span: 14 } }"
          class="mt-12"
        >
          <a-form-item>
            <template #label>
              Secret key <SyncOutlined class="random-icon" @click="regenInboundWg" />
            </template>
            <a-input v-model:value="inbound.settings.secretKey" />
          </a-form-item>
          <a-form-item label="Public key">
            <a-input v-model:value="inbound.settings.pubKey" disabled />
          </a-form-item>
          <a-form-item label="MTU">
            <a-input-number v-model:value="inbound.settings.mtu" />
          </a-form-item>
          <a-form-item label="No-kernel TUN">
            <a-switch v-model:checked="inbound.settings.noKernelTun" />
          </a-form-item>
          <a-form-item label="Peers">
            <a-button size="small" @click="inbound.settings.addPeer()">
              <template #icon><PlusOutlined /></template>
              Add peer
            </a-button>
          </a-form-item>
          <div v-for="(peer, idx) in inbound.settings.peers" :key="idx" class="wg-peer">
            <a-divider style="margin: 8px 0">
              Peer {{ idx + 1 }}
              <DeleteOutlined
                v-if="inbound.settings.peers.length > 1"
                class="danger-icon"
                @click="inbound.settings.delPeer(idx)"
              />
            </a-divider>
            <a-form-item>
              <template #label>
                Secret key <SyncOutlined class="random-icon" @click="regenWgKeypair(peer)" />
              </template>
              <a-input v-model:value="peer.privateKey" />
            </a-form-item>
            <a-form-item label="Public key">
              <a-input v-model:value="peer.publicKey" />
            </a-form-item>
            <a-form-item label="PSK">
              <a-input v-model:value="peer.psk" />
            </a-form-item>
            <a-form-item label="Allowed IPs">
              <a-button size="small" @click="peer.allowedIPs.push('')">
                <template #icon><PlusOutlined /></template>
              </a-button>
              <a-input
                v-for="(_ip, j) in peer.allowedIPs"
                :key="j"
                v-model:value="peer.allowedIPs[j]"
                class="mt-4"
              >
                <template #addonAfter>
                  <a-button v-if="peer.allowedIPs.length > 1" size="small" @click="peer.allowedIPs.splice(j, 1)">
                    <template #icon><MinusOutlined /></template>
                  </a-button>
                </template>
              </a-input>
            </a-form-item>
            <a-form-item label="Keep-alive">
              <a-input-number v-model:value="peer.keepAlive" :min="0" />
            </a-form-item>
          </div>
        </a-form>
      </a-tab-pane>

      <!-- ============================== STREAM ============================== -->
      <a-tab-pane v-if="canEnableStream" key="stream" tab="Stream">
        <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
          <a-form-item v-if="protocol !== Protocols.HYSTERIA" label="Transmission">
            <a-select v-model:value="network" :style="{ width: '75%' }">
              <a-select-option value="tcp">TCP (RAW)</a-select-option>
              <a-select-option value="kcp">mKCP</a-select-option>
              <a-select-option value="ws">WebSocket</a-select-option>
              <a-select-option value="grpc">gRPC</a-select-option>
              <a-select-option value="httpupgrade">HTTPUpgrade</a-select-option>
              <a-select-option value="xhttp">XHTTP</a-select-option>
            </a-select>
          </a-form-item>

          <!-- TCP — minimal: just the proxy-protocol toggle + http camouflage -->
          <template v-if="network === 'tcp'">
            <a-form-item v-if="canEnableTls" label="Proxy protocol">
              <a-switch v-model:checked="inbound.stream.tcp.acceptProxyProtocol" />
            </a-form-item>
            <a-form-item label="HTTP camouflage">
              <a-switch
                :checked="inbound.stream.tcp.type === 'http'"
                @change="(v) => (inbound.stream.tcp.type = v ? 'http' : 'none')"
              />
            </a-form-item>
            <a-alert
              v-if="inbound.stream.tcp.type === 'http'"
              type="info"
              show-icon
              message="HTTP camouflage detail (request/response paths/headers) lives in the Advanced tab for now."
              class="mt-8"
            />
          </template>

          <!-- WS -->
          <template v-if="network === 'ws'">
            <a-form-item label="Proxy protocol">
              <a-switch v-model:checked="inbound.stream.ws.acceptProxyProtocol" />
            </a-form-item>
            <a-form-item label="Host">
              <a-input v-model:value="inbound.stream.ws.host" />
            </a-form-item>
            <a-form-item label="Path">
              <a-input v-model:value="inbound.stream.ws.path" />
            </a-form-item>
            <a-form-item label="Heartbeat (s)">
              <a-input-number v-model:value="inbound.stream.ws.heartbeatPeriod" :min="0" />
            </a-form-item>
            <a-form-item label="Headers">
              <a-button size="small" @click="inbound.stream.ws.addHeader('', '')">
                <template #icon><PlusOutlined /></template>
              </a-button>
            </a-form-item>
            <a-form-item :wrapper-col="{ span: 24 }">
              <a-input-group v-for="(h, idx) in inbound.stream.ws.headers" :key="idx" compact class="mb-8">
                <a-input :style="{ width: '45%' }" v-model:value="h.name" placeholder="Name">
                  <template #addonBefore>{{ idx + 1 }}</template>
                </a-input>
                <a-input :style="{ width: '45%' }" v-model:value="h.value" placeholder="Value" />
                <a-button @click="inbound.stream.ws.removeHeader(idx)">
                  <template #icon><MinusOutlined /></template>
                </a-button>
              </a-input-group>
            </a-form-item>
          </template>

          <!-- gRPC -->
          <template v-if="network === 'grpc'">
            <a-form-item label="Service name">
              <a-input v-model:value="inbound.stream.grpc.serviceName" />
            </a-form-item>
            <a-form-item label="Multi mode">
              <a-switch v-model:checked="inbound.stream.grpc.multiMode" />
            </a-form-item>
          </template>

          <!-- HTTPUpgrade -->
          <template v-if="network === 'httpupgrade'">
            <a-form-item label="Host">
              <a-input v-model:value="inbound.stream.httpupgrade.host" />
            </a-form-item>
            <a-form-item label="Path">
              <a-input v-model:value="inbound.stream.httpupgrade.path" />
            </a-form-item>
          </template>

          <a-alert
            v-if="network === 'kcp' || network === 'xhttp'"
            type="info"
            show-icon
            :message="`${network.toUpperCase()} settings are still edited via the Advanced JSON tab.`"
            class="mt-12"
          />

          <!-- ====== Security section ====== -->
          <a-divider>Security</a-divider>
          <a-form-item label="Security">
            <a-select v-model:value="security" :style="{ width: '160px' }" :disabled="!canEnableTls">
              <a-select-option value="none">none</a-select-option>
              <a-select-option value="tls">tls</a-select-option>
              <a-select-option v-if="canEnableReality" value="reality">reality</a-select-option>
            </a-select>
          </a-form-item>

          <template v-if="security === 'tls' && inbound.stream.tls">
            <a-form-item label="SNI">
              <a-input v-model:value="inbound.stream.tls.sni" />
            </a-form-item>
            <a-form-item label="ALPN">
              <a-select
                v-model:value="inbound.stream.tls.alpn"
                mode="multiple"
                :style="{ width: '100%' }"
                :token-separators="[',']"
              >
                <a-select-option value="h2">h2</a-select-option>
                <a-select-option value="http/1.1">http/1.1</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="uTLS fingerprint">
              <a-select v-model:value="inbound.stream.tls.settings.fingerprint">
                <a-select-option value="">none</a-select-option>
                <a-select-option v-for="fp in ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random']" :key="fp" :value="fp">{{ fp }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-alert
              type="info"
              show-icon
              message="TLS certificate paths/configs and ECH settings live in the Advanced JSON tab."
              class="mt-8"
            />
          </template>

          <template v-if="security === 'reality' && inbound.stream.reality">
            <a-form-item label="Show">
              <a-switch v-model:checked="inbound.stream.reality.show" />
            </a-form-item>
            <a-form-item label="Target">
              <a-input v-model:value="inbound.stream.reality.target" placeholder="e.g. example.com:443" />
            </a-form-item>
            <a-form-item label="Server names">
              <a-select
                v-model:value="inbound.stream.reality.serverNames"
                mode="tags"
                :token-separators="[',']"
                :style="{ width: '100%' }"
              />
            </a-form-item>
            <a-form-item label="Private key">
              <a-input v-model:value="inbound.stream.reality.privateKey" />
            </a-form-item>
            <a-form-item label="Public key">
              <a-input v-model:value="inbound.stream.reality.settings.publicKey" />
            </a-form-item>
            <a-form-item :wrapper-col="{ span: 24 }">
              <a-button type="primary" :loading="saving" @click="genRealityKeypair">
                Generate X25519 keypair
              </a-button>
            </a-form-item>
            <a-form-item label="Short IDs">
              <a-select
                v-model:value="inbound.stream.reality.shortIds"
                mode="tags"
                :token-separators="[',']"
                :style="{ width: '100%' }"
              />
            </a-form-item>
            <a-form-item label="Fingerprint">
              <a-select v-model:value="inbound.stream.reality.settings.fingerprint">
                <a-select-option v-for="fp in ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random']" :key="fp" :value="fp">{{ fp }}</a-select-option>
              </a-select>
            </a-form-item>
          </template>
        </a-form>
      </a-tab-pane>

      <!-- ============================== SNIFFING ============================== -->
      <a-tab-pane key="sniffing" tab="Sniffing">
        <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
          <a-form-item label="Enabled">
            <a-switch v-model:checked="inbound.sniffing.enabled" />
          </a-form-item>
          <template v-if="inbound.sniffing.enabled">
            <a-form-item :wrapper-col="{ span: 24 }">
              <a-checkbox-group v-model:value="inbound.sniffing.destOverride">
                <a-checkbox v-for="(value, key) in SNIFFING_OPTION" :key="key" :value="value">{{ key }}</a-checkbox>
              </a-checkbox-group>
            </a-form-item>
            <a-form-item label="Metadata only">
              <a-switch v-model:checked="inbound.sniffing.metadataOnly" />
            </a-form-item>
            <a-form-item label="Route only">
              <a-switch v-model:checked="inbound.sniffing.routeOnly" />
            </a-form-item>
            <a-form-item label="IPs excluded">
              <a-select
                v-model:value="inbound.sniffing.ipsExcluded"
                mode="tags"
                :token-separators="[',']"
                placeholder="IP/CIDR/geoip:*/ext:*"
                :style="{ width: '100%' }"
              />
            </a-form-item>
            <a-form-item label="Domains excluded">
              <a-select
                v-model:value="inbound.sniffing.domainsExcluded"
                mode="tags"
                :token-separators="[',']"
                placeholder="domain:*/ext:*"
                :style="{ width: '100%' }"
              />
            </a-form-item>
          </template>
        </a-form>
      </a-tab-pane>

      <!-- ============================== ADVANCED ============================== -->
      <a-tab-pane key="advanced" tab="Advanced (JSON)">
        <a-alert
          type="info"
          show-icon
          message="Edit raw stream JSON to access KCP/XHTTP/sockopt/finalmask and full TLS cert configs."
          class="mb-12"
        />
        <a-form layout="vertical">
          <a-form-item label="streamSettings">
            <a-textarea
              v-model:value="advancedJson.stream"
              :auto-size="{ minRows: 10, maxRows: 24 }"
              spellcheck="false"
              class="json-editor"
            />
          </a-form-item>
          <a-form-item label="sniffing (overrides the Sniffing tab when set)">
            <a-textarea
              v-model:value="advancedJson.sniffing"
              :auto-size="{ minRows: 6, maxRows: 16 }"
              spellcheck="false"
              class="json-editor"
            />
          </a-form-item>
        </a-form>
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<style scoped>
.mt-4 { margin-top: 4px; }
.mt-8 { margin-top: 8px; }
.mt-12 { margin-top: 12px; }
.mb-8 { margin-bottom: 8px; }
.mb-12 { margin-bottom: 12px; }

.random-icon {
  margin-left: 4px;
  cursor: pointer;
  color: var(--ant-primary-color, #1890ff);
}

.danger-icon {
  margin-left: 6px;
  cursor: pointer;
  color: #ff4d4f;
}

.json-editor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.client-summary {
  width: 100%;
  border-collapse: collapse;
}
.client-summary th,
.client-summary td {
  padding: 4px 8px;
  text-align: left;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
}

.wg-peer {
  margin-top: 4px;
}
</style>
