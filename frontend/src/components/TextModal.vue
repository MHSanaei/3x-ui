<script setup>
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import { ClipboardManager, FileManager } from '@/utils';

// Read-only text modal — used to surface multi-line export blobs
// (subscription URLs, raw inbound JSON, generated share links) the
// way the legacy txtModal did.

defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  content: { type: String, default: '' },
  // When set, surfaces a download button that writes `content` to a
  // text file with this name.
  fileName: { type: String, default: '' },
});

const emit = defineEmits(['update:open']);

function close() {
  emit('update:open', false);
}

async function copy(value) {
  const ok = await ClipboardManager.copyText(value || '');
  if (ok) {
    message.success('Copied');
    close();
  }
}

function download(content, name) {
  if (!name) return;
  FileManager.downloadTextFile(content, name);
}
</script>

<template>
  <a-modal :open="open" :title="title" :closable="true" @cancel="close">
    <a-textarea :value="content" readonly :auto-size="{ minRows: 10, maxRows: 20 }" class="text-modal-content" />
    <template #footer>
      <a-button v-if="fileName" @click="download(content, fileName)">
        <template #icon>
          <DownloadOutlined />
        </template>
        {{ fileName }}
      </a-button>
      <a-button type="primary" @click="copy(content)">
        <template #icon>
          <CopyOutlined />
        </template>
        Copy
      </a-button>
    </template>
  </a-modal>
</template>

<style scoped>
.text-modal-content {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  overflow-y: auto;
}
</style>
