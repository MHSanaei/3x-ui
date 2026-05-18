<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import { SyncOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { HttpUtil, RandomUtil, SizeFormatter } from '@/utils';
import DateTimePicker from '@/components/DateTimePicker.vue';
import { TLS_FLOW_CONTROL } from '@/models/inbound.js';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  inbounds: { type: Array, default: () => [] },
  ipLimitEnable: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open', 'saved']);

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

const saving = ref(false);
const delayedStart = ref(false);

const form = reactive({
  emailMethod: 0,
  firstNum: 1,
  lastNum: 1,
  emailPrefix: '',
  emailPostfix: '',
  quantity: 1,
  subId: '',
  comment: '',
  flow: '',
  limitIp: 0,
  totalGB: 0,
  expiryTime: 0,
  inboundIds: [],
});

const flowCapableIds = computed(() => {
  const ids = new Set();
  for (const row of props.inbounds || []) {
    if (row?.tlsFlowCapable) ids.add(row.id);
  }
  return ids;
});

const showFlow = computed(() =>
  (form.inboundIds || []).some((id) => flowCapableIds.value.has(id)),
);

watch(showFlow, (next) => {
  if (!next) form.flow = '';
});

const expiryDate = computed({
  get: () => (form.expiryTime > 0 ? dayjs(form.expiryTime) : null),
  set: (next) => { form.expiryTime = next ? next.valueOf() : 0; },
});

const delayedExpireDays = computed({
  get: () => (form.expiryTime < 0 ? form.expiryTime / -86400000 : 0),
  set: (days) => { form.expiryTime = -86400000 * (days || 0); },
});

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria', 'hysteria2',
]);

const inboundOptions = computed(() =>
  (props.inbounds || [])
    .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol))
    .map((ib) => ({
      label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
      value: ib.id,
    })),
);

watch(() => props.open, (next) => {
  if (!next) return;
  form.emailMethod = 0;
  form.firstNum = 1;
  form.lastNum = 1;
  form.emailPrefix = '';
  form.emailPostfix = '';
  form.quantity = 1;
  form.subId = '';
  form.comment = '';
  form.flow = '';
  form.limitIp = 0;
  form.totalGB = 0;
  form.expiryTime = 0;
  form.inboundIds = [];
  delayedStart.value = false;
});

function close() {
  emit('update:open', false);
}

function buildEmails() {
  const method = form.emailMethod;
  const out = [];
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
    let email = '';
    if (method !== 4) email = RandomUtil.randomLowerAndNum(6);
    email += useNum ? prefix + String(i) + postfix : prefix + postfix;
    out.push(email);
  }
  return out;
}

