<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';
import dayjs from 'dayjs';
import { HttpUtil, RandomUtil } from '@/utils';
import { DBInbound } from '@/models/dbinbound.js';
import { TLS_FLOW_CONTROL } from '@/models/inbound.js';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add' },
  client: { type: Object, default: null },
  inbounds: { type: Array, default: () => [] },
  attachedIds: { type: Array, default: () => [] },
  ipLimitEnable: { type: Boolean, default: false },
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
    expiryTime: null,
    limitIp: 0,
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
      form.expiryTime = props.client.expiryTime ? dayjs(props.client.expiryTime) : null;
      form.limitIp = props.client.limitIp || 0;
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

const inboundOptions = computed(() =>
  (props.inbounds || []).map((ib) => ({
    label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
    value: ib.id,
    title: `${ib.remark || ''} (${ib.protocol}:${ib.port})`,
  })),
);

const flowCapableIds = computed(() => {
  const ids = new Set();
  for (const row of props.inbounds || []) {
    try {
      const parsed = new DBInbound(row).toInbound();
      if (parsed.canEnableTlsFlow?.()) ids.add(row.id);
    } catch (_e) { /* ignore unparsable */ }
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
    if (row && (row.protocol === 'vless' || row.protocol === 'portfallback')) {
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
  form.email = RandomUtil.randomLowerAndNum(9);
}

async function onSubmit() {
  if (!form.email || form.email.trim() === '') {
    message.error(t('pages.clients.email') + ' *');
    return;
  }
  if (!isEdit.value && (!form.inboundIds || form.inboundIds.length === 0)) {
    message.error(t('pages.clients.selectInbound'));
    return;
  }
  const clientPayload = {
    email: form.email.trim(),
    subId: form.subId,
    id: form.uuid,
    password: form.password,
    auth: form.auth,
    flow: showFlow.value ? (form.flow || '') : '',
    totalGB: gbToBytes(form.totalGB),
    expiryTime: form.expiryTime ? form.expiryTime.valueOf() : 0,
    limitIp: Number(form.limitIp) || 0,
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
    :destroy-on-close="true" :ok-text="isEdit ? t('save') : t('add')" :cancel-text="t('cancel')"
    :ok-button-props="{ loading: submitting }" :width="720" @ok="onSubmit" @cancel="close">
    <a-form layout="vertical" :model="form">
      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item :label="t('pages.clients.email')" required>
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.email" :placeholder="t('pages.clients.email')" style="flex: 1" />
              <a-button @click="regenerateEmail">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.clients.subId') || 'subId'">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.subId" style="flex: 1" />
              <a-button @click="regenerateSubId">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item label="UUID">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.uuid" style="flex: 1" />
              <a-button @click="regenerateUUID">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.clients.password') || 'Password'">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.password" style="flex: 1" />
              <a-button @click="regeneratePassword">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item label="Auth (Hysteria)">
            <a-input-group compact style="display: flex">
              <a-input v-model:value="form.auth" style="flex: 1" />
              <a-button @click="regenerateAuth">↻</a-button>
            </a-input-group>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.clients.limitIp') || 'IP limit'">
            <a-input-number v-model:value="form.limitIp" :min="0" :disabled="!ipLimitEnable" style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item :label="t('pages.clients.totalGB') || 'Total (GB, 0 = unlimited)'">
            <a-input-number v-model:value="form.totalGB" :min="0" :step="0.1" style="width: 100%" />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.clients.expiryTime') || 'Expiry'">
            <a-date-picker v-model:value="form.expiryTime" show-time style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row v-if="showFlow || showReverseTag" :gutter="16">
        <a-col v-if="showFlow" :span="12">
          <a-form-item label="Flow">
            <a-select v-model:value="form.flow">
              <a-select-option value="">none</a-select-option>
              <a-select-option v-for="k in FLOW_OPTIONS" :key="k" :value="k">{{ k }}</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col v-if="showReverseTag" :span="12">
          <a-form-item label="Reverse tag">
            <a-input v-model:value="form.reverseTag" placeholder="Optional reverse tag" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item :label="t('pages.clients.comment') || 'Comment'">
        <a-input v-model:value="form.comment" />
      </a-form-item>

      <a-form-item :label="t('pages.clients.attachedInbounds') || 'Attached inbounds'" :required="!isEdit">
        <a-select v-model:value="form.inboundIds" mode="multiple" :options="inboundOptions" :show-search="true"
          :placeholder="t('pages.clients.selectInbound') || 'Select one or more inbounds'"
          :filter-option="(input, option) => (option.label || '').toLowerCase().includes(input.toLowerCase())" />
      </a-form-item>

      <a-form-item>
        <a-switch v-model:checked="form.enable" />
        <span style="margin-left: 8px">{{ t('enable') }}</span>
      </a-form-item>

      <a-form-item v-if="isEdit && ipLimitEnable" :label="t('pages.clients.ipLog') || 'IP Log'">
        <a-space style="margin-bottom: 8px">
          <a-button size="small" :loading="ipsLoading" @click="loadIps">{{ t('refresh') }}</a-button>
          <a-button size="small" danger :loading="ipsClearing" :disabled="clientIps.length === 0" @click="clearIps">
            {{ t('clearAll') || 'Clear' }}
          </a-button>
        </a-space>
        <div v-if="clientIps.length > 0">
          <a-tag v-for="(ip, idx) in clientIps" :key="idx" color="blue" style="margin-bottom: 4px">{{ ip }}</a-tag>
        </div>
        <a-tag v-else>{{ t('tgbot.noIpRecord') || 'No IP record' }}</a-tag>
      </a-form-item>
    </a-form>
  </a-modal>
</template>
