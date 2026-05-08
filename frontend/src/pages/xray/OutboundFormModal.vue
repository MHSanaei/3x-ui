<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { message } from 'ant-design-vue';

import { Protocols, OutboundDomainStrategies } from '@/models/outbound.js';

// Outbound add/edit modal. The legacy modal is huge (1.3k lines)
// because it covers every protocol's nested settings/streamSettings
// inline. We take the same pragmatic approach we did for the inbound
// modal: a Basics tab covers the always-relevant fields (tag,
// protocol, sendThrough, domain strategy) and a JSON tab exposes
// the full settings + streamSettings trees verbatim. Full structured
// per-protocol forms can land later — the JSON path supports every
// field today and matches what the Advanced page-level JSON tab
// already does.

const props = defineProps({
  open: { type: Boolean, default: false },
  // null when adding, the outbound object when editing.
  outbound: { type: Object, default: null },
  // Existing tags so we can flag duplicates client-side.
  existingTags: { type: Array, default: () => [] },
});

const emit = defineEmits(['update:open', 'confirm']);

const PROTOCOL_OPTIONS = Object.values(Protocols);

const form = reactive({
  tag: '',
  protocol: Protocols.Freedom,
  sendThrough: '',
  domainStrategy: 'AsIs',
  settingsText: '',
  streamSettingsText: '',
});
const isEdit = ref(false);

function pretty(value) {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') {
    try { return JSON.stringify(JSON.parse(value), null, 2); }
    catch (_e) { return value; }
  }
  try { return JSON.stringify(value, null, 2); }
  catch (_e) { return ''; }
}

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.outbound) {
    isEdit.value = true;
    const o = props.outbound;
    form.tag = o.tag || '';
    form.protocol = o.protocol || Protocols.Freedom;
    form.sendThrough = o.sendThrough || '';
    form.domainStrategy = o.domainStrategy || 'AsIs';
    form.settingsText = pretty(o.settings);
    form.streamSettingsText = pretty(o.streamSettings);
  } else {
    isEdit.value = false;
    form.tag = '';
    form.protocol = Protocols.Freedom;
    form.sendThrough = '';
    form.domainStrategy = 'AsIs';
    form.settingsText = '';
    form.streamSettingsText = '';
  }
});

function close() { emit('update:open', false); }

function buildResult() {
  // Empty JSON tabs collapse to undefined keys so the wire shape
  // doesn't carry empty objects we never had in the first place.
  let settings;
  let streamSettings;
  try {
    settings = form.settingsText.trim() ? JSON.parse(form.settingsText) : undefined;
  } catch (e) {
    message.error(`settings JSON invalid: ${e.message}`);
    throw e;
  }
  try {
    streamSettings = form.streamSettingsText.trim()
      ? JSON.parse(form.streamSettingsText)
      : undefined;
  } catch (e) {
    message.error(`streamSettings JSON invalid: ${e.message}`);
    throw e;
  }
  const out = {
    tag: form.tag,
    protocol: form.protocol,
  };
  if (form.sendThrough) out.sendThrough = form.sendThrough;
  if (form.domainStrategy && form.domainStrategy !== 'AsIs') {
    out.domainStrategy = form.domainStrategy;
  }
  if (settings !== undefined) out.settings = settings;
  if (streamSettings !== undefined) out.streamSettings = streamSettings;
  return out;
}

function onOk() {
  if (!form.tag.trim()) {
    message.error('Tag is required.');
    return;
  }
  // Block tag collisions client-side — server enforces too but this
  // surfaces faster.
  const conflict = (props.existingTags || []).includes(form.tag.trim());
  if (conflict) {
    message.error('An outbound with this tag already exists.');
    return;
  }
  let result;
  try { result = buildResult(); } catch (_e) { return; }
  emit('confirm', result);
}

const title = computed(() => (isEdit.value ? 'Edit outbound' : 'Add outbound'));
const okText = computed(() => (isEdit.value ? 'Update' : 'Add outbound'));
</script>

<template>
  <a-modal
    :open="open"
    :title="title"
    :ok-text="okText"
    cancel-text="Close"
    :mask-closable="false"
    width="720px"
    @ok="onOk"
    @cancel="close"
  >
    <a-tabs default-active-key="basic">
      <a-tab-pane key="basic" tab="Basics">
        <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
          <a-form-item label="Tag">
            <a-input v-model:value="form.tag" placeholder="unique-tag" />
          </a-form-item>
          <a-form-item label="Protocol">
            <a-select v-model:value="form.protocol">
              <a-select-option v-for="p in PROTOCOL_OPTIONS" :key="p" :value="p">{{ p }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="Send through">
            <a-input v-model:value="form.sendThrough" placeholder="local IP to bind to (optional)" />
          </a-form-item>
          <a-form-item label="Domain strategy">
            <a-select v-model:value="form.domainStrategy">
              <a-select-option v-for="s in OutboundDomainStrategies" :key="s" :value="s">{{ s }}</a-select-option>
            </a-select>
          </a-form-item>
        </a-form>
      </a-tab-pane>

      <a-tab-pane key="settings" tab="settings (JSON)">
        <a-alert
          type="info"
          show-icon
          message="Edit the protocol-specific settings tree directly. Leave empty to omit."
          class="mb-12"
        />
        <a-textarea
          v-model:value="form.settingsText"
          :auto-size="{ minRows: 12, maxRows: 28 }"
          spellcheck="false"
          class="json-editor"
        />
      </a-tab-pane>

      <a-tab-pane key="stream" tab="streamSettings (JSON)">
        <a-alert
          type="info"
          show-icon
          message="Transport / TLS / Reality / mux options. Leave empty to omit."
          class="mb-12"
        />
        <a-textarea
          v-model:value="form.streamSettingsText"
          :auto-size="{ minRows: 12, maxRows: 28 }"
          spellcheck="false"
          class="json-editor"
        />
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<style scoped>
.mb-12 { margin-bottom: 12px; }
.json-editor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}
</style>
