<script setup>
import { useI18n } from 'vue-i18n';
import { DownloadOutlined, UploadOutlined } from '@ant-design/icons-vue';
import { HttpUtil, PromiseUtil } from '@/utils';

const { t } = useI18n();

defineProps({
  open: { type: Boolean, default: false },
  basePath: { type: String, default: '' },
});

const emit = defineEmits(['update:open', 'busy']);

function close() {
  emit('update:open', false);
}

function exportDb() {
  // The Go endpoint streams x-ui.db as a download. Setting
  // window.location triggers a browser download without leaving
  // the page (the Go side responds with Content-Disposition: attachment).
  window.location = window.X_UI_BASE_PATH+'panel/api/server/getDb';
}

function importDb() {
  const fileInput = document.createElement('input');
  fileInput.type = 'file';
  fileInput.accept = '.db';
  fileInput.addEventListener('change', async (e) => {
    const dbFile = e.target.files?.[0];
    if (!dbFile) return;

    const formData = new FormData();
    formData.append('db', dbFile);

    close();
    emit('busy', { busy: true, tip: t('pages.index.importDatabase') + '…' });

    const upload = await HttpUtil.post('/panel/api/server/importDB', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    if (!upload?.success) {
      emit('busy', { busy: false });
      return;
    }

    emit('busy', { busy: true, tip: t('pages.settings.restartPanel') + '…' });
    const restart = await HttpUtil.post('/panel/setting/restartPanel');
    if (restart?.success) {
      await PromiseUtil.sleep(5000);
      window.location.reload();
    } else {
      emit('busy', { busy: false });
    }
  });
  fileInput.click();
}
</script>

<template>
  <a-modal :open="open" :title="t('pages.index.backupTitle')" :closable="true" :footer="null" @cancel="close">
    <a-list bordered class="backup-list">
      <a-list-item class="backup-item">
        <a-list-item-meta>
          <template #title>{{ t('pages.index.exportDatabase') }}</template>
          <template #description>{{ t('pages.index.exportDatabaseDesc') }}</template>
        </a-list-item-meta>
        <a-button type="primary" @click="exportDb">
          <template #icon>
            <DownloadOutlined />
          </template>
        </a-button>
      </a-list-item>

      <a-list-item class="backup-item">
        <a-list-item-meta>
          <template #title>{{ t('pages.index.importDatabase') }}</template>
          <template #description>{{ t('pages.index.importDatabaseDesc') }}</template>
        </a-list-item-meta>
        <a-button type="primary" @click="importDb">
          <template #icon>
            <UploadOutlined />
          </template>
        </a-button>
      </a-list-item>
    </a-list>
  </a-modal>
</template>

<style scoped>
.backup-list {
  width: 100%;
}

.backup-item {
  display: flex;
  align-items: center;
  gap: 16px;
}
</style>
