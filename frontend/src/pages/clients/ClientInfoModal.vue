<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { CopyOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import { SizeFormatter, IntlUtil, ClipboardManager, HttpUtil } from '@/utils';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  client: { type: Object, default: null },
  inboundsById: { type: Object, default: () => ({}) },
  isOnline: { type: Boolean, default: false },
  subSettings: {
    type: Object,
    default: () => ({ enable: false, subURI: '', subJsonURI: '', subJsonEnable: false }),
  },
});

const emit = defineEmits(['update:open']);

const links = ref([]);
const linksLoading = ref(false);

const traffic = computed(() => props.client?.traffic || null);
const totalBytes = computed(() => props.client?.totalGB || 0);
const used = computed(() => (traffic.value?.up || 0) + (traffic.value?.down || 0));
const remaining = computed(() => {
  if (totalBytes.value <= 0) return -1;
  const r = totalBytes.value - used.value;
  return r > 0 ? r : 0;
});

const subLink = computed(() => {
  if (!props.client?.subId || !props.subSettings?.subURI) return '';
  return props.subSettings.subURI + props.client.subId;
});

const subJsonLink = computed(() => {
  if (!props.client?.subId) return '';
  if (!props.subSettings?.subJsonEnable || !props.subSettings?.subJsonURI) return '';
  return props.subSettings.subJsonURI + props.client.subId;
});

const showSubscription = computed(
  () => !!(props.subSettings?.enable && props.client?.subId),
);

function expiryLabel(ts) {
  if (!ts || ts <= 0) return '∞';
  return IntlUtil.formatDate(ts);
}

function expiryRelative(ts) {
  if (!ts || ts <= 0) return '';
  return IntlUtil.formatRelativeTime(ts);
}

function lastOnlineLabel(ts) {
  if (!ts || ts <= 0) return '-';
  return IntlUtil.formatDate(ts);
}

function dateLabel(ts) {
  if (!ts || ts <= 0) return '-';
  return IntlUtil.formatDate(ts);
}

async function copyValue(text) {
  if (!text) return;
  const ok = await ClipboardManager.copyText(String(text));
  if (ok) message.success(t('copied'));
}

async function loadLinks() {
  if (!props.client?.subId) {
    links.value = [];
    return;
  }
  linksLoading.value = true;
  try {
    const msg = await HttpUtil.get(
      `/panel/api/clients/subLinks/${encodeURIComponent(props.client.subId)}`,
    );
    links.value = msg?.success && Array.isArray(msg.obj) ? msg.obj : [];
  } finally {
    linksLoading.value = false;
  }
}

watch(() => props.open, (next) => {
  if (next) loadLinks();
  else links.value = [];
});

function close() {
  emit('update:open', false);
}
</script>

