<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons-vue';

import { HttpUtil, FileManager, PromiseUtil } from '@/utils';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open']);

const rows = ref('20');
const level = ref('info');
const syslog = ref(false);
const loading = ref(false);
const logs = ref([]);

const LEVELS = ['DEBUG', 'INFO', 'NOTICE', 'WARNING', 'ERROR'];
const LEVEL_COLORS = ['#3c89e8', '#008771', '#008771', '#f37b24', '#e04141', '#bcbcbc'];

function escapeHtml(value) {
  if (value == null) return '';
  return String(value)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function formatLogs(lines) {
  // Each line: "YYYY-MM-DD HH:MM:SS LEVEL - message"
  // Color the timestamp + level prefix and bold the originating service.
  let out = '';
  lines.forEach((log, idx) => {
    const [data, message] = log.split(' - ', 2);
    const parts = data.split(' ');
    if (idx > 0) out += '<br>';

    if (parts.length === 3) {
      const d = escapeHtml(parts[0]);
      const t = escapeHtml(parts[1]);
      const levelRaw = parts[2];
      const li = LEVELS.indexOf(levelRaw);
      const levelIndex = li >= 0 ? li : 5;
      out += `<span style="color: ${LEVEL_COLORS[0]};">${d} ${t}</span> `;
      out += `<span style="color: ${LEVEL_COLORS[levelIndex]}">${escapeHtml(levelRaw)}</span>`;
    } else {
      const li = LEVELS.indexOf(data);
      const levelIndex = li >= 0 ? li : 5;
      out += `<span style="color: ${LEVEL_COLORS[levelIndex]}">${escapeHtml(data)}</span>`;
    }

    if (message) {
      const prefix = message.startsWith('XRAY:') ? '<b>XRAY: </b>' : '<b>X-UI: </b>';
      const tail = message.startsWith('XRAY:') ? message.substring(5) : message;
      out += ' - ' + prefix + escapeHtml(tail);
    }
  });
  return out;
}

const formattedLogs = computed(() => (logs.value.length > 0 ? formatLogs(logs.value) : 'No Record...'));

async function refresh() {
  loading.value = true;
  try {
    const msg = await HttpUtil.post(`/panel/api/server/logs/${rows.value}`, {
      level: level.value,
      syslog: syslog.value,
    });
    if (msg?.success) {
      logs.value = msg.obj || [];
    }
    // Keep the spinner visible long enough that rapid filter changes
    // feel intentional rather than flickery.
    await PromiseUtil.sleep(300);
  } finally {
    loading.value = false;
  }
}

function close() {
  emit('update:open', false);
}

function download() {
  FileManager.downloadTextFile(logs.value.join('\n'), 'x-ui.log');
}

// Re-fetch whenever the modal opens or any filter changes.
watch(() => props.open, (next) => { if (next) refresh(); });
watch([rows, level, syslog], () => { if (props.open) refresh(); });
</script>

<template>
  <a-modal :open="open" :closable="true" :footer="null" width="800px" @cancel="close">
    <template #title>
      {{ t('pages.index.logs') }}
      <SyncOutlined :spin="loading" class="reload-icon" @click="refresh" />
    </template>

    <a-form layout="inline">
      <a-form-item>
        <a-input-group compact>
          <a-select v-model:value="rows" size="small" :style="{ width: '70px' }">
            <a-select-option value="10">10</a-select-option>
            <a-select-option value="20">20</a-select-option>
            <a-select-option value="50">50</a-select-option>
            <a-select-option value="100">100</a-select-option>
            <a-select-option value="500">500</a-select-option>
          </a-select>
          <a-select v-model:value="level" size="small" :style="{ width: '95px' }">
            <a-select-option value="debug">Debug</a-select-option>
            <a-select-option value="info">Info</a-select-option>
            <a-select-option value="notice">Notice</a-select-option>
            <a-select-option value="warning">Warning</a-select-option>
            <a-select-option value="err">Error</a-select-option>
          </a-select>
        </a-input-group>
      </a-form-item>
      <a-form-item>
        <a-checkbox v-model:checked="syslog">SysLog</a-checkbox>
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
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 60vh;
  overflow-y: auto;
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.04);
}

:global(body.dark) .log-container {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
}
</style>
