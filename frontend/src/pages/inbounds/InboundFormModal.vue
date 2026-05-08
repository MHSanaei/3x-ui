<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { message } from 'ant-design-vue';
import dayjs from 'dayjs';

import { HttpUtil, RandomUtil, NumberFormatter, SizeFormatter } from '@/utils';
import { Inbound, Protocols } from '@/models/inbound.js';
import { DBInbound } from '@/models/dbinbound.js';

// Phase 5f-iii scope: full Basics tab + a JSON-edit fallback for the
// protocol settings, transport settings, and sniffing. The protocol-
// specific and transport-specific forms (TCP/WS/Reality/etc.) come in
// 5f-iii-b, which will replace these textareas with proper field
// editors. Saving JSON works today though — so users can both add new
// inbounds (with a default template stamped per protocol) and edit
// existing ones without losing settings.

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add', validator: (v) => ['add', 'edit'].includes(v) },
  // Required when mode === 'edit'; the modal clones it on open so
  // cancel doesn't leak edits back to the row.
  dbInbound: { type: Object, default: null },
});

const emit = defineEmits(['update:open', 'saved']);

const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'];
const PROTOCOLS = Object.values(Protocols);

// Reactive form state — flat fields the Basics tab edits directly,
// plus the three JSON strings the textarea tabs edit as text.
const form = reactive({
  enable: true,
  remark: '',
  protocol: Protocols.VMESS,
  listen: '',
  port: 0,
  totalGB: 0,
  trafficReset: 'never',
  expiryTime: 0, // ms epoch; 0 == never expire
  // JSON-edit fields:
  settingsText: '',
  streamSettingsText: '',
  sniffingText: '',
});
const saving = ref(false);

// AD-Vue's a-date-picker emits a Day.js value; convert to/from epoch ms.
const expiryDate = computed({
  get: () => (form.expiryTime > 0 ? dayjs(form.expiryTime) : null),
  set: (next) => { form.expiryTime = next ? next.valueOf() : 0; },
});

// On open, populate `form` from the supplied dbInbound (edit mode) or
// stamp a fresh default per protocol (add mode).
function loadFromDbInbound(dbIn) {
  form.enable = dbIn.enable ?? true;
  form.remark = dbIn.remark || '';
  form.protocol = dbIn.protocol || Protocols.VMESS;
  form.listen = dbIn.listen || '';
  form.port = dbIn.port || 0;
  form.totalGB = NumberFormatter.toFixed((dbIn.total || 0) / SizeFormatter.ONE_GB, 2);
  form.trafficReset = dbIn.trafficReset || 'never';
  form.expiryTime = dbIn.expiryTime || 0;

  // For edit mode the wire JSON strings are already strings; pretty-print
  // them so the textarea is readable.
  form.settingsText = prettyJson(dbIn.settings);
  form.streamSettingsText = prettyJson(dbIn.streamSettings);
  form.sniffingText = prettyJson(dbIn.sniffing);
}

function prettyJson(maybeJson) {
  if (!maybeJson) return '';
  try {
    return JSON.stringify(JSON.parse(maybeJson), null, 2);
  } catch (_e) {
    return maybeJson;
  }
}

function stampDefaultsForNew() {
  const inbound = new Inbound();
  inbound.protocol = form.protocol;
  inbound.settings = Inbound.Settings.getSettings(inbound.protocol);
  form.port = RandomUtil.randomInteger(10000, 60000);
  form.settingsText = prettyJson(inbound.settings.toString());
  form.streamSettingsText = prettyJson(inbound.stream.toString());
  form.sniffingText = prettyJson(inbound.sniffing.toString());
}

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.mode === 'edit' && props.dbInbound) {
    loadFromDbInbound(props.dbInbound);
  } else {
    form.enable = true;
    form.remark = '';
    form.protocol = Protocols.VMESS;
    form.listen = '';
    form.totalGB = 0;
    form.trafficReset = 'never';
    form.expiryTime = 0;
    stampDefaultsForNew();
  }
});

// When the user changes protocol in add mode, restamp the JSON
// templates so they match the new protocol's shape.
watch(() => form.protocol, (next) => {
  if (props.mode === 'edit') return;
  const inbound = new Inbound();
  inbound.protocol = next;
  inbound.settings = Inbound.Settings.getSettings(next);
  form.settingsText = prettyJson(inbound.settings.toString());
  form.streamSettingsText = prettyJson(inbound.stream.toString());
  form.sniffingText = prettyJson(inbound.sniffing.toString());
});

function close() {
  emit('update:open', false);
}

// Validate each JSON field; show a message and bail if any is malformed.
function parseOrFail(label, text) {
  const trimmed = (text || '').trim();
  if (!trimmed) return null;
  try {
    return JSON.parse(trimmed);
  } catch (e) {
    message.error(`${label} is not valid JSON: ${e.message}`);
    throw e;
  }
}

