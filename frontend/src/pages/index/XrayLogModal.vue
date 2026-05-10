<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons-vue';

import { HttpUtil, FileManager, IntlUtil, PromiseUtil } from '@/utils';
import { useDatepicker } from '@/composables/useDatepicker.js';
import { useMediaQuery } from '@/composables/useMediaQuery.js';

const { t } = useI18n();
const { datepicker } = useDatepicker();
const { isMobile } = useMediaQuery();

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

// Newest first.
const orderedLogs = computed(() => [...logs.value].reverse());

const EVENT_LABELS = { 0: 'DIRECT', 1: 'BLOCKED', 2: 'PROXY' };
const EVENT_COLORS = { 0: 'green', 1: 'red', 2: 'blue' };

function eventLabel(ev) { return EVENT_LABELS[ev] || String(ev ?? ''); }
function eventColor(ev) { return EVENT_COLORS[ev] || 'default'; }

function fullDate(value) {
  return IntlUtil.formatDate(value, datepicker.value);
}
function shortTime(value) {
  if (!value) return '';
  const d = new Date(value);
  if (isNaN(d.getTime())) return '';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${hh}:${mm}:${ss}`;
}

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
  const lines = logs.value.map((l) => {
    try {
      const dt = l.DateTime ? new Date(l.DateTime) : null;
      const dateStr = dt && !isNaN(dt.getTime()) ? dt.toISOString() : '';
      const eventText = eventLabel(l.Event);
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

const modalWidth = computed(() => (isMobile.value ? '100vw' : '80vw'));
</script>

<template>
  <a-modal :open="open" :closable="true" :footer="null" :width="modalWidth"
    :class="{ 'xraylog-modal-mobile': isMobile }" @cancel="close">
    <template #title>
      {{ t('pages.index.logs') }}
      <SyncOutlined :spin="loading" class="reload-icon" @click="refresh" />
    </template>

    <a-form layout="inline" class="log-toolbar">
      <a-form-item>
        <a-select v-model:value="rows" size="small" :style="{ width: '70px' }">
          <a-select-option value="10">10</a-select-option>
          <a-select-option value="20">20</a-select-option>
          <a-select-option value="50">50</a-select-option>
          <a-select-option value="100">100</a-select-option>
          <a-select-option value="500">500</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item :label="t('filter')" class="filter-item">
        <a-input v-model:value="filter" size="small" @keyup.enter="refresh" />
      </a-form-item>
      <a-form-item>
        <a-checkbox v-model:checked="showDirect">Direct</a-checkbox>
        <a-checkbox v-model:checked="showBlocked">Blocked</a-checkbox>
        <a-checkbox v-model:checked="showProxy">Proxy</a-checkbox>
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
      <div v-if="orderedLogs.length === 0" class="log-empty">No Record...</div>

      <template v-else-if="isMobile">
        <div v-for="(log, idx) in orderedLogs" :key="idx" class="log-card">
          <div class="log-card-head">
            <span class="log-time" :title="fullDate(log.DateTime)">{{ shortTime(log.DateTime) }}</span>
            <a-tag :color="eventColor(log.Event)" class="log-event-tag">{{ eventLabel(log.Event) }}</a-tag>
          </div>
          <div class="log-route">
            <span class="log-addr">{{ log.FromAddress }}</span>
            <span class="log-arrow">→</span>
            <span class="log-addr">{{ log.ToAddress }}</span>
          </div>
          <div class="log-meta">
            <span v-if="log.Inbound" class="log-meta-pair">
              <span class="log-meta-key">in</span>
              <span class="log-meta-val">{{ log.Inbound }}</span>
            </span>
            <span v-if="log.Outbound" class="log-meta-pair">
              <span class="log-meta-key">out</span>
              <span class="log-meta-val">{{ log.Outbound }}</span>
            </span>
            <span v-if="log.Email" class="log-meta-pair">
              <span class="log-meta-key">email</span>
              <span class="log-meta-val">{{ log.Email }}</span>
            </span>
          </div>
        </div>
      </template>

      <table v-else class="xraylog-table">
        <thead>
          <tr>
            <th>Date</th>
            <th>From</th>
            <th>To</th>
            <th>Inbound</th>
            <th>Outbound</th>
            <th>Email</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(log, idx) in orderedLogs" :key="idx" :class="`log-row-${log.Event}`">
            <td><b>{{ fullDate(log.DateTime) }}</b></td>
            <td>{{ log.FromAddress }}</td>
            <td>{{ log.ToAddress }}</td>
            <td>{{ log.Inbound }}</td>
            <td>{{ log.Outbound }}</td>
            <td>{{ log.Email }}</td>
          </tr>
        </tbody>
      </table>
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

.log-toolbar .filter-item {
  flex: 1 1 160px;
}

.log-toolbar .download-item {
  margin-left: auto;
}

.log-container {
  /* Per-theme palette — overridden in body.dark / [data-theme="ultra-dark"]
     below so blocked/proxy rows keep ≥4.5:1 contrast on darker surfaces. */
  --log-blocked: #e04141;
  --log-proxy: #3c89e8;
  --log-divider: rgba(128, 128, 128, 0.18);

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

.log-container-mobile {
  padding: 8px;
  font-size: 12px;
  max-height: 70vh;
}

.log-empty {
  text-align: center;
  opacity: 0.5;
  padding: 20px 0;
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
  font-weight: 600;
  font-size: 12px;
  letter-spacing: 0.02em;
}

.log-event-tag {
  margin: 0;
  font-size: 10px;
  line-height: 16px;
  padding: 0 6px;
}

.log-route {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 12px;
  margin-bottom: 4px;
}

.log-addr {
  word-break: break-all;
}

.log-arrow {
  opacity: 0.5;
}

.log-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  font-size: 11px;
  opacity: 0.75;
}

.log-meta-pair {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  word-break: break-all;
}

.log-meta-key {
  font-size: 10px;
  text-transform: uppercase;
  opacity: 0.6;
  letter-spacing: 0.04em;
}

:global(body.dark) .log-container {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.88);

  --log-blocked: #ff7575;
  --log-proxy: #6aa6ee;
  --log-divider: rgba(255, 255, 255, 0.1);
}

:global([data-theme="ultra-dark"]) .log-container {
  --log-blocked: #ff8a8a;
  --log-proxy: #7fb6f1;
  --log-divider: rgba(255, 255, 255, 0.12);
}

/* Mobile: pull the modal flush with the screen edges. */
:global(.xraylog-modal-mobile) {
  top: 0 !important;
  padding-bottom: 0 !important;
  max-width: 100vw !important;
}

:global(.xraylog-modal-mobile .ant-modal-content) {
  border-radius: 0;
  height: 100vh;
}

:global(.xraylog-modal-mobile .ant-modal-body) {
  padding: 12px;
}

.xraylog-table {
  border-collapse: collapse;
  width: 100%;
}

.xraylog-table td,
.xraylog-table th {
  padding: 2px 15px;
  text-align: left;
}

.xraylog-table .log-row-1 {
  color: var(--log-blocked);
}

.xraylog-table .log-row-2 {
  color: var(--log-proxy);
}
</style>
