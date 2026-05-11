<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { CopyOutlined, SyncOutlined, DeleteOutlined, DownloadOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import {
  HttpUtil,
  IntlUtil,
  SizeFormatter,
  ColorUtils,
  ClipboardManager,
  FileManager,
} from '@/utils';
import { Protocols } from '@/models/inbound.js';
import InfinityIcon from '@/components/InfinityIcon.vue';
import { useDatepicker } from '@/composables/useDatepicker.js';

const { t } = useI18n();
const { datepicker } = useDatepicker();

// One modal handles every protocol's info / share view because the
// legacy template did the same. The big v-if forks at the top decide
// which sub-block of the body renders:
//   • multi-user inbound (VMess/VLess/Trojan/SS-multi/Hysteria) → per-
//     client row + share links
//   • SS single-user → connection details + share link
//   • WireGuard → secret/peers + per-peer config download
//   • Mixed/HTTP/Tunnel → connection details only
//
// We display links via QrPanel — each link gets its own QR + copy +
// (for WireGuard configs) download button.

const props = defineProps({
  open: { type: Boolean, default: false },
  // Result of inbounds-page checkFallback() so the link-gen sees the
  // root inbound's listen/port/security when the dbInbound is a
  // domain-socket fallback (`@<name>`).
  dbInbound: { type: Object, default: null },
  // Index into inbound.clients to focus on for multi-user inbounds.
  clientIndex: { type: Number, default: 0 },
  // Sidecar config the legacy panel keyed off `app.*`.
  remarkModel: { type: String, default: '-ieo' },
  expireDiff: { type: Number, default: 0 },
  trafficDiff: { type: Number, default: 0 },
  ipLimitEnable: { type: Boolean, default: false },
  tgBotEnable: { type: Boolean, default: false },
  // Address of the node hosting this inbound; '' for local. Wired
  // through to share/QR link generation so node-managed inbounds
  // produce links that connect to the node, not the central panel.
  nodeAddress: { type: String, default: '' },
  subSettings: {
    type: Object,
    default: () => ({ enable: false, subURI: '', subJsonURI: '', subJsonEnable: false }),
  },
  // Email -> ts (last-online unix-ms) map fetched at the page level.
  lastOnlineMap: { type: Object, default: () => ({}) },
});

const emit = defineEmits(['update:open']);

// Cloned state on open so cancel doesn't leak edits onto the row's
// parsed-cache copy. The local ref intentionally shadows the prop —
// templates read this ref's frozen-on-open value, not props.dbInbound.
// eslint-disable-next-line vue/no-dupe-keys
const dbInbound = ref(null);
const inbound = ref(null);
const clientSettings = ref(null);
const clientStats = ref(null);

const links = ref([]); // generic share links (for VMess/VLess/Trojan/SS/Hysteria)
const wireguardConfigs = ref([]); // multi-line .conf bodies (one per peer)
const wireguardLinks = ref([]); // wg:// share URIs (one per peer)

const subLink = ref('');
const subJsonLink = ref('');

// IP-log state (matches the legacy refresh / clear flow).
const refreshing = ref(false);
const clientIpsArray = ref([]);
const clientIpsText = ref('');

// === Status flags shown as tags ====================================
const isEnable = computed(() => {
  if (clientSettings.value) return !!clientSettings.value.enable;
  return dbInbound.value?.enable ?? true;
});

const isDepleted = computed(() => {
  const stats = clientStats.value;
  const settings = clientSettings.value;
  if (!stats || !settings) return false;
  const total = stats.total ?? 0;
  const used = (stats.up ?? 0) + (stats.down ?? 0);
  if (total > 0 && used >= total) return true;
  const expiry = settings.expiryTime ?? 0;
  if (expiry > 0 && Date.now() >= expiry) return true;
  return false;
});

function statsColor(stats) {
  return ColorUtils.usageColor(stats.up + stats.down, props.trafficDiff, stats.total);
}

function getRemainingStats() {
  if (!clientStats.value || !clientSettings.value) return '-';
  const remained = clientStats.value.total - clientStats.value.up - clientStats.value.down;
  return remained > 0 ? SizeFormatter.sizeFormat(remained) : '-';
}

function formatLastOnline(email) {
  const ts = props.lastOnlineMap[email];
  if (!ts) return '-';
  return IntlUtil.formatDate(ts, datepicker.value);
}

// === IP log ========================================================
function formatIpInfo(record) {
  if (record == null) return '';
  if (typeof record === 'string' || typeof record === 'number') return String(record);
  const ip = record.ip || record.IP || '';
  const ts = record.timestamp || record.Timestamp || 0;
  if (!ip) return String(record);
  if (!ts) return String(ip);
  const date = new Date(Number(ts) * 1000);
  const timeStr = date
    .toLocaleString('en-GB', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    })
    .replace(',', '');
  return `${ip} (${timeStr})`;
}

