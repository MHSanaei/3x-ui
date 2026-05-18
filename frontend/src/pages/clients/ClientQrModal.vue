<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { HttpUtil } from '@/utils';
import QrPanel from '@/pages/inbounds/QrPanel.vue';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  client: { type: Object, default: null },
  subSettings: {
    type: Object,
    default: () => ({ enable: false, subURI: '', subJsonURI: '', subJsonEnable: false }),
  },
});

const emit = defineEmits(['update:open']);

const links = ref([]);
const loading = ref(false);

const subLink = computed(() => {
  if (!props.client?.subId || !props.subSettings?.enable || !props.subSettings?.subURI) return '';
  return props.subSettings.subURI + props.client.subId;
});

const subJsonLink = computed(() => {
  if (!props.client?.subId || !props.subSettings?.enable) return '';
  if (!props.subSettings?.subJsonEnable || !props.subSettings?.subJsonURI) return '';
  return props.subSettings.subJsonURI + props.client.subId;
});

const activeKeys = computed(() => {
  const keys = [];
  if (subLink.value) keys.push('sub');
  if (subJsonLink.value) keys.push('subJson');
  if (links.value.length > 0) keys.push('l0');
  return keys;
});

const hasAnything = computed(
  () => !!subLink.value || !!subJsonLink.value || links.value.length > 0,
);

watch(() => props.open, async (next) => {
  if (!next || !props.client?.subId) {
    links.value = [];
    return;
  }
  loading.value = true;
  try {
    const msg = await HttpUtil.get(`/panel/api/clients/subLinks/${encodeURIComponent(props.client.subId)}`);
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
        {{ t('pages.clients.noSubId') }}
      </div>
      <div v-else-if="!hasAnything && !loading" class="empty">
        {{ t('pages.clients.noLinks') }}
      </div>
      <a-collapse v-else :active-key="activeKeys" accordion>
        <a-collapse-panel v-if="subLink" key="sub" :header="t('subscription.title')">
          <QrPanel :value="subLink" :remark="`${client?.email || ''} — ${t('subscription.title')}`" />
        </a-collapse-panel>
        <a-collapse-panel v-if="subJsonLink" key="subJson" :header="`${t('subscription.title')} (JSON)`">
          <QrPanel :value="subJsonLink" :remark="`${client?.email || ''} — JSON`" />
        </a-collapse-panel>
        <a-collapse-panel v-for="(link, idx) in links" :key="`l${idx}`"
          :header="`${t('pages.clients.link')} ${idx + 1}`">
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
