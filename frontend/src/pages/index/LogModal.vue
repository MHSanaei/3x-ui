<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons-vue';

import { HttpUtil, FileManager, PromiseUtil } from '@/utils';
import { useMediaQuery } from '@/composables/useMediaQuery.js';

const { t } = useI18n();
const { isMobile } = useMediaQuery();

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
const LEVEL_CLASSES = ['level-debug', 'level-info', 'level-notice', 'level-warning', 'level-error'];

// Parses "YYYY-MM-DD HH:MM:SS LEVEL - message". Lines without the
// 3-token header degrade gracefully: the unparsed head becomes the
// level so it still gets color-coded.
function parseLogLine(line) {
  const [head, ...rest] = (line || '').split(' - ');
  const message = rest.join(' - ');
  const parts = head.split(' ');

  let date = '';
  let time = '';
  let levelText;
  if (parts.length >= 3) {
    [date, time, levelText] = parts;
  } else {
    levelText = head;
  }

  const li = LEVELS.indexOf(levelText);
  const levelClass = li >= 0 ? LEVEL_CLASSES[li] : 'level-unknown';

  let service = '';
  let body = message || '';
  if (body.startsWith('XRAY:')) {
    service = 'XRAY:';
    body = body.slice('XRAY:'.length).trimStart();
  } else if (body) {
    service = 'X-UI:';
  }

  return { date, time, levelText, levelClass, service, body };
}

const parsedLogs = computed(() => logs.value.map(parseLogLine));

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

watch(() => props.open, (next) => { if (next) refresh(); });
watch([rows, level, syslog], () => { if (props.open) refresh(); });

const modalWidth = computed(() => (isMobile.value ? '100vw' : '800px'));
</script>

<template>
  <a-modal :open="open" :closable="true" :footer="null" :width="modalWidth" :class="{ 'logmodal-mobile': isMobile }"
    @cancel="close">
    <template #title>
      {{ t('pages.index.logs') }}
      <SyncOutlined :spin="loading" class="reload-icon" @click="refresh" />
    </template>

    <a-form layout="inline" class="log-toolbar">
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
      <a-form-item class="download-item">
        <a-button type="primary" @click="download">
          <template #icon>
            <DownloadOutlined />
          </template>
        </a-button>
      </a-form-item>
    </a-form>

    <div class="log-container" :class="{ 'log-container-mobile': isMobile }">
      <div v-if="parsedLogs.length === 0" class="log-empty">No Record...</div>

      <template v-else-if="isMobile">
        <div v-for="(log, idx) in parsedLogs" :key="idx" class="log-card">
          <div class="log-card-head">
            <span v-if="log.date || log.time" class="log-time">
              <span v-if="log.time">{{ log.time }}</span>
              <span v-if="log.date" class="log-date">{{ log.date }}</span>
            </span>
            <span v-if="log.levelText" class="log-level-badge" :class="log.levelClass">
              {{ log.levelText }}
            </span>
          </div>
          <div v-if="log.body || log.service" class="log-body">
            <b v-if="log.service">{{ log.service }}</b>
            <span v-if="log.body" class="log-body-text">{{ log.body }}</span>
          </div>
        </div>
      </template>

      <template v-else>
        <div v-for="(log, idx) in parsedLogs" :key="idx" class="log-line">
          <span v-if="log.date || log.time" class="log-stamp">
            {{ log.date }}<template v-if="log.date && log.time"> </template>{{ log.time }}
          </span>
          <span v-if="log.levelText" class="log-level" :class="log.levelClass">
            {{ log.levelText }}
          </span>
          <template v-if="log.body || log.service">
            <span> - </span>
            <b v-if="log.service">{{ log.service }} </b>
            <span>{{ log.body }}</span>
          </template>
        </div>
      </template>
    </div>
  </a-modal>
</template>

<style scoped>
.reload-icon {
  cursor: pointer;
  vertical-align: middle;
  margin-left: 10px;
}

.log-toolbar {
  flex-wrap: wrap;
  row-gap: 8px;
}

.log-toolbar .download-item {
  margin-left: auto;
}

.log-container {
  /* Per-theme palette — overridden in body.dark / [data-theme="ultra-dark"]
     below so each level keeps ≥4.5:1 contrast against the container. */
  --log-stamp: #3c89e8;
  --log-debug: #3c89e8;
  --log-info: #008771;
  --log-notice: #008771;
  --log-warning: #f37b24;
  --log-error: #e04141;
  --log-unknown: #595959;
  --log-divider: rgba(128, 128, 128, 0.18);

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

.log-stamp {
  color: var(--log-stamp);
}

.log-level {
  margin-left: 4px;
}

.level-debug {
  color: var(--log-debug);
}

.level-info {
  color: var(--log-info);
}

.level-notice {
  color: var(--log-notice);
}

.level-warning {
  color: var(--log-warning);
}

.level-error {
  color: var(--log-error);
}

.level-unknown {
  color: var(--log-unknown);
}

.log-container-mobile {
  padding: 8px;
  white-space: normal;
  max-height: 70vh;
}

.log-empty {
  text-align: center;
  opacity: 0.5;
  padding: 20px 0;
}

.log-line+.log-line {
  margin-top: 2px;
}

.log-card {
  border-bottom: 1px solid var(--log-divider);
  padding: 8px 0;
}

.log-card:last-child {
  border-bottom: 0;
}

.log-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}

.log-time {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  font-weight: 600;
  font-size: 12px;
  letter-spacing: 0.02em;
}

.log-date {
  font-size: 10px;
  font-weight: 500;
  opacity: 0.55;
}

.log-level-badge {
  display: inline-block;
  font-size: 10px;
  line-height: 14px;
  padding: 0 6px;
  border-radius: 4px;
  border: 1px solid currentColor;
  letter-spacing: 0.04em;
  font-weight: 600;
  white-space: nowrap;
  background: color-mix(in srgb, currentColor 14%, transparent);
}

.log-body {
  font-size: 12px;
  word-break: break-word;
}

.log-body-text {
  margin-left: 4px;
}

:global(body.dark) .log-container {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.88);

  --log-stamp: #6aa6ee;
  --log-debug: #6aa6ee;
  --log-info: #4ed3a6;
  --log-notice: #4ed3a6;
  --log-warning: #ffb872;
  --log-error: #ff7575;
  --log-unknown: #b5b5b5;
  --log-divider: rgba(255, 255, 255, 0.1);
}

:global([data-theme="ultra-dark"]) .log-container {
  --log-stamp: #7fb6f1;
  --log-debug: #7fb6f1;
  --log-info: #5fd9b0;
  --log-notice: #5fd9b0;
  --log-warning: #ffcc88;
  --log-error: #ff8a8a;
  --log-unknown: #c4c4c4;
  --log-divider: rgba(255, 255, 255, 0.12);
}

/* Mobile: pull the modal flush with the screen edges. */
:global(.logmodal-mobile) {
  top: 0 !important;
  padding-bottom: 0 !important;
  max-width: 100vw !important;
}

:global(.logmodal-mobile .ant-modal-content) {
  border-radius: 0;
  height: 100vh;
}

:global(.logmodal-mobile .ant-modal-body) {
  padding: 12px;
}
</style>