async function loadClientIps() {
  if (!clientStats.value?.email) return;
  refreshing.value = true;
  try {
    const msg = await HttpUtil.post(`/panel/api/inbounds/clientIps/${clientStats.value.email}`);
    if (!msg?.success) {
      clientIpsText.value = msg?.obj || 'No IP record';
      clientIpsArray.value = [];
      return;
    }
    let ips = msg.obj;
    if (typeof ips === 'string') {
      try { ips = JSON.parse(ips); }
      catch (_e) { clientIpsText.value = String(ips); clientIpsArray.value = [String(ips)]; return; }
    }
    if (ips && !Array.isArray(ips) && typeof ips === 'object') ips = [ips];
    if (Array.isArray(ips) && ips.length > 0) {
      const arr = ips.map(formatIpInfo).filter(Boolean);
      clientIpsArray.value = arr;
      clientIpsText.value = arr.join(' | ');
    } else {
      clientIpsArray.value = [];
      clientIpsText.value = String(ips || t('tgbot.noIpRecord'));
    }
  } finally {
    refreshing.value = false;
  }
}

async function clearClientIps() {
  if (!clientStats.value?.email) return;
  const msg = await HttpUtil.post(`/panel/api/inbounds/clearClientIps/${clientStats.value.email}`);
  if (msg?.success) {
    clientIpsArray.value = [];
    clientIpsText.value = t('tgbot.noIpRecord');
  }
}

async function copyText(value) {
  const ok = await ClipboardManager.copyText(String(value ?? ''));
  if (ok) message.success(t('copied'));
}

function downloadText(content, filename) {
  FileManager.downloadTextFile(content, filename);
}

const activeTab = ref('client');

// === Build state on open ===========================================
function genSubLink(subId) {
  return (props.subSettings.subURI || '') + subId;
}
function genSubJsonLink(subId) {
  return (props.subSettings.subJsonURI || '') + subId;
}

watch(() => props.open, (next) => {
  if (!next) return;
  if (!props.dbInbound) return;

  activeTab.value = props.dbInbound.toInbound().clients?.length ? 'client' : 'inbound';
  dbInbound.value = props.dbInbound;
  inbound.value = props.dbInbound.toInbound();

  const idx = props.clientIndex ?? 0;
  if (inbound.value.clients?.length) {
    clientSettings.value = inbound.value.clients[idx] || null;
  } else {
    clientSettings.value = null;
  }
  clientStats.value = clientSettings.value
    ? (props.dbInbound.clientStats || []).find((s) => s.email === clientSettings.value.email) || null
    : null;

  // Generate links per protocol — WireGuard has its own .conf body
  // path; everything else flows through genAllLinks.
  if (inbound.value.protocol === Protocols.WIREGUARD) {
    wireguardConfigs.value = inbound.value.genWireguardConfigs(props.dbInbound.remark, '-ieo', props.nodeAddress).split('\r\n');
    wireguardLinks.value = inbound.value.genWireguardLinks(props.dbInbound.remark, '-ieo', props.nodeAddress).split('\r\n');
    links.value = [];
  } else {
    links.value = inbound.value.genAllLinks(
      props.dbInbound.remark,
      props.remarkModel,
      clientSettings.value,
      props.nodeAddress,
    );
    wireguardConfigs.value = [];
    wireguardLinks.value = [];
  }

  // Subscription link is per-client because each client has its own subId.
  if (clientSettings.value?.subId) {
    subLink.value = genSubLink(clientSettings.value.subId);
    subJsonLink.value = props.subSettings.subJsonEnable
      ? genSubJsonLink(clientSettings.value.subId)
      : '';
  } else {
    subLink.value = '';
    subJsonLink.value = '';
  }

  // Auto-load IP log if it'll be visible.
  clientIpsArray.value = [];
  clientIpsText.value = '';
  if (
    props.ipLimitEnable
    && clientSettings.value?.limitIp > 0
    && clientStats.value?.email
  ) {
    loadClientIps();
  }
});