<template>
  <a-modal :open="open" :title="client ? client.email : t('info')" :footer="null" :width="640" @cancel="close">
    <template v-if="client">
      <table class="info-table block">
        <tbody>
          <tr>
            <td>{{ t('pages.clients.online') }}</td>
            <td>
              <a-tag v-if="client.enable && isOnline" color="green">{{ t('pages.clients.online') }}</a-tag>
              <a-tag v-else>{{ t('pages.clients.offline') }}</a-tag>
              <span class="hint">{{ t('lastOnline') }}: {{ lastOnlineLabel(traffic?.lastOnline) }}</span>
            </td>
          </tr>

          <tr>
            <td>{{ t('status') }}</td>
            <td>
              <a-tag :color="client.enable ? 'green' : 'default'">
                {{ client.enable ? t('enabled') : t('disabled') }}
              </a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.clients.email') }}</td>
            <td>
              <a-tag v-if="client.email" color="green">{{ client.email }}</a-tag>
              <a-tag v-else color="red">{{ t('none') }}</a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.clients.subId') }}</td>
            <td>
              <a-tag class="info-large-tag">{{ client.subId || '-' }}</a-tag>
              <a-button v-if="client.subId" size="small" type="text" @click="copyValue(client.subId)">
                <CopyOutlined />
              </a-button>
            </td>
          </tr>

          <tr v-if="client.uuid">
            <td>{{ t('pages.clients.uuid') }}</td>
            <td>
              <a-tag class="info-large-tag">{{ client.uuid }}</a-tag>
              <a-button size="small" type="text" @click="copyValue(client.uuid)">
                <CopyOutlined />
              </a-button>
            </td>
          </tr>

          <tr v-if="client.password">
            <td>{{ t('password') }}</td>
            <td>
              <a-tag class="info-large-tag">{{ client.password }}</a-tag>
              <a-button size="small" type="text" @click="copyValue(client.password)">
                <CopyOutlined />
              </a-button>
            </td>
          </tr>

          <tr v-if="client.auth">
            <td>{{ t('pages.clients.auth') }}</td>
            <td>
              <a-tag class="info-large-tag">{{ client.auth }}</a-tag>
              <a-button size="small" type="text" @click="copyValue(client.auth)">
                <CopyOutlined />
              </a-button>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.clients.flow') }}</td>
            <td>
              <a-tag v-if="client.flow">{{ client.flow }}</a-tag>
              <a-tag v-else color="orange">{{ t('none') }}</a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.inbounds.traffic') }}</td>
            <td>
              <a-tag>
                ↑ {{ SizeFormatter.sizeFormat(traffic?.up || 0) }}
                / ↓ {{ SizeFormatter.sizeFormat(traffic?.down || 0) }}
              </a-tag>
              <span class="hint">
                {{ SizeFormatter.sizeFormat(used) }}
                /
                {{ totalBytes > 0 ? SizeFormatter.sizeFormat(totalBytes) : '∞' }}
              </span>
            </td>
          </tr>

          <tr>
            <td>{{ t('remained') }}</td>
            <td>
              <a-tag v-if="remaining < 0" color="purple">∞</a-tag>
              <a-tag v-else :color="remaining > 0 ? '' : 'red'">
                {{ SizeFormatter.sizeFormat(remaining) }}
              </a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.inbounds.expireDate') }}</td>
            <td>
              <a-tag v-if="!client.expiryTime || client.expiryTime <= 0" color="purple">∞</a-tag>
              <a-tag v-else>{{ expiryLabel(client.expiryTime) }}</a-tag>
              <span v-if="client.expiryTime > 0" class="hint">{{ expiryRelative(client.expiryTime) }}</span>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.clients.ipLimit') }}</td>
            <td>
              <a-tag v-if="!client.limitIp">∞</a-tag>
              <a-tag v-else>{{ client.limitIp }}</a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.inbounds.createdAt') }}</td>
            <td>
              <a-tag>{{ dateLabel(client.createdAt) }}</a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.inbounds.updatedAt') }}</td>
            <td>
              <a-tag>{{ dateLabel(client.updatedAt) }}</a-tag>
            </td>
          </tr>

          <tr v-if="client.comment">
            <td>{{ t('pages.clients.comment') }}</td>
            <td>
              <a-tag class="info-large-tag">{{ client.comment }}</a-tag>
            </td>
          </tr>

          <tr>
            <td>{{ t('pages.clients.attachedInbounds') }}</td>
            <td>
              <div class="chips">
                <a-tag v-for="id in (client.inboundIds || [])" :key="id" color="blue">
                  <template v-if="inboundsById[id]">
                    {{ inboundsById[id].remark || `#${id}` }} ({{ inboundsById[id].protocol }}:{{ inboundsById[id].port }})
                  </template>
                  <template v-else>#{{ id }}</template>
                </a-tag>
                <span v-if="!client.inboundIds || client.inboundIds.length === 0" class="hint">—</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <template v-if="links.length > 0">
        <a-divider>{{ t('pages.inbounds.copyLink') }}</a-divider>
        <div v-for="(link, idx) in links" :key="idx" class="link-panel">
          <div class="link-panel-header">
            <a-tag color="green">{{ `${t('pages.clients.link')} ${idx + 1}` }}</a-tag>
            <a-tooltip :title="t('copy')">
              <a-button size="small" @click="copyValue(link)">
                <template #icon>
                  <CopyOutlined />
                </template>
              </a-button>
            </a-tooltip>
          </div>
          <code class="link-panel-text">{{ link }}</code>
        </div>
      </template>

      <template v-if="showSubscription && subLink">
        <a-divider>{{ t('subscription.title') }}</a-divider>
        <div class="link-panel">
          <div class="link-panel-header">
            <a-tag color="green">{{ t('subscription.title') }}</a-tag>
            <a-tooltip :title="t('copy')">
              <a-button size="small" @click="copyValue(subLink)">
                <template #icon>
                  <CopyOutlined />
                </template>
              </a-button>
            </a-tooltip>
          </div>
          <a :href="subLink" target="_blank" rel="noopener noreferrer" class="link-panel-anchor">{{ subLink }}</a>
        </div>

        <div v-if="subJsonLink" class="link-panel">
          <div class="link-panel-header">
            <a-tag color="green">JSON</a-tag>
            <a-tooltip :title="t('copy')">
              <a-button size="small" @click="copyValue(subJsonLink)">
                <template #icon>
                  <CopyOutlined />
                </template>
              </a-button>
            </a-tooltip>
          </div>
          <a :href="subJsonLink" target="_blank" rel="noopener noreferrer" class="link-panel-anchor">{{ subJsonLink }}</a>
        </div>
      </template>
    </template>
  </a-modal>
</template>

<style scoped>
.info-table {
  width: 100%;
  border-collapse: collapse;
}

.info-table.block {
  margin-bottom: 10px;
}

.info-table td {
  padding: 4px 8px;
  vertical-align: top;
}

.info-table td:first-child {
  width: 140px;
  font-size: 13px;
  opacity: 0.75;
  white-space: nowrap;
}

.info-large-tag {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
}

.hint {
  font-size: 12px;
  opacity: 0.55;
  margin-left: 6px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.link-panel {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 8px;
  padding: 10px;
  margin-bottom: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.link-panel-header {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.link-panel-text {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  word-break: break-all;
  white-space: pre-wrap;
  padding: 6px 8px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 4px;
  user-select: all;
}

:global(body.dark) .link-panel-text {
  background: rgba(255, 255, 255, 0.05);
}

.link-panel-anchor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  word-break: break-all;
  padding: 6px 8px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 4px;
  color: var(--ant-color-primary, #1677ff);
  text-decoration: underline;
  text-decoration-color: rgba(22, 119, 255, 0.4);
  transition: background 120ms ease, text-decoration-color 120ms ease;
}

.link-panel-anchor:hover {
  background: rgba(22, 119, 255, 0.08);
  text-decoration-color: var(--ant-color-primary, #1677ff);
}

:global(body.dark) .link-panel-anchor {
  background: rgba(255, 255, 255, 0.05);
}

:global(body.dark) .link-panel-anchor:hover {
  background: rgba(22, 119, 255, 0.16);
}
</style>
