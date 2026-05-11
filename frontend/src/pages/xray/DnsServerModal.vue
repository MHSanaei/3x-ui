<script setup>
import { computed, reactive, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { PlusOutlined, MinusOutlined } from '@ant-design/icons-vue';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  server: { type: [Object, String, null], default: null },
  isEdit: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open', 'confirm']);

const DEFAULT_SERVER = () => ({
  address: 'localhost',
  port: 53,
  domains: [],
  expectedIPs: [],
  unexpectedIPs: [],
  queryStrategy: 'UseIP',
  skipFallback: false,
  disableCache: false,
  finalQuery: false,
  tag: '',
  clientIP: '',
  serveStale: false,
  serveExpiredTTL: 0,
  timeoutMs: 4000,
});

const STRATEGIES = ['UseSystem', 'UseIP', 'UseIPv4', 'UseIPv6'];

const form = reactive(DEFAULT_SERVER());

watch(() => props.open, (next) => {
  if (!next) return;
  Object.assign(form, DEFAULT_SERVER());
  if (props.server == null) return;
  if (typeof props.server === 'string') {
    form.address = props.server;
    return;
  }
  const incoming = props.server;
  Object.assign(form, {
    ...DEFAULT_SERVER(),
    ...incoming,
    domains: [...(incoming.domains || [])],
    expectedIPs: [...(incoming.expectedIPs || incoming.expectIPs || [])],
    unexpectedIPs: [...(incoming.unexpectedIPs || [])],
  });
});

function close() { emit('update:open', false); }

function onOk() {
  const isPlain = form.domains.length === 0
    && form.expectedIPs.length === 0
    && form.unexpectedIPs.length === 0
    && form.port === 53
    && form.queryStrategy === 'UseIP'
    && form.skipFallback === false
    && form.disableCache === false
    && form.finalQuery === false
    && !form.tag
    && !form.clientIP
    && form.serveStale === false
    && form.serveExpiredTTL === 0
    && form.timeoutMs === 4000;
  if (isPlain) {
    emit('confirm', form.address);
    return;
  }
  const out = {
    address: form.address,
    port: form.port,
    domains: [...form.domains].filter(Boolean),
    expectedIPs: [...form.expectedIPs].filter(Boolean),
    unexpectedIPs: [...form.unexpectedIPs].filter(Boolean),
    queryStrategy: form.queryStrategy,
    skipFallback: form.skipFallback,
    disableCache: form.disableCache,
    finalQuery: form.finalQuery,
    serveStale: form.serveStale,
    serveExpiredTTL: form.serveExpiredTTL,
    timeoutMs: form.timeoutMs,
  };
  if (form.tag) out.tag = form.tag;
  if (form.clientIP) out.clientIP = form.clientIP;
  emit('confirm', out);
}

const title = computed(() =>
  props.isEdit ? t('pages.xray.dns.edit') : t('pages.xray.dns.add'),
);
</script>

<template>
  <a-modal :open="open" :title="title" :ok-text="t('confirm')" :cancel-text="t('close')" :mask-closable="false"
    @ok="onOk" @cancel="close">
    <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
      <a-form-item :label="t('pages.inbounds.address')">
        <a-input v-model:value="form.address" />
      </a-form-item>
      <a-form-item :label="t('pages.inbounds.port')">
        <a-input-number v-model:value="form.port" :min="1" :max="65535" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.tag')">
        <a-input v-model:value="form.tag" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.clientIp')">
        <a-input v-model:value="form.clientIP" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.strategy')">
        <a-select v-model:value="form.queryStrategy" :style="{ width: '100%' }">
          <a-select-option v-for="s in STRATEGIES" :key="s" :value="s">{{ s }}</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.timeoutMs')">
        <a-input-number v-model:value="form.timeoutMs" :min="0" :step="500" />
      </a-form-item>

      <a-divider :style="{ margin: '5px 0' }" />

      <a-form-item :label="t('pages.xray.dns.domains')">
        <a-button size="small" type="primary" @click="form.domains.push('')">
          <template #icon>
            <PlusOutlined />
          </template>
        </a-button>
        <template v-for="(_, idx) in form.domains" :key="`d${idx}`">
          <a-input v-model:value="form.domains[idx]" :style="{ marginTop: '4px' }">
            <template #addonAfter>
              <MinusOutlined @click="form.domains.splice(idx, 1)" />
            </template>
          </a-input>
        </template>
      </a-form-item>

      <a-form-item :label="t('pages.xray.dns.expectIPs')">
        <a-button size="small" type="primary" @click="form.expectedIPs.push('')">
          <template #icon>
            <PlusOutlined />
          </template>
        </a-button>
        <template v-for="(_, idx) in form.expectedIPs" :key="`e${idx}`">
          <a-input v-model:value="form.expectedIPs[idx]" :style="{ marginTop: '4px' }">
            <template #addonAfter>
              <MinusOutlined @click="form.expectedIPs.splice(idx, 1)" />
            </template>
          </a-input>
        </template>
      </a-form-item>

      <a-form-item :label="t('pages.xray.dns.unexpectIPs')">
        <a-button size="small" type="primary" @click="form.unexpectedIPs.push('')">
          <template #icon>
            <PlusOutlined />
          </template>
        </a-button>
        <template v-for="(_, idx) in form.unexpectedIPs" :key="`u${idx}`">
          <a-input v-model:value="form.unexpectedIPs[idx]" :style="{ marginTop: '4px' }">
            <template #addonAfter>
              <MinusOutlined @click="form.unexpectedIPs.splice(idx, 1)" />
            </template>
          </a-input>
        </template>
      </a-form-item>

      <a-divider :style="{ margin: '5px 0' }" />

      <a-form-item :label="t('pages.xray.dns.skipFallback')">
        <a-switch v-model:checked="form.skipFallback" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.finalQuery')">
        <a-switch v-model:checked="form.finalQuery" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.disableCache')">
        <a-switch v-model:checked="form.disableCache" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.serveStale')">
        <a-switch v-model:checked="form.serveStale" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.serveExpiredTTL')">
        <a-input-number v-model:value="form.serveExpiredTTL" :min="0" :step="60" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>
