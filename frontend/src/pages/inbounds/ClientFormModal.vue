<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import { SyncOutlined, RetweetOutlined, DeleteOutlined } from '@ant-design/icons-vue';

import {
  HttpUtil,
  RandomUtil,
  SizeFormatter,
  ColorUtils,
} from '@/utils';
import { Inbound, Protocols, USERS_SECURITY, TLS_FLOW_CONTROL } from '@/models/inbound.js';
import DateTimePicker from '@/components/DateTimePicker.vue';

const { t } = useI18n();

// Add OR edit a single client on a multi-user inbound (VMess / VLess /
// Trojan / Shadowsocks-multi / Hysteria). The legacy panel routes both
// flows through the same modal — same here.
//
// On submit we serialize the client via its toString() (which is just
// JSON.stringify of toJson()) and post it inside a one-element clients
// array so the Go side reuses the same parsing path as the inbound
// settings update.

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add', validator: (v) => ['add', 'edit'].includes(v) },
  dbInbound: { type: Object, default: null },
  clientIndex: { type: Number, default: null },
  // Sidecar config from the inbounds page — controls visibility of
  // the Subscription, Telegram, and IP-limit fields.
  subEnable: { type: Boolean, default: false },
  tgBotEnable: { type: Boolean, default: false },
  ipLimitEnable: { type: Boolean, default: false },
  trafficDiff: { type: Number, default: 0 },
});

const emit = defineEmits(['update:open', 'saved']);

// === Reactive draft =================================================
const inbound = ref(null);
const client = ref(null);
const oldClientId = ref('');
const clientStats = ref(null);

const saving = ref(false);
const delayedStart = ref(false);

const SECURITY_OPTIONS = Object.values(USERS_SECURITY);
const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const protocol = computed(() => inbound.value?.protocol);
const isVmessOrVless = computed(() =>
  protocol.value === Protocols.VMESS || protocol.value === Protocols.VLESS,
);
const isTrojanOrSS = computed(() =>
  protocol.value === Protocols.TROJAN || protocol.value === Protocols.SHADOWSOCKS,
);

const expiryDate = computed({
  get: () => (client.value?.expiryTime > 0 ? dayjs(client.value.expiryTime) : null),
  set: (next) => { if (client.value) client.value.expiryTime = next ? next.valueOf() : 0; },
});

const delayedExpireDays = computed({
  get: () => {
    if (!client.value || client.value.expiryTime >= 0) return 0;
    return client.value.expiryTime / -86400000;
  },
  set: (days) => {
    if (!client.value) return;
    client.value.expiryTime = -86400000 * (days || 0);
  },
});

const totalGB = computed({
  get: () => {
    if (!client.value || !client.value.totalGB) return 0;
    return Math.round((client.value.totalGB / SizeFormatter.ONE_GB) * 100) / 100;
  },
  set: (gb) => {
    if (!client.value) return;
    client.value.totalGB = Math.round((gb || 0) * SizeFormatter.ONE_GB);
  },
});

const isExpired = computed(() => {
  if (props.mode !== 'edit' || !client.value) return false;
  return client.value.expiryTime > 0 && client.value.expiryTime < Date.now();
});
const isTrafficExhausted = computed(() => {
  if (!clientStats.value || clientStats.value.total <= 0) return false;
  return clientStats.value.up + clientStats.value.down >= clientStats.value.total;
});

function getClientId(proto, c) {
  switch (proto) {
    case Protocols.TROJAN: return c.password;
    case Protocols.SHADOWSOCKS: return c.email;
    case Protocols.HYSTERIA: return c.auth;
    default: return c.id;
  }
}

function makeNewClient(proto, parsed) {
  switch (proto) {
    case Protocols.VMESS: return new Inbound.VmessSettings.VMESS();
    case Protocols.VLESS: return new Inbound.VLESSSettings.VLESS();
    case Protocols.TROJAN: return new Inbound.TrojanSettings.Trojan();
    case Protocols.SHADOWSOCKS: {
      const method = parsed.settings.method;
      return new Inbound.ShadowsocksSettings.Shadowsocks(
        method,
        RandomUtil.randomShadowsocksPassword(method),
      );
    }
    case Protocols.HYSTERIA: return new Inbound.HysteriaSettings.Hysteria();
    default: return null;
  }
}

