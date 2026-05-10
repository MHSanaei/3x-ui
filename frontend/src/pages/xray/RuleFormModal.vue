<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { PlusOutlined, MinusOutlined, QuestionCircleOutlined } from '@ant-design/icons-vue';

// Routing-rule editor — mirrors xray_rule_modal.html. We keep the
// CSV-style fields (domain / ip / sourceIP / user / port / sourcePort /
// vlessRoute) as plain strings while the modal is open and split them
// back to arrays on submit, just like the legacy ruleModal.getResult.

const props = defineProps({
  open: { type: Boolean, default: false },
  // null when adding, the rule object when editing.
  rule: { type: Object, default: null },
  // Tag pools sourced from templateSettings.{inbounds,outbounds,routing.balancers}
  // and the parent's inboundTags / clientReverseTags / dnsTag.
  inboundTags: { type: Array, default: () => [] },
  outboundTags: { type: Array, default: () => [] },
  balancerTags: { type: Array, default: () => [''] },
});

const emit = defineEmits(['update:open', 'confirm']);

const form = reactive({
  domain: '',
  ip: '',
  port: '',
  sourcePort: '',
  vlessRoute: '',
  network: '',
  sourceIP: '',
  user: '',
  inboundTag: [],
  protocol: [],
  attrs: [], // [[key, value], ...]
  outboundTag: '',
  balancerTag: '',
});

const isEdit = ref(false);

function reset() {
  form.domain = '';
  form.ip = '';
  form.port = '';
  form.sourcePort = '';
  form.vlessRoute = '';
  form.network = '';
  form.sourceIP = '';
  form.user = '';
  form.inboundTag = [];
  form.protocol = [];
  form.attrs = [];
  form.outboundTag = '';
  form.balancerTag = '';
}

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.rule) {
    isEdit.value = true;
    const r = props.rule;
    form.domain = Array.isArray(r.domain) ? r.domain.join(',') : (r.domain || '');
    form.ip = Array.isArray(r.ip) ? r.ip.join(',') : (r.ip || '');
    form.port = r.port || '';
    form.sourcePort = r.sourcePort || '';
    form.vlessRoute = r.vlessRoute || '';
    form.network = r.network || '';
    form.sourceIP = Array.isArray(r.sourceIP) ? r.sourceIP.join(',') : (r.sourceIP || '');
    form.user = Array.isArray(r.user) ? r.user.join(',') : (r.user || '');
    form.inboundTag = r.inboundTag || [];
    form.protocol = r.protocol || [];
    // Attrs in the wire shape are an object — flatten to [[k,v]] pairs.
    form.attrs = r.attrs ? Object.entries(r.attrs) : [];
    form.outboundTag = r.outboundTag || '';
    form.balancerTag = r.balancerTag || '';
  } else {
    isEdit.value = false;
    reset();
  }
});

function close() { emit('update:open', false); }

function csv(value) {
  if (!value) return [];
  return String(value).split(',').map((s) => s.trim()).filter(Boolean);
}

function buildResult() {
  const rule = {
    type: 'field',
    domain: csv(form.domain),
    ip: csv(form.ip),
    port: form.port,
    sourcePort: form.sourcePort,
    vlessRoute: form.vlessRoute,
    network: form.network,
    sourceIP: csv(form.sourceIP),
    user: csv(form.user),
    inboundTag: form.inboundTag,
    protocol: form.protocol,
    attrs: Object.fromEntries(form.attrs.filter(([k]) => k)),
    outboundTag: form.outboundTag === '' ? undefined : form.outboundTag,
    balancerTag: form.balancerTag === '' ? undefined : form.balancerTag,
  };
  // Strip empty arrays / objects / strings so the final wire JSON
  // matches what the legacy `getResult` produces.
  const out = {};
  for (const [k, v] of Object.entries(rule)) {
    if (v == null) continue;
    if (Array.isArray(v) && v.length === 0) continue;
    if (typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length === 0) continue;
    if (v === '') continue;
    out[k] = v;
  }
  return out;
}

function onOk() {
  emit('confirm', buildResult());
}

import { useI18n } from 'vue-i18n';
const { t } = useI18n();

const title = computed(() =>
  isEdit.value
    ? `${t('edit')} ${t('pages.xray.Routings')}`
    : `+ ${t('pages.xray.Routings')}`,
);
const okText = computed(() =>
  isEdit.value ? t('pages.client.submitEdit') : t('create'),
);

const NETWORKS = ['', 'TCP', 'UDP', 'TCP,UDP'];
const PROTOCOLS = ['http', 'tls', 'bittorrent', 'quic'];
</script>

<template>
  <a-modal :open="open" :title="title" :ok-text="okText" :cancel-text="t('close')" :mask-closable="false" width="640px"
    @ok="onOk" @cancel="close">
    <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">
            Source IPs
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.sourceIP" placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">
            Source port
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.sourcePort" placeholder="53,443,1000-2000" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">
            VLESS route
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.vlessRoute" placeholder="53,443,1000-2000" />
      </a-form-item>

      <a-form-item label="Network">
        <a-select v-model:value="form.network">
          <a-select-option v-for="n in NETWORKS" :key="n" :value="n">{{ n || '(any)' }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="Protocol">
        <a-select v-model:value="form.protocol" mode="multiple">
          <a-select-option v-for="p in PROTOCOLS" :key="p" :value="p">{{ p }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="Attributes">
        <a-button size="small" @click="form.attrs.push(['', ''])">
          <template #icon>
            <PlusOutlined />
          </template>
        </a-button>
      </a-form-item>
      <a-form-item :wrapper-col="{ span: 24 }">
        <a-input-group v-for="(attr, idx) in form.attrs" :key="idx" compact class="mb-8">
          <a-input :style="{ width: '45%' }" v-model:value="attr[0]" placeholder="Name">
            <template #addonBefore>{{ idx + 1 }}</template>
          </a-input>
          <a-input :style="{ width: '45%' }" v-model:value="attr[1]" placeholder="Value" />
          <a-button @click="form.attrs.splice(idx, 1)">
            <template #icon>
              <MinusOutlined />
            </template>
          </a-button>
        </a-input-group>
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">IP
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.ip" placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">Domain
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.domain" placeholder="google.com, geosite:cn" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">User
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.user" placeholder="email address" />
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Comma-separated list">Port
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-input v-model:value="form.port" placeholder="53,443,1000-2000" />
      </a-form-item>

      <a-form-item label="Inbound tags">
        <a-select v-model:value="form.inboundTag" mode="multiple">
          <a-select-option v-for="tag in inboundTags" :key="tag" :value="tag">{{ tag }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="Outbound tag">
        <a-select v-model:value="form.outboundTag">
          <a-select-option v-for="tag in outboundTags" :key="tag || '__empty'" :value="tag">{{ tag || '(none)'
            }}</a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item>
        <template #label>
          <a-tooltip title="Routes traffic through one of the configured load balancers">
            Balancer tag
            <QuestionCircleOutlined />
          </a-tooltip>
        </template>
        <a-select v-model:value="form.balancerTag">
          <a-select-option v-for="tag in balancerTags" :key="tag || '__empty'" :value="tag">{{ tag || '(none)'
            }}</a-select-option>
        </a-select>
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<style scoped>
.mb-8 {
  margin-bottom: 8px;
}
</style>