async function submit() {
  let parsedSettings;
  let parsedStream;
  let parsedSniffing;
  try {
    parsedSettings = parseOrFail('Settings', form.settingsText);
    parsedStream = parseOrFail('Stream settings', form.streamSettingsText);
    parsedSniffing = parseOrFail('Sniffing', form.sniffingText);
  } catch (_e) {
    return;
  }

  // Compute total bytes from totalGB; preserve fractional GB precision.
  const total = NumberFormatter.toFixed((form.totalGB || 0) * SizeFormatter.ONE_GB, 0);

  const payload = {
    up: props.dbInbound?.up ?? 0,
    down: props.dbInbound?.down ?? 0,
    total,
    remark: form.remark,
    enable: form.enable,
    expiryTime: form.expiryTime,
    trafficReset: form.trafficReset,
    lastTrafficResetTime: props.dbInbound?.lastTrafficResetTime ?? 0,
    listen: form.listen,
    port: form.port,
    protocol: form.protocol,
    settings: parsedSettings ? JSON.stringify(parsedSettings) : '',
    streamSettings: parsedStream ? JSON.stringify(parsedStream) : '',
    sniffing: parsedSniffing ? JSON.stringify(parsedSniffing) : '',
  };

  saving.value = true;
  try {
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

// Surface helper buttons for filling defaults manually so users can
// recover after editing the textareas badly.
function resetSettingsTemplate() {
  const s = Inbound.Settings.getSettings(form.protocol);
  form.settingsText = prettyJson(s.toString());
}
function resetStreamTemplate() {
  form.streamSettingsText = prettyJson(new Inbound().stream.toString());
}
function resetSniffingTemplate() {
  form.sniffingText = prettyJson(new Inbound().sniffing.toString());
}

const title = computed(() => (props.mode === 'edit' ? 'Edit inbound' : 'Add inbound'));
const okText = computed(() => (props.mode === 'edit' ? 'Update' : 'Create'));

// Avoid an unused-import warning — DBInbound is referenced by the parent
// via its prop, but importing it here keeps the file self-documenting.
void DBInbound;
</script>

<template>
  <a-modal
    :open="open"
    :title="title"
    :ok-text="okText"
    cancel-text="Close"
    :confirm-loading="saving"
    :mask-closable="false"
    width="720px"
    @ok="submit"
    @cancel="close"
  >
    <a-tabs default-active-key="basic">
      <a-tab-pane key="basic" tab="Basics">
        <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
          <a-form-item label="Enable">
            <a-switch v-model:checked="form.enable" />
          </a-form-item>
          <a-form-item label="Remark">
            <a-input v-model:value="form.remark" />
          </a-form-item>
          <a-form-item label="Protocol">
            <a-select v-model:value="form.protocol" :disabled="mode === 'edit'">
              <a-select-option v-for="p in PROTOCOLS" :key="p" :value="p">{{ p }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="Listen IP">
            <a-input v-model:value="form.listen" placeholder="(blank = all interfaces)" />
          </a-form-item>
          <a-form-item label="Port">
            <a-input-number v-model:value="form.port" :min="1" :max="65535" />
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip title="0 means no limit">Total traffic (GB)</a-tooltip>
            </template>
            <a-input-number v-model:value="form.totalGB" :min="0" :step="0.1" />
          </a-form-item>
          <a-form-item label="Traffic reset">
            <a-select v-model:value="form.trafficReset">
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

      <a-tab-pane key="settings" tab="Settings (JSON)">
        <a-alert
          type="info"
          show-icon
          message="Protocol settings — protocol-specific form coming in 5f-iii-b."
          class="mb-12"
        />
        <a-textarea
          v-model:value="form.settingsText"
          :auto-size="{ minRows: 10, maxRows: 24 }"
          spellcheck="false"
          class="json-editor"
        />
        <div class="textarea-toolbar">
          <a-button size="small" @click="resetSettingsTemplate">Reset to default for {{ form.protocol }}</a-button>
        </div>
      </a-tab-pane>

      <a-tab-pane key="stream" tab="Stream (JSON)">
        <a-alert
          type="info"
          show-icon
          message="Transport / TLS / Reality settings — proper form coming in 5f-iii-b."
          class="mb-12"
        />
        <a-textarea
          v-model:value="form.streamSettingsText"
          :auto-size="{ minRows: 10, maxRows: 24 }"
          spellcheck="false"
          class="json-editor"
        />
        <div class="textarea-toolbar">
          <a-button size="small" @click="resetStreamTemplate">Reset to default</a-button>
        </div>
      </a-tab-pane>

      <a-tab-pane key="sniffing" tab="Sniffing (JSON)">
        <a-textarea
          v-model:value="form.sniffingText"
          :auto-size="{ minRows: 8, maxRows: 24 }"
          spellcheck="false"
          class="json-editor"
        />
        <div class="textarea-toolbar">
          <a-button size="small" @click="resetSniffingTemplate">Reset to default</a-button>
        </div>
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

.textarea-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}
</style>