watch(() => props.open, (next) => {
  if (!next) return;
  if (!props.dbInbound) return;
  const parsed = Inbound.fromJson(props.dbInbound.toInbound().toJson());
  inbound.value = parsed;
  delayedStart.value = false;

  if (props.mode === 'edit') {
    const idx = props.clientIndex ?? 0;
    client.value = parsed.clients[idx];
    if (client.value && client.value.expiryTime < 0) delayedStart.value = true;
    oldClientId.value = getClientId(parsed.protocol, client.value);
  } else {
    const c = makeNewClient(parsed.protocol, parsed);
    if (c) parsed.clients.push(c);
    client.value = parsed.clients[parsed.clients.length - 1];
    oldClientId.value = '';
  }

  clientStats.value = (props.dbInbound.clientStats || []).find(
    (s) => s.email === client.value?.email,
  ) || null;
});

function close() {
  emit('update:open', false);
}

function randomEmail() {
  if (client.value) client.value.email = RandomUtil.randomLowerAndNum(9);
}
function randomId() {
  if (client.value) client.value.id = RandomUtil.randomUUID();
}
function randomPassword() {
  if (!client.value || !inbound.value) return;
  if (inbound.value.protocol === Protocols.SHADOWSOCKS) {
    client.value.password = RandomUtil.randomShadowsocksPassword(
      inbound.value.settings.method,
    );
  } else {
    client.value.password = RandomUtil.randomSeq(10);
  }
}
function randomAuth() {
  if (client.value) client.value.auth = RandomUtil.randomSeq(10);
}
function randomSubId() {
  if (client.value) client.value.subId = RandomUtil.randomLowerAndNum(16);
}

const clientIpsText = ref('');
async function loadClientIps() {
  if (!client.value?.email) return;
  const msg = await HttpUtil.post(`/panel/api/inbounds/clientIps/${client.value.email}`);
  if (!msg?.success) {
    clientIpsText.value = msg?.obj || '';
    return;
  }
  let ips = msg.obj;
  if (typeof ips === 'string' && ips.startsWith('[') && ips.endsWith(']')) {
    try {
      const parsed = JSON.parse(ips);
      ips = Array.isArray(parsed) ? parsed.join('\n') : ips;
    } catch (_e) {
      // leave as raw
    }
  }
  clientIpsText.value = ips || '';
}
async function clearClientIps() {
  if (!client.value?.email) return;
  const msg = await HttpUtil.post(`/panel/api/inbounds/clearClientIps/${client.value.email}`);
  if (msg?.success) clientIpsText.value = '';
}

async function resetClientTraffic() {
  if (!clientStats.value || !client.value?.email) return;
  const msg = await HttpUtil.post(
    `/panel/api/inbounds/${props.dbInbound.id}/resetClientTraffic/${client.value.email}`,
  );
  if (msg?.success) {
    clientStats.value.up = 0;
    clientStats.value.down = 0;
  }
}

async function submit() {
  if (!client.value || !inbound.value) return;
  saving.value = true;
  try {
    const payload = {
      id: props.dbInbound.id,
      settings: `{"clients": [${client.value.toString()}]}`,
    };
    const url = props.mode === 'edit'
      ? `/panel/api/inbounds/updateClient/${oldClientId.value}`
      : '/panel/api/inbounds/addClient';
    const msg = await HttpUtil.post(url, payload);
    if (msg?.success) {
      emit('saved');
      close();
    }
  } finally {
    saving.value = false;
  }
}

const title = computed(() =>
  props.mode === 'edit' ? t('pages.client.edit') : t('pages.client.add'),
);
</script>

