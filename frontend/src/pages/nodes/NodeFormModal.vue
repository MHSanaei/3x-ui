<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add' }, // 'add' | 'edit'
  node: { type: Object, default: null },
  testConnection: { type: Function, required: true },
  save: { type: Function, required: true }, // (payload) => Promise<msg>
});

const emit = defineEmits(['update:open']);

const { t } = useI18n();

// Default form shape — used for "add" mode and to reset between
// edits. Sane defaults: HTTPS, port 2053, base path '/', enabled.
function defaultForm() {
  return {
    id: 0,
    name: '',
    remark: '',
    scheme: 'https',
    address: '',
    port: 2053,
    basePath: '/',
    apiToken: '',
    enable: true,
  };
}

const form = reactive(defaultForm());
const submitting = ref(false);
const testing = ref(false);
const testResult = ref(null); // { status, latencyMs, xrayVersion, error }
// Reset the form whenever the modal is opened. In edit mode we copy
// the existing node into the form fields; in add mode we wipe back
// to defaults so a previous edit doesn't leak through.
watch(
  () => props.open,
  (open) => {
    if (!open) return;
    Object.assign(form, defaultForm());
    testResult.value = null;
    if (props.mode === 'edit' && props.node) {
      Object.assign(form, props.node);
    }
  },
);

const title = computed(() =>
  props.mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode'),
);

function close() {
  if (!submitting.value) emit('update:open', false);
}

function buildPayload() {
  return {
    id: form.id || 0,
    name: form.name?.trim() || '',
    remark: form.remark?.trim() || '',
    scheme: form.scheme || 'https',
    address: form.address?.trim() || '',
    port: Number(form.port) || 0,
    basePath: form.basePath?.trim() || '/',
    apiToken: form.apiToken?.trim() || '',
    enable: !!form.enable,
  };
}

async function onTest() {
  testing.value = true;
  testResult.value = null;
  try {
    const payload = buildPayload();
    if (!payload.address || !payload.port) {
      message.error(t('pages.nodes.toasts.fillRequired'));
      return;
    }
    const msg = await props.testConnection(payload);
    if (msg?.success) {
      testResult.value = msg.obj;
    } else {
      testResult.value = { status: 'offline', error: msg?.msg || 'unknown error' };
    }
  } finally {
    testing.value = false;
  }
}

async function onSave() {
  const payload = buildPayload();
  if (!payload.name || !payload.address || !payload.port) {
    message.error(t('pages.nodes.toasts.fillRequired'));
    return;
  }
  submitting.value = true;
  try {
    const msg = await props.save(payload);
    if (msg?.success) {
      emit('update:open', false);
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <a-modal :open="open" :title="title" :confirm-loading="submitting" :ok-text="t('save')" :cancel-text="t('cancel')"
    :mask-closable="false" width="640px" @ok="onSave" @cancel="close">
    <a-form layout="vertical" :model="form">
      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item :label="t('pages.nodes.name')" required>
            <a-input v-model:value="form.name" :placeholder="t('pages.nodes.namePlaceholder')" />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.nodes.remark')">
            <a-input v-model:value="form.remark" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="6">
          <a-form-item :label="t('pages.nodes.scheme')">
            <a-select v-model:value="form.scheme">
              <a-select-option value="https">https</a-select-option>
              <a-select-option value="http">http</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.nodes.address')" required>
            <a-input v-model:value="form.address" :placeholder="t('pages.nodes.addressPlaceholder')" />
          </a-form-item>
        </a-col>
        <a-col :span="6">
          <a-form-item :label="t('pages.nodes.port')" required>
            <a-input-number v-model:value="form.port" :min="1" :max="65535" style="width: 100%" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item :label="t('pages.nodes.basePath')">
            <a-input v-model:value="form.basePath" placeholder="/" />
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item :label="t('pages.nodes.enable')">
            <a-switch v-model:checked="form.enable" />
          </a-form-item>
        </a-col>
      </a-row>

      <a-form-item :label="t('pages.nodes.apiToken')" required>
        <a-input-password v-model:value="form.apiToken" :placeholder="t('pages.nodes.apiTokenPlaceholder')" />
        <div class="hint">{{ t('pages.nodes.apiTokenHint') }}</div>
      </a-form-item>

      <div class="test-row">
        <a-button :loading="testing" @click="onTest">
          {{ t('pages.nodes.testConnection') }}
        </a-button>
        <div v-if="testResult" class="test-result">
          <a-alert v-if="testResult.status === 'online'" type="success" show-icon
            :message="t('pages.nodes.connectionOk', { ms: testResult.latencyMs })"
            :description="testResult.xrayVersion ? `Xray ${testResult.xrayVersion}` : undefined" />
          <a-alert v-else type="error" show-icon :message="t('pages.nodes.connectionFailed')"
            :description="testResult.error" />
        </div>
      </div>
    </a-form>
  </a-modal>
</template>

<style scoped>
.hint {
  font-size: 12px;
  opacity: 0.6;
  margin-top: 4px;
}

.test-row {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 8px;
}

.test-result {
  width: 100%;
}
</style>
