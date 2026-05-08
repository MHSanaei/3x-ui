<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons-vue';

import { HttpUtil, FileManager, IntlUtil, PromiseUtil } from '@/utils';
import { useDatepicker } from '@/composables/useDatepicker.js';

const { t } = useI18n();
const { datepicker } = useDatepicker();

const props = defineProps({
  open: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open']);

const rows = ref('20');
const filter = ref('');
const showDirect = ref(true);
const showBlocked = ref(true);
const showProxy = ref(true);
const loading = ref(false);
const logs = ref([]);

function escapeHtml(value) {
  if (value == null) return '';
  return String(value)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// Renders a `<table>` with one row per log entry. Event 1 = blocked
// (red); Event 2 = proxy (blue); Event 0 = direct.
function formatLogs(lines) {
  let out = '<table class="xraylog-table"><tr>'
    + '<th>Date</th><th>From</th><th>To</th><th>Inbound</th><th>Outbound</th><th>Email</th>'
    + '</tr>';

  // Reverse a copy — the legacy code mutated state with `.reverse()`.
  [...lines].reverse().forEach((log) => {
    let rowStyle = '';
    if (log.Event === 1) rowStyle = ' style="color: #e04141;"';
    else if (log.Event === 2) rowStyle = ' style="color: #3c89e8;"';

    const emailCell = log.Email ? `<td>${escapeHtml(log.Email)}</td>` : '<td></td>';

    out += `<tr${rowStyle}>`
      + `<td><b>${escapeHtml(IntlUtil.formatDate(log.DateTime, datepicker.value))}</b></td>`
      + `<td>${escapeHtml(log.FromAddress)}</td>`
      + `<td>${escapeHtml(log.ToAddress)}</td>`
      + `<td>${escapeHtml(log.Inbound)}</td>`
      + `<td>${escapeHtml(log.Outbound)}</td>`
      + emailCell
      + '</tr>';
  });

  return out + '</table>';
}

const formattedLogs = computed(() => (logs.value.length > 0 ? formatLogs(logs.value) : 'No Record...'));

async function refresh() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post(`/panel/api/server/xraylogs/${rows.value}`, {
      filter: filter.value,
      showDirect: showDirect.value,
      showBlocked: showBlocked.value,
      showProxy: showProxy.value,
    });
    if (msg?.success) logs.value = msg.obj || [];
    await PromiseUtil.sleep(300);
  } finally {
    loading.value = false;
  }
}

function close() {
  emit('update:open', false);
}

function download() {
  if (!Array.isArray(logs.value) || logs.value.length === 0) {
    FileManager.downloadTextFile('', 'x-ui.log');
    return;
  }
  const eventMap = { 0: 'DIRECT', 1: 'BLOCKED', 2: 'PROXY' };
  const lines = logs.value.map((l) => {
    try {
      const dt = l.DateTime ? new Date(l.DateTime) : null;
      const dateStr = dt && !isNaN(dt.getTime()) ? dt.toISOString() : '';
      const eventText = eventMap[l.Event] || String(l.Event ?? '');
      const emailPart = l.Email ? ` Email=${l.Email}` : '';
      return `${dateStr} FROM=${l.FromAddress || ''} TO=${l.ToAddress || ''} INBOUND=${l.Inbound || ''} OUTBOUND=${l.Outbound || ''}${emailPart} EVENT=${eventText}`.trim();
    } catch (_e) {
      return JSON.stringify(l);
    }
  }).join('\n');
  FileManager.downloadTextFile(lines, 'x-ui.log');
}

watch(() => props.open, (next) => { if (next) refresh(); });
watch([rows, showDirect, showBlocked, showProxy], () => { if (props.open) refresh(); });
</script>

<template>
  <a-modal :open="open" :closable="true" :footer="null" width="80vw" @cancel="close">
    <template #title>
      {{ t('pages.index.logs') }}
      <SyncOutlined :spin="loading" class="reload-icon" @click="refresh" />
    </template>

    <a-form layout="inline">
      <a-form-item>
        <a-select v-model:value="rows" size="small" :style="{ width: '70px' }">
          <a-select-option value="10">10</a-select-option>
          <a-select-option value="20">20</a-select-option>
          <a-select-option value="50">50</a-select-option>
          <a-select-option value="100">100</a-select-option>
          <a-select-option value="500">500</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item :label="t('filter')">
        <a-input v-model:value="filter" size="small" @keyup.enter="refresh" />
      </a-form-item>
      <a-form-item>
        <a-checkbox v-model:checked="showDirect">Direct</a-checkbox>
        <a-checkbox v-model:checked="showBlocked">Blocked</a-checkbox>
        <a-checkbox v-model:checked="showProxy">Proxy</a-checkbox>
      </a-form-item>
      <a-form-item style="margin-left: auto">
        <a-button type="primary" @click="download">
          <template #icon>
            <DownloadOutlined />
          </template>
        </a-button>
      </a-form-item>
    </a-form>

    <div class="log-container" v-html="formattedLogs" />
  </a-modal>
</template>

<style scoped>
.reload-icon {
  cursor: pointer;
  vertical-align: middle;
  margin-left: 10px;
}

.log-container {
  margin-top: 12px;
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  max-height: 60vh;
  overflow: auto;
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.04);
}

:global(body.dark) .log-container {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
}
</style>

<style>
/* Global so the v-html'd table picks up these styles. */
.xraylog-table {
  border-collapse: collapse;
  width: auto;
}

.xraylog-table td,
.xraylog-table th {
  padding: 2px 15px;
}
</style>
