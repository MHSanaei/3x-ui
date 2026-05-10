<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { HttpUtil } from '@/utils';
import Sparkline from '@/components/Sparkline.vue';

const { t } = useI18n();

const props = defineProps({
  node: { type: Object, required: true },
  // Bucket size in seconds — matches the SystemHistoryModal selector.
  bucket: { type: Number, default: 30 },
});

// Two parallel series so the panel renders CPU and Mem side-by-side
// in a single fetch round-trip per refresh.
const cpuPoints = ref([]);
const cpuLabels = ref([]);
const memPoints = ref([]);
const memLabels = ref([]);

const REFRESH_MS = 15000;
let timer = null;

function bucketLabel(unixSec) {
  const d = new Date(unixSec * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  if (props.bucket >= 60) return `${hh}:${mm}`;
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${hh}:${mm}:${ss}`;
}

async function fetchSeries(metric) {
  try {
    const url = `/panel/api/nodes/history/${props.node.id}/${metric}/${props.bucket}`;
    const msg = await HttpUtil.get(url);
    if (msg?.success && Array.isArray(msg.obj)) {
      const vals = [];
      const labs = [];
      for (const p of msg.obj) {
        labs.push(bucketLabel(p.t));
        vals.push(Math.max(0, Math.min(100, Number(p.v) || 0)));
      }
      return { vals, labs };
    }
  } catch (e) {
    console.error('node history fetch failed', metric, e);
  }
  return { vals: [], labs: [] };
}

async function refresh() {
  const [cpu, mem] = await Promise.all([fetchSeries('cpu'), fetchSeries('mem')]);
  cpuPoints.value = cpu.vals;
  cpuLabels.value = cpu.labs;
  memPoints.value = mem.vals;
  memLabels.value = mem.labs;
}

onMounted(() => {
  refresh();
  timer = window.setInterval(refresh, REFRESH_MS);
});

onBeforeUnmount(() => {
  if (timer != null) window.clearInterval(timer);
});

// If the parent table re-emits a node row with a different id (rare —
// happens when the list is sorted or filtered while the panel is open),
// reset and re-fetch.
watch(() => props.node?.id, (a, b) => {
  if (a !== b) refresh();
});
</script>

<template>
  <div class="node-history-panel">
    <div class="series">
      <div class="series-title">{{ t('pages.nodes.cpu') }}</div>
      <Sparkline :data="cpuPoints" :labels="cpuLabels" :vb-width="640" :height="120" stroke="#008771" :show-grid="true"
        :show-axes="true" :tick-count-x="4" :max-points="cpuPoints.length || 1" :fill-opacity="0.18"
        :marker-radius="2.6" :show-tooltip="true" />
    </div>
    <div class="series">
      <div class="series-title">{{ t('pages.nodes.mem') }}</div>
      <Sparkline :data="memPoints" :labels="memLabels" :vb-width="640" :height="120" stroke="#7c4dff" :show-grid="true"
        :show-axes="true" :tick-count-x="4" :max-points="memPoints.length || 1" :fill-opacity="0.18"
        :marker-radius="2.6" :show-tooltip="true" />
    </div>
  </div>
</template>

<style scoped>
.node-history-panel {
  padding: 8px 0;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}

@media (max-width: 768px) {
  .node-history-panel {
    grid-template-columns: 1fr;
    gap: 12px;
  }
}

.series-title {
  font-size: 12px;
  font-weight: 500;
  opacity: 0.75;
  margin-bottom: 4px;
}
</style>