async function submit() {
  if (!Array.isArray(form.inboundIds) || form.inboundIds.length === 0) {
    message.error(t('pages.clients.selectInbound'));
    return;
  }
  const emails = buildEmails();
  if (emails.length === 0) return;

  saving.value = true;
  const silentJsonOpts = { ...JSON_HEADERS, silent: true };
  try {
    const results = await Promise.all(emails.map((email) => {
      const client = {
        email,
        subId: form.subId || RandomUtil.randomLowerAndNum(16),
        id: RandomUtil.randomUUID(),
        password: RandomUtil.randomLowerAndNum(16),
        auth: RandomUtil.randomLowerAndNum(16),
        flow: showFlow.value ? (form.flow || '') : '',
        totalGB: Math.round((form.totalGB || 0) * SizeFormatter.ONE_GB),
        expiryTime: form.expiryTime,
        limitIp: Number(form.limitIp) || 0,
        comment: form.comment,
        enable: true,
      };
      const payload = { client, inboundIds: form.inboundIds };
      return HttpUtil.post('/panel/api/clients/add', payload, silentJsonOpts);
    }));
    let ok = 0;
    let failed = 0;
    let firstError = '';
    for (const msg of results) {
      if (msg?.success) ok++;
      else {
        failed++;
        if (!firstError && msg?.msg) firstError = msg.msg;
      }
    }
    if (failed === 0) {
      message.success(t('pages.clients.toasts.bulkCreated', { count: ok }));
    } else {
      message.warning(firstError
        ? `${t('pages.clients.toasts.bulkCreatedMixed', { ok, failed })} — ${firstError}`
        : t('pages.clients.toasts.bulkCreatedMixed', { ok, failed }));
    }
    emit('saved');
    close();
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <a-modal :open="open" :title="t('pages.clients.bulk')" :ok-text="t('create')" :cancel-text="t('close')"
    :confirm-loading="saving" :mask-closable="false" :width="640" @ok="submit" @cancel="close">
    <a-form :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }">
      <a-form-item :label="t('pages.clients.attachedInbounds')" required>
        <a-select v-model:value="form.inboundIds" mode="multiple" :options="inboundOptions"
          :placeholder="t('pages.clients.selectInbound')" :show-search="true"
          :filter-option="(input, option) => (option.label || '').toLowerCase().includes(input.toLowerCase())" />
      </a-form-item>

      <a-form-item :label="t('pages.clients.method')">
        <a-select v-model:value="form.emailMethod">
          <a-select-option :value="0">Random</a-select-option>
          <a-select-option :value="1">Random + Prefix</a-select-option>
          <a-select-option :value="2">Random + Prefix + Num</a-select-option>
          <a-select-option :value="3">Random + Prefix + Num + Postfix</a-select-option>
          <a-select-option :value="4">Prefix + Num + Postfix</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="form.emailMethod > 1" :label="t('pages.clients.first')">
        <a-input-number v-model:value="form.firstNum" :min="1" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod > 1" :label="t('pages.clients.last')">
        <a-input-number v-model:value="form.lastNum" :min="form.firstNum" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod > 0" :label="t('pages.clients.prefix')">
        <a-input v-model:value="form.emailPrefix" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod > 2" :label="t('pages.clients.postfix')">
        <a-input v-model:value="form.emailPostfix" />
      </a-form-item>
      <a-form-item v-if="form.emailMethod < 2" :label="t('pages.clients.clientCount')">
        <a-input-number v-model:value="form.quantity" :min="1" :max="100" />
      </a-form-item>

      <a-form-item>
        <template #label>
          {{ t('subscription.title') }}
          <SyncOutlined class="random-icon" @click="form.subId = RandomUtil.randomLowerAndNum(16)" />
        </template>
        <a-input v-model:value="form.subId" />
      </a-form-item>

      <a-form-item :label="t('comment')">
        <a-input v-model:value="form.comment" />
      </a-form-item>

      <a-form-item v-if="showFlow" :label="t('pages.clients.flow')">
        <a-select v-model:value="form.flow" :style="{ width: '220px' }">
          <a-select-option value="">{{ t('none') }}</a-select-option>
          <a-select-option v-for="k in FLOW_OPTIONS" :key="k" :value="k">{{ k }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="ipLimitEnable" :label="t('pages.clients.limitIp')">
        <a-input-number v-model:value="form.limitIp" :min="0" />
      </a-form-item>

      <a-form-item :label="t('pages.clients.totalGB')">
        <a-input-number v-model:value="form.totalGB" :min="0" :step="0.1" />
      </a-form-item>

      <a-form-item :label="t('pages.clients.delayedStart')">
        <a-switch v-model:checked="delayedStart" @click="form.expiryTime = 0" />
      </a-form-item>

      <a-form-item v-if="delayedStart" :label="t('pages.clients.expireDays')">
        <a-input-number v-model:value="delayedExpireDays" :min="0" />
      </a-form-item>

      <a-form-item v-else :label="t('pages.inbounds.expireDate')">
        <DateTimePicker v-model:value="expiryDate" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<style scoped>
.random-icon {
  margin-left: 4px;
  cursor: pointer;
  color: var(--ant-color-primary, #1677ff);
}
</style>
