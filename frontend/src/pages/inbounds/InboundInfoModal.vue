<script setup>
import { computed, ref, watch } from 'vue';
import { CopyOutlined, SyncOutlined, DeleteOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

import {
  HttpUtil,
  IntlUtil,
  SizeFormatter,
  ColorUtils,
  ClipboardManager,
} from '@/utils';
import { Inbound, Protocols } from '@/models/inbound.js';
import QrPanel from './QrPanel.vue';

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
  subSettings: {
    type: Object,
    default: () => ({ enable: false, subURI: '', subJsonURI: '', subJsonEnable: false }),
  },
  // Email -> ts (last-online unix-ms) map fetched at the page level.
  lastOnlineMap: { type: Object, default: () => ({}) },
});

const emit = defineEmits(['update:open']);

// Cloned state on open so cancel doesn't leak edits onto the row's
// parsed-cache copy.
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
  return IntlUtil.formatDate(ts);
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
      clientIpsText.value = String(ips || 'No IP record');
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
    clientIpsText.value = 'No IP record';
  }
}

async function copyText(value) {
  const ok = await ClipboardManager.copyText(String(value ?? ''));
  if (ok) message.success('Copied');
}

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
    wireguardConfigs.value = inbound.value.genWireguardConfigs(props.dbInbound.remark).split('\r\n');
    wireguardLinks.value = inbound.value.genWireguardLinks(props.dbInbound.remark).split('\r\n');
    links.value = [];
  } else {
    links.value = inbound.value.genAllLinks(
      props.dbInbound.remark,
      props.remarkModel,
      clientSettings.value,
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
</script>

<template>
  <a-modal
    :open="open"
    title="Inbound details"
    :footer="null"
    width="640px"
    @cancel="close"
  >
    <template v-if="dbInbound && inbound">
      <!-- ============== Inbound summary ============== -->
      <a-row :gutter="[12, 12]">
        <a-col :xs="24" :md="12">
          <table class="info-table">
            <tbody>
              <tr>
                <td>Protocol</td>
                <td><a-tag color="purple">{{ dbInbound.protocol }}</a-tag></td>
              </tr>
              <tr>
                <td>Address</td>
                <td>
                  <a-tooltip :title="dbInbound.address">
                    <a-tag class="info-large-tag">{{ dbInbound.address }}</a-tag>
                  </a-tooltip>
                </td>
              </tr>
              <tr>
                <td>Port</td>
                <td><a-tag>{{ dbInbound.port }}</a-tag></td>
              </tr>
            </tbody>
          </table>
        </a-col>

        <a-col :xs="24" :md="12">
          <template v-if="dbInbound.isVMess || dbInbound.isVLess || dbInbound.isTrojan || dbInbound.isSS">
            <table class="info-table">
              <tbody>
                <tr>
                  <td>Transmission</td>
                  <td><a-tag color="green">{{ networkLabel }}</a-tag></td>
                </tr>
                <template v-if="inbound.isTcp || inbound.isWs || inbound.isHttpupgrade || inbound.isXHTTP">
                  <tr>
                    <td>Host</td>
                    <td>
                      <a-tag v-if="inbound.host" class="info-large-tag">{{ inbound.host }}</a-tag>
                      <a-tag v-else color="orange">none</a-tag>
                    </td>
                  </tr>
                  <tr>
                    <td>Path</td>
                    <td>
                      <a-tag v-if="inbound.path" class="info-large-tag">{{ inbound.path }}</a-tag>
                      <a-tag v-else color="orange">none</a-tag>
                    </td>
                  </tr>
                </template>
                <template v-if="inbound.isXHTTP">
                  <tr>
                    <td>Mode</td>
                    <td><a-tag>{{ inbound.stream.xhttp.mode }}</a-tag></td>
                  </tr>
                </template>
                <template v-if="inbound.isGrpc">
                  <tr>
                    <td>grpc serviceName</td>
                    <td><a-tag class="info-large-tag">{{ inbound.serviceName }}</a-tag></td>
                  </tr>
                  <tr>
                    <td>grpc multiMode</td>
                    <td><a-tag>{{ inbound.stream.grpc.multiMode }}</a-tag></td>
                  </tr>
                </template>
              </tbody>
            </table>
          </template>
        </a-col>
      </a-row>

      <!-- ============== Security / encryption / SNI ============== -->
      <div v-if="dbInbound.hasLink()" class="security-line">
        <span>Security</span>
        <a-tag :color="securityColor">{{ securityLabel }}</a-tag>
        <span v-if="encryptionLabel">Encryption</span>
        <a-tag
          v-if="encryptionLabel"
          class="info-large-tag"
          :color="encryptionLabel !== 'none' ? 'green' : 'red'"
        >
          {{ encryptionLabel }}
        </a-tag>
        <a-tooltip v-if="encryptionLabel" title="Copy">
          <a-button size="small" @click="copyText(encryptionLabel)">
            <template #icon><CopyOutlined /></template>
          </a-button>
        </a-tooltip>
        <template v-if="securityLabel !== 'none'">
          <span>Domain</span>
          <a-tag v-if="serverNameLabel" color="green">{{ serverNameLabel }}</a-tag>
          <a-tag v-else color="orange">none</a-tag>
        </template>
      </div>

      <!-- ============== Shadowsocks single-user details ============== -->
      <table v-if="dbInbound.isSS" class="info-table block">
        <tbody>
          <tr>
            <td>Encryption</td>
            <td><a-tag color="green">{{ inbound.settings.method }}</a-tag></td>
          </tr>
          <tr v-if="inbound.isSS2022">
            <td>Password</td>
            <td><a-tag class="info-large-tag">{{ inbound.settings.password }}</a-tag></td>
          </tr>
          <tr>
            <td>Network</td>
            <td><a-tag color="green">{{ inbound.settings.network }}</a-tag></td>
          </tr>
        </tbody>
      </table>

      <!-- ============== Per-client info (multi-user) ============== -->
      <template v-if="clientSettings">
        <a-divider>Client</a-divider>
        <table class="info-table block">
          <tbody>
            <tr>
              <td>Email</td>
              <td>
                <a-tag v-if="clientSettings.email" color="green">{{ clientSettings.email }}</a-tag>
                <a-tag v-else color="red">none</a-tag>
              </td>
            </tr>
            <tr v-if="clientSettings.id">
              <td>ID</td>
              <td><a-tag>{{ clientSettings.id }}</a-tag></td>
            </tr>
            <tr v-if="dbInbound.isVMess">
              <td>Security</td>
              <td><a-tag>{{ clientSettings.security }}</a-tag></td>
            </tr>
            <tr v-if="inbound.canEnableTlsFlow()">
              <td>Flow</td>
              <td>
                <a-tag v-if="clientSettings.flow">{{ clientSettings.flow }}</a-tag>
                <a-tag v-else color="orange">none</a-tag>
              </td>
            </tr>
            <tr v-if="clientSettings.password">
              <td>Password</td>
              <td><a-tag class="info-large-tag">{{ clientSettings.password }}</a-tag></td>
            </tr>
            <tr>
              <td>Status</td>
              <td>
                <a-tag v-if="isDepleted" color="red">depleted</a-tag>
                <a-tag v-else-if="isEnable" color="green">enabled</a-tag>
                <a-tag v-else>disabled</a-tag>
              </td>
            </tr>
            <tr v-if="clientStats">
              <td>Usage</td>
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
              <td>Created</td>
              <td>
                <a-tag v-if="clientSettings.created_at">{{ IntlUtil.formatDate(clientSettings.created_at) }}</a-tag>
                <a-tag v-else>-</a-tag>
              </td>
            </tr>
            <tr>
              <td>Updated</td>
              <td>
                <a-tag v-if="clientSettings.updated_at">{{ IntlUtil.formatDate(clientSettings.updated_at) }}</a-tag>
                <a-tag v-else>-</a-tag>
              </td>
            </tr>
            <tr>
              <td>Last online</td>
              <td><a-tag>{{ formatLastOnline(clientSettings.email || '') }}</a-tag></td>
            </tr>
            <tr v-if="clientSettings.comment">
              <td>Comment</td>
              <td><a-tag class="info-large-tag">{{ clientSettings.comment }}</a-tag></td>
            </tr>
            <tr v-if="ipLimitEnable">
              <td>IP limit</td>
              <td><a-tag>{{ clientSettings.limitIp }}</a-tag></td>
            </tr>
            <tr v-if="ipLimitEnable && clientSettings.limitIp > 0">
              <td>IP log</td>
              <td>
                <div class="ip-log">
                  <div v-if="clientIpsArray.length > 0">
                    <a-tag
                      v-for="(item, idx) in clientIpsArray"
                      :key="idx"
                      color="blue"
                      class="ip-log-row"
                    >{{ item }}</a-tag>
                  </div>
                  <a-tag v-else>{{ clientIpsText || 'No IP record' }}</a-tag>
                </div>
                <div class="ip-log-actions">
                  <SyncOutlined :spin="refreshing" @click="loadClientIps" />
                  <a-tooltip title="Clear IP log">
                    <DeleteOutlined @click="clearClientIps" />
                  </a-tooltip>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- ============== Remaining / total / expiry ============== -->
        <table class="info-table summary-table">
          <thead>
            <tr>
              <th>Remaining</th>
              <th>Total</th>
              <th>Expiry</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>
                <a-tag
                  v-if="clientStats && clientSettings.totalGB > 0"
                  :color="statsColor(clientStats)"
                >{{ getRemainingStats() }}</a-tag>
              </td>
              <td>
                <a-tag
                  v-if="clientSettings.totalGB > 0"
                  :color="clientStats ? statsColor(clientStats) : 'default'"
                >{{ SizeFormatter.sizeFormat(clientSettings.totalGB) }}</a-tag>
                <a-tag v-else color="purple">∞</a-tag>
              </td>
              <td>
                <a-tag
                  v-if="clientSettings.expiryTime > 0"
                  :color="ColorUtils.usageColor(Date.now(), expireDiff, clientSettings.expiryTime)"
                >{{ IntlUtil.formatDate(clientSettings.expiryTime) }}</a-tag>
                <a-tag v-else-if="clientSettings.expiryTime < 0" color="green">
                  {{ clientSettings.expiryTime / -86400000 }} days
                </a-tag>
                <a-tag v-else color="purple">∞</a-tag>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- ============== Subscription URLs ============== -->
        <template v-if="subSettings.enable && clientSettings.subId">
          <a-divider>Subscription URL</a-divider>
          <QrPanel
            :value="subLink"
            remark="Subscription link"
            :show-qr="false"
          />
          <QrPanel
            v-if="subSettings.subJsonEnable && subJsonLink"
            :value="subJsonLink"
            remark="JSON link"
            :show-qr="false"
          />
        </template>

        <!-- ============== Telegram chat id ============== -->
        <template v-if="tgBotEnable && clientSettings.tgId">
          <a-divider>Telegram chat ID</a-divider>
          <div class="tg-row">
            <a-tag color="blue">{{ clientSettings.tgId }}</a-tag>
            <a-tooltip title="Copy">
              <a-button size="small" @click="copyText(clientSettings.tgId)">
                <template #icon><CopyOutlined /></template>
              </a-button>
            </a-tooltip>
          </div>
        </template>

        <!-- ============== Share links + QR codes ============== -->
        <template v-if="dbInbound.hasLink() && links.length > 0">
          <a-divider>Share links</a-divider>
          <QrPanel
            v-for="(link, idx) in links"
            :key="idx"
            :value="link.link"
            :remark="link.remark || `Link ${idx + 1}`"
          />
        </template>
      </template>

      <!-- ============== Single-user SS share link ============== -->
      <template v-else-if="dbInbound.isSS && !inbound.isSSMultiUser && links.length > 0">
        <a-divider>Share link</a-divider>
        <QrPanel
          v-for="(link, idx) in links"
          :key="idx"
          :value="link.link"
          :remark="link.remark || `Link ${idx + 1}`"
        />
      </template>

      <!-- ============== Tunnel ============== -->
      <table v-if="inbound.protocol === Protocols.TUNNEL" class="info-table protocol-table">
        <thead>
          <tr>
            <th>Target address</th>
            <th>Destination port</th>
            <th>Network</th>
            <th>FollowRedirect</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><a-tag color="green">{{ inbound.settings.address }}</a-tag></td>
            <td><a-tag color="green">{{ inbound.settings.port }}</a-tag></td>
            <td><a-tag color="green">{{ inbound.settings.network }}</a-tag></td>
            <td><a-tag color="green">{{ inbound.settings.followRedirect }}</a-tag></td>
          </tr>
        </tbody>
      </table>

      <!-- ============== Mixed ============== -->
      <table v-if="dbInbound.isMixed" class="info-table protocol-table">
        <thead>
          <tr>
            <th>Auth</th>
            <th>UDP</th>
            <th>IP</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><a-tag color="green">{{ inbound.settings.auth }}</a-tag></td>
            <td><a-tag color="green">{{ inbound.settings.udp }}</a-tag></td>
            <td><a-tag color="green">{{ inbound.settings.ip }}</a-tag></td>
          </tr>
          <template v-if="inbound.settings.auth === 'password'">
            <tr>
              <td></td>
              <td>Username</td>
              <td>Password</td>
            </tr>
            <tr v-for="(account, idx) in inbound.settings.accounts" :key="idx">
              <td>{{ idx }}</td>
              <td><a-tag color="green">{{ account.user }}</a-tag></td>
              <td><a-tag color="green">{{ account.pass }}</a-tag></td>
            </tr>
          </template>
        </tbody>
      </table>

      <!-- ============== HTTP accounts ============== -->
      <table v-if="dbInbound.isHTTP" class="info-table protocol-table">
        <thead>
          <tr>
            <th></th>
            <th>Username</th>
            <th>Password</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(account, idx) in inbound.settings.accounts" :key="idx">
            <td>{{ idx }}</td>
            <td><a-tag color="green">{{ account.user }}</a-tag></td>
            <td><a-tag color="green">{{ account.pass }}</a-tag></td>
          </tr>
        </tbody>
      </table>

      <!-- ============== WireGuard ============== -->
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
                <QrPanel
                  :value="wireguardConfigs[idx]"
                  :remark="`Peer ${idx + 1} config`"
                  :download-name="`peer-${idx + 1}.conf`"
                />
              </td>
            </tr>
            <tr v-if="wireguardLinks[idx]">
              <td colspan="2">
                <QrPanel
                  :value="wireguardLinks[idx]"
                  remark="Link"
                />
              </td>
            </tr>
          </template>
        </tbody>
      </table>
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
</style>