function close() {
  emit('update:open', false);
}

// === Convenience displays ===========================================
const networkLabel = computed(() => inbound.value?.stream?.network || '');
const securityLabel = computed(() => inbound.value?.stream?.security || 'none');
const securityColor = computed(() => (securityLabel.value === 'none' ? 'red' : 'green'));
const encryptionLabel = computed(() => inbound.value?.settings?.encryption || '');
const serverNameLabel = computed(() => inbound.value?.serverName || '');

// === Tab visibility =================================================
const showClientTab = computed(() => !!clientSettings.value);
const showSubscriptionTab = computed(
  () => !!(props.subSettings.enable && clientSettings.value?.subId),
);
</script>

<template>
  <a-modal :open="open" :title="t('pages.inbounds.inboundData')" :footer="null" width="640px" @cancel="close">
    <template v-if="dbInbound && inbound">
      <a-tabs v-model:active-key="activeTab">
        <!-- ============================================================
             TAB 1 — Client: per-client info + share links + subscription
             (subscription is folded in here so users don't need a third
             tab — the sub URLs are per-client anyway).
        ============================================================== -->
        <a-tab-pane v-if="showClientTab" key="client" :tab="t('pages.inbounds.client')">
          <table class="info-table block">
            <tbody>
              <tr>
                <td>{{ t('pages.inbounds.email') }}</td>
                <td>
                  <a-tag v-if="clientSettings.email" color="green">{{ clientSettings.email }}</a-tag>
                  <a-tag v-else color="red">{{ t('none') }}</a-tag>
                </td>
              </tr>
              <tr v-if="clientSettings.id">
                <td>ID</td>
                <td><a-tag>{{ clientSettings.id }}</a-tag></td>
              </tr>
              <tr v-if="dbInbound.isVMess">
                <td>{{ t('security') }}</td>
                <td><a-tag>{{ clientSettings.security }}</a-tag></td>
              </tr>
              <tr v-if="inbound.canEnableTlsFlow()">
                <td>Flow</td>
                <td>
                  <a-tag v-if="clientSettings.flow">{{ clientSettings.flow }}</a-tag>
                  <a-tag v-else color="orange">{{ t('none') }}</a-tag>
                </td>
              </tr>
              <tr v-if="clientSettings.password">
                <td>{{ t('password') }}</td>
                <td><a-tag class="info-large-tag">{{ clientSettings.password }}</a-tag></td>
              </tr>
              <tr>
                <td>{{ t('status') }}</td>
                <td>
                  <a-tag v-if="isDepleted" color="red">{{ t('depleted') }}</a-tag>
                  <a-tag v-else-if="isEnable" color="green">{{ t('enabled') }}</a-tag>
                  <a-tag v-else>{{ t('disabled') }}</a-tag>
                </td>
              </tr>
              <tr v-if="clientStats">
                <td>{{ t('usage') }}</td>
                <td>
                  <a-tag color="green">
                    {{ SizeFormatter.sizeFormat(clientStats.up + clientStats.down) }}
                  </a-tag>
                  <a-tag>
                    ↑ {{ SizeFormatter.sizeFormat(clientStats.up) }} /
                    {{ SizeFormatter.sizeFormat(clientStats.down) }} ↓
                  </a-tag>
                </td>
              </tr>
              <tr>
                <td>{{ t('pages.inbounds.createdAt') }}</td>
                <td>
                  <a-tag v-if="clientSettings.created_at">{{ IntlUtil.formatDate(clientSettings.created_at, datepicker)
                  }}</a-tag>
                  <a-tag v-else>-</a-tag>
                </td>
              </tr>
              <tr>
                <td>{{ t('pages.inbounds.updatedAt') }}</td>
                <td>
                  <a-tag v-if="clientSettings.updated_at">{{ IntlUtil.formatDate(clientSettings.updated_at, datepicker)
                  }}</a-tag>
                  <a-tag v-else>-</a-tag>
                </td>
              </tr>
              <tr>
                <td>{{ t('lastOnline') }}</td>
                <td><a-tag>{{ formatLastOnline(clientSettings.email || '') }}</a-tag></td>
              </tr>
              <tr v-if="clientSettings.comment">
                <td>{{ t('comment') }}</td>
                <td><a-tag class="info-large-tag">{{ clientSettings.comment }}</a-tag></td>
              </tr>
              <tr v-if="ipLimitEnable">
                <td>{{ t('pages.inbounds.IPLimit') }}</td>
                <td><a-tag>{{ clientSettings.limitIp }}</a-tag></td>
              </tr>
              <tr v-if="ipLimitEnable && clientSettings.limitIp > 0">
                <td>{{ t('pages.inbounds.IPLimitlog') }}</td>
                <td>
                  <div class="ip-log">
                    <div v-if="clientIpsArray.length > 0">
                      <a-tag v-for="(item, idx) in clientIpsArray" :key="idx" color="blue" class="ip-log-row">{{ item
                      }}</a-tag>
                    </div>
                    <a-tag v-else>{{ clientIpsText || t('tgbot.noIpRecord') }}</a-tag>
                  </div>
                  <div class="ip-log-actions">
                    <SyncOutlined :spin="refreshing" @click="loadClientIps" />
                    <a-tooltip :title="t('pages.inbounds.IPLimitlogclear')">
                      <DeleteOutlined @click="clearClientIps" />
                    </a-tooltip>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>

          <!-- Remaining / total / expiry -->
          <table class="info-table summary-table">
            <thead>
              <tr>
                <th>{{ t('remained') }}</th>
                <th>{{ t('pages.inbounds.totalUsage') }}</th>
                <th>{{ t('pages.inbounds.expireDate') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <a-tag v-if="clientStats && clientSettings.totalGB > 0" :color="statsColor(clientStats)">{{
                    getRemainingStats() }}</a-tag>
                  <a-tag v-else-if="!clientSettings.totalGB || clientSettings.totalGB <= 0" color="purple">
                    <InfinityIcon />
                  </a-tag>
                </td>
                <td>
                  <a-tag v-if="clientSettings.totalGB > 0" :color="clientStats ? statsColor(clientStats) : 'default'">{{
                    SizeFormatter.sizeFormat(clientSettings.totalGB) }}</a-tag>
                  <a-tag v-else color="purple">
                    <InfinityIcon />
                  </a-tag>
                </td>
                <td>
                  <a-tag v-if="clientSettings.expiryTime > 0"
                    :color="ColorUtils.usageColor(Date.now(), expireDiff, clientSettings.expiryTime)">{{
                      IntlUtil.formatDate(clientSettings.expiryTime, datepicker) }}</a-tag>
                  <a-tag v-else-if="clientSettings.expiryTime < 0" color="green">
                    {{ clientSettings.expiryTime / -86400000 }} {{ t('day') }}
                  </a-tag>
                  <a-tag v-else color="purple">
                    <InfinityIcon />
                  </a-tag>
                </td>
              </tr>
            </tbody>
          </table>

          <!-- Telegram chat id -->
          <template v-if="tgBotEnable && clientSettings.tgId">
            <a-divider>Telegram</a-divider>
            <div class="tg-row">
              <a-tag color="blue">{{ clientSettings.tgId }}</a-tag>
              <a-tooltip :title="t('copy')">
                <a-button size="small" @click="copyText(clientSettings.tgId)">
                  <template #icon>
                    <CopyOutlined />
                  </template>
                </a-button>
              </a-tooltip>
            </div>
          </template>

          <!-- Per-client share links (no QR) -->
          <template v-if="dbInbound.hasLink() && links.length > 0">
            <a-divider>{{ t('pages.inbounds.copyLink') }}</a-divider>
            <div v-for="(link, idx) in links" :key="idx" class="link-panel">
              <div class="link-panel-header">
                <a-tag color="green">{{ link.remark || `Link ${idx + 1}` }}</a-tag>
                <a-tooltip :title="t('copy')">
                  <a-button size="small" @click="copyText(link.link)">
                    <template #icon>
                      <CopyOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </div>
              <code class="link-panel-text">{{ link.link }}</code>
            </div>
          </template>

          <!-- Subscription URLs — folded into the client tab so they sit
               with the rest of the per-client data. Only visible when
               subscriptions are enabled and this client has a subId. -->
          <template v-if="showSubscriptionTab">
            <a-divider>{{ t('subscription.title') }}</a-divider>
            <div class="link-panel">
              <div class="link-panel-header">
                <a-tag color="green">{{ t('subscription.title') }}</a-tag>
                <a-tooltip :title="t('copy')">
                  <a-button size="small" @click="copyText(subLink)">
                    <template #icon>
                      <CopyOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </div>
              <a :href="subLink" target="_blank" rel="noopener noreferrer" class="link-panel-anchor">{{ subLink }}</a>
            </div>

            <div v-if="subSettings.subJsonEnable && subJsonLink" class="link-panel">
              <div class="link-panel-header">
                <a-tag color="green">JSON</a-tag>
                <a-tooltip :title="t('copy')">
                  <a-button size="small" @click="copyText(subJsonLink)">
                    <template #icon>
                      <CopyOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </div>
              <a :href="subJsonLink" target="_blank" rel="noopener noreferrer" class="link-panel-anchor">{{ subJsonLink
              }}</a>
            </div>
          </template>
        </a-tab-pane>

        <!-- ============================================================
             TAB 2 — Inbound: protocol, transport, security, per-protocol
        ============================================================== -->
        <a-tab-pane key="inbound" :tab="t('pages.xray.rules.inbound')">
          <dl class="info-list">
            <div class="info-row">
              <dt>{{ t('pages.inbounds.protocol') }}</dt>
              <dd><a-tag color="purple">{{ dbInbound.protocol }}</a-tag></dd>
            </div>
            <div class="info-row">
              <dt>{{ t('pages.inbounds.address') }}</dt>
              <dd><a-tag class="value-tag">{{ dbInbound.address }}</a-tag></dd>
            </div>
            <div class="info-row">
              <dt>{{ t('pages.inbounds.port') }}</dt>
              <dd><a-tag>{{ dbInbound.port }}</a-tag></dd>
            </div>

            <template v-if="dbInbound.isVMess || dbInbound.isVLess || dbInbound.isTrojan || dbInbound.isSS">
              <div class="info-row">
                <dt>{{ t('transmission') }}</dt>
                <dd><a-tag color="green">{{ networkLabel }}</a-tag></dd>
              </div>
              <template v-if="inbound.isTcp || inbound.isWs || inbound.isHttpupgrade || inbound.isXHTTP">
                <div class="info-row">
                  <dt>{{ t('host') }}</dt>
                  <dd>
                    <a-tag v-if="inbound.host" class="value-tag">{{ inbound.host }}</a-tag>
                    <a-tag v-else color="orange">{{ t('none') }}</a-tag>
                  </dd>
                </div>
                <div class="info-row">
                  <dt>{{ t('path') }}</dt>
                  <dd>
                    <a-tag v-if="inbound.path" class="value-tag">{{ inbound.path }}</a-tag>
                    <a-tag v-else color="orange">{{ t('none') }}</a-tag>
                  </dd>
                </div>
              </template>
              <template v-if="inbound.isXHTTP">
                <div class="info-row">
                  <dt>Mode</dt>
                  <dd><a-tag>{{ inbound.stream.xhttp.mode }}</a-tag></dd>
                </div>
              </template>
              <template v-if="inbound.isGrpc">
                <div class="info-row">
                  <dt>grpc serviceName</dt>
                  <dd><a-tag class="value-tag">{{ inbound.serviceName }}</a-tag></dd>
                </div>
                <div class="info-row">
                  <dt>grpc multiMode</dt>
                  <dd><a-tag>{{ inbound.stream.grpc.multiMode }}</a-tag></dd>
                </div>
              </template>
            </template>

            <template v-if="dbInbound.hasLink()">
              <div class="info-row">
                <dt>{{ t('security') }}</dt>
                <dd><a-tag :color="securityColor">{{ securityLabel }}</a-tag></dd>
              </div>
              <div v-if="encryptionLabel" class="info-row">
                <dt>{{ t('encryption') }}</dt>
                <dd class="value-block">
                  <code class="value-code">{{ encryptionLabel }}</code>
                  <a-tooltip :title="t('copy')">
                    <a-button size="small" class="value-copy" @click="copyText(encryptionLabel)">
                      <template #icon>
                        <CopyOutlined />
                      </template>
                    </a-button>
                  </a-tooltip>
                </dd>
              </div>
              <div v-if="securityLabel !== 'none'" class="info-row">
                <dt>{{ t('domainName') }}</dt>
                <dd>
                  <a-tag v-if="serverNameLabel" color="green" class="value-tag">{{ serverNameLabel }}</a-tag>
                  <a-tag v-else color="orange">{{ t('none') }}</a-tag>
                </dd>
              </div>
            </template>
          </dl>

          <!-- Shadowsocks single-user details -->
          <table v-if="dbInbound.isSS" class="info-table block">
            <tbody>
              <tr>
                <td>{{ t('encryption') }}</td>
                <td><a-tag color="green">{{ inbound.settings.method }}</a-tag></td>
              </tr>
              <tr v-if="inbound.isSS2022">
                <td>{{ t('password') }}</td>
                <td><a-tag class="info-large-tag">{{ inbound.settings.password }}</a-tag></td>
              </tr>
              <tr>
                <td>{{ t('pages.inbounds.network') }}</td>
                <td><a-tag color="green">{{ inbound.settings.network }}</a-tag></td>
              </tr>
            </tbody>
          </table>

          <!-- Tunnel -->
          <dl v-if="inbound.protocol === Protocols.TUNNEL" class="info-list info-list-block">
            <div class="info-row">
              <dt>{{ t('pages.inbounds.targetAddress') }}</dt>
              <dd><a-tag color="green" class="value-tag">{{ inbound.settings.address }}</a-tag></dd>
            </div>
            <div class="info-row">
              <dt>{{ t('pages.inbounds.destinationPort') }}</dt>
              <dd><a-tag color="green">{{ inbound.settings.port }}</a-tag></dd>
            </div>
            <div class="info-row">
              <dt>{{ t('pages.inbounds.network') }}</dt>
              <dd><a-tag color="green">{{ inbound.settings.network }}</a-tag></dd>
            </div>
            <div class="info-row">
              <dt>FollowRedirect</dt>
              <dd>
                <a-tag :color="inbound.settings.followRedirect ? 'green' : 'red'">
                  {{ inbound.settings.followRedirect ? t('enabled') : t('disabled') }}
                </a-tag>
              </dd>
            </div>
          </dl>

          <!-- Mixed -->
          <dl v-if="dbInbound.isMixed" class="info-list info-list-block">
            <div class="info-row">
              <dt>Auth</dt>
              <dd>
                <a-tag :color="inbound.settings.auth === 'password' ? 'green' : 'orange'">
                  {{ inbound.settings.auth }}
                </a-tag>
              </dd>
            </div>
            <div class="info-row">
              <dt>UDP</dt>
              <dd>
                <a-tag :color="inbound.settings.udp ? 'green' : 'red'">
                  {{ inbound.settings.udp ? t('enabled') : t('disabled') }}
                </a-tag>
              </dd>
            </div>
            <div v-if="inbound.settings.ip" class="info-row">
              <dt>IP</dt>
              <dd><a-tag class="value-tag">{{ inbound.settings.ip }}</a-tag></dd>
            </div>
            <template v-if="inbound.settings.auth === 'password' && inbound.settings.accounts?.length">
              <div v-for="(account, idx) in inbound.settings.accounts" :key="idx" class="info-row">
                <dt>{{ t('username') }} #{{ idx + 1 }}</dt>
                <dd class="account-row">
                  <a-tag color="green" class="value-tag">{{ account.user }}</a-tag>
                  <span class="account-sep">:</span>
                  <a-tag class="value-tag">{{ account.pass }}</a-tag>
                  <a-tooltip :title="t('copy')">
                    <a-button size="small" @click="copyText(`${account.user}:${account.pass}`)">
                      <template #icon>
                        <CopyOutlined />
                      </template>
                    </a-button>
                  </a-tooltip>
                </dd>
              </div>
            </template>
          </dl>

          <!-- HTTP accounts -->
          <dl v-if="dbInbound.isHTTP && inbound.settings.accounts?.length" class="info-list info-list-block">
            <div v-for="(account, idx) in inbound.settings.accounts" :key="idx" class="info-row">
              <dt>{{ t('username') }} #{{ idx + 1 }}</dt>
              <dd class="account-row">
                <a-tag color="green" class="value-tag">{{ account.user }}</a-tag>
                <span class="account-sep">:</span>
                <a-tag class="value-tag">{{ account.pass }}</a-tag>
                <a-tooltip :title="t('copy')">
                  <a-button size="small" @click="copyText(`${account.user}:${account.pass}`)">
                    <template #icon>
                      <CopyOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </dd>
            </div>
          </dl>

          <!-- WireGuard server config + peers -->
          <table v-if="dbInbound.isWireguard" class="info-table protocol-table wg-table">
            <tbody>
              <tr>
                <td>Secret key</td>
                <td>{{ inbound.settings.secretKey }}</td>
              </tr>
              <tr>
                <td>Public key</td>
                <td>{{ inbound.settings.pubKey }}</td>
              </tr>
              <tr>
                <td>MTU</td>
                <td>{{ inbound.settings.mtu }}</td>
              </tr>
              <tr>
                <td>No-kernel TUN</td>
                <td>{{ inbound.settings.noKernelTun }}</td>
              </tr>
              <template v-for="(peer, idx) in inbound.settings.peers" :key="idx">
                <tr>
                  <td colspan="2"><a-divider>Peer {{ idx + 1 }}</a-divider></td>
                </tr>
                <tr>
                  <td>Secret key</td>
                  <td>{{ peer.privateKey }}</td>
                </tr>
                <tr>
                  <td>Public key</td>
                  <td>{{ peer.publicKey }}</td>
                </tr>
                <tr>
                  <td>PSK</td>
                  <td>{{ peer.psk }}</td>
                </tr>
                <tr>
                  <td>Allowed IPs</td>
                  <td>{{ (peer.allowedIPs || []).join(',') }}</td>
                </tr>
                <tr>
                  <td>Keep alive</td>
                  <td>{{ peer.keepAlive }}</td>
                </tr>
                <tr v-if="wireguardConfigs[idx]">
                  <td colspan="2">
                    <div class="link-panel">
                      <div class="link-panel-header">
                        <a-tag color="green">Peer {{ idx + 1 }} config</a-tag>
                        <a-tooltip :title="t('copy')">
                          <a-button size="small" @click="copyText(wireguardConfigs[idx])">
                            <template #icon>
                              <CopyOutlined />
                            </template>
                          </a-button>
                        </a-tooltip>
                        <a-tooltip :title="t('download')">
                          <a-button size="small" @click="downloadText(wireguardConfigs[idx], `peer-${idx + 1}.conf`)">
                            <template #icon>
                              <DownloadOutlined />
                            </template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <code class="link-panel-text">{{ wireguardConfigs[idx] }}</code>
                    </div>
                  </td>
                </tr>
                <tr v-if="wireguardLinks[idx]">
                  <td colspan="2">
                    <div class="link-panel">
                      <div class="link-panel-header">
                        <a-tag color="green">Peer {{ idx + 1 }} link</a-tag>
                        <a-tooltip :title="t('copy')">
                          <a-button size="small" @click="copyText(wireguardLinks[idx])">
                            <template #icon>
                              <CopyOutlined />
                            </template>
                          </a-button>
                        </a-tooltip>
                      </div>
                      <code class="link-panel-text">{{ wireguardLinks[idx] }}</code>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>

          <!-- Single-user SS share link (no QR) -->
          <template v-if="dbInbound.isSS && !inbound.isSSMultiUser && links.length > 0">
            <a-divider>{{ t('pages.inbounds.copyLink') }}</a-divider>
            <div v-for="(link, idx) in links" :key="idx" class="link-panel">
              <div class="link-panel-header">
                <a-tag color="green">{{ link.remark || `Link ${idx + 1}` }}</a-tag>
                <a-tooltip :title="t('copy')">
                  <a-button size="small" @click="copyText(link.link)">
                    <template #icon>
                      <CopyOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </div>
              <code class="link-panel-text">{{ link.link }}</code>
            </div>
          </template>
        </a-tab-pane>
      </a-tabs>
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

.info-table td,
.info-table th {
  padding: 4px 8px;
  vertical-align: top;
}

.info-table th {
  text-align: center;
  font-weight: 500;
}

.info-large-tag {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
}

/* Stacked label/value list — one row per field. Long values wrap
 * (or fall through to a code block) so they never blow out the modal. */
.info-list {
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.info-row {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  padding: 6px 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.12);
}

.info-row:last-child {
  border-bottom: none;
}

/* When info-list is rendered as a second block (e.g. protocol details
 * after the top transport/security block), give it a small top spacing
 * so the two groups read as separate. */
.info-list-block {
  margin-top: 10px;
}

.account-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.account-sep {
  opacity: 0.55;
  font-weight: 600;
}

.info-row dt {
  margin: 0;
  font-size: 13px;
  opacity: 0.75;
}

.info-row dd {
  margin: 0;
  min-width: 0;
}

.value-tag {
  max-width: 100%;
  white-space: normal;
  word-break: break-all;
  display: inline-block;
}

.value-block {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  min-width: 0;
}

.value-code {
  flex: 1;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  word-break: break-all;
  white-space: pre-wrap;
  padding: 4px 8px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 4px;
  user-select: all;
  min-width: 0;
}

:global(body.dark) .value-code {
  background: rgba(255, 255, 255, 0.05);
}

.value-copy {
  flex-shrink: 0;
}

.security-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin: 8px 0;
}

.security-line span {
  font-size: 13px;
  opacity: 0.75;
}

.summary-table {
  width: 100%;
  text-align: center;
  margin: 10px 0;
}

.tg-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ip-log {
  max-height: 150px;
  overflow-y: auto;
  text-align: left;
}

.ip-log-row {
  display: block;
  margin: 2px 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
}

.ip-log-actions {
  display: flex;
  gap: 12px;
  margin-top: 5px;
  font-size: 16px;
  cursor: pointer;
}

.protocol-table {
  margin-top: 10px;
}

.wg-table td {
  word-break: break-all;
}

/* Reusable copy/link panel that replaces QrPanel for the no-QR design. */
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
