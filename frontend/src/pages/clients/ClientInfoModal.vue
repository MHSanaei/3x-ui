<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { SizeFormatter, IntlUtil, ClipboardManager } from '@/utils';
import { CopyOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  client: { type: Object, default: null },
  inboundsById: { type: Object, default: () => ({}) },
  isOnline: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open']);

const traffic = computed(() => props.client?.traffic || null);
const totalBytes = computed(() => props.client?.totalGB || 0);
const used = computed(() => (traffic.value?.up || 0) + (traffic.value?.down || 0));
const remaining = computed(() => {
  if (totalBytes.value <= 0) return -1;
  const r = totalBytes.value - used.value;
  return r > 0 ? r : 0;
});

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

async function copyValue(text) {
  if (!text) return;
  const ok = await ClipboardManager.copyText(String(text));
  if (ok) message.success(t('copied'));
}

function close() {
  emit('update:open', false);
}
</script>

<template>
  <a-modal :open="open" :title="client ? client.email : t('info')" :footer="null" :width="560"
    @cancel="close">
    <div v-if="client" class="info-grid">
      <div class="row">
        <span class="label">{{ t('online') }}</span>
        <a-tag v-if="client.enable && isOnline" color="green">{{ t('online') }}</a-tag>
        <a-tag v-else>{{ t('offline') }}</a-tag>
        <span class="hint">{{ t('lastOnline') }}: {{ lastOnlineLabel(traffic?.lastOnline) }}</span>
      </div>

      <div class="row">
        <span class="label">{{ t('enable') }}</span>
        <a-tag :color="client.enable ? 'green' : 'default'">
          {{ client.enable ? t('enable') : t('disable') }}
        </a-tag>
      </div>

      <div class="row">
        <span class="label">subId</span>
        <span class="value mono">{{ client.subId || '-' }}</span>
        <a-button v-if="client.subId" size="small" type="text" @click="copyValue(client.subId)">
          <CopyOutlined />
        </a-button>
      </div>

      <div v-if="client.uuid" class="row">
        <span class="label">UUID</span>
        <span class="value mono">{{ client.uuid }}</span>
        <a-button size="small" type="text" @click="copyValue(client.uuid)">
          <CopyOutlined />
        </a-button>
      </div>

      <div v-if="client.password" class="row">
        <span class="label">Password</span>
        <span class="value mono">{{ client.password }}</span>
        <a-button size="small" type="text" @click="copyValue(client.password)">
          <CopyOutlined />
        </a-button>
      </div>

      <div v-if="client.auth" class="row">
        <span class="label">Auth</span>
        <span class="value mono">{{ client.auth }}</span>
        <a-button size="small" type="text" @click="copyValue(client.auth)">
          <CopyOutlined />
        </a-button>
      </div>

      <div class="row">
        <span class="label">{{ t('pages.inbounds.traffic') }}</span>
        <a-tag>
          ↑ {{ SizeFormatter.sizeFormat(traffic?.up || 0) }}
          / ↓ {{ SizeFormatter.sizeFormat(traffic?.down || 0) }}
        </a-tag>
        <span class="hint">
          {{ SizeFormatter.sizeFormat(used) }}
          /
          {{ totalBytes > 0 ? SizeFormatter.sizeFormat(totalBytes) : '∞' }}
        </span>
      </div>

      <div class="row">
        <span class="label">{{ t('remained') || 'Remaining' }}</span>
        <a-tag v-if="remaining < 0" color="purple">∞</a-tag>
        <a-tag v-else :color="remaining > 0 ? '' : 'red'">
          {{ SizeFormatter.sizeFormat(remaining) }}
        </a-tag>
      </div>

      <div class="row">
        <span class="label">{{ t('pages.inbounds.allTimeTraffic') || 'All-time' }}</span>
        <a-tag>{{ SizeFormatter.sizeFormat(traffic?.allTime || (used)) }}</a-tag>
      </div>

      <div class="row">
        <span class="label">{{ t('pages.inbounds.expireDate') || 'Expiry' }}</span>
        <a-tag v-if="!client.expiryTime || client.expiryTime <= 0" color="purple">∞</a-tag>
        <a-tag v-else>{{ expiryLabel(client.expiryTime) }}</a-tag>
        <span v-if="client.expiryTime > 0" class="hint">{{ expiryRelative(client.expiryTime) }}</span>
      </div>

      <div class="row">
        <span class="label">IP limit</span>
        <a-tag v-if="!client.limitIp">∞</a-tag>
        <a-tag v-else>{{ client.limitIp }}</a-tag>
      </div>

      <div v-if="client.comment" class="row">
        <span class="label">{{ t('pages.inbounds.client.comment') || 'Comment' }}</span>
        <span class="value">{{ client.comment }}</span>
      </div>

      <div class="row">
        <span class="label">{{ t('pages.clients.attachedInbounds') || 'Attached inbounds' }}</span>
        <div class="chips">
          <a-tag v-for="id in (client.inboundIds || [])" :key="id" color="blue">
            <template v-if="inboundsById[id]">
              {{ inboundsById[id].remark || `#${id}` }} ({{ inboundsById[id].protocol }}:{{ inboundsById[id].port }})
            </template>
            <template v-else>#{{ id }}</template>
          </a-tag>
          <span v-if="!client.inboundIds || client.inboundIds.length === 0" class="hint">—</span>
        </div>
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
.info-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.label {
  min-width: 120px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  opacity: 0.6;
  flex-shrink: 0;
}

.value {
  word-break: break-all;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.hint {
  font-size: 12px;
  opacity: 0.55;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
