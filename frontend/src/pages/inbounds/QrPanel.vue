<script setup>
import { onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import QRious from 'qrious';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { ClipboardManager, FileManager } from '@/utils';

const { t } = useI18n();

// Renders a single share-link as a clickable QR code + a copy button
// + (optional) a download button. Used per-link inside the inbound
// info modal — the canvas is repainted whenever `value` changes.

const props = defineProps({
  // The link or config text to encode + display.
  value: { type: String, required: true },
  // Header label shown next to the copy button.
  remark: { type: String, default: '' },
  // Optional download filename — when set, surfaces a download button.
  downloadName: { type: String, default: '' },
  // Final on-screen QR size in CSS pixels. The canvas drawing buffer
  // is rounded down to a multiple of the QR matrix width (so the QR
  // fills it edge-to-edge) and CSS then scales the canvas to exactly
  // this size — so a denser QR (e.g. WireGuard config) and a sparser
  // one (its link) display at identical dimensions.
  size: { type: Number, default: 240 },
  // Toggle the QR rendering off when callers only want the "row of buttons"
  // styling (used when the legacy panel rendered links without QRs).
  showQr: { type: Boolean, default: true },
});

const canvas = ref(null);

// Byte-mode capacities (level M) for QR versions 1..40 — used to pick
// the matrix width up front so we can size the canvas as a multiple
// of pixelSize. Without this, QRious renders at floor(size/matrix)
// and centers, leaving a white margin whenever size isn't divisible.
const QR_M_BYTE_CAPACITY = [
  14, 26, 42, 62, 84, 106, 122, 152, 180, 213,
  251, 287, 331, 362, 412, 450, 504, 560, 624, 666,
  711, 779, 857, 911, 997, 1059, 1125, 1190, 1264, 1370,
  1452, 1538, 1628, 1722, 1809, 1911, 1989, 2099, 2213, 2331,
];

function pickQrMatrixWidth(value) {
  const byteLen = new TextEncoder().encode(value).length;
  for (let i = 0; i < QR_M_BYTE_CAPACITY.length; i++) {
    if (byteLen <= QR_M_BYTE_CAPACITY[i]) return 17 + 4 * (i + 1);
  }
  return 17 + 4 * 40; // version 40 (177 modules)
}

function paint() {
  if (!props.showQr || !canvas.value || !props.value) return;
  // Canvas size = matrixWidth × pixelSize, so the QR fills it edge-to-
  // edge. pixelSize is floored against the requested size so the QR
  // never grows past the host's expected box.
  const matrixWidth = pickQrMatrixWidth(props.value);
  const pixelSize = Math.max(1, Math.floor(props.size / matrixWidth));
  const exactSize = matrixWidth * pixelSize;
  new QRious({
    element: canvas.value,
    size: exactSize,
    value: props.value,
    background: 'white',
    backgroundAlpha: 1,
    foreground: 'black',
    padding: 0,
    level: 'M',
  });
}

onMounted(paint);
watch(() => props.value, paint);
watch(() => props.size, paint);

async function copy() {
  const ok = await ClipboardManager.copyText(props.value);
  if (ok) message.success(t('copied'));
}

function download() {
  if (!props.downloadName) return;
  FileManager.downloadTextFile(props.value, props.downloadName);
}
</script>

<template>
  <div class="qr-panel">
    <div class="qr-panel-header">
      <a-tag color="green" class="qr-remark">{{ remark }}</a-tag>
      <a-tooltip :title="t('copy')">
        <a-button size="small" @click="copy">
          <template #icon>
            <CopyOutlined />
          </template>
        </a-button>
      </a-tooltip>
      <a-tooltip v-if="downloadName" :title="t('download')">
        <a-button size="small" @click="download">
          <template #icon>
            <DownloadOutlined />
          </template>
        </a-button>
      </a-tooltip>
    </div>
    <div v-if="showQr" class="qr-panel-canvas">
      <canvas ref="canvas" :style="{ width: `${size}px`, height: `${size}px` }" @click="copy" />
    </div>
  </div>
</template>

<style scoped>
.qr-panel {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 8px;
  padding: 10px;
  margin-bottom: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.qr-panel-header {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.qr-remark {
  margin: 0;
}

.qr-panel-canvas {
  display: flex;
  justify-content: center;
  padding: 6px 0;
}

.qr-panel-canvas canvas {
  cursor: pointer;
  display: block;
  border-radius: 4px;
  /* Drawing buffer is matrix-snapped (smaller than display size for
   * dense QRs); scale up crisply so dense and sparse QRs share the
   * same on-screen footprint without blurring. */
  image-rendering: pixelated;
  image-rendering: crisp-edges;
}
</style>
