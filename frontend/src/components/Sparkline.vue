<script setup>
import { computed, ref } from 'vue';

const props = defineProps({
  data: { type: Array, required: true },
  labels: { type: Array, default: () => [] },
  vbWidth: { type: Number, default: 320 },
  height: { type: Number, default: 80 },
  stroke: { type: String, default: '#008771' },
  strokeWidth: { type: Number, default: 2 },
  maxPoints: { type: Number, default: 120 },
  showGrid: { type: Boolean, default: true },
  gridColor: { type: String, default: 'rgba(0,0,0,0.1)' },
  fillOpacity: { type: Number, default: 0.15 },
  showMarker: { type: Boolean, default: true },
  markerRadius: { type: Number, default: 2.8 },
  showAxes: { type: Boolean, default: false },
  yTickStep: { type: Number, default: 25 },
  tickCountX: { type: Number, default: 4 },
  paddingLeft: { type: Number, default: 32 },
  paddingRight: { type: Number, default: 6 },
  paddingTop: { type: Number, default: 6 },
  paddingBottom: { type: Number, default: 20 },
  showTooltip: { type: Boolean, default: false },
});

const hoverIdx = ref(-1);

const viewBoxAttr = computed(() => `0 0 ${props.vbWidth} ${props.height}`);
const drawWidth = computed(() => Math.max(1, props.vbWidth - props.paddingLeft - props.paddingRight));
const drawHeight = computed(() => Math.max(1, props.height - props.paddingTop - props.paddingBottom));
const nPoints = computed(() => Math.min(props.data.length, props.maxPoints));

const dataSlice = computed(() => {
  const n = nPoints.value;
  if (n === 0) return [];
  return props.data.slice(props.data.length - n);
});

const labelsSlice = computed(() => {
  const n = nPoints.value;
  if (!props.labels?.length || n === 0) return [];
  const start = Math.max(0, props.labels.length - n);
  return props.labels.slice(start);
});

const pointsArr = computed(() => {
  const n = nPoints.value;
  if (n === 0) return [];
  const slice = dataSlice.value;
  const w = drawWidth.value;
  const h = drawHeight.value;
  const dx = n > 1 ? w / (n - 1) : 0;
  return slice.map((v, i) => {
    const x = Math.round(props.paddingLeft + i * dx);
    const y = Math.round(props.paddingTop + (h - (Math.max(0, Math.min(100, v)) / 100) * h));
    return [x, y];
  });
});

const pointsStr = computed(() => pointsArr.value.map((p) => `${p[0]},${p[1]}`).join(' '));

const areaPath = computed(() => {
  if (pointsArr.value.length === 0) return '';
  const first = pointsArr.value[0];
  const last = pointsArr.value[pointsArr.value.length - 1];
  const baseY = props.paddingTop + drawHeight.value;
  const line = pointsStr.value.replace(/ /g, ' L ');
  return `M ${first[0]},${baseY} L ${line} L ${last[0]},${baseY} Z`;
});

const gridLines = computed(() => {
  if (!props.showGrid) return [];
  const h = drawHeight.value;
  const w = drawWidth.value;
  return [0, 0.25, 0.5, 0.75, 1].map((r) => {
    const y = Math.round(props.paddingTop + h * r);
    return { x1: props.paddingLeft, y1: y, x2: props.paddingLeft + w, y2: y };
  });
});

const lastPoint = computed(() => {
  if (pointsArr.value.length === 0) return null;
  return pointsArr.value[pointsArr.value.length - 1];
});

const yTicks = computed(() => {
  if (!props.showAxes) return [];
  const step = Math.max(1, props.yTickStep);
  const out = [];
  for (let p = 0; p <= 100; p += step) {
    const y = Math.round(props.paddingTop + (drawHeight.value - (p / 100) * drawHeight.value));
    out.push({ y, label: `${p}%` });
  }
  return out;
});

const xTicks = computed(() => {
  if (!props.showAxes) return [];
  const labels = labelsSlice.value;
  const n = nPoints.value;
  if (n === 0) return [];
  const m = Math.max(2, props.tickCountX);
  const w = drawWidth.value;
  const dx = n > 1 ? w / (n - 1) : 0;
  const out = [];
  for (let i = 0; i < m; i++) {
    const idx = Math.round((i * (n - 1)) / (m - 1));
    const label = labels[idx] != null ? String(labels[idx]) : String(idx);
    const x = Math.round(props.paddingLeft + idx * dx);
    out.push({ x, label });
  }
  return out;
});

