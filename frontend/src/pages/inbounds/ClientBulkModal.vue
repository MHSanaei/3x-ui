<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import { SyncOutlined } from '@ant-design/icons-vue';

import { HttpUtil, RandomUtil, SizeFormatter } from '@/utils';

const { t } = useI18n();
import {
  Inbound,
  Protocols,
  USERS_SECURITY,
  TLS_FLOW_CONTROL,
} from '@/models/inbound.js';
import DateTimePicker from '@/components/DateTimePicker.vue';

// Bulk-add up to 500 clients in one go. The legacy panel offers five
// generation modes — this component preserves them all:
//   0: Random         — N fully-random emails (no prefix)
//   1: Random+Prefix  — N random emails preceded by `prefix`
//   2: Random+Prefix+Num     — emails like `<rand><prefix><num>` for num in [first..last]
//   3: Random+Prefix+Num+Postfix — same + appended postfix
//   4: Prefix+Num+Postfix    — no random part, just `<prefix><num><postfix>`

const props = defineProps({
  open: { type: Boolean, default: false },
  dbInbound: { type: Object, default: null },
  subEnable: { type: Boolean, default: false },
  tgBotEnable: { type: Boolean, default: false },
  ipLimitEnable: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open', 'saved']);

const SECURITY_OPTIONS = Object.values(USERS_SECURITY);
const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

// === Reactive form state ===========================================
// Cloned inbound (so canEnableTlsFlow() works).
const inbound = ref(null);
const saving = ref(false);
const delayedStart = ref(false);

const form = reactive({
  emailMethod: 0,
  firstNum: 1,
  lastNum: 1,
  emailPrefix: '',
  emailPostfix: '',
  quantity: 1,
  security: USERS_SECURITY.AUTO,
  flow: '',
  subId: '',
  tgId: 0,
  comment: '',
  limitIp: 0,
  totalGB: 0,
  expiryTime: 0, // ms epoch; negative => delayed start days
  reset: 0,
});

const expiryDate = computed({
  get: () => (form.expiryTime > 0 ? dayjs(form.expiryTime) : null),
  set: (next) => { form.expiryTime = next ? next.valueOf() : 0; },
});

const delayedExpireDays = computed({
  get: () => (form.expiryTime < 0 ? form.expiryTime / -86400000 : 0),
  set: (days) => { form.expiryTime = -86400000 * (days || 0); },
});

watch(() => props.open, (next) => {
  if (!next) return;
  if (!props.dbInbound) return;
  inbound.value = Inbound.fromJson(props.dbInbound.toInbound().toJson());
  // Reset all form fields on every open — bulk add is intentionally
  // stateless between sessions (legacy resets on .show()).
  form.emailMethod = 0;
  form.firstNum = 1;
  form.lastNum = 1;
  form.emailPrefix = '';
  form.emailPostfix = '';
  form.quantity = 1;
  form.security = USERS_SECURITY.AUTO;
  form.flow = '';
  form.subId = '';
  form.tgId = 0;
  form.comment = '';
  form.limitIp = 0;
  form.totalGB = 0;
  form.expiryTime = 0;
  form.reset = 0;
  delayedStart.value = false;
});

function close() {
  emit('update:open', false);
}

function makeNewClient(parsed) {
  switch (parsed.protocol) {
    case Protocols.VMESS: return new Inbound.VmessSettings.VMESS();
    case Protocols.VLESS: return new Inbound.VLESSSettings.VLESS();
    case Protocols.TROJAN: return new Inbound.TrojanSettings.Trojan();
    case Protocols.SHADOWSOCKS: {
      const method = parsed.settings.shadowsockses[0]?.method || parsed.settings.method;
      return new Inbound.ShadowsocksSettings.Shadowsocks(method);
    }
    case Protocols.HYSTERIA: return new Inbound.HysteriaSettings.Hysteria();
    default: return null;
  }
}

function buildClients() {
  if (!inbound.value) return [];
  const out = [];
  const method = form.emailMethod;
  let start;
  let end;
  if (method > 1) {
    start = form.firstNum;
    end = form.lastNum + 1;
  } else {
    start = 0;
    end = form.quantity;
  }
  const prefix = method > 0 && form.emailPrefix.length > 0 ? form.emailPrefix : '';
  const useNum = method > 1;
  const postfix = method > 2 && form.emailPostfix.length > 0 ? form.emailPostfix : '';

  for (let i = start; i < end; i++) {
    const c = makeNewClient(inbound.value);
    if (!c) continue;
    if (method === 4) c.email = '';
    c.email += useNum ? prefix + String(i) + postfix : prefix + postfix;

    if (form.subId.length > 0) c.subId = form.subId;
    c.tgId = form.tgId;
    if (form.comment.length > 0) c.comment = form.comment;
    c.security = form.security;
    c.limitIp = form.limitIp;
    // Use the clien's totalGB setter (ms epoch and bytes already handled
    // identically for bulk and single client paths).
    c.totalGB = Math.round((form.totalGB || 0) * SizeFormatter.ONE_GB);
    c.expiryTime = form.expiryTime;
    if (inbound.value.canEnableTlsFlow()) c.flow = form.flow;
    c.reset = form.reset;
    out.push(c);
  }
  return out;
}

async function submit() {
  const clients = buildClients();
  if (clients.length === 0) return;

  saving.value = true;
  try {
    const payload = {
      id: props.dbInbound.id,
      // Clients all serialize via toString() — same shape the single-
      // client modal posts. Joining with `,` lets the Go side parse the
      // outer array directly.
      settings: `{"clients": [${clients.map((c) => c.toString()).join(',')}]}`,
    };
    const msg = await HttpUtil.post('/panel/api/inbounds/addClient', payload);
    if (msg?.success) {
      emit('saved');
      close();
    }
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <a-modal :open="open" :title="t('pages.client.bulk')" :ok-text="t('create')" :cancel-text="t('close')"
    :confirm-loading="saving" :mask-closable="false" @ok="submit" @cancel="close">
    <a-form v-if="inbound" :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }">
      <a-form-item :label="t('pages.client.method')">
        <a-select v-model:value="form.emailMethod">
          <a-select-option :value="0">Random</a-select-option>
          <a-select-option :value="1">Random + Prefix</a-select-option>
          <a-select-option :value="2">Random + Prefix + Num</a-select-option>
          <a-select-option :value="3">Random + Prefix + Num + Postfix</a-select-option>
          <a-select-option :value="4">Prefix + Num + Postfix</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="form.emailMethod > 1" :label="t('pages.client.first')">
        <a-input-number v-model:value="form.firstNum" :min="1" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod > 1" :label="t('pages.client.last')">
        <a-input-number v-model:value="form.lastNum" :min="form.firstNum" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod > 0" :label="t('pages.client.prefix')">
        <a-input v-model:value="form.emailPrefix" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod > 2" :label="t('pages.client.postfix')">
        <a-input v-model:value="form.emailPostfix" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod < 2" :label="t('pages.client.clientCount')">
        <a-input-number v-model:value="form.quantity" :min="1" :max="500" />
      </a-form-item>

      <a-form-item v-if="inbound.protocol === Protocols.VMESS" :label="t('security')">
        <a-select v-model:value="form.security">
          <a-select-option v-for="key in SECURITY_OPTIONS" :key="key" :value="key">{{ key }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="inbound.canEnableTlsFlow()" label="Flow">
        <a-select v-model:value="form.flow">
          <a-select-option value="">{{ t('none') }}</a-select-option>
          <a-select-option v-for="key in FLOW_OPTIONS" :key="key" :value="key">{{ key }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="subEnable">
        <template #label>
          {{ t('subscription.title') }}
          <SyncOutlined class="random-icon" @click="form.subId = RandomUtil.randomLowerAndNum(16)" />
        </template>
        <a-input v-model:value="form.subId" />
      </a-form-item>

      <a-form-item v-if="tgBotEnable" label="Telegram ID">
        <a-input-number v-model:value="form.tgId" :min="0" :style="{ width: '50%' }" />
      </a-form-item>

      <a-form-item :label="t('comment')">
        <a-input v-model:value="form.comment" />
      </a-form-item>

      <a-form-item v-if="ipLimitEnable" :label="t('pages.inbounds.IPLimit')">
        <a-input-number v-model:value="form.limitIp" :min="0" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip :title="t('pages.inbounds.meansNoLimit')">{{ t('pages.inbounds.totalFlow') }}</a-tooltip>
        </template>
        <a-input-number v-model:value="form.totalGB" :min="0" :step="0.1" />
      </a-form-item>

      <a-form-item :label="t('pages.client.delayedStart')">
        <a-switch v-model:checked="delayedStart" @click="form.expiryTime = 0" />
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
      </a-form-item>

      <a-form-item v-if="form.expiryTime !== 0">
        <template #label>
          <a-tooltip :title="t('pages.client.renewDesc')">{{ t('pages.client.renew') }}</a-tooltip>
        </template>
        <a-input-number v-model:value="form.reset" :min="0" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<style scoped>
.random-icon {
  margin-left: 4px;
  cursor: pointer;
  color: var(--ant-primary-color, #1890ff);
}
</style>
