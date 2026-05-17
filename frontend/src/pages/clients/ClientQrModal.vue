<script setup>
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { HttpUtil } from '@/utils';
import QrPanel from '@/pages/inbounds/QrPanel.vue';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  client: { type: Object, default: null },
});

const emit = defineEmits(['update:open']);

const links = ref([]);
const loading = ref(false);

watch(() => props.open, async (next) => {
  if (!next || !props.client?.subId) {
    links.value = [];
    return;
  }
  loading.value = true;
  try {
    const msg = await HttpUtil.get(`/panel/api/inbounds/getSubLinks/${encodeURIComponent(props.client.subId)}`);
    links.value = msg?.success && Array.isArray(msg.obj) ? msg.obj : [];
  } finally {
    loading.value = false;
  }
});

function close() {
  emit('update:open', false);
}
</script>

<template>
  <a-modal :open="open" :title="client ? client.email : t('qrCode')" :footer="null" :width="520" centered
    @cancel="close">
    <a-spin :spinning="loading">
      <div v-if="!client?.subId && !loading" class="empty">
        {{ t('pages.clients.noSubId') || 'This client has no subId, no shareable link.' }}
      </div>
      <div v-else-if="links.length === 0 && !loading" class="empty">
        {{ t('pages.clients.noLinks') || 'No shareable links — attach this client to a protocol-capable inbound first.' }}
      </div>
      <a-collapse v-else :default-active-key="['l0']" accordion>
        <a-collapse-panel v-for="(link, idx) in links" :key="`l${idx}`"
          :header="`${t('pages.clients.link') || 'Link'} ${idx + 1}`">
          <QrPanel :value="link" :remark="`${client?.email || ''} #${idx + 1}`" />
        </a-collapse-panel>
      </a-collapse>
    </a-spin>
  </a-modal>
</template>

<style scoped>
.empty {
  padding: 24px;
  text-align: center;
  opacity: 0.6;
}
</style>
