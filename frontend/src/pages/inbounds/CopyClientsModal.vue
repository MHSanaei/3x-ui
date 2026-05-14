<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';

import { HttpUtil, SizeFormatter, IntlUtil } from '@/utils';
import { TLS_FLOW_CONTROL } from '@/models/inbound.js';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  dbInbound: { type: Object, default: null },
  dbInbounds: { type: Array, default: () => [] },
});

const emit = defineEmits(['update:open', 'saved']);

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const sourceInboundId = ref(null);
const selectedEmails = ref([]);
const flow = ref('');
const saving = ref(false);

const sources = computed(() => {
  if (!props.dbInbound) return [];
  return props.dbInbounds
    .filter(
      (row) =>
        row.id !== props.dbInbound.id &&
        typeof row.isMultiUser === 'function' &&
        row.isMultiUser(),
    )
    .map((row) => {
      let count = 0;
      try { count = (row.toInbound().clients || []).length; } catch (_e) { /* ignore */ }
      return { id: row.id, label: `${row.remark || `#${row.id}`} (${row.protocol}, ${count})` };
    });
});

const sourceInbound = computed(() => {
  if (!sourceInboundId.value) return null;
  return props.dbInbounds.find((r) => r.id === sourceInboundId.value) || null;
});

const sourceClients = computed(() => {
  const sb = sourceInbound.value;
  if (!sb) return [];
  let list = [];
  try { list = sb.toInbound().clients || []; } catch (_e) { /* ignore */ }
  const stats = new Map((sb.clientStats || []).map((s) => [s.email, s]));
  return list
    .filter((c) => c.email)
    .map((c) => {
      const s = stats.get(c.email);
      const used = s ? (s.up || 0) + (s.down || 0) : 0;
      let expiryLabel = t('unlimited');
      if (c.expiryTime > 0) expiryLabel = IntlUtil.formatDate(c.expiryTime);
      else if (c.expiryTime < 0) expiryLabel = `${-c.expiryTime / 86400000}d`;
      return { email: c.email, trafficLabel: SizeFormatter.sizeFormat(used), expiryLabel };
    });
});

const showFlow = computed(() => {
  if (!props.dbInbound) return false;
  try {
    const inb = props.dbInbound.toInbound();
    return !!(inb && typeof inb.canEnableTlsFlow === 'function' && inb.canEnableTlsFlow());
  } catch (_e) { return false; }
});

const columns = computed(() => [
  { title: t('pages.inbounds.email'), dataIndex: 'email', width: 280 },
  { title: t('pages.inbounds.traffic'), dataIndex: 'trafficLabel', width: 140 },
  { title: t('pages.inbounds.expireDate'), dataIndex: 'expiryLabel', width: 160 },
]);

const rowSelection = computed(() => ({
  selectedRowKeys: selectedEmails.value,
  onChange: (keys) => { selectedEmails.value = keys; },
}));

const title = computed(() => {
  if (!props.dbInbound) return t('pages.client.copyFromInbound');
  const target = props.dbInbound.remark || `#${props.dbInbound.id}`;
  return `${t('pages.client.copyToInbound')} ${target}`;
});

watch(() => props.open, (next) => {
  if (!next) return;
  sourceInboundId.value = null;
  selectedEmails.value = [];
  flow.value = '';
  saving.value = false;
});

watch(sourceInboundId, () => {
  selectedEmails.value = [];
});

function selectAll() {
  selectedEmails.value = sourceClients.value.map((c) => c.email);
}
function clearAll() {
  selectedEmails.value = [];
}

async function ok() {
  if (!sourceInboundId.value) {
    message.error(t('pages.client.copySelectSourceFirst'));
    return;
  }
  if (!props.dbInbound) return;
  saving.value = true;
  try {
    const payload = {
      sourceInboundId: sourceInboundId.value,
      clientEmails: selectedEmails.value,
    };
    if (showFlow.value && flow.value) payload.flow = flow.value;
    const msg = await HttpUtil.post(
      `/panel/api/inbounds/${props.dbInbound.id}/copyClients`,
      payload,
    );
    if (!msg?.success) return;
    const obj = msg.obj || {};
    const addedCount = (obj.added || []).length;
    const errorList = obj.errors || [];
    if (addedCount > 0) {
      message.success(`${t('pages.client.copyResultSuccess')}: ${addedCount}`);
    } else {
      message.warning(t('pages.client.copyResultNone'));
    }
    if (errorList.length > 0) {
      message.error(`${t('pages.client.copyResultErrors')}: ${errorList.join('; ')}`);
    }
    emit('saved');
    emit('update:open', false);
  } finally {
    saving.value = false;
  }
}

function close() {
  if (saving.value) return;
  emit('update:open', false);
}
</script>

<template>
  <a-modal :open="open" :title="title" :ok-text="t('pages.client.copySelected')" :cancel-text="t('close')"
    :confirm-loading="saving" :mask-closable="false" width="720px" @ok="ok" @cancel="close">
    <a-space direction="vertical" :style="{ width: '100%' }">
      <div>
        <div :style="{ marginBottom: '6px' }">{{ t('pages.client.copySource') }}</div>
        <a-select v-model:value="sourceInboundId" :style="{ width: '100%' }" allow-clear>
          <a-select-option v-for="item in sources" :key="item.id" :value="item.id">
            {{ item.label }}
          </a-select-option>
        </a-select>
      </div>

      <div v-if="sourceInboundId">
        <a-space :style="{ marginBottom: '8px' }">
          <a-button size="small" @click="selectAll">{{ t('pages.client.selectAll') }}</a-button>
          <a-button size="small" @click="clearAll">{{ t('pages.client.clearAll') }}</a-button>
        </a-space>
        <a-table :columns="columns" :data-source="sourceClients" :pagination="false" size="small"
          :row-key="(r) => r.email" :row-selection="rowSelection" :scroll="{ y: 280 }" />
      </div>

      <div v-if="showFlow">
        <div :style="{ marginBottom: '6px' }">{{ t('pages.client.copyFlowLabel') }}</div>
        <a-select v-model:value="flow" :style="{ width: '100%' }" allow-clear>
          <a-select-option value="">{{ t('none') }}</a-select-option>
          <a-select-option v-for="key in FLOW_OPTIONS" :key="key" :value="key">{{ key }}</a-select-option>
        </a-select>
        <div :style="{ marginTop: '4px', fontSize: '12px', opacity: 0.7 }">
          {{ t('pages.client.copyFlowHint') }}
        </div>
      </div>
    </a-space>
  </a-modal>
</template>
