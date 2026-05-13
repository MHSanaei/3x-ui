<script setup>
import { useI18n } from 'vue-i18n';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { ClipboardManager, FileManager } from '@/utils';

const { t } = useI18n();

const props = defineProps({
  value: { type: String, required: true },
  remark: { type: String, default: '' },
  downloadName: { type: String, default: '' },
  size: { type: Number, default: 240 },
  showQr: { type: Boolean, default: true },
});

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
      <a-qrcode class="qr-code" :value="value" :size="size" type="svg" :bordered="false"
        color="#000000" bg-color="#ffffff" :title="t('copy')" @click="copy" />
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
  padding: 0 !important;
  background: #fff;
  border-radius: 4px;
}
</style>
