<script setup>
import { useI18n } from 'vue-i18n';
import { Modal, message } from 'ant-design-vue';
import { CloudDownloadOutlined } from '@ant-design/icons-vue';
import { HttpUtil, PromiseUtil } from '@/utils';
import axios from 'axios';

const { t } = useI18n();

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
    title: t('pages.index.panelUpdateDialog'),
    content: t('pages.index.panelUpdateDialogDesc').replace('#version#', props.info.latestVersion || ''),
    okText: t('confirm'),
    cancelText: t('cancel'),
    onOk: async () => {
      const baseTip = t('pages.index.dontRefresh');
      const tip = props.info.latestVersion ? `${baseTip} (${props.info.latestVersion})` : baseTip;
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
        message.success(t('pages.index.panelUpdateStartedPopover'));
        await PromiseUtil.sleep(800);
      }
      window.location.reload();
    },
  });
}
</script>

<template>
  <a-modal :open="open" :title="t('pages.index.updatePanel')" :closable="true" :footer="null" @cancel="close">
    <a-alert v-if="info.updateAvailable" type="warning" class="mb-12" :message="t('pages.index.panelUpdateDesc')"
      show-icon />

    <a-list bordered class="version-list">
      <a-list-item class="version-list-item">
        <span>{{ t('pages.index.currentPanelVersion') }}</span>
        <a-tag color="green">v{{ info.currentVersion || '?' }}</a-tag>
      </a-list-item>
      <a-list-item v-if="info.updateAvailable" class="version-list-item">
        <span>{{ t('pages.index.latestPanelVersion') }}</span>
        <a-tag color="purple">{{ info.latestVersion || '-' }}</a-tag>
      </a-list-item>
      <a-list-item v-else class="version-list-item">
        <span>{{ t('pages.index.panelUpToDate') }}</span>
        <a-tag color="green">{{ t('pages.index.panelUpToDate') }}</a-tag>
      </a-list-item>
    </a-list>

    <div class="actions-row">
      <a-button type="primary" :disabled="!info.updateAvailable" @click="updatePanel">
        <template #icon>
          <CloudDownloadOutlined />
        </template>
        {{ t('pages.index.updatePanel') }}
      </a-button>
    </div>
  </a-modal>
</template>

<style scoped>
.mb-12 {
  margin-bottom: 12px;
}

.version-list {
  width: 100%;
}

.version-list-item {
  display: flex;
  justify-content: space-between;
}

.actions-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
