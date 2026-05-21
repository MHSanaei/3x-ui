<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';
import dayjs from 'dayjs';
import { HttpUtil, RandomUtil } from '@/utils';
import { TLS_FLOW_CONTROL } from '@/models/inbound.js';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add' },
  client: { type: Object, default: null },
  inbounds: { type: Array, default: () => [] },
  attachedIds: { type: Array, default: () => [] },
  ipLimitEnable: { type: Boolean, default: false },
  tgBotEnable: { type: Boolean, default: false },
  save: { type: Function, required: true },
});

const emit = defineEmits(['update:open']);
const { t } = useI18n();

const submitting = ref(false);
const form = reactive(emptyForm());

function emptyForm() {
  return {
    email: '',
    subId: '',
    uuid: '',
    password: '',
    auth: '',
    flow: '',
    reverseTag: '',
    totalGB: 0,
    expiryDate: null,
    delayedStart: false,
    delayedDays: 0,
    limitIp: 0,
    tgId: 0,
    comment: '',
    enable: true,
    inboundIds: [],
  };
}

const isEdit = computed(() => props.mode === 'edit');

watch(
  () => props.open,
  (next) => {
    if (!next) return;
    Object.assign(form, emptyForm());
    if (isEdit.value && props.client) {
      form.email = props.client.email || '';
      form.subId = props.client.subId || '';
      form.uuid = props.client.uuid || '';
      form.password = props.client.password || '';
      form.auth = props.client.auth || '';
      form.flow = props.client.flow || '';
      form.reverseTag = props.client.reverse?.tag || '';
      form.totalGB = bytesToGB(props.client.totalGB || 0);
      const et = Number(props.client.expiryTime) || 0;
      if (et < 0) {
        form.delayedStart = true;
        form.delayedDays = Math.round(et / -86400000);
        form.expiryDate = null;
      } else {
        form.delayedStart = false;
        form.delayedDays = 0;
        form.expiryDate = et > 0 ? dayjs(et) : null;
      }
      form.limitIp = props.client.limitIp || 0;
      form.tgId = Number(props.client.tgId) || 0;
      form.comment = props.client.comment || '';
      form.enable = !!props.client.enable;
      form.inboundIds = Array.isArray(props.attachedIds) ? [...props.attachedIds] : [];
      void loadIps();
    } else {
      form.email = RandomUtil.randomLowerAndNum(9);
      form.uuid = RandomUtil.randomUUID();
      form.subId = RandomUtil.randomLowerAndNum(16);
      form.password = RandomUtil.randomLowerAndNum(16);
      form.auth = RandomUtil.randomLowerAndNum(16);
    }
  },
);

function bytesToGB(bytes) {
  if (!bytes || bytes <= 0) return 0;
  return Math.round((bytes / (1024 * 1024 * 1024)) * 100) / 100;
}

function gbToBytes(gb) {
  if (!gb || gb <= 0) return 0;
  return Math.round(gb * 1024 * 1024 * 1024);
}

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria', 'hysteria2',
]);

const inboundOptions = computed(() =>
  (props.inbounds || [])
    .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol))
    .map((ib) => ({
      label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
      value: ib.id,
      title: `${ib.remark || ''} (${ib.protocol}:${ib.port})`,
    })),
);

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

const vlessLikeIds = computed(() => {
  const ids = new Set();
  for (const row of props.inbounds || []) {
    if (row && row.protocol === 'vless') {
      ids.add(row.id);
    }
  }
  return ids;
});

const showReverseTag = computed(() =>
  (form.inboundIds || []).some((id) => vlessLikeIds.value.has(id)),
);

watch(showReverseTag, (next) => {
  if (!next) form.reverseTag = '';
});

const clientIps = ref([]);
const ipsLoading = ref(false);
const ipsClearing = ref(false);

async function loadIps() {
  if (!isEdit.value || !props.client?.email) return;
  ipsLoading.value = true;
  try {
    const msg = await HttpUtil.post(`/panel/api/clients/ips/${encodeURIComponent(props.client.email)}`);
    if (!msg?.success) { clientIps.value = []; return; }
    const arr = Array.isArray(msg.obj) ? msg.obj : [];
    clientIps.value = arr.filter((x) => typeof x === 'string' && x.length > 0);
  } finally {
    ipsLoading.value = false;
  }
}

async function clearIps() {
  if (!isEdit.value || !props.client?.email) return;
  ipsClearing.value = true;
  try {
    const msg = await HttpUtil.post(`/panel/api/clients/clearIps/${encodeURIComponent(props.client.email)}`);
    if (msg?.success) clientIps.value = [];
  } finally {
    ipsClearing.value = false;
  }
}

function close() {
  emit('update:open', false);
}

function regenerateUUID() {
  form.uuid = RandomUtil.randomUUID();
}

function regeneratePassword() {
  form.password = RandomUtil.randomLowerAndNum(16);
}

function regenerateAuth() {
  form.auth = RandomUtil.randomLowerAndNum(16);
}

function regenerateSubId() {
  form.subId = RandomUtil.randomLowerAndNum(16);
}

function regenerateEmail() {
  form.email = RandomUtil.randomLowerAndNum(12);
}

function onDelayedStartToggle(next) {
  if (next) {
    form.expiryDate = null;
  } else {
    form.delayedDays = 0;
  }
}

