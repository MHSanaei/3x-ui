<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { CopyOutlined, DownloadOutlined, PictureOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { ClipboardManager, FileManager } from '@/utils';

const { t } = useI18n();

const props = defineProps({
  value: { type: String, required: true },
  remark: { type: String, default: '' },
  downloadName: { type: String, default: '' },
  size: { type: Number, default: 360 },
  showQr: { type: Boolean, default: true },
});

const qrRef = ref(null);

async function copy() {
  const ok = await ClipboardManager.copyText(props.value);
  if (ok) message.success(t('copied'));
}

function download() {
  if (!props.downloadName) return;
  FileManager.downloadTextFile(props.value, props.downloadName);
}

function svgToPngBlob(size = 360) {
  const svgEl = qrRef.value?.querySelector('svg');
  if (!svgEl) return Promise.resolve(null);
  const svgData = new XMLSerializer().serializeToString(svgEl);
  const svgBlob = new Blob([svgData], { type: 'image/svg+xml;charset=utf-8' });
  const url = URL.createObjectURL(svgBlob);
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext('2d');
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, size, size);
      ctx.drawImage(img, 0, 0, size, size);
      URL.revokeObjectURL(url);
      canvas.toBlob(resolve, 'image/png');
    };
    img.onerror = () => { URL.revokeObjectURL(url); resolve(null); };
    img.src = url;
  });
}

async function copyImage() {
  const blob = await svgToPngBlob(props.size);
  if (!blob) return;
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
    message.success(t('copied'));
  } catch {
    downloadImageBlob(blob);
  }
}

function downloadImageBlob(blob) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${props.remark || 'qrcode'}.png`;
  link.click();
  URL.revokeObjectURL(url);
}

async function downloadImage() {
  const blob = await svgToPngBlob(props.size);
  if (blob) downloadImageBlob(blob);
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
      <a-tooltip v-if="showQr" :title="t('downloadImage', 'Download Image')">
        <a-button size="small" @click="downloadImage">
          <template #icon>
            <PictureOutlined />
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
    <div v-if="showQr" ref="qrRef" class="qr-panel-canvas">
      <a-tooltip :title="t('copy')">
        <a-qrcode class="qr-code" :value="value" :size="size" type="svg" :bordered="false" color="#000000"
          bg-color="#ffffff" @click="copyImage" />
      </a-tooltip>
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

.qr-panel-canvas .qr-code {
  cursor: pointer;
  background: #fff;
  border-radius: 4px;
  line-height: 0;
}

.qr-panel-canvas .qr-code :deep(svg) {
  display: block;
  width: 100%;
  height: auto;
  max-width: 360px;
}
</style>
