<script setup>
import { Modal, message } from 'ant-design-vue';
import { CloudDownloadOutlined } from '@ant-design/icons-vue';
import { HttpUtil, PromiseUtil } from '@/utils';
import axios from 'axios';

const props = defineProps({
  open: { type: Boolean, default: false },
  info: {
    type: Object,
    default: () => ({ currentVersion: '', latestVersion: '', updateAvailable: false }),
  },
});

const emit = defineEmits(['update:open', 'busy']);

function close() {
  emit('update:open', false);
}

function updatePanel() {
  Modal.confirm({
    title: 'Update panel',
    content: `The panel will be updated to ${props.info.latestVersion || ''} and restarted. Continue?`,
    okText: 'Confirm',
    cancelText: 'Cancel',
    onOk: async () => {
      const tip = props.info.latestVersion
        ? `Installation in progress, please do not refresh (${props.info.latestVersion})`
        : 'Installation in progress, please do not refresh';
      close();
      emit('busy', { busy: true, tip });
      const msg = await HttpUtil.post('/panel/api/server/updatePanel');
      if (!msg?.success) {
        emit('busy', { busy: false });
        return;
      }
      // Wait for the running process to exit, then poll the new panel
      // until it answers (up to ~90s). Reload as soon as it's back.
      await PromiseUtil.sleep(5000);
      const deadline = Date.now() + 90_000;
      let back = false;
      while (Date.now() < deadline) {
        try {
          const r = await axios.get('/panel/api/server/status', { timeout: 2000 });
          if (r?.data?.success) { back = true; break; }
        } catch (_) { /* still restarting */ }
        await PromiseUtil.sleep(2000);
      }
      if (back) {
        message.success('Panel update started');
        await PromiseUtil.sleep(800);
      }
      window.location.reload();
    },
  });
}
</script>

<template>
  <a-modal :open="open" title="Update panel" :closable="true" :footer="null" @cancel="close">
    <a-alert
      v-if="info.updateAvailable"
      type="warning"
      class="mb-12"
      message="A new panel version is available. Update will restart the service."
      show-icon
    />

    <a-list bordered class="version-list">
      <a-list-item class="version-list-item">
        <span>Current version</span>
        <a-tag color="green">v{{ info.currentVersion || 'unknown' }}</a-tag>
      </a-list-item>
      <a-list-item v-if="info.updateAvailable" class="version-list-item">
        <span>Latest version</span>
        <a-tag color="purple">{{ info.latestVersion || '-' }}</a-tag>
      </a-list-item>
      <a-list-item v-else class="version-list-item">
        <span>Panel is up to date</span>
        <a-tag color="green">Up to date</a-tag>
      </a-list-item>
    </a-list>

    <div class="actions-row">
      <a-button type="primary" :disabled="!info.updateAvailable" @click="updatePanel">
        <template #icon><CloudDownloadOutlined /></template>
        Update panel
      </a-button>
    </div>
  </a-modal>
</template>

<style scoped>
.mb-12 { margin-bottom: 12px; }
.version-list { width: 100%; }
.version-list-item { display: flex; justify-content: space-between; }
.actions-row { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>