async function onSubmit() {
  if (!form.email || form.email.trim() === '') {
    message.error(`${t('pages.clients.email')} *`);
    return;
  }
  if (!isEdit.value && (!form.inboundIds || form.inboundIds.length === 0)) {
    message.error(t('pages.clients.selectInbound'));
    return;
  }
  const expiryTime = form.delayedStart
    ? -86400000 * (Number(form.delayedDays) || 0)
    : (form.expiryDate ? form.expiryDate.valueOf() : 0);
  const clientPayload = {
    email: form.email.trim(),
    subId: form.subId,
    id: form.uuid,
    password: form.password,
    auth: form.auth,
    flow: showFlow.value ? (form.flow || '') : '',
    totalGB: gbToBytes(form.totalGB),
    expiryTime,
    limitIp: Number(form.limitIp) || 0,
    tgId: Number(form.tgId) || 0,
    comment: form.comment,
    enable: !!form.enable,
  };
  const reverseTag = showReverseTag.value ? (form.reverseTag || '').trim() : '';
  if (reverseTag) {
    clientPayload.reverse = { tag: reverseTag };
  }

  submitting.value = true;
  try {
    let msg;
    if (isEdit.value) {
      const original = new Set(props.attachedIds || []);
      const next = new Set(form.inboundIds || []);
      const toAttach = [...next].filter((id) => !original.has(id));
      const toDetach = [...original].filter((id) => !next.has(id));
      msg = await props.save(clientPayload, {
        isEdit: true,
        email: props.client.email,
        attach: toAttach,
        detach: toDetach,
      });
    } else {
      msg = await props.save(
        { client: clientPayload, inboundIds: form.inboundIds },
        { isEdit: false },
      );
    }
    if (msg?.success) close();
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <a-modal :open="open" :title="isEdit ? t('pages.clients.editTitle') : t('pages.clients.addTitle')"
    :destroy-on-close="true" :ok-text="isEdit ? t('save') : t('create')" :cancel-text="t('cancel')"
    :ok-button-props="{ loading: submitting }" :width="720" @ok="onSubmit" @cancel="close">
    <a-form layout="vertical" :model="form">
      <a-row :gutter="16">
        <a-col :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.email')" required>
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.email" :placeholder="t('pages.clients.email')" style="flex: 1" />
              <a-button @click="regenerateEmail">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
        <a-col :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.subId')">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.subId" style="flex: 1" />
              <a-button @click="regenerateSubId">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.hysteriaAuth')">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.auth" style="flex: 1" />
              <a-button @click="regenerateAuth">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
        <a-col :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.password')">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.password" style="flex: 1" />
              <a-button @click="regeneratePassword">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.uuid')">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.uuid" style="flex: 1" />
              <a-button @click="regenerateUUID">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
        <a-col :xs="24" :md="ipLimitEnable ? 8 : 12">
          <a-form-item :label="t('pages.clients.totalGB')">
            <a-input-number v-model:value="form.totalGB" :min="0" :step="0.1" style="width: 100%" />
          </a-form-item>
        </a-col>
        <a-col v-if="ipLimitEnable" :xs="24" :md="4">
          <a-form-item :label="t('pages.clients.limitIp')">
            <a-input-number v-model:value="form.limitIp" :min="0" style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :xs="24" :md="12">
          <a-form-item v-if="form.delayedStart" :label="t('pages.clients.expireDays')">
            <a-input-number v-model:value="form.delayedDays" :min="0" style="width: 100%" />
          </a-form-item>
          <a-form-item v-else :label="t('pages.clients.expiryTime')">
            <a-date-picker v-model:value="form.expiryDate" show-time style="width: 100%" />
          </a-form-item>
        </a-col>
        <a-col :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.delayedStart')">
            <a-switch v-model:checked="form.delayedStart" @change="onDelayedStartToggle" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row v-if="showFlow || showReverseTag" :gutter="16">
        <a-col v-if="showFlow" :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.flow')">
            <a-select v-model:value="form.flow">
              <a-select-option value="">{{ t('none') }}</a-select-option>
              <a-select-option v-for="k in FLOW_OPTIONS" :key="k" :value="k">{{ k }}</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col v-if="showReverseTag" :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.reverseTag')">
            <a-input v-model:value="form.reverseTag" :placeholder="t('pages.clients.reverseTagPlaceholder')" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col v-if="tgBotEnable" :xs="24" :md="12">
          <a-form-item :label="t('pages.clients.telegramId')">
            <a-input-number v-model:value="form.tgId" :min="0" :controls="false"
              :placeholder="t('pages.clients.telegramIdPlaceholder')" style="width: 100%" />
          </a-form-item>
        </a-col>
        <a-col :xs="24" :md="tgBotEnable ? 12 : 24">
          <a-form-item :label="t('pages.clients.comment')">
            <a-input v-model:value="form.comment" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item :label="t('pages.clients.attachedInbounds')" :required="!isEdit">
        <a-select v-model:value="form.inboundIds" mode="multiple" :options="inboundOptions" :show-search="true"
          :placeholder="t('pages.clients.selectInbound')"
          :filter-option="(input, option) => (option.label || '').toLowerCase().includes(input.toLowerCase())" />
      </a-form-item>

      <a-form-item>
        <a-switch v-model:checked="form.enable" />
        <span style="margin-left: 8px">{{ t('enable') }}</span>
      </a-form-item>

      <a-form-item v-if="isEdit && ipLimitEnable" :label="t('pages.clients.ipLog')">
        <a-space style="margin-bottom: 8px">
          <a-button size="small" :loading="ipsLoading" @click="loadIps">{{ t('refresh') }}</a-button>
          <a-button size="small" danger :loading="ipsClearing" :disabled="clientIps.length === 0" @click="clearIps">
            {{ t('pages.clients.clearAll') }}
          </a-button>
        </a-space>
        <div v-if="clientIps.length > 0">
          <a-tag v-for="(ip, idx) in clientIps" :key="idx" color="blue" style="margin-bottom: 4px">{{ ip }}</a-tag>
        </div>
        <a-tag v-else>{{ t('tgbot.noIpRecord') }}</a-tag>
      </a-form-item>
    </a-form>
  </a-modal>
</template>
