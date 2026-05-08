<script setup>
import { reactive, ref, watch } from 'vue';
import { message } from 'ant-design-vue';
import { HttpUtil } from '@/utils';

const props = defineProps({
  open: { type: Boolean, default: false },
  // Populate with the record when editing; null/undefined when adding.
  record: { type: Object, default: null },
});

const emit = defineEmits(['update:open', 'saved']);

const form = reactive({ type: 'geosite', alias: '', url: '' });
const saving = ref(false);

const editing = ref(false);
const editId = ref(null);

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.record) {
    editing.value = true;
    editId.value = props.record.id;
    form.type = props.record.type;
    form.alias = props.record.alias;
    form.url = props.record.url;
  } else {
    editing.value = false;
    editId.value = null;
    form.type = 'geosite';
    form.alias = '';
    form.url = '';
  }
});

function close() {
  emit('update:open', false);
}

function validate() {
  // Backend expects a filesystem-safe alias; legacy enforces the same regex.
  if (!/^[a-z0-9_-]+$/.test(form.alias || '')) {
    message.error('Alias must contain only lowercase letters, digits, dashes or underscores.');
    return false;
  }
  const u = (form.url || '').trim();
  if (!/^https?:\/\//i.test(u)) {
    message.error('URL must start with http:// or https://');
    return false;
  }
  try {
    const parsed = new URL(u);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      message.error('URL must start with http:// or https://');
      return false;
    }
  } catch (_e) {
    message.error('URL must start with http:// or https://');
    return false;
  }
  return true;
}

async function submit() {
  if (!validate()) return;
  saving.value = true;
  try {
    const url = editing.value
      ? `/panel/api/custom-geo/update/${editId.value}`
      : '/panel/api/custom-geo/add';
    const msg = await HttpUtil.post(url, form);
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
  <a-modal
    :open="open"
    :title="editing ? 'Edit custom geo entry' : 'Add custom geo entry'"
    :confirm-loading="saving"
    ok-text="Save"
    cancel-text="Close"
    @ok="submit"
    @cancel="close"
  >
    <a-form layout="vertical">
      <a-form-item label="Type">
        <a-select v-model:value="form.type" :disabled="editing">
          <a-select-option value="geosite">geosite</a-select-option>
          <a-select-option value="geoip">geoip</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="Alias">
        <a-input
          v-model:value="form.alias"
          :disabled="editing"
          placeholder="lowercase letters, digits, dashes, underscores"
        />
      </a-form-item>
      <a-form-item label="URL">
        <a-input v-model:value="form.url" placeholder="https://" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>
