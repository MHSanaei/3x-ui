<script setup>
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { HttpUtil } from '@/utils';
import Sparkline from '@/components/Sparkline.vue';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  status: { type: Object, required: true },
});

const emit = defineEmits(['update:open']);

// Bucket size in seconds per data point — matches legacy options.
const bucket = ref(2);
const points = ref([]);
const labels = ref([]);

async function fetchBucket() {
  try {
    const msg = await HttpUtil.get(`/panel/api/server/cpuHistory/${bucket.value}`);
    if (msg?.success && Array.isArray(msg.obj)) {
      const vals = [];
      const labs = [];
      for (const p of msg.obj) {
        const d = new Date(p.t * 1000);
        const hh = String(d.getHours()).padStart(2, '0');
        const mm = String(d.getMinutes()).padStart(2, '0');
        const ss = String(d.getSeconds()).padStart(2, '0');
        labs.push(bucket.value >= 60 ? `${hh}:${mm}` : `${hh}:${mm}:${ss}`);
        vals.push(Math.max(0, Math.min(100, p.cpu)));
      }
      labels.value = labs;
      points.value = vals;
    }
  } catch (e) {
    console.error('Failed to fetch bucketed cpu history', e);
  }
}

function close() {
  emit('update:open', false);
}

watch(() => props.open, (next) => { if (next) fetchBucket(); });
watch(bucket, () => { if (props.open) fetchBucket(); });
</script>

<template>
  <a-modal :open="open" :closable="true" :footer="null" width="900px" @cancel="close">
    <template #title>
      {{ t('pages.index.cpu') }}
      <a-select v-model:value="bucket" size="small" class="bucket-select">
        <a-select-option :value="2">2m</a-select-option>
        <a-select-option :value="30">30m</a-select-option>
        <a-select-option :value="60">1h</a-select-option>
        <a-select-option :value="120">2h</a-select-option>
        <a-select-option :value="180">3h</a-select-option>
        <a-select-option :value="300">5h</a-select-option>
      </a-select>
    </template>

    <div class="cpu-chart-wrap">
      <Sparkline :data="points" :labels="labels" :vb-width="840" :height="220" :stroke="status?.cpu?.color || '#008771'"
        :stroke-width="2.2" :show-grid="true" :show-axes="true" :tick-count-x="5" :max-points="points.length || 1"
        :fill-opacity="0.18" :marker-radius="3.2" :show-tooltip="true" />
      <div class="cpu-chart-meta">
        Timeframe: {{ bucket }} sec per point (total {{ points.length }} points)
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
.bucket-select {
  width: 80px;
  margin-left: 10px;
}

.cpu-chart-wrap {
  padding: 16px;
}

.cpu-chart-meta {
  margin-top: 4px;
  font-size: 11px;
  opacity: 0.65;
}
</style>
