<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';
import dayjs from 'dayjs';
import { RandomUtil } from '@/utils';

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add' },
  client: { type: Object, default: null },
  inbounds: { type: Array, default: () => [] },
  attachedIds: { type: Array, default: () => [] },
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
      form.totalGB = bytesToGB(props.client.totalGB || 0);
      form.expiryTime = props.client.expiryTime ? dayjs(props.client.expiryTime) : null;
      form.limitIp = props.client.limitIp || 0;
      form.comment = props.client.comment || '';
      form.enable = !!props.client.enable;
      form.inboundIds = Array.isArray(props.attachedIds) ? [...props.attachedIds] : [];
    } else {
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

async function onSubmit() {
  if (!form.email || form.email.trim() === '') {
    message.error(t('pages.inbounds.client.email') + ' *');
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
    totalGB: gbToBytes(form.totalGB),
    expiryTime: form.expiryTime ? form.expiryTime.valueOf() : 0,
    limitIp: Number(form.limitIp) || 0,
    comment: form.comment,
    enable: !!form.enable,
  };

  submitting.value = true;
  try {
    let msg;
    if (isEdit.value) {
      msg = await props.save(clientPayload, { isEdit: true, id: props.client.id });
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
          <a-form-item :label="t('pages.inbounds.client.email')" required>
            <a-input v-model:value="form.email" :placeholder="t('pages.inbounds.client.email')" />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.inbounds.client.subId') || 'subId'">
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
          <a-form-item :label="t('pages.inbounds.client.password') || 'Password'">
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
          <a-form-item :label="t('pages.inbounds.client.limitIp') || 'IP limit'">
            <a-input-number v-model:value="form.limitIp" :min="0" style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item :label="t('pages.inbounds.client.totalGB') || 'Total (GB, 0 = unlimited)'">
            <a-input-number v-model:value="form.totalGB" :min="0" :step="0.1" style="width: 100%" />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.inbounds.client.expiryTime') || 'Expiry'">
            <a-date-picker v-model:value="form.expiryTime" show-time style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item :label="t('pages.inbounds.client.comment') || 'Comment'">
        <a-input v-model:value="form.comment" />
      </a-form-item>

      <a-form-item v-if="!isEdit" :label="t('pages.clients.attachedInbounds') || 'Attach to inbounds'" required>
        <a-select v-model:value="form.inboundIds" mode="multiple" :options="inboundOptions" :show-search="true"
          :placeholder="t('pages.clients.selectInbound') || 'Select one or more inbounds'"
          :filter-option="(input, option) => (option.label || '').toLowerCase().includes(input.toLowerCase())" />
      </a-form-item>

      <a-form-item>
        <a-switch v-model:checked="form.enable" />
        <span style="margin-left: 8px">{{ t('enable') }}</span>
      </a-form-item>
    </a-form>
  </a-modal>
</template>
