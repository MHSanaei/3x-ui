<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { HttpUtil, SizeFormatter } from '@/utils';
import Sparkline from '@/components/Sparkline.vue';
import { useMediaQuery } from '@/composables/useMediaQuery.js';

const { t } = useI18n();
const { isMobile } = useMediaQuery();
const modalWidth = computed(() => (isMobile.value ? '95vw' : '900px'));

const props = defineProps({
  open: { type: Boolean, default: false },
});
const emit = defineEmits(['update:open']);

const OBS_KEY = 'xrObs';

const metrics = [
  { key: 'xrAlloc', tab: 'Heap', unit: 'B', stroke: '#7c4dff' },
  { key: 'xrSys', tab: 'Sys', unit: 'B', stroke: '#1890ff' },
  { key: 'xrHeapObjects', tab: 'Objects', unit: '', stroke: '#13c2c2' },
  { key: 'xrNumGC', tab: 'GC Count', unit: '', stroke: '#fa8c16' },
  { key: 'xrPauseNs', tab: 'GC Pause', unit: 'ns', stroke: '#f5222d' },
  { key: OBS_KEY, tab: 'Observatory', unit: 'ms', stroke: '#52c41a' },
];

const activeKey = ref('xrAlloc');
const bucket = ref(2);
const points = ref([]);
const labels = ref([]);
const state = ref({ enabled: false, listen: '', reason: '' });
const obsTags = ref([]);
const obsActiveTag = ref('');
let obsTimer = null;

const activeMetric = computed(() => metrics.find((m) => m.key === activeKey.value));
const isObservatory = computed(() => activeKey.value === OBS_KEY);
const strokeColor = computed(() => activeMetric.value?.stroke || '#008771');
const activeObsTag = computed(() => obsTags.value.find((tg) => tg.tag === obsActiveTag.value) || null);

function unitFormatter(unit) {
  if (unit === 'B') {
    return (v) => SizeFormatter.sizeFormat(Math.max(0, Number(v) || 0));
  }
  if (unit === 'ns') {
    return (v) => {
      const n = Math.max(0, Number(v) || 0);
      if (n >= 1e6) return `${(n / 1e6).toFixed(2)} ms`;
      if (n >= 1e3) return `${(n / 1e3).toFixed(1)} µs`;
      return `${n.toFixed(0)} ns`;
    };
  }
  if (unit === 'ms') {
    return (v) => `${Math.round(Number(v) || 0)} ms`;
  }
  return (v) => {
    const n = Number(v) || 0;
    return Math.round(n).toLocaleString();
  };
}

const yFormatter = computed(() => unitFormatter(activeMetric.value?.unit ?? ''));