function onMouseMove(evt) {
  if (!props.showTooltip || pointsArr.value.length === 0) return;
  const rect = evt.currentTarget.getBoundingClientRect();
  const px = evt.clientX - rect.left;
  const x = (px / rect.width) * props.vbWidth;
  const n = nPoints.value;
  const dx = n > 1 ? drawWidth.value / (n - 1) : 0;
  const idx = Math.max(0, Math.min(n - 1, Math.round((x - props.paddingLeft) / (dx || 1))));
  hoverIdx.value = idx;
}

function onMouseLeave() {
  hoverIdx.value = -1;
}

function fmtHoverText() {
  const idx = hoverIdx.value;
  if (idx < 0 || idx >= dataSlice.value.length) return '';
  const raw = Math.max(0, Math.min(100, Number(dataSlice.value[idx] || 0)));
  const val = Number.isFinite(raw) ? raw.toFixed(2) : raw;
  const lab = labelsSlice.value[idx] != null ? labelsSlice.value[idx] : '';
  return `${val}%${lab ? ' • ' + lab : ''}`;
}

// Stable per-instance gradient id so multiple sparklines on a page
// don't clobber each other's <defs id="spkGrad">.
const gradId = `spkGrad-${Math.random().toString(36).slice(2, 9)}`;
</script>

<template>
  <svg
    width="100%"
    :height="height"
    :viewBox="viewBoxAttr"
    preserveAspectRatio="none"
    class="sparkline-svg"
    @mousemove="onMouseMove"
    @mouseleave="onMouseLeave"
  >
    <defs>
      <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" :stop-color="stroke" :stop-opacity="fillOpacity" />
        <stop offset="100%" :stop-color="stroke" stop-opacity="0" />
      </linearGradient>
    </defs>

    <g v-if="showGrid">
      <line
        v-for="(g, i) in gridLines"
        :key="i"
        :x1="g.x1" :y1="g.y1" :x2="g.x2" :y2="g.y2"
        :stroke="gridColor" stroke-width="1"
        class="cpu-grid-line"
      />
    </g>

    <g v-if="showAxes">
      <text
        v-for="(t, i) in yTicks"
        :key="'y' + i"
        class="cpu-grid-y-text"
        :x="Math.max(0, paddingLeft - 4)"
        :y="t.y + 4"
        text-anchor="end"
        font-size="10"
      >{{ t.label }}</text>
      <text
        v-for="(t, i) in xTicks"
        :key="'x' + i"
        class="cpu-grid-x-text"
        :x="t.x"
        :y="paddingTop + drawHeight + 22"
        text-anchor="middle"
        font-size="10"
      >{{ t.label }}</text>
    </g>

    <path v-if="areaPath" :d="areaPath" :fill="`url(#${gradId})`" stroke="none" />
    <polyline
      :points="pointsStr"
      fill="none"
      :stroke="stroke"
      :stroke-width="strokeWidth"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <circle
      v-if="showMarker && lastPoint"
      :cx="lastPoint[0]" :cy="lastPoint[1]"
      :r="markerRadius"
      :fill="stroke"
    />

    <g v-if="showTooltip && hoverIdx >= 0 && pointsArr[hoverIdx]">
      <line
        class="cpu-grid-h-line"
        :x1="pointsArr[hoverIdx][0]" :x2="pointsArr[hoverIdx][0]"
        :y1="paddingTop" :y2="paddingTop + drawHeight"
        stroke="rgba(0,0,0,0.2)" stroke-width="1"
      />
      <circle
        :cx="pointsArr[hoverIdx][0]" :cy="pointsArr[hoverIdx][1]"
        r="3.5" :fill="stroke"
      />
      <text
        class="cpu-grid-text"
        :x="pointsArr[hoverIdx][0]"
        :y="paddingTop + 12"
        text-anchor="middle"
        font-size="11"
      >{{ fmtHoverText() }}</text>
    </g>
  </svg>
</template>

<style scoped>
.sparkline-svg {
  display: block;
  width: 100%;
}

.cpu-grid-y-text,
.cpu-grid-x-text {
  fill: rgba(0, 0, 0, 0.45);
}
.cpu-grid-text {
  fill: rgba(0, 0, 0, 0.8);
}

:global(body.dark) .sparkline-svg .cpu-grid-y-text,
:global(body.dark) .sparkline-svg .cpu-grid-x-text {
  fill: rgba(255, 255, 255, 0.55);
}
:global(body.dark) .sparkline-svg .cpu-grid-text {
  fill: rgba(255, 255, 255, 0.85);
}
</style>
