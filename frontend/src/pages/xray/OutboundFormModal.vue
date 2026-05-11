<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { message } from 'ant-design-vue';
import { SyncOutlined, PlusOutlined, MinusOutlined, DeleteOutlined } from '@ant-design/icons-vue';

import { Wireguard } from '@/utils';
import {
  Outbound,
  Protocols,
  SSMethods,
  TLS_FLOW_CONTROL,
  UTLS_FINGERPRINT,
  ALPN_OPTION,
  SNIFFING_OPTION,
  USERS_SECURITY,
  OutboundDomainStrategies,
  WireguardDomainStrategy,
  Address_Port_Strategy,
  MODE_OPTION,
  DNSRuleActions,
} from '@/models/outbound.js';
import FinalMaskForm from '@/components/FinalMaskForm.vue';

const { t } = useI18n();

// Structured outbound add/edit modal — mirrors the legacy
// web/html/form/outbound.html. Covers every protocol + transport
// combination the legacy panel exposes; the JSON tab still lets
// power-users hand-edit fields the structured form doesn't surface
// (reverse-sniffing, exotic outbound DNS rules, etc.).

const props = defineProps({
  open: { type: Boolean, default: false },
  outbound: { type: Object, default: null },
  existingTags: { type: Array, default: () => [] },
  inboundTags: { type: Array, default: () => [] },
});

const emit = defineEmits(['update:open', 'confirm']);

const PROTOCOL_OPTIONS = Object.values(Protocols);
const SECURITY_OPTIONS = Object.values(USERS_SECURITY);
const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);
const UTLS_OPTIONS = Object.values(UTLS_FINGERPRINT);
const ALPN_OPTIONS = Object.values(ALPN_OPTION);
const NETWORKS = ['tcp', 'kcp', 'ws', 'grpc', 'httpupgrade', 'xhttp'];
const NETWORK_LABELS = {
  tcp: 'TCP (RAW)',
  kcp: 'mKCP',
  ws: 'WebSocket',
  grpc: 'gRPC',
  httpupgrade: 'HTTPUpgrade',
  xhttp: 'XHTTP',
};

// Reactive draft — Outbound instance built from the prop on open.
// Intentionally shadows the prop name; the template reads the draft.
// eslint-disable-next-line vue/no-dupe-keys
const outbound = ref(null);
const isEdit = ref(false);
const activeKey = ref('1');
const linkInput = ref('');

// Advanced JSON editor — kept in sync with the parsed Outbound on tab
// switch so users can copy/paste a full JSON config when the structured
// form doesn't reach a field.
const advancedJson = ref('');

watch(() => props.open, (next) => {
  if (!next) return;
  if (props.outbound) {
    isEdit.value = true;
    outbound.value = Outbound.fromJson(props.outbound);
  } else {
    isEdit.value = false;
    outbound.value = new Outbound();
  }
  activeKey.value = '1';
  linkInput.value = '';
  primeAdvancedJson();
});

watch(activeKey, (key) => {
  if (key === '2') primeAdvancedJson();
});

function primeAdvancedJson() {
  if (!outbound.value) { advancedJson.value = ''; return; }
  try {
    advancedJson.value = JSON.stringify(outbound.value.toJson(), null, 2);
  } catch (_e) {
    advancedJson.value = '';
  }
}

function close() { emit('update:open', false); }

function onProtocolChange(next) {
  if (!outbound.value) return;
  outbound.value.protocol = next;
}

function streamNetworkChange(next) {
  if (!outbound.value?.stream) return;
  outbound.value.stream.network = next;
  if (!outbound.value.canEnableTls()) outbound.value.stream.security = 'none';
}

const duplicateTag = computed(() => {
  if (!outbound.value?.tag) return false;
  const myTag = outbound.value.tag.trim();
  if (!myTag) return false;
  if (isEdit.value && props.outbound?.tag === myTag) return false;
  return (props.existingTags || []).includes(myTag);
});

const tagEmpty = computed(() => !outbound.value?.tag?.trim());

const tagValidateStatus = computed(() => {
  if (tagEmpty.value) return 'error';
  if (duplicateTag.value) return 'warning';
  return 'success';
});

const tagHelp = computed(() => {
  if (tagEmpty.value) return 'Tag is required';
  if (duplicateTag.value) return 'Tag already used by another outbound';
  return '';
});

// ============== Submit ==============
function onOk() {
  if (!outbound.value) return;
  if (!outbound.value.tag?.trim()) {
    message.error(t('somethingWentWrong'));
    return;
  }
  if (duplicateTag.value) {
    message.error(t('somethingWentWrong'));
    return;
  }
  // If user spent time in the JSON tab, prefer that body — round-trip
  // it through Outbound.fromJson so the wire shape stays consistent.
  if (activeKey.value === '2' && advancedJson.value.trim()) {
    try {
      const parsed = JSON.parse(advancedJson.value);
      const built = Outbound.fromJson(parsed);
      emit('confirm', built.toJson());
      return;
    } catch (e) {
      message.error(`JSON: ${e.message}`);
      return;
    }
  }
  emit('confirm', outbound.value.toJson());
}