function fmtTimestamp(unixSec) {
  if (!unixSec) return '—';
  const d = new Date(unixSec * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${d.toLocaleDateString()} ${hh}:${mm}:${ss}`;
}

async function fetchState() {
  try {
    const msg = await HttpUtil.get('/panel/api/server/xrayMetricsState');
    if (msg?.success && msg.obj) state.value = msg.obj;
  } catch (e) {
    console.error('Failed to fetch xray metrics state', e);
  }
}

async function fetchObservatory() {
  try {
    const msg = await HttpUtil.get('/panel/api/server/xrayObservatory');
    if (msg?.success && Array.isArray(msg.obj)) {
      obsTags.value = msg.obj;
      if (!obsTags.value.find((tg) => tg.tag === obsActiveTag.value)) {
        obsActiveTag.value = obsTags.value[0]?.tag || '';
      }
    } else {
      obsTags.value = [];
    }
  } catch (e) {
    console.error('Failed to fetch observatory snapshot', e);
    obsTags.value = [];
  }
}

async function fetchMetricBucket() {
  const m = activeMetric.value;
  if (!m) return;
  try {
    const url = `/panel/api/server/xrayMetricsHistory/${m.key}/${bucket.value}`;
    const msg = await HttpUtil.get(url);
    applyHistory(msg);
  } catch (e) {
    console.error('Failed to fetch xray metrics bucket', e);
    labels.value = [];
    points.value = [];
  }
}

async function fetchObsBucket() {
  const tag = obsActiveTag.value;
  if (!tag) {
    labels.value = [];
    points.value = [];
    return;
  }
  try {
    const url = `/panel/api/server/xrayObservatoryHistory/${encodeURIComponent(tag)}/${bucket.value}`;
    const msg = await HttpUtil.get(url);
    applyHistory(msg);
  } catch (e) {
    console.error('Failed to fetch observatory bucket', e);
    labels.value = [];
    points.value = [];
  }
}

function applyHistory(msg) {
  if (msg?.success && Array.isArray(msg.obj)) {
    const vals = [];
    const labs = [];
    for (const p of msg.obj) {
      const d = new Date(p.t * 1000);
      const hh = String(d.getHours()).padStart(2, '0');
      const mm = String(d.getMinutes()).padStart(2, '0');
      const ss = String(d.getSeconds()).padStart(2, '0');
      labs.push(bucket.value >= 60 ? `${hh}:${mm}` : `${hh}:${mm}:${ss}`);
      vals.push(Number(p.v) || 0);
    }
    labels.value = labs;
    points.value = vals;
  } else {
    labels.value = [];
    points.value = [];
  }
}

function refreshActive() {
  if (isObservatory.value) {
    fetchObsBucket();
  } else {
    fetchMetricBucket();
  }
}

function startObsPolling() {
  stopObsPolling();
  obsTimer = window.setInterval(async () => {
    if (!props.open || !isObservatory.value) return;
    await fetchObservatory();
    fetchObsBucket();
  }, 2000);
}

function stopObsPolling() {
  if (obsTimer != null) {
    window.clearInterval(obsTimer);
    obsTimer = null;
  }
}

function close() {
  emit('update:open', false);
}

watch(() => props.open, (next) => {
  if (next) {
    activeKey.value = 'xrAlloc';
    fetchState();
    fetchMetricBucket();
  } else {
    stopObsPolling();
  }
});

watch(activeKey, async (key) => {
  if (!props.open) return;
  if (key === OBS_KEY) {
    await fetchObservatory();
    fetchObsBucket();
    startObsPolling();
  } else {
    stopObsPolling();
    fetchMetricBucket();
  }
});

watch(bucket, () => {
  if (props.open) refreshActive();
});

watch(obsActiveTag, () => {
  if (props.open && isObservatory.value) fetchObsBucket();
});
</script>

<template>
  <a-modal :open="open" :closable="true" :footer="null" :width="modalWidth" @cancel="close">
    <template #title>
      {{ t('pages.index.xrayMetricsTitle') }}
      <a-select v-model:value="bucket" size="small" class="bucket-select">
        <a-select-option :value="2">2m</a-select-option>
        <a-select-option :value="30">30m</a-select-option>
        <a-select-option :value="60">1h</a-select-option>
        <a-select-option :value="120">2h</a-select-option>
        <a-select-option :value="180">3h</a-select-option>
        <a-select-option :value="300">5h</a-select-option>
      </a-select>
    </template>

    <a-alert v-if="!state.enabled" type="warning" show-icon class="metrics-alert"
      :message="t('pages.index.xrayMetricsDisabled')"
      :description="state.reason || t('pages.index.xrayMetricsHint')" />

    <a-tabs v-model:active-key="activeKey" size="small" class="history-tabs">
      <a-tab-pane v-for="m in metrics" :key="m.key" :tab="m.tab" />
    </a-tabs>

    <div v-if="isObservatory" class="obs-pane">
      <a-alert v-if="state.enabled && obsTags.length === 0" type="info" show-icon class="metrics-alert"
        :message="t('pages.index.xrayObservatoryEmpty')"
        :description="t('pages.index.xrayObservatoryHint')" />

      <div v-else class="obs-controls">
        <a-select v-model:value="obsActiveTag" size="small" class="obs-select"
          :placeholder="t('pages.index.xrayObservatoryTagPlaceholder')">
          <a-select-option v-for="tg in obsTags" :key="tg.tag" :value="tg.tag">
            <span class="obs-dot" :class="tg.alive ? 'is-alive' : 'is-dead'" />
            {{ tg.tag }}
          </a-select-option>
        </a-select>

        <div v-if="activeObsTag" class="obs-stats">
          <a-tag :color="activeObsTag.alive ? 'green' : 'red'">
            {{ activeObsTag.alive ? t('pages.index.xrayObservatoryAlive') : t('pages.index.xrayObservatoryDead') }}
          </a-tag>
          <a-tag color="blue">{{ activeObsTag.delay }} ms</a-tag>
          <span class="obs-stamp">
            {{ t('pages.index.xrayObservatoryLastSeen') }}: {{ fmtTimestamp(activeObsTag.lastSeenTime) }}
          </span>
          <span class="obs-stamp">
            {{ t('pages.index.xrayObservatoryLastTry') }}: {{ fmtTimestamp(activeObsTag.lastTryTime) }}
          </span>
        </div>
      </div>
    </div>

    <div class="cpu-chart-wrap">
      <div class="cpu-chart-meta">
        Timeframe: {{ bucket }} sec per point (total {{ points.length }} points)
        <span v-if="state.enabled && state.listen" class="listen-tag"> · {{ state.listen }}</span>
      </div>
      <Sparkline :data="points" :labels="labels" :vb-width="840" :height="220" :stroke="strokeColor" :stroke-width="2.2"
        :show-grid="true" :show-axes="true" :tick-count-x="5" :max-points="points.length || 1" :fill-opacity="0.18"
        :marker-radius="3.2" :show-tooltip="true" :value-min="0" :value-max="null" :y-formatter="yFormatter" />
    </div>
  </a-modal>
</template>

<style scoped>
.bucket-select {
  width: 80px;
  margin-left: 10px;
}

.metrics-alert {
  margin-bottom: 10px;
}

.history-tabs {
  margin-bottom: 4px;
}

.obs-pane {
  padding: 4px 16px 0;
}

.obs-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.obs-select {
  min-width: 240px;
}

.obs-stats {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  opacity: 0.85;
}

.obs-stamp {
  opacity: 0.7;
}

.obs-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}

.obs-dot.is-alive {
  background: #52c41a;
}

.obs-dot.is-dead {
  background: #f5222d;
}

.cpu-chart-wrap {
  padding: 8px 16px 16px;
}

.cpu-chart-meta {
  margin-bottom: 10px;
  font-size: 11px;
  opacity: 0.65;
}

.listen-tag {
  opacity: 0.7;
}
</style>