<template>
  <a-modal :open="open" :title="title"
    :ok-text="mode === 'edit' ? t('pages.client.submitEdit') : t('pages.client.submitAdd')" :cancel-text="t('close')"
    :confirm-loading="saving" :mask-closable="false" @ok="submit" @cancel="close">
    <a-tag v-if="mode === 'edit' && (isExpired || isTrafficExhausted)" color="red" class="status-banner">
      {{ t('depleted') }}
    </a-tag>

    <a-form v-if="client && inbound" layout="horizontal" :colon="false" :label-col="{ sm: { span: 8 } }"
      :wrapper-col="{ sm: { span: 14 } }">
      <a-form-item :label="t('enable')">
        <a-switch v-model:checked="client.enable" />
      </a-form-item>

      <a-form-item>
        <template #label>
          {{ t('pages.inbounds.email') }}
          <SyncOutlined class="random-icon" @click="randomEmail" />
        </template>
        <a-input v-model:value="client.email" />
      </a-form-item>

      <a-form-item v-if="isTrojanOrSS">
        <template #label>
          {{ t('password') }}
          <SyncOutlined class="random-icon" @click="randomPassword" />
        </template>
        <a-input v-model:value="client.password" />
      </a-form-item>

      <a-form-item v-if="protocol === Protocols.HYSTERIA">
        <template #label>
          {{ t('password') }}
          <SyncOutlined class="random-icon" @click="randomAuth" />
        </template>
        <a-input v-model:value="client.auth" />
      </a-form-item>

      <a-form-item v-if="isVmessOrVless">
        <template #label>
          ID
          <SyncOutlined class="random-icon" @click="randomId" />
        </template>
        <a-input v-model:value="client.id" />
      </a-form-item>

      <a-form-item v-if="protocol === Protocols.VMESS" :label="t('security')">
        <a-select v-model:value="client.security">
          <a-select-option v-for="key in SECURITY_OPTIONS" :key="key" :value="key">
            {{ key }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="client.email && subEnable">
        <template #label>
          {{ t('subscription.title') }}
          <SyncOutlined class="random-icon" @click="randomSubId" />
        </template>
        <a-input v-model:value="client.subId" />
      </a-form-item>

      <a-form-item v-if="client.email && tgBotEnable" label="Telegram ID">
        <a-input-number v-model:value="client.tgId" :min="0" :style="{ width: '50%' }" />
      </a-form-item>

      <a-form-item v-if="client.email" :label="t('comment')">
        <a-input v-model:value="client.comment" />
      </a-form-item>

      <a-form-item v-if="ipLimitEnable" :label="t('pages.inbounds.IPLimit')">
        <a-input-number v-model:value="client.limitIp" :min="0" />
      </a-form-item>

      <a-form-item v-if="ipLimitEnable && client.limitIp > 0 && client.email && mode === 'edit'"
        :label="t('pages.inbounds.IPLimitlog')">
        <a-textarea v-model:value="clientIpsText" readonly :placeholder="t('pages.inbounds.IPLimitlogDesc')"
          :auto-size="{ minRows: 3, maxRows: 8 }" @click="loadClientIps" />
        <a-button type="link" size="small" danger @click="clearClientIps">
          <template #icon>
            <DeleteOutlined />
          </template>
          {{ t('pages.inbounds.IPLimitlogclear') }}
        </a-button>
      </a-form-item>

      <a-form-item v-if="inbound.canEnableTlsFlow()" label="Flow">
        <a-select v-model:value="client.flow">
          <a-select-option value="">{{ t('none') }}</a-select-option>
          <a-select-option v-for="key in FLOW_OPTIONS" :key="key" :value="key">
            {{ key }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="protocol === Protocols.VLESS" label="Reverse tag">
        <a-input v-model:value="client.reverseTag" placeholder="Optional reverse tag" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip :title="t('pages.inbounds.meansNoLimit')">{{ t('pages.inbounds.totalFlow') }}</a-tooltip>
        </template>
        <a-input-number v-model:value="totalGB" :min="0" :step="0.1" />
      </a-form-item>

      <a-form-item v-if="mode === 'edit' && clientStats" :label="t('usage')">
        <a-tag :color="ColorUtils.clientUsageColor(clientStats, trafficDiff)">
          {{ SizeFormatter.sizeFormat(clientStats.up) }} /
          {{ SizeFormatter.sizeFormat(clientStats.down) }}
          ({{ SizeFormatter.sizeFormat(clientStats.up + clientStats.down) }})
        </a-tag>
        <a-tooltip v-if="client.email" :title="t('pages.inbounds.resetTraffic')">
          <RetweetOutlined class="action-icon" @click="resetClientTraffic" />
        </a-tooltip>
      </a-form-item>

      <a-form-item :label="t('pages.client.delayedStart')">
        <a-switch v-model:checked="delayedStart" @click="client.expiryTime = 0" />
      </a-form-item>

      <a-form-item v-if="delayedStart" :label="t('pages.client.expireDays')">
        <a-input-number v-model:value="delayedExpireDays" :min="0" />
      </a-form-item>

      <a-form-item v-else>
        <template #label>
          <a-tooltip :title="t('pages.inbounds.leaveBlankToNeverExpire')">{{ t('pages.inbounds.expireDate')
          }}</a-tooltip>
        </template>
        <DateTimePicker v-model:value="expiryDate" />
        <a-tag v-if="mode === 'edit' && isExpired" color="red">{{ t('depleted') }}</a-tag>
      </a-form-item>

      <a-form-item v-if="client.expiryTime !== 0">
        <template #label>
          <a-tooltip :title="t('pages.client.renewDesc')">{{ t('pages.client.renew') }}</a-tooltip>
        </template>
        <a-input-number v-model:value="client.reset" :min="0" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<style scoped>
.status-banner {
  display: block;
  margin-bottom: 10px;
  text-align: center;
}

.random-icon,
.action-icon {
  margin-left: 4px;
  cursor: pointer;
  color: var(--ant-primary-color, #1890ff);
}
</style>