// ============== Link → outbound ==============
// Mirrors the legacy convertLink: dispatches into Outbound.fromLink,
// which handles vmess:// (base64 JSON), vless://, trojan://, ss://
// (param-link form), and hysteria(2)://. Anything else returns null
// from the model and we surface "Wrong Link!" the same as legacy.
function convertLink() {
  const link = linkInput.value.trim();
  if (!link) return;
  try {
    const next = Outbound.fromLink(link);
    if (!next) {
      message.error('Wrong Link!');
      return;
    }
    outbound.value = next;
    linkInput.value = '';
    message.success('Link imported successfully...');
    activeKey.value = '1';
  } catch (e) {
    message.error(`Link parse: ${e.message}`);
  }
}

const title = computed(() =>
  isEdit.value
    ? `${t('edit')} ${t('pages.xray.Outbounds')}`
    : `+ ${t('pages.xray.Outbounds')}`,
);
const okText = computed(() =>
  isEdit.value ? t('pages.client.submitEdit') : t('create'),
);

// Helper getters / shortcuts used by the template.
const proto = computed(() => outbound.value?.protocol);
const isVMess = computed(() => proto.value === Protocols.VMess);
const isVLESS = computed(() => proto.value === Protocols.VLESS);
const isVMessOrVLess = computed(() => isVMess.value || isVLESS.value);
const isTrojan = computed(() => proto.value === Protocols.Trojan);
const isShadowsocks = computed(() => proto.value === Protocols.Shadowsocks);
const isFreedom = computed(() => proto.value === Protocols.Freedom);
const isBlackhole = computed(() => proto.value === Protocols.Blackhole);
const isDNS = computed(() => proto.value === Protocols.DNS);
const isWireguard = computed(() => proto.value === Protocols.Wireguard);
const isHysteria = computed(() => proto.value === Protocols.Hysteria);
const isLoopback = computed(() => proto.value === Protocols.Loopback);

function regenerateWgKeys() {
  if (!outbound.value?.settings) return;
  const pair = Wireguard.generateKeypair();
  outbound.value.settings.secretKey = pair.privateKey;
  outbound.value.settings.pubKey = pair.publicKey;
}
</script>

