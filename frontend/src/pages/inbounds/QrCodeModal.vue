<script setup>
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import { Inbound, Protocols } from '@/models/inbound.js';
import QrPanel from './QrPanel.vue';

const { t } = useI18n();

// Light QR-only modal — used for the "qrcode" row action on
// single-user Shadowsocks and WireGuard inbounds. The big info modal
// (InboundInfoModal) is too detailed when the user just wants the
// share link as a QR.

const props = defineProps({
  open: { type: Boolean, default: false },
  dbInbound: { type: Object, default: null },
  remarkModel: { type: String, default: '-ieo' },
});

const emit = defineEmits(['update:open']);

const links = ref([]);
const wireguardConfigs = ref([]);
const wireguardLinks = ref([]);

watch(() => props.open, (next) => {
  if (!next || !props.dbInbound) return;
  const inbound = props.dbInbound.toInbound();
  if (inbound.protocol === Protocols.WIREGUARD) {
    wireguardConfigs.value = inbound.genWireguardConfigs(props.dbInbound.remark).split('\r\n');
    wireguardLinks.value = inbound.genWireguardLinks(props.dbInbound.remark).split('\r\n');
    links.value = [];
  } else {
    // SS single-user — pass null client; genAllLinks falls back to
    // the inbound's settings.
    links.value = inbound.genAllLinks(props.dbInbound.remark, props.remarkModel, null);
    wireguardConfigs.value = [];
    wireguardLinks.value = [];
  }
});

function close() {
  emit('update:open', false);
}
</script>

<template>
  <a-modal :open="open" :title="t('qrCode')" :footer="null" width="420px" @cancel="close">
    <template v-if="dbInbound">
      <QrPanel
        v-for="(link, idx) in links"
        :key="`l${idx}`"
        :value="link.link"
        :remark="link.remark || `Link ${idx + 1}`"
      />
      <template v-for="(cfg, idx) in wireguardConfigs" :key="`w${idx}`">
        <QrPanel
          :value="cfg"
          :remark="`Peer ${idx + 1} config`"
          :download-name="`peer-${idx + 1}.conf`"
        />
        <QrPanel
          v-if="wireguardLinks[idx]"
          :value="wireguardLinks[idx]"
          :remark="`Peer ${idx + 1} link`"
        />
      </template>
    </template>
  </a-modal>
</template>
