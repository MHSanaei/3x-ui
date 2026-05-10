<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
});

const emit = defineEmits(['update:open', 'install']);

const PRESETS = [
  {
    name: 'Google DNS',
    family: false,
    data: [
      '8.8.8.8',
      '8.8.4.4',
      '2001:4860:4860::8888',
      '2001:4860:4860::8844',
    ],
  },
  {
    name: 'Cloudflare DNS',
    family: false,
    data: [
      '1.1.1.1',
      '1.0.0.1',
      '2606:4700:4700::1111',
      '2606:4700:4700::1001',
    ],
  },
  {
    name: 'AdGuard DNS',
    family: false,
    data: [
      '94.140.14.14',
      '94.140.15.15',
      '2a10:50c0::ad1:ff',
      '2a10:50c0::ad2:ff',
    ],
  },
  {
    name: 'AdGuard Family DNS',
    family: true,
    data: [
      '94.140.14.15',
      '94.140.15.16',
      '2a10:50c0::bad1:ff',
      '2a10:50c0::bad2:ff',
    ],
  },
  {
    name: 'Cloudflare Family DNS',
    family: true,
    data: [
      '1.1.1.3',
      '1.0.0.3',
      '2606:4700:4700::1113',
      '2606:4700:4700::1003',
    ],
  },
];

const title = computed(() => t('pages.xray.dns.dnsPresetTitle'));

function close() { emit('update:open', false); }
function install(preset) {
  emit('install', [...preset.data]);
}
</script>

<template>
  <a-modal :open="open" :title="title" :footer="null" :mask-closable="false" @cancel="close">
    <a-list bordered>
      <a-list-item v-for="preset in PRESETS" :key="preset.name" class="preset-row">
        <a-space size="small" align="center">
          <a-tag :color="preset.family ? 'purple' : 'green'">
            {{ preset.family ? t('pages.xray.dns.dnsPresetFamily') : 'DNS' }}
          </a-tag>
          <span class="preset-name">{{ preset.name }}</span>
        </a-space>
        <a-button type="primary" size="small" @click="install(preset)">
          {{ t('install') }}
        </a-button>
      </a-list-item>
    </a-list>
  </a-modal>
</template>

<style scoped>
.preset-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.preset-name {
  font-weight: 500;
}
</style>