<template>
  <a-modal :open="open" :title="title" :ok-text="okText" :cancel-text="t('close')" :mask-closable="false" width="780px"
    @ok="onOk" @cancel="close">
    <a-tabs v-if="outbound" v-model:active-key="activeKey">
      <!-- ============================== FORM ============================== -->
      <a-tab-pane key="1" :tab="t('pages.xray.basicTemplate')">
        <a-form :colon="false" :label-col="{ md: { span: 8 } }" :wrapper-col="{ md: { span: 14 } }">
          <!-- Protocol -->
          <a-form-item :label="t('protocol')">
            <a-select :value="proto" @change="onProtocolChange">
              <a-select-option v-for="p in PROTOCOL_OPTIONS" :key="p" :value="p">{{ p }}</a-select-option>
            </a-select>
          </a-form-item>

          <!-- Tag -->
          <a-form-item label="Tag" :validate-status="tagValidateStatus" :help="tagHelp" has-feedback>
            <a-input v-model:value="outbound.tag" placeholder="unique-tag" />
          </a-form-item>

          <!-- Send through -->
          <a-form-item label="Send through">
            <a-input v-model:value="outbound.sendThrough" placeholder="local IP" />
          </a-form-item>

          <!-- ============== Freedom ============== -->
          <template v-if="isFreedom">
            <a-form-item label="Strategy">
              <a-select v-model:value="outbound.settings.domainStrategy">
                <a-select-option v-for="s in OutboundDomainStrategies" :key="s" :value="s">{{ s }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Redirect">
              <a-input v-model:value="outbound.settings.redirect" />
            </a-form-item>

            <a-form-item label="Fragment">
              <a-switch :checked="!!outbound.settings.fragment && Object.keys(outbound.settings.fragment).length > 0"
                @change="(checked) => outbound.settings.fragment = checked ? { packets: 'tlshello', length: '100-200', interval: '10-20', maxSplit: '300-400' } : {}" />
            </a-form-item>
            <template v-if="outbound.settings.fragment && Object.keys(outbound.settings.fragment).length > 0">
              <a-form-item label="Packets">
                <a-select v-model:value="outbound.settings.fragment.packets">
                  <a-select-option v-for="p in ['1-3', 'tlshello']" :key="p" :value="p">{{ p }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Length">
                <a-input v-model:value="outbound.settings.fragment.length" placeholder="100-200" />
              </a-form-item>
              <a-form-item label="Interval">
                <a-input v-model:value="outbound.settings.fragment.interval" placeholder="10-20" />
              </a-form-item>
              <a-form-item label="Max Split">
                <a-input v-model:value="outbound.settings.fragment.maxSplit" placeholder="300-400" />
              </a-form-item>
            </template>

            <a-form-item label="Noises">
              <a-switch :checked="(outbound.settings.noises || []).length > 0"
                @change="(checked) => outbound.settings.noises = checked ? [new Outbound.FreedomSettings.Noise()] : []" />
              <a-button v-if="outbound.settings.noises && outbound.settings.noises.length > 0" size="small"
                type="primary" class="ml-8"
                @click="outbound.settings.noises.push(new Outbound.FreedomSettings.Noise())">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
            </a-form-item>
            <template v-for="(noise, index) in outbound.settings.noises || []" :key="index">
              <a-form-item :wrapper-col="{ md: { span: 14, offset: 8 } }" :colon="false">
                <div class="item-heading">
                  <span>Noise {{ index + 1 }}</span>
                  <DeleteOutlined v-if="outbound.settings.noises.length > 1" class="danger-icon"
                    @click="outbound.settings.noises.splice(index, 1)" />
                </div>
              </a-form-item>
              <a-form-item label="Type">
                <a-select v-model:value="noise.type">
                  <a-select-option v-for="x in ['rand', 'base64', 'str', 'hex']" :key="x" :value="x">{{ x
                  }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Packet">
                <a-input v-model:value="noise.packet" />
              </a-form-item>
              <a-form-item label="Delay (ms)">
                <a-input v-model:value="noise.delay" />
              </a-form-item>
              <a-form-item label="Apply to">
                <a-select v-model:value="noise.applyTo">
                  <a-select-option v-for="x in ['ip', 'ipv4', 'ipv6']" :key="x" :value="x">{{ x }}</a-select-option>
                </a-select>
              </a-form-item>
            </template>
          </template>

          <!-- ============== Blackhole ============== -->
          <template v-if="isBlackhole">
            <a-form-item label="Response Type">
              <a-select v-model:value="outbound.settings.type">
                <a-select-option v-for="x in ['', 'none', 'http']" :key="x" :value="x">{{ x || '(empty)'
                }}</a-select-option>
              </a-select>
            </a-form-item>
          </template>

          <!-- ============== Loopback ============== -->
          <template v-if="isLoopback">
            <a-form-item label="Inbound tag">
              <a-auto-complete v-model:value="outbound.settings.inboundTag"
                :options="inboundTags.map((tag) => ({ value: tag }))"
                :filter-option="(input, option) => option.value.toLowerCase().includes(input.toLowerCase())"
                placeholder="tag of an existing inbound to re-route into" />
            </a-form-item>
          </template>

          <!-- ============== DNS ============== -->
          <template v-if="isDNS">
            <a-form-item label="Rewrite network">
              <a-select v-model:value="outbound.settings.rewriteNetwork" allow-clear placeholder="(unchanged)">
                <a-select-option v-for="x in ['udp', 'tcp']" :key="x" :value="x">{{ x }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Rewrite address">
              <a-input v-model:value="outbound.settings.rewriteAddress" placeholder="(unchanged) e.g. 1.1.1.1" />
            </a-form-item>
            <a-form-item label="Rewrite port">
              <a-input-number v-model:value="outbound.settings.rewritePort" :min="0" :max="65535"
                :style="{ width: '100%' }" placeholder="(unchanged)" />
            </a-form-item>
            <a-form-item label="User level">
              <a-input-number v-model:value="outbound.settings.userLevel" :min="0" :style="{ width: '100%' }" />
            </a-form-item>
            <a-form-item label="Rules">
              <a-button size="small" type="primary" @click="outbound.settings.rules.push(new Outbound.DNSRule())">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
            </a-form-item>
            <template v-for="(rule, index) in outbound.settings.rules || []" :key="index">
              <a-form-item :wrapper-col="{ md: { span: 14, offset: 8 } }" :colon="false">
                <div class="item-heading">
                  <span>Rule {{ index + 1 }}</span>
                  <DeleteOutlined class="danger-icon" @click="outbound.settings.rules.splice(index, 1)" />
                </div>
              </a-form-item>
              <a-form-item label="Action">
                <a-select v-model:value="rule.action">
                  <a-select-option v-for="a in DNSRuleActions" :key="a" :value="a">{{ a }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="QType">
                <a-input v-model:value="rule.qtype" placeholder="1,3,23-24" />
              </a-form-item>
              <a-form-item :label="t('domainName')">
                <a-input v-model:value="rule.domain" placeholder="domain:example.com" />
              </a-form-item>
            </template>
          </template>

          <!-- ============== WireGuard ============== -->
          <template v-if="isWireguard">
            <a-form-item :label="t('pages.inbounds.address')">
              <a-input v-model:value="outbound.settings.address" />
            </a-form-item>
            <a-form-item>
              <template #label>
                {{ t('pages.inbounds.privatekey') }}
                <SyncOutlined class="random-icon" @click="regenerateWgKeys" />
              </template>
              <a-input v-model:value="outbound.settings.secretKey" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.publicKey')">
              <a-input :value="outbound.settings.pubKey" disabled />
            </a-form-item>
            <a-form-item label="Domain strategy">
              <a-select v-model:value="outbound.settings.domainStrategy">
                <a-select-option v-for="x in ['', ...WireguardDomainStrategy]" :key="x || '__'" :value="x">
                  {{ x || `(${t('none')})` }}
                </a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="MTU">
              <a-input-number v-model:value="outbound.settings.mtu" :min="0" />
            </a-form-item>
            <a-form-item label="Workers">
              <a-input-number v-model:value="outbound.settings.workers" :min="0" />
            </a-form-item>
            <a-form-item label="No-kernel TUN">
              <a-switch v-model:checked="outbound.settings.noKernelTun" />
            </a-form-item>
            <a-form-item label="Reserved">
              <a-input v-model:value="outbound.settings.reserved" />
            </a-form-item>
            <a-form-item label="Peers">
              <a-button size="small" type="primary"
                @click="outbound.settings.peers.push(new Outbound.WireguardSettings.Peer())">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
            </a-form-item>
            <template v-for="(peer, index) in outbound.settings.peers || []" :key="index">
              <a-form-item :wrapper-col="{ md: { span: 14, offset: 8 } }" :colon="false">
                <div class="item-heading">
                  <span>Peer {{ index + 1 }}</span>
                  <DeleteOutlined v-if="outbound.settings.peers.length > 1" class="danger-icon"
                    @click="outbound.settings.peers.splice(index, 1)" />
                </div>
              </a-form-item>
              <a-form-item label="Endpoint">
                <a-input v-model:value="peer.endpoint" />
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.publicKey')">
                <a-input v-model:value="peer.publicKey" />
              </a-form-item>
              <a-form-item label="PSK">
                <a-input v-model:value="peer.psk" />
              </a-form-item>
              <a-form-item label="Allowed IPs">
                <template v-for="(_, idx) in peer.allowedIPs" :key="idx">
                  <a-input v-model:value="peer.allowedIPs[idx]" :style="{ marginBottom: '4px' }">
                    <template v-if="peer.allowedIPs.length > 1" #addonAfter>
                      <MinusOutlined @click="peer.allowedIPs.splice(idx, 1)" />
                    </template>
                  </a-input>
                </template>
                <a-button size="small" @click="peer.allowedIPs.push('')">
                  <template #icon>
                    <PlusOutlined />
                  </template>
                </a-button>
              </a-form-item>
              <a-form-item label="Keep alive">
                <a-input-number v-model:value="peer.keepAlive" :min="0" />
              </a-form-item>
            </template>
          </template>

          <!-- ============== Address + Port (most protocols) ============== -->
          <template v-if="outbound.hasAddressPort()">
            <a-form-item :label="t('pages.inbounds.address')">
              <a-input v-model:value="outbound.settings.address" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.port')">
              <a-input-number v-model:value="outbound.settings.port" :min="1" :max="65535" />
            </a-form-item>
          </template>

          <!-- ============== VMess / VLess user ============== -->
          <template v-if="isVMessOrVLess">
            <a-form-item label="ID">
              <a-input v-model:value="outbound.settings.id" />
            </a-form-item>
            <a-form-item v-if="isVMess" :label="t('security')">
              <a-select v-model:value="outbound.settings.security">
                <a-select-option v-for="s in SECURITY_OPTIONS" :key="s" :value="s">{{ s }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item v-if="isVLESS" :label="t('encryption')">
              <a-input v-model:value="outbound.settings.encryption" />
            </a-form-item>
            <a-form-item v-if="isVLESS" label="Reverse tag">
              <a-input v-model:value="outbound.settings.reverseTag" placeholder="optional" />
            </a-form-item>

            <!-- Reverse-Sniffing — surfaced only when a reverse tag is set,
                 mirroring the legacy form. Defaults populated by the model
                 so the toggle/checkboxes always have a backing field. -->
            <template v-if="isVLESS && outbound.settings.reverseTag">
              <a-form-item label="Reverse Sniffing">
                <a-switch v-model:checked="outbound.settings.reverseSniffing.enabled" />
              </a-form-item>
              <template v-if="outbound.settings.reverseSniffing.enabled">
                <!-- Align the checkbox row with the input fields above —
                     same span as wrapper-col (14), offset by label-col (8)
                     so the row starts where Reverse Tag's input starts. -->
                <a-form-item :wrapper-col="{ md: { span: 14, offset: 8 } }">
                  <a-checkbox-group v-model:value="outbound.settings.reverseSniffing.destOverride"
                    class="sniffing-options">
                    <a-checkbox v-for="(value, label) in SNIFFING_OPTION" :key="value" :value="value">{{ label
                    }}</a-checkbox>
                  </a-checkbox-group>
                </a-form-item>
                <a-form-item label="Metadata Only">
                  <a-switch v-model:checked="outbound.settings.reverseSniffing.metadataOnly" />
                </a-form-item>
                <a-form-item label="Route Only">
                  <a-switch v-model:checked="outbound.settings.reverseSniffing.routeOnly" />
                </a-form-item>
                <a-form-item label="IPs Excluded">
                  <a-select v-model:value="outbound.settings.reverseSniffing.ipsExcluded" mode="tags"
                    :token-separators="[',']" placeholder="IP/CIDR/geoip:*/ext:*" :style="{ width: '100%' }" />
                </a-form-item>
                <a-form-item label="Domains Excluded">
                  <a-select v-model:value="outbound.settings.reverseSniffing.domainsExcluded" mode="tags"
                    :token-separators="[',']" placeholder="domain:*/ext:*" :style="{ width: '100%' }" />
                </a-form-item>
              </template>
            </template>
            <a-form-item v-if="outbound.canEnableTlsFlow()" label="Flow">
              <a-select v-model:value="outbound.settings.flow">
                <a-select-option value="">{{ t('none') }}</a-select-option>
                <a-select-option v-for="key in FLOW_OPTIONS" :key="key" :value="key">{{ key }}</a-select-option>
              </a-select>
            </a-form-item>
          </template>

          <!-- ============== Trojan / Shadowsocks ============== -->
          <template v-if="isTrojan || isShadowsocks">
            <a-form-item :label="t('password')">
              <a-input v-model:value="outbound.settings.password" />
            </a-form-item>
          </template>
          <template v-if="isShadowsocks">
            <a-form-item :label="t('encryption')">
              <a-select v-model:value="outbound.settings.method">
                <a-select-option v-for="(m, k) in SSMethods" :key="m" :value="m">{{ k }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="UDP over TCP">
              <a-switch v-model:checked="outbound.settings.uot" />
            </a-form-item>
            <a-form-item label="UoT version">
              <a-input-number v-model:value="outbound.settings.UoTVersion" :min="1" :max="2" />
            </a-form-item>
          </template>

          <!-- ============== SOCKS / HTTP ============== -->
          <template v-if="outbound.hasUsername()">
            <a-form-item :label="t('username')">
              <a-input v-model:value="outbound.settings.user" />
            </a-form-item>
            <a-form-item :label="t('password')">
              <a-input v-model:value="outbound.settings.pass" />
            </a-form-item>
          </template>

          <!-- ============== Hysteria ============== -->
          <template v-if="isHysteria">
            <a-form-item label="Version">
              <a-input-number :value="outbound.settings.version || 2" :min="2" :max="2" disabled />
            </a-form-item>
          </template>

          <!-- ============== Stream settings ============== -->
          <template v-if="outbound.canEnableStream()">
            <a-form-item :label="t('transmission')">
              <a-select :value="outbound.stream.network" @change="streamNetworkChange">
                <a-select-option v-for="net in (isHysteria ? [...NETWORKS, 'hysteria'] : NETWORKS)" :key="net"
                  :value="net">
                  {{ NETWORK_LABELS[net] || net }}
                </a-select-option>
              </a-select>
            </a-form-item>

            <!-- TCP -->
            <template v-if="outbound.stream.network === 'tcp'">
              <a-form-item :label="`HTTP ${t('camouflage')}`">
                <a-switch :checked="outbound.stream.tcp.type === 'http'"
                  @change="(checked) => outbound.stream.tcp.type = checked ? 'http' : 'none'" />
              </a-form-item>
              <template v-if="outbound.stream.tcp.type === 'http'">
                <a-form-item :label="t('host')">
                  <a-input v-model:value="outbound.stream.tcp.host" />
                </a-form-item>
                <a-form-item :label="t('path')">
                  <a-input v-model:value="outbound.stream.tcp.path" />
                </a-form-item>
              </template>
            </template>

            <!-- KCP -->
            <template v-if="outbound.stream.network === 'kcp'">
              <a-form-item label="MTU">
                <a-input-number v-model:value="outbound.stream.kcp.mtu" :min="0" />
              </a-form-item>
              <a-form-item label="TTI (ms)">
                <a-input-number v-model:value="outbound.stream.kcp.tti" :min="0" />
              </a-form-item>
              <a-form-item label="Uplink (MB/s)">
                <a-input-number v-model:value="outbound.stream.kcp.upCap" :min="0" />
              </a-form-item>
              <a-form-item label="Downlink (MB/s)">
                <a-input-number v-model:value="outbound.stream.kcp.downCap" :min="0" />
              </a-form-item>
              <a-form-item label="CWND multiplier">
                <a-input-number v-model:value="outbound.stream.kcp.cwndMultiplier" :min="1" />
              </a-form-item>
              <a-form-item label="Max sending window">
                <a-input-number v-model:value="outbound.stream.kcp.maxSendingWindow" :min="0" />
              </a-form-item>
            </template>

            <!-- WebSocket -->
            <template v-if="outbound.stream.network === 'ws'">
              <a-form-item :label="t('host')">
                <a-input v-model:value="outbound.stream.ws.host" />
              </a-form-item>
              <a-form-item :label="t('path')">
                <a-input v-model:value="outbound.stream.ws.path" />
              </a-form-item>
              <a-form-item label="Heartbeat (s)">
                <a-input-number v-model:value="outbound.stream.ws.heartbeatPeriod" :min="0" />
              </a-form-item>
            </template>

            <!-- gRPC -->
            <template v-if="outbound.stream.network === 'grpc'">
              <a-form-item label="Service name">
                <a-input v-model:value="outbound.stream.grpc.serviceName" />
              </a-form-item>
              <a-form-item label="Authority">
                <a-input v-model:value="outbound.stream.grpc.authority" />
              </a-form-item>
              <a-form-item label="Multi mode">
                <a-switch v-model:checked="outbound.stream.grpc.multiMode" />
              </a-form-item>
            </template>

            <!-- HTTPUpgrade -->
            <template v-if="outbound.stream.network === 'httpupgrade'">
              <a-form-item :label="t('host')">
                <a-input v-model:value="outbound.stream.httpupgrade.host" />
              </a-form-item>
              <a-form-item :label="t('path')">
                <a-input v-model:value="outbound.stream.httpupgrade.path" />
              </a-form-item>
            </template>

            <!-- XHTTP — full parity with legacy outbound form. The model
                 already carries every field below; we just surface them. -->
            <template v-if="outbound.stream.network === 'xhttp'">
              <a-form-item :label="t('host')">
                <a-input v-model:value="outbound.stream.xhttp.host" />
              </a-form-item>
              <a-form-item :label="t('path')">
                <a-input v-model:value="outbound.stream.xhttp.path" />
              </a-form-item>

              <a-form-item :label="t('pages.inbounds.stream.tcp.requestHeader')">
                <a-button size="small" @click="outbound.stream.xhttp.addHeader('', '')">
                  <template #icon>
                    <PlusOutlined />
                  </template>
                </a-button>
              </a-form-item>
              <a-form-item :wrapper-col="{ span: 24 }">
                <a-input-group v-for="(header, idx) in outbound.stream.xhttp.headers" :key="idx" compact class="mb-8">
                  <a-input v-model:value="header.name" :style="{ width: '45%' }" placeholder="Name">
                    <template #addonBefore>{{ idx + 1 }}</template>
                  </a-input>
                  <a-input v-model:value="header.value" :style="{ width: '45%' }" placeholder="Value" />
                  <a-button @click="outbound.stream.xhttp.removeHeader(idx)">
                    <template #icon>
                      <MinusOutlined />
                    </template>
                  </a-button>
                </a-input-group>
              </a-form-item>

              <a-form-item label="Mode">
                <a-select v-model:value="outbound.stream.xhttp.mode">
                  <a-select-option v-for="m in Object.values(MODE_OPTION)" :key="m" :value="m">{{ m }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item v-if="outbound.stream.xhttp.mode === 'packet-up'" label="Max Upload Size (Byte)">
                <a-input v-model:value="outbound.stream.xhttp.scMaxEachPostBytes" />
              </a-form-item>
              <a-form-item v-if="outbound.stream.xhttp.mode === 'packet-up'" label="Min Upload Interval (Ms)">
                <a-input v-model:value="outbound.stream.xhttp.scMinPostsIntervalMs" />
              </a-form-item>

              <a-form-item label="Padding Bytes">
                <a-input v-model:value="outbound.stream.xhttp.xPaddingBytes" />
              </a-form-item>
              <a-form-item label="Padding Obfs Mode">
                <a-switch v-model:checked="outbound.stream.xhttp.xPaddingObfsMode" />
              </a-form-item>
              <template v-if="outbound.stream.xhttp.xPaddingObfsMode">
                <a-form-item label="Padding Key">
                  <a-input v-model:value="outbound.stream.xhttp.xPaddingKey" placeholder="x_padding" />
                </a-form-item>
                <a-form-item label="Padding Header">
                  <a-input v-model:value="outbound.stream.xhttp.xPaddingHeader" placeholder="X-Padding" />
                </a-form-item>
                <a-form-item label="Padding Placement">
                  <a-select v-model:value="outbound.stream.xhttp.xPaddingPlacement">
                    <a-select-option value="">Default (queryInHeader)</a-select-option>
                    <a-select-option value="queryInHeader">queryInHeader</a-select-option>
                    <a-select-option value="header">header</a-select-option>
                    <a-select-option value="cookie">cookie</a-select-option>
                    <a-select-option value="query">query</a-select-option>
                  </a-select>
                </a-form-item>
                <a-form-item label="Padding Method">
                  <a-select v-model:value="outbound.stream.xhttp.xPaddingMethod">
                    <a-select-option value="">Default (repeat-x)</a-select-option>
                    <a-select-option value="repeat-x">repeat-x</a-select-option>
                    <a-select-option value="tokenish">tokenish</a-select-option>
                  </a-select>
                </a-form-item>
              </template>

              <a-form-item label="Uplink HTTP Method">
                <a-select v-model:value="outbound.stream.xhttp.uplinkHTTPMethod">
                  <a-select-option value="">Default (POST)</a-select-option>
                  <a-select-option value="POST">POST</a-select-option>
                  <a-select-option value="PUT">PUT</a-select-option>
                  <a-select-option value="GET" :disabled="outbound.stream.xhttp.mode !== 'packet-up'">GET (packet-up
                    only)</a-select-option>
                </a-select>
              </a-form-item>

              <a-form-item label="Session Placement">
                <a-select v-model:value="outbound.stream.xhttp.sessionPlacement">
                  <a-select-option value="">Default (path)</a-select-option>
                  <a-select-option value="path">path</a-select-option>
                  <a-select-option value="header">header</a-select-option>
                  <a-select-option value="cookie">cookie</a-select-option>
                  <a-select-option value="query">query</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item
                v-if="outbound.stream.xhttp.sessionPlacement && outbound.stream.xhttp.sessionPlacement !== 'path'"
                label="Session Key">
                <a-input v-model:value="outbound.stream.xhttp.sessionKey" placeholder="x_session" />
              </a-form-item>

              <a-form-item label="Sequence Placement">
                <a-select v-model:value="outbound.stream.xhttp.seqPlacement">
                  <a-select-option value="">Default (path)</a-select-option>
                  <a-select-option value="path">path</a-select-option>
                  <a-select-option value="header">header</a-select-option>
                  <a-select-option value="cookie">cookie</a-select-option>
                  <a-select-option value="query">query</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item v-if="outbound.stream.xhttp.seqPlacement && outbound.stream.xhttp.seqPlacement !== 'path'"
                label="Sequence Key">
                <a-input v-model:value="outbound.stream.xhttp.seqKey" placeholder="x_seq" />
              </a-form-item>

              <a-form-item v-if="outbound.stream.xhttp.mode === 'packet-up'" label="Uplink Data Placement">
                <a-select v-model:value="outbound.stream.xhttp.uplinkDataPlacement">
                  <a-select-option value="">Default (body)</a-select-option>
                  <a-select-option value="body">body</a-select-option>
                  <a-select-option value="header">header</a-select-option>
                  <a-select-option value="cookie">cookie</a-select-option>
                  <a-select-option value="query">query</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item v-if="outbound.stream.xhttp.mode === 'packet-up'
                && outbound.stream.xhttp.uplinkDataPlacement
                && outbound.stream.xhttp.uplinkDataPlacement !== 'body'" label="Uplink Data Key">
                <a-input v-model:value="outbound.stream.xhttp.uplinkDataKey" placeholder="x_data" />
              </a-form-item>
              <a-form-item v-if="outbound.stream.xhttp.mode === 'packet-up'
                && outbound.stream.xhttp.uplinkDataPlacement
                && outbound.stream.xhttp.uplinkDataPlacement !== 'body'" label="Uplink Chunk Size">
                <a-input-number v-model:value="outbound.stream.xhttp.uplinkChunkSize" :min="0"
                  placeholder="0 (unlimited)" />
              </a-form-item>

              <a-form-item
                v-if="outbound.stream.xhttp.mode === 'stream-up' || outbound.stream.xhttp.mode === 'stream-one'"
                label="No gRPC Header">
                <a-switch v-model:checked="outbound.stream.xhttp.noGRPCHeader" />
              </a-form-item>

              <a-form-item label="XMUX">
                <a-switch v-model:checked="outbound.stream.xhttp.enableXmux" />
              </a-form-item>
              <template v-if="outbound.stream.xhttp.enableXmux">
                <a-form-item v-if="!outbound.stream.xhttp.xmux.maxConnections" label="Max Concurrency">
                  <a-input v-model:value="outbound.stream.xhttp.xmux.maxConcurrency" />
                </a-form-item>
                <a-form-item v-if="!outbound.stream.xhttp.xmux.maxConcurrency" label="Max Connections">
                  <a-input v-model:value="outbound.stream.xhttp.xmux.maxConnections" />
                </a-form-item>
                <a-form-item label="Max Reuse Times">
                  <a-input v-model:value="outbound.stream.xhttp.xmux.cMaxReuseTimes" />
                </a-form-item>
                <a-form-item label="Max Request Times">
                  <a-input v-model:value="outbound.stream.xhttp.xmux.hMaxRequestTimes" />
                </a-form-item>
                <a-form-item label="Max Reusable Secs">
                  <a-input v-model:value="outbound.stream.xhttp.xmux.hMaxReusableSecs" />
                </a-form-item>
                <a-form-item label="Keep Alive Period">
                  <a-input-number v-model:value="outbound.stream.xhttp.xmux.hKeepAlivePeriod" :min="0" />
                </a-form-item>
              </template>
            </template>

            <!-- Hysteria transport -->
            <template v-if="outbound.stream.network === 'hysteria'">
              <a-form-item label="Auth password">
                <a-input v-model:value="outbound.stream.hysteria.auth" />
              </a-form-item>
              <a-form-item label="Congestion">
                <a-select v-model:value="outbound.stream.hysteria.congestion">
                  <a-select-option value="">BBR (auto)</a-select-option>
                  <a-select-option value="brutal">Brutal</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Upload">
                <a-input v-model:value="outbound.stream.hysteria.up" placeholder="100 mbps" />
              </a-form-item>
              <a-form-item label="Download">
                <a-input v-model:value="outbound.stream.hysteria.down" placeholder="100 mbps" />
              </a-form-item>
              <a-form-item label="UDP hop port">
                <a-input v-model:value="outbound.stream.hysteria.udphopPort" placeholder="1145-1919" />
              </a-form-item>
              <a-form-item label="Max idle (s)">
                <a-input-number v-model:value="outbound.stream.hysteria.maxIdleTimeout" :min="4" :max="120" />
              </a-form-item>
              <a-form-item label="Keep alive (s)">
                <a-input-number v-model:value="outbound.stream.hysteria.keepAlivePeriod" :min="2" :max="60" />
              </a-form-item>
              <a-form-item label="Disable Path MTU">
                <a-switch v-model:checked="outbound.stream.hysteria.disablePathMTUDiscovery" />
              </a-form-item>
            </template>
          </template>

          <!-- ============== TLS / Reality ============== -->
          <template v-if="outbound.canEnableTls()">
            <a-form-item :label="t('security')">
              <a-radio-group v-model:value="outbound.stream.security" button-style="solid">
                <a-radio-button value="none">{{ t('none') }}</a-radio-button>
                <a-radio-button value="tls">TLS</a-radio-button>
                <a-radio-button v-if="outbound.canEnableReality()" value="reality">Reality</a-radio-button>
              </a-radio-group>
            </a-form-item>

            <template v-if="outbound.stream.isTls">
              <a-form-item label="SNI">
                <a-input v-model:value="outbound.stream.tls.serverName" placeholder="server name" />
              </a-form-item>
              <a-form-item label="uTLS">
                <a-select v-model:value="outbound.stream.tls.fingerprint">
                  <a-select-option value="">{{ t('none') }}</a-select-option>
                  <a-select-option v-for="key in UTLS_OPTIONS" :key="key" :value="key">{{ key }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="ALPN">
                <a-select v-model:value="outbound.stream.tls.alpn" mode="multiple">
                  <a-select-option v-for="alpn in ALPN_OPTIONS" :key="alpn" :value="alpn">{{ alpn }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="ECH">
                <a-input v-model:value="outbound.stream.tls.echConfigList" />
              </a-form-item>
              <a-form-item label="Verify peer name">
                <a-input v-model:value="outbound.stream.tls.verifyPeerCertByName" placeholder="cloudflare-dns.com" />
              </a-form-item>
              <a-form-item label="Pinned SHA256">
                <a-input v-model:value="outbound.stream.tls.pinnedPeerCertSha256" placeholder="base64 SHA256" />
              </a-form-item>
            </template>

            <template v-if="outbound.stream.isReality">
              <a-form-item label="SNI">
                <a-input v-model:value="outbound.stream.reality.serverName" />
              </a-form-item>
              <a-form-item label="uTLS">
                <a-select v-model:value="outbound.stream.reality.fingerprint">
                  <a-select-option v-for="key in UTLS_OPTIONS" :key="key" :value="key">{{ key }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Short ID">
                <a-input v-model:value="outbound.stream.reality.shortId" />
              </a-form-item>
              <a-form-item label="SpiderX">
                <a-input v-model:value="outbound.stream.reality.spiderX" />
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.publicKey')">
                <a-textarea v-model:value="outbound.stream.reality.publicKey" :auto-size="{ minRows: 2 }" />
              </a-form-item>
              <a-form-item label="mldsa65 verify">
                <a-textarea v-model:value="outbound.stream.reality.mldsa65Verify" :auto-size="{ minRows: 2 }" />
              </a-form-item>
            </template>
          </template>

          <!-- ============== sockopt ============== -->
          <template v-if="outbound.stream">
            <a-form-item label="Sockopts">
              <a-switch v-model:checked="outbound.stream.sockoptSwitch" />
            </a-form-item>
            <template v-if="outbound.stream.sockoptSwitch">
              <a-form-item label="Dialer proxy">
                <a-input v-model:value="outbound.stream.sockopt.dialerProxy" />
              </a-form-item>
              <a-form-item label="Address+Port strategy">
                <a-select v-model:value="outbound.stream.sockopt.addressPortStrategy">
                  <a-select-option v-for="key in Object.values(Address_Port_Strategy)" :key="key" :value="key">
                    {{ key }}
                  </a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Keep alive interval">
                <a-input-number v-model:value="outbound.stream.sockopt.tcpKeepAliveInterval" :min="0" />
              </a-form-item>
              <a-form-item label="TCP Fast Open">
                <a-switch v-model:checked="outbound.stream.sockopt.tcpFastOpen" />
              </a-form-item>
              <a-form-item label="Multipath TCP">
                <a-switch v-model:checked="outbound.stream.sockopt.tcpMptcp" />
              </a-form-item>
              <a-form-item label="Penetrate">
                <a-switch v-model:checked="outbound.stream.sockopt.penetrate" />
              </a-form-item>
            </template>
          </template>

          <!-- ============== Mux ============== -->
          <template v-if="outbound.canEnableMux()">
            <a-form-item :label="t('pages.settings.mux')">
              <a-switch v-model:checked="outbound.mux.enabled" />
            </a-form-item>
            <template v-if="outbound.mux.enabled">
              <a-form-item label="Concurrency">
                <a-input-number v-model:value="outbound.mux.concurrency" :min="-1" :max="1024" />
              </a-form-item>
              <a-form-item label="xudp concurrency">
                <a-input-number v-model:value="outbound.mux.xudpConcurrency" :min="-1" :max="1024" />
              </a-form-item>
              <a-form-item label="xudp UDP 443">
                <a-select v-model:value="outbound.mux.xudpProxyUDP443">
                  <a-select-option v-for="x in ['reject', 'allow', 'skip']" :key="x" :value="x">{{ x
                  }}</a-select-option>
                </a-select>
              </a-form-item>
            </template>
          </template>
        </a-form>

        <!-- ============== FinalMask (TCP/UDP masks + QUIC params) ============== -->
        <!-- Gated by canEnableStream() so TCP masks don't leak into
             Freedom / Blackhole / DNS / Socks / HTTP / Wireguard outbounds
             (they don't have a stream config at all). Matches legacy. -->
        <FinalMaskForm v-if="outbound.stream && outbound.canEnableStream()" :stream="outbound.stream"
          :protocol="proto" />
      </a-tab-pane>

      <!-- ============================== JSON ============================== -->
      <a-tab-pane key="2" tab="JSON">
        <a-space direction="vertical" :size="10" :style="{ width: '100%', marginTop: '10px' }">
          <a-input-search v-model:value="linkInput" placeholder="vmess:// vless:// trojan:// ss:// hysteria2://"
            @search="convertLink">
            <template #enterButton>
              <a-button>Convert</a-button>
            </template>
          </a-input-search>
          <a-textarea v-model:value="advancedJson" :auto-size="{ minRows: 14, maxRows: 30 }" spellcheck="false"
            class="json-editor" />
        </a-space>
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<style scoped>
.random-icon {
  cursor: pointer;
  color: var(--ant-primary-color, #1890ff);
  margin-left: 4px;
}

.danger-icon {
  cursor: pointer;
  color: #ff4d4f;
  margin-left: 8px;
}

.ml-8 {
  margin-left: 8px;
}

.mb-8 {
  margin-bottom: 8px;
}

.section-heading {
  font-weight: 500;
  margin: 12px 0 6px;
  opacity: 0.85;
}

.item-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-weight: 500;
  opacity: 0.85;
}

.json-editor {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

/* AD-Vue 4 renders a-checkbox children inside a-checkbox-group as
 * inline-block, but inside a narrow form wrapper they can wrap
 * inconsistently. Force a clean horizontal row with even gaps. */
.sniffing-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
}

.sniffing-options :deep(.ant-checkbox-wrapper) {
  margin-inline-start: 0;
}
</style>
