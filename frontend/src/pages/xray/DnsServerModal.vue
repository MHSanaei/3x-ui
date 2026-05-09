<script setup>
import { computed, reactive, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { PlusOutlined, MinusOutlined } from '@ant-design/icons-vue';

const { t } = useI18n();

// DNS server add/edit modal — mirrors web/html/modals/xray_dns_modal.html.
// The legacy panel allowed both string-form ("8.8.8.8") and object-form
// servers; we always edit as an object and the parent can decide
// whether to collapse to a string when nothing besides address is set.

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
  expectIPs: [],
  unexpectedIPs: [],
  queryStrategy: 'UseIP',
  skipFallback: true,
  disableCache: false,
  finalQuery: false,
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
  // Object — copy fields, defaulting missing arrays to empty.
  Object.assign(form, {
    ...DEFAULT_SERVER(),
    ...props.server,
    domains: [...(props.server.domains || [])],
    expectIPs: [...(props.server.expectIPs || [])],
    unexpectedIPs: [...(props.server.unexpectedIPs || [])],
  });
});

function close() { emit('update:open', false); }

function onOk() {
  // If the user only set an address (everything else default), emit a
  // bare string — that's the wire shape the legacy panel uses for
  // servers like "8.8.8.8" and keeps the JSON tidy.
  const isPlain = form.domains.length === 0
    && form.expectIPs.length === 0
    && form.unexpectedIPs.length === 0
    && form.port === 53
    && form.queryStrategy === 'UseIP'
    && form.skipFallback === true
    && form.disableCache === false
    && form.finalQuery === false;
  if (isPlain) {
    emit('confirm', form.address);
  } else {
    emit('confirm', {
      address: form.address,
      port: form.port,
      domains: [...form.domains].filter(Boolean),
      expectIPs: [...form.expectIPs].filter(Boolean),
      unexpectedIPs: [...form.unexpectedIPs].filter(Boolean),
      queryStrategy: form.queryStrategy,
      skipFallback: form.skipFallback,
      disableCache: form.disableCache,
      finalQuery: form.finalQuery,
    });
  }
}

const title = computed(() =>
  props.isEdit ? t('pages.xray.dns.edit') : t('pages.xray.dns.add'),
);
</script>

<template>
  <a-modal
    :open="open"
    :title="title"
    :ok-text="t('confirm')"
    :cancel-text="t('close')"
    :mask-closable="false"
    @ok="onOk"
    @cancel="close"
  >
    <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
      <a-form-item :label="t('pages.inbounds.address')">
        <a-input v-model:value="form.address" />
      </a-form-item>
      <a-form-item :label="t('pages.inbounds.port')">
        <a-input-number v-model:value="form.port" :min="1" :max="65535" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.strategy')">
        <a-select v-model:value="form.queryStrategy" :style="{ width: '100%' }">
          <a-select-option v-for="s in STRATEGIES" :key="s" :value="s">{{ s }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-divider :style="{ margin: '5px 0' }" />

      <a-form-item :label="t('pages.xray.dns.domains')">
        <a-button size="small" type="primary" @click="form.domains.push('')">
          <template #icon><PlusOutlined /></template>
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
        <a-button size="small" type="primary" @click="form.expectIPs.push('')">
          <template #icon><PlusOutlined /></template>
        </a-button>
        <template v-for="(_, idx) in form.expectIPs" :key="`e${idx}`">
          <a-input v-model:value="form.expectIPs[idx]" :style="{ marginTop: '4px' }">
            <template #addonAfter>
              <MinusOutlined @click="form.expectIPs.splice(idx, 1)" />
            </template>
          </a-input>
        </template>
      </a-form-item>

      <a-form-item :label="t('pages.xray.dns.unexpectIPs')">
        <a-button size="small" type="primary" @click="form.unexpectedIPs.push('')">
          <template #icon><PlusOutlined /></template>
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

      <a-form-item label="Skip fallback">
        <a-switch v-model:checked="form.skipFallback" />
      </a-form-item>
      <a-form-item :label="t('pages.xray.dns.disableCache')">
        <a-switch v-model:checked="form.disableCache" />
      </a-form-item>
      <a-form-item label="Final query">
        <a-switch v-model:checked="form.finalQuery" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>
