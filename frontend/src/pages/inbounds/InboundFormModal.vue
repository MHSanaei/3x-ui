<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import { message } from 'ant-design-vue';
import {
  SyncOutlined,
  PlusOutlined,
  MinusOutlined,
  DeleteOutlined,
  CaretUpOutlined,
  CaretDownOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue';

import {
  HttpUtil,
  RandomUtil,
  NumberFormatter,
  SizeFormatter,
  Wireguard,
} from '@/utils';
import { getRandomRealityTarget } from '@/models/reality-targets';
import {
  Inbound,
  Protocols,
  SSMethods,
  SNIFFING_OPTION,
  TLS_VERSION_OPTION,
  TLS_CIPHER_OPTION,
  UTLS_FINGERPRINT,
  ALPN_OPTION,
  USAGE_OPTION,
  DOMAIN_STRATEGY_OPTION,
  TCP_CONGESTION_OPTION,
  MODE_OPTION,
} from '@/models/inbound.js';
import { DBInbound } from '@/models/dbinbound.js';
import FinalMaskForm from '@/components/FinalMaskForm.vue';
import DateTimePicker from '@/components/DateTimePicker.vue';
import JsonEditor from '@/components/JsonEditor.vue';
import { useNodeList } from '@/composables/useNodeList.js';

const { t } = useI18n();
const { nodes: availableNodes } = useNodeList();
const selectableNodes = computed(() => (availableNodes.value || []).filter((n) => n.enable));
const NODE_ELIGIBLE_PROTOCOLS = new Set([
  Protocols.VLESS,
  Protocols.VMESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
  Protocols.WIREGUARD,
]);
const isNodeEligible = computed(() => NODE_ELIGIBLE_PROTOCOLS.has(inbound.value?.protocol));
const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: 'add', validator: (v) => ['add', 'edit'].includes(v) },
  dbInbound: { type: Object, default: null },
  dbInbounds: { type: Array, default: () => [] },
});

const emit = defineEmits(['update:open', 'saved']);

const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'];
const PROTOCOLS = Object.values(Protocols);

// === Reactive state ================================================
// Cloned on every open so cancelling the modal doesn't mutate the row.
const inbound = ref(null);
const dbForm = ref(null);
const saving = ref(false);
const advancedStreamText = ref('');
const advancedSniffingText = ref('');
const advancedSettingsText = ref('');
const activeTabKey = ref('basic');
const advancedSectionKey = ref('all');
const defaultCert = ref('');
const defaultKey = ref('');

// Lookup tables for the option dropdowns.
const TLS_VERSIONS = Object.values(TLS_VERSION_OPTION);
const CIPHER_SUITES = Object.entries(TLS_CIPHER_OPTION); // [label, value]
const FINGERPRINTS = Object.values(UTLS_FINGERPRINT);
const ALPNS = Object.values(ALPN_OPTION);
const USAGES = Object.values(USAGE_OPTION);
const DOMAIN_STRATEGIES = Object.values(DOMAIN_STRATEGY_OPTION);
const TCP_CONGESTIONS = Object.values(TCP_CONGESTION_OPTION);
const MODE_OPTIONS = Object.values(MODE_OPTION);

// External proxy is a single switch in the UI but a list in the model:
// flipping it on seeds one row pre-filled with the current host:port.
const externalProxy = computed({
  get: () => Array.isArray(inbound.value?.stream?.externalProxy)
    && inbound.value.stream.externalProxy.length > 0,
  set: (v) => {
    if (!inbound.value?.stream) return;
    if (v) {
      inbound.value.stream.externalProxy = [{
        forceTls: 'same',
        dest: window.location.hostname,
        port: inbound.value.port,
        remark: '',
      }];
    } else {
      inbound.value.stream.externalProxy = [];
    }
  },
});

// Derived helpers — each is a computed off `inbound` so flips of
// protocol / network / security re-render the right blocks.
const protocol = computed(() => inbound.value?.protocol);
const network = computed({
  get: () => inbound.value?.stream?.network,
  set: (v) => onNetworkChange(v),
});
const security = computed({
  get: () => inbound.value?.stream?.security,
  set: (v) => { if (inbound.value?.stream) inbound.value.stream.security = v; },
});

const isVlessLike = computed(() => {
  if (!inbound.value) return false;
  return inbound.value.protocol === Protocols.VLESS;
});

const FALLBACK_ELIGIBLE_TRANSPORTS = new Set(['tcp', 'ws', 'grpc', 'httpupgrade', 'xhttp']);

const isFallbackHost = computed(() => {
  const ib = inbound.value;
  if (!ib) return false;
  if (ib.protocol !== Protocols.VLESS && ib.protocol !== Protocols.TROJAN) return false;
  if (ib.stream?.network !== 'tcp') return false;
  const sec = ib.stream?.security;
  return sec === 'tls' || sec === 'reality';
});

const fallbacks = ref([]);
let fallbackRowKey = 0;
const fallbackEditing = ref(new Set());

const fallbackChildOptions = computed(() => {
  const list = props.dbInbounds || [];
  const masterId = props.dbInbound?.id;
  return list
    .filter((ib) => ib.id !== masterId)
    .map((ib) => ({
      label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
      value: ib.id,
    }));
});

function getChildStream(childDb) {
  if (!childDb) return null;
  try { return childDb.toInbound()?.stream || null; } catch (_e) { return null; }
}

function deriveFallbackDefaults(childDb) {
  const out = { name: '', alpn: '', path: '', xver: 0 };
  const stream = getChildStream(childDb);
  if (!stream) return out;
  switch (stream.network) {
    case 'tcp': {
      const tcp = stream.tcp;
      if (tcp?.type === 'http') {
        const p = tcp?.request?.path;
        if (Array.isArray(p) && p.length) out.path = p[0];
      }
      if (tcp?.acceptProxyProtocol) out.xver = 2;
      break;
    }
    case 'ws': {
      out.path = stream.ws?.path || '';
      if (stream.ws?.acceptProxyProtocol) out.xver = 2;
      break;
    }
    case 'grpc': {
      out.path = stream.grpc?.serviceName || '';
      out.alpn = 'h2';
      break;
    }
    case 'httpupgrade': {
      out.path = stream.httpupgrade?.path || '';
      if (stream.httpupgrade?.acceptProxyProtocol) out.xver = 2;
      break;
    }
    case 'xhttp': {
      out.path = stream.xhttp?.path || '';
      break;
    }
  }
  return out;
}

function addFallback(childId = null) {
  const row = { rowKey: `fb-${++fallbackRowKey}`, childId: childId || null, name: '', alpn: '', path: '', xver: 0 };
  if (childId) {
    const child = (props.dbInbounds || []).find((ib) => ib.id === childId);
    Object.assign(row, deriveFallbackDefaults(child));
  }
  fallbacks.value.push(row);
}

function removeFallback(idx) {
  fallbacks.value.splice(idx, 1);
}

function moveFallback(idx, dir) {
  const arr = fallbacks.value;
  const j = idx + dir;
  if (j < 0 || j >= arr.length) return;
  const tmp = arr[idx];
  arr[idx] = arr[j];
  arr[j] = tmp;
}

function onFallbackChildPicked(record, childId) {
  record.childId = childId;
  const child = (props.dbInbounds || []).find((ib) => ib.id === childId);
  const defaults = deriveFallbackDefaults(child);
  record.name = defaults.name;
  record.alpn = defaults.alpn;
  record.path = defaults.path;
  record.xver = defaults.xver;
}

function rederiveFallback(record) {
  if (!record?.childId) return;
  const child = (props.dbInbounds || []).find((ib) => ib.id === record.childId);
  const defaults = deriveFallbackDefaults(child);
  record.name = defaults.name;
  record.alpn = defaults.alpn;
  record.path = defaults.path;
  record.xver = defaults.xver;
  message.success(t('pages.inbounds.fallbacks.rederived') || 'Re-filled from child');
}

function quickAddAllFallbacks() {
  const masterId = props.dbInbound?.id;
  const list = props.dbInbounds || [];
  const existing = new Set(fallbacks.value.map((r) => r.childId).filter(Boolean));
  let added = 0;
  for (const ib of list) {
    if (ib.id === masterId) continue;
    if (existing.has(ib.id)) continue;
    const stream = getChildStream(ib);
    if (!stream || !FALLBACK_ELIGIBLE_TRANSPORTS.has(stream.network)) continue;
    addFallback(ib.id);
    added += 1;
  }
  if (added > 0) {
    message.success(t('pages.inbounds.fallbacks.quickAdded', { n: added }) || `Added ${added} fallback(s)`);
  } else {
    message.info(t('pages.inbounds.fallbacks.quickAddedNone') || 'No new eligible inbounds to add');
  }
}

function isFallbackEditing(rowKey) { return fallbackEditing.value.has(rowKey); }
function toggleFallbackEdit(rowKey) {
  const next = new Set(fallbackEditing.value);
  if (next.has(rowKey)) next.delete(rowKey); else next.add(rowKey);
  fallbackEditing.value = next;
}

function describeFallback(record) {
  const parts = [];
  if (record.name) parts.push(`SNI=${record.name}`);
  if (record.alpn) parts.push(`ALPN=${record.alpn}`);
  if (record.path) parts.push(`path=${record.path}`);
  const condition = parts.length
    ? `${t('pages.inbounds.fallbacks.routesWhen') || 'Routes when'} ${parts.join(' · ')}`
    : (t('pages.inbounds.fallbacks.defaultCatchAll') || 'Default — catches anything else');
  const proxyTag = record.xver === 2 ? ' · PROXY v2' : record.xver === 1 ? ' · PROXY v1' : '';
  return { condition, proxyTag };
}

async function loadFallbacks(masterId) {
  fallbacks.value = [];
  if (!masterId) return;
  const msg = await HttpUtil.get(`/panel/api/inbounds/${masterId}/fallbacks`);
  if (!msg?.success || !Array.isArray(msg.obj)) return;
  fallbacks.value = msg.obj.map((r) => ({
    rowKey: `fb-${++fallbackRowKey}`,
    childId: r.childId,
    name: r.name || '',
    alpn: r.alpn || '',
    path: r.path || '',
    xver: r.xver || 0,
  }));
}

async function saveFallbacks(masterId) {
  if (!masterId) return true;
  const payload = {
    fallbacks: fallbacks.value
      .filter((c) => c.childId)
      .map((c, i) => ({
        childId: c.childId,
        name: c.name,
        alpn: c.alpn,
        path: c.path,
        xver: Number(c.xver) || 0,
        sortOrder: i,
      })),
  };
  const msg = await HttpUtil.post(
    `/panel/api/inbounds/${masterId}/fallbacks`,
    payload,
    { headers: { 'Content-Type': 'application/json' } },
  );
  return !!msg?.success;
}

const canEnableStream = computed(() => inbound.value?.canEnableStream?.() === true);
const canEnableTls = computed(() => inbound.value?.canEnableTls?.() === true);
const canEnableReality = computed(() => inbound.value?.canEnableReality?.() === true);

const hasProtocolTabContent = computed(() => {
  if (!inbound.value) return false;
  if (isVlessLike.value) return true;
  if (isFallbackHost.value) return true;
  switch (inbound.value.protocol) {
    case Protocols.SHADOWSOCKS:
    case Protocols.HTTP:
    case Protocols.MIXED:
    case Protocols.TUNNEL:
    case Protocols.TUN:
    case Protocols.WIREGUARD:
      return true;
    default:
      return false;
  }
});

// Date / GB bridges (legacy used moment via _expiryTime; we go direct).
const expiryDate = computed({
  get: () => (dbForm.value?.expiryTime > 0 ? dayjs(dbForm.value.expiryTime) : null),
  set: (next) => { if (dbForm.value) dbForm.value.expiryTime = next ? next.valueOf() : 0; },
});
const totalGB = computed({
  get: () => (dbForm.value?.total ? Math.round((dbForm.value.total / SizeFormatter.ONE_GB) * 100) / 100 : 0),
  set: (gb) => { if (dbForm.value) dbForm.value.total = NumberFormatter.toFixed((gb || 0) * SizeFormatter.ONE_GB, 0); },
});

// === Open / state management =======================================
function loadFromDbInbound(dbIn) {
  // Round-trip through Inbound.fromJson so subsequent edits get the
  // structured class hierarchy (StreamSettings, TLS, Reality, etc.).
  const parsed = Inbound.fromJson(dbIn.toInbound().toJson());
  inbound.value = parsed;
  // DBForm carries the persisted-fields the parsed Inbound doesn't:
  // remark, enable, total, expiryTime, trafficReset, etc.
  dbForm.value = new DBInbound(dbIn);
  primeAdvancedJson();
}

function makeFreshInbound(proto) {
  const ib = new Inbound();
  ib.protocol = proto;
  ib.settings = Inbound.Settings.getSettings(proto);
  ib.port = RandomUtil.randomInteger(10000, 60000);
  return ib;
}

function freshDbForm() {
  const next = new DBInbound();
  next.enable = true;
  next.remark = '';
  next.total = 0;
  next.expiryTime = 0;
  next.trafficReset = 'never';
  return next;
}

function primeAdvancedJson() {
  if (!inbound.value) return;
  ['stream', 'sniffing', 'settings'].forEach(stampAdvancedTextFor);
}

watch(() => props.open, (next) => {
  if (!next) return;
  fallbackEditing.value = new Set();
  if (props.mode === 'edit' && props.dbInbound) {
    loadFromDbInbound(props.dbInbound);
    const proto = props.dbInbound.protocol;
    if (proto === Protocols.VLESS || proto === Protocols.TROJAN) {
      loadFallbacks(props.dbInbound.id);
    } else {
      fallbacks.value = [];
    }
  } else {
    inbound.value = makeFreshInbound(Protocols.VLESS);
    dbForm.value = freshDbForm();
    primeAdvancedJson();
    fallbacks.value = [];
  }
  activeTabKey.value = 'basic';
  advancedSectionKey.value = 'all';
  fetchDefaultCertSettings();
});

function applyAdvancedJsonToBasic() {
  if (!inbound.value) return true;
  let settings; let streamSettings; let sniffing;
  try {
    settings = parseAdvancedSliceWithLabel(advancedSettingsText.value, settingsFallback(), t('pages.inbounds.advanced.settings'));
    streamSettings = parseAdvancedSliceWithLabel(advancedStreamText.value, streamFallback(), t('pages.inbounds.advanced.stream'));
    sniffing = parseAdvancedSliceWithLabel(advancedSniffingText.value, sniffingFallback(), t('pages.inbounds.advanced.sniffing'));
  } catch (_e) { return false; }

  try {
    inbound.value = Inbound.fromJson({
      port: inbound.value.port,
      listen: inbound.value.listen,
      protocol: inbound.value.protocol,
      settings,
      streamSettings,
      tag: inbound.value.tag,
      sniffing,
      clientStats: inbound.value.clientStats,
    });
  } catch (e) {
    message.error(`${t('pages.inbounds.advanced.jsonErrorPrefix')}: ${e.message}`);
    return false;
  }
  return true;
}

let isRevertingTab = false;
watch(activeTabKey, (next, prev) => {
  if (isRevertingTab) { isRevertingTab = false; return; }
  if (prev === 'advanced' && next !== 'advanced') {
    if (!applyAdvancedJsonToBasic()) {
      isRevertingTab = true;
      activeTabKey.value = 'advanced';
    }
  }
});

watch(hasProtocolTabContent, (next) => {
  if (!next && activeTabKey.value === 'protocol') {
    activeTabKey.value = 'basic';
  }
});

// In add mode, switching protocol restamps settings + re-syncs port.
function onProtocolChange(next) {
  if (props.mode === 'edit' || !inbound.value) return;
  inbound.value.protocol = next;
  inbound.value.settings = Inbound.Settings.getSettings(next);
  if (!NODE_ELIGIBLE_PROTOCOLS.has(next)) {
    dbForm.value.nodeId = null;
  }
  primeAdvancedJson();
}

function onNetworkChange(next) {
  if (!inbound.value?.stream) return;
  inbound.value.stream.network = next;
  // Mirror legacy streamNetworkChange: clear flow when TLS/Reality
  // become unavailable; reset finalmask.udp when not KCP.
  if (!inbound.value.canEnableTls()) inbound.value.stream.security = 'none';
  if (!inbound.value.canEnableReality()) inbound.value.reality = false;
  if (
    inbound.value.protocol === Protocols.VLESS
    && !inbound.value.canEnableTlsFlow()
    && Array.isArray(inbound.value.settings.vlesses)
  ) {
    inbound.value.settings.vlesses.forEach((c) => { c.flow = ''; });
  }
  if (next !== 'kcp' && inbound.value.stream.finalmask) {
    inbound.value.stream.finalmask.udp = [];
  }
}

function parseAdvancedSliceOrFallback(rawText, fallbackValue) {
  if (!rawText?.trim()) return fallbackValue;
  return JSON.parse(rawText);
}

function unwrapWrappedObject(parsed, key) {
  if (
    parsed
    && typeof parsed === 'object'
    && !Array.isArray(parsed)
    && parsed[key] !== undefined
  ) {
    return parsed[key];
  }
  return parsed;
}

const settingsFallback = () => inbound.value?.settings?.toJson?.() || {};
const sniffingFallback = () => inbound.value?.sniffing?.toJson?.() || {};
const streamFallback = () => inbound.value?.stream?.toJson?.() || {};

const advancedTextRefs = {
  stream: advancedStreamText,
  sniffing: advancedSniffingText,
  settings: advancedSettingsText,
};

function stampAdvancedTextFor(slice) {
  const textRef = advancedTextRefs[slice];
  if (!textRef) return;
  if (slice === 'stream' && !canEnableStream.value) {
    textRef.value = '{}';
    return;
  }
  const obj = inbound.value?.[slice];
  if (!obj) return;
  try {
    textRef.value = JSON.stringify(JSON.parse(obj.toString()), null, 2);
  } catch (_e) { /* keep prior text */ }
}

function parseAdvancedSliceWithLabel(rawText, fallback, label) {
  try {
    return parseAdvancedSliceOrFallback(rawText, fallback);
  } catch (e) {
    message.error(`${label} JSON invalid: ${e.message}`);
    throw e;
  }
}

function compactAdvancedJson(raw, fallback, label) {
  try {
    return JSON.stringify(JSON.parse(raw || fallback));
  } catch (e) {
    message.error(`${label} JSON invalid: ${e.message}`);
    throw e;
  }
}

async function withSaving(fn) {
  saving.value = true;
  try { return await fn(); } finally { saving.value = false; }
}

function makeWrappedAdvancedConfig({ key, textRef, getFallback, label }) {
  const invalid = `${label} JSON invalid`;
  return computed({
    get: () => {
      if (!inbound.value) return '';
      try {
        const value = parseAdvancedSliceOrFallback(textRef.value, getFallback());
        return JSON.stringify({ [key]: value }, null, 2);
      } catch (_e) {
        return '';
      }
    },
    set: (next) => {
      let parsed;
      try {
        parsed = JSON.parse(next);
      } catch (e) {
        message.error(`${invalid}: ${e.message}`);
        return;
      }
      const unwrapped = unwrapWrappedObject(parsed, key);
      if (!unwrapped || typeof unwrapped !== 'object' || Array.isArray(unwrapped)) {
        message.error(`${label} JSON must be an object or { ${key}: { ... } }.`);
        return;
      }
      try {
        textRef.value = JSON.stringify(unwrapped, null, 2);
      } catch (e) {
        message.error(`${invalid}: ${e.message}`);
      }
    },
  });
}

const advancedAllConfig = computed({
  get: () => {
    if (!inbound.value) return '';
    try {
      const result = {
        listen: inbound.value.listen,
        port: inbound.value.port,
        protocol: inbound.value.protocol,
        settings: parseAdvancedSliceOrFallback(advancedSettingsText.value, settingsFallback()),
        sniffing: parseAdvancedSliceOrFallback(advancedSniffingText.value, sniffingFallback()),
        tag: inbound.value.tag,
      };
      if (canEnableStream.value) {
        result.streamSettings = parseAdvancedSliceOrFallback(advancedStreamText.value, streamFallback());
      }
      return JSON.stringify(result, null, 2);
    } catch (_e) {
      return '';
    }
  },
  set: (next) => {
    let parsed;
    try {
      parsed = JSON.parse(next);
    } catch (e) {
      message.error(`All JSON invalid: ${e.message}`);
      return;
    }
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      message.error('All JSON must be an inbound object.');
      return;
    }

    try {
      if (typeof parsed.listen === 'string') inbound.value.listen = parsed.listen;
      if (parsed.port !== undefined) {
        const port = Number(parsed.port);
        if (Number.isFinite(port)) inbound.value.port = port;
      }
      if (typeof parsed.protocol === 'string' && PROTOCOLS.includes(parsed.protocol)) {
        inbound.value.protocol = parsed.protocol;
      }
      if (typeof parsed.tag === 'string') inbound.value.tag = parsed.tag;

      const existingSettings = parseAdvancedSliceOrFallback(advancedSettingsText.value, settingsFallback());
      advancedSettingsText.value = JSON.stringify(parsed.settings ?? existingSettings, null, 2);
      advancedSniffingText.value = JSON.stringify(parsed.sniffing ?? sniffingFallback(), null, 2);
      advancedStreamText.value = canEnableStream.value
        ? JSON.stringify(parsed.streamSettings ?? streamFallback(), null, 2)
        : '{}';
    } catch (e) {
      message.error(`All JSON invalid: ${e.message}`);
    }
  },
});

const advancedSettingsConfig = makeWrappedAdvancedConfig({
  key: 'settings',
  textRef: advancedSettingsText,
  getFallback: settingsFallback,
  label: 'Settings',
});

const advancedSniffingConfig = makeWrappedAdvancedConfig({
  key: 'sniffing',
  textRef: advancedSniffingText,
  getFallback: sniffingFallback,
  label: 'Sniffing',
});

const advancedStreamConfig = makeWrappedAdvancedConfig({
  key: 'streamSettings',
  textRef: advancedStreamText,
  getFallback: streamFallback,
  label: 'Stream',
});

function randomSSPassword(target) {
  if (target) target.password = RandomUtil.randomShadowsocksPassword(inbound.value.settings.method);
}
function regenWgKeypair(target) {
  const kp = Wireguard.generateKeypair();
  target.publicKey = kp.publicKey;
  target.privateKey = kp.privateKey;
}
function regenInboundWg() {
  const kp = Wireguard.generateKeypair();
  inbound.value.settings.pubKey = kp.publicKey;
  inbound.value.settings.secretKey = kp.privateKey;
}

// === Reality keygen via existing API =================================
async function genRealityKeypair() {
  await withSaving(async () => {
    const msg = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
    if (msg?.success) {
      inbound.value.stream.reality.privateKey = msg.obj.privateKey;
      inbound.value.stream.reality.settings.publicKey = msg.obj.publicKey;
    }
  });
}

function clearRealityKeypair() {
  if (!inbound.value?.stream?.reality) return;
  inbound.value.stream.reality.privateKey = '';
  inbound.value.stream.reality.settings.publicKey = '';
}

async function genMldsa65() {
  await withSaving(async () => {
    const msg = await HttpUtil.get('/panel/api/server/getNewmldsa65');
    if (msg?.success) {
      inbound.value.stream.reality.mldsa65Seed = msg.obj.seed;
      inbound.value.stream.reality.settings.mldsa65Verify = msg.obj.verify;
    }
  });
}

function clearMldsa65() {
  if (!inbound.value?.stream?.reality) return;
  inbound.value.stream.reality.mldsa65Seed = '';
  inbound.value.stream.reality.settings.mldsa65Verify = '';
}

function randomizeRealityTarget() {
  if (!inbound.value?.stream?.reality) return;
  const t = getRandomRealityTarget();
  inbound.value.stream.reality.target = t.target;
  inbound.value.stream.reality.serverNames = t.sni;
}

function randomizeShortIds() {
  if (!inbound.value?.stream?.reality) return;
  inbound.value.stream.reality.shortIds = RandomUtil.randomShortIds();
}

// === ECH cert helpers ================================================
async function getNewEchCert() {
  if (!inbound.value?.stream?.tls) return;
  await withSaving(async () => {
    const msg = await HttpUtil.post('/panel/api/server/getNewEchCert', {
      sni: inbound.value.stream.tls.sni,
    });
    if (msg?.success) {
      inbound.value.stream.tls.echServerKeys = msg.obj.echServerKeys;
      inbound.value.stream.tls.settings.echConfigList = msg.obj.echConfigList;
    }
  });
}

function clearEchCert() {
  if (!inbound.value?.stream?.tls) return;
  inbound.value.stream.tls.echServerKeys = '';
  inbound.value.stream.tls.settings.echConfigList = '';
}

function setDefaultCertData(idx) {
  if (!inbound.value?.stream?.tls?.certs?.[idx]) return;
  inbound.value.stream.tls.certs[idx].certFile = defaultCert.value;
  inbound.value.stream.tls.certs[idx].keyFile = defaultKey.value;
}

async function fetchDefaultCertSettings() {
  try {
    const msg = await HttpUtil.post('/panel/setting/defaultSettings');
    if (msg?.success && msg.obj) {
      defaultCert.value = msg.obj.defaultCert || '';
      defaultKey.value = msg.obj.defaultKey || '';
    }
  } catch (_e) { /* non-fatal — leave Set Default disabled */ }
}

// === VLESS encryption helpers =======================================
// `xray vlessenc` returns both X25519 and ML-KEM-768 auth variants every
// call; the user clicks one button to pick which block goes into
// decryption/encryption. Both generated strings share the same hybrid
// mlkem768x25519plus prefix; the auth choice is the final key block.
function normalizeVlessAuthLabel(label = '') {
  return label.toLowerCase().replace(/[-_\s]/g, '');
}

function matchesVlessAuth(block, authId) {
  if (block?.id === authId) return true;
  const label = normalizeVlessAuthLabel(block?.label);
  if (authId === 'mlkem768') return label.includes('mlkem768');
  if (authId === 'x25519') return label.includes('x25519');
  return false;
}

async function getNewVlessEnc(authId) {
  if (!authId || !inbound.value?.settings) return;
  await withSaving(async () => {
    const msg = await HttpUtil.get('/panel/api/server/getNewVlessEnc');
    if (!msg?.success) return;
    const block = (msg.obj?.auths || []).find((a) => matchesVlessAuth(a, authId));
    if (!block) return;
    inbound.value.settings.decryption = block.decryption;
    inbound.value.settings.encryption = block.encryption;
  });
}

function clearVlessEnc() {
  if (!inbound.value?.settings) return;
  inbound.value.settings.decryption = 'none';
  inbound.value.settings.encryption = 'none';
}

const selectedVlessAuth = computed(() => {
  const encryption = inbound.value?.settings?.encryption;
  if (!encryption || encryption === 'none') return 'None';

  const parts = encryption.split('.').filter(Boolean);
  const authKey = parts[parts.length - 1] || '';
  if (!authKey) return t('pages.inbounds.vlessAuthCustom');

  return authKey.length > 300
    ? t('pages.inbounds.vlessAuthMlkem768')
    : t('pages.inbounds.vlessAuthX25519');
});

function onSSMethodChange() {
  inbound.value.settings.password = RandomUtil.randomShadowsocksPassword(inbound.value.settings.method);
  if (inbound.value.isSSMultiUser) {
    inbound.value.settings.shadowsockses.forEach((c) => {
      c.method = inbound.value.isSS2022 ? '' : inbound.value.settings.method;
      c.password = RandomUtil.randomShadowsocksPassword(inbound.value.settings.method);
    });
  } else {
    inbound.value.settings.shadowsockses = [];
  }
}

// === Submit ==========================================================
function close() {
  emit('update:open', false);
}

async function submit() {
  if (!inbound.value || !dbForm.value) return;
  saving.value = true;
  try {
    let streamSettings; let sniffing; let settings;
    try {
      streamSettings = canEnableStream.value
        ? compactAdvancedJson(advancedStreamText.value, '', t('pages.inbounds.advanced.stream'))
        : (inbound.value.stream?.sockopt
          ? JSON.stringify({ sockopt: inbound.value.stream.sockopt.toJson() })
          : '');
      sniffing = compactAdvancedJson(advancedSniffingText.value, inbound.value.sniffing.toString(), t('pages.inbounds.advanced.sniffing'));
      settings = compactAdvancedJson(advancedSettingsText.value, inbound.value.settings.toString(), t('pages.inbounds.advanced.settings'));
    } catch (_e) { return; }

    // The structured form mutates `inbound.stream` directly when the
    // user edits TCP/WS/gRPC/HTTPUpgrade fields, but if they touched
    // the Advanced JSON tab their edits live there. Keep the JSON tab
    // authoritative — it was populated from the live model on open
    // and watch handlers below sync in either direction.
    const payload = {
      up: dbForm.value.up || 0,
      down: dbForm.value.down || 0,
      total: dbForm.value.total,
      remark: dbForm.value.remark,
      enable: dbForm.value.enable,
      expiryTime: dbForm.value.expiryTime,
      trafficReset: dbForm.value.trafficReset,
      lastTrafficResetTime: dbForm.value.lastTrafficResetTime || 0,
      listen: inbound.value.listen,
      port: inbound.value.port,
      protocol: inbound.value.protocol,
      settings: settings,
      streamSettings: streamSettings,
      sniffing: sniffing,
    };
    // Multi-node deployment: only include nodeId when the user picked a
    // remote node. Sending nodeId=null over qs.stringify becomes an
    // empty form value, which Go's form binding for *int parses as 0
    // — not nil — and we'd then try to look up node id 0 and fail with
    // "record not found". Omitting the key entirely keeps NodeID nil.
    if (dbForm.value.nodeId != null) {
      payload.nodeId = dbForm.value.nodeId;
    }

    const url = props.mode === 'edit'
      ? `/panel/api/inbounds/update/${props.dbInbound.id}`
      : '/panel/api/inbounds/add';
    const msg = await HttpUtil.post(url, payload);
    if (msg?.success) {
      if (isFallbackHost.value) {
        const masterId = props.mode === 'edit'
          ? props.dbInbound.id
          : (msg.obj?.id || msg.obj?.Id);
        if (masterId) await saveFallbacks(masterId);
      }
      emit('saved');
      close();
    }
  } finally {
    saving.value = false;
  }
}

const title = computed(() =>
  props.mode === 'edit'
    ? t('pages.inbounds.modifyInbound')
    : t('pages.inbounds.addInbound'),
);
const okText = computed(() =>
  props.mode === 'edit' ? t('pages.clients.submitEdit') : t('create'),
);

// Whenever the structured form mutates stream / sniffing / settings,
// refresh the matching slice of the Advanced JSON tab so the user
// always sees the live state.
['stream', 'sniffing', 'settings'].forEach((slice) => {
  watch(
    () => inbound.value && JSON.stringify(inbound.value[slice]?.toJson?.() || {}),
    () => stampAdvancedTextFor(slice),
  );
});

watch(() => inbound.value?.protocol, () => stampAdvancedTextFor('stream'));
</script>

<template>
  <a-modal :open="open" :title="title" :ok-text="okText" :cancel-text="t('close')" :confirm-loading="saving"
    :mask-closable="false" width="780px" @ok="submit" @cancel="close">
    <a-tabs v-if="inbound && dbForm" v-model:active-key="activeTabKey">
      <!-- ============================== BASICS ============================== -->
      <a-tab-pane key="basic" :tab="t('pages.xray.basicTemplate')">
        <a-form :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }">
          <a-form-item :label="t('enable')">
            <a-switch v-model:checked="dbForm.enable" />
          </a-form-item>
          <a-form-item :label="t('pages.inbounds.remark')">
            <a-input v-model:value="dbForm.remark" />
          </a-form-item>
          <a-form-item v-if="selectableNodes.length > 0 && isNodeEligible" :label="t('pages.inbounds.deployTo')">
            <a-select v-model:value="dbForm.nodeId" :disabled="mode === 'edit'"
              :placeholder="t('pages.inbounds.localPanel')" allow-clear>
              <a-select-option :value="null">{{ t('pages.inbounds.localPanel') }}</a-select-option>
              <a-select-option v-for="n in selectableNodes" :key="n.id" :value="n.id"
                :disabled="n.status === 'offline'">
                {{ n.name }}{{ n.status === 'offline' ? ' (offline)' : '' }}
              </a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item :label="t('pages.inbounds.protocol')">
            <a-select :value="protocol" :disabled="mode === 'edit'" @change="onProtocolChange">
              <a-select-option v-for="p in PROTOCOLS" :key="p" :value="p">{{ p }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item :label="t('pages.inbounds.address')">
            <a-input v-model:value="inbound.listen" :placeholder="t('pages.inbounds.monitorDesc')" />
          </a-form-item>
          <a-form-item :label="t('pages.inbounds.port')">
            <a-input-number v-model:value="inbound.port" :min="1" :max="65535" />
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip :title="t('pages.inbounds.meansNoLimit')">{{ t('pages.inbounds.totalFlow') }}</a-tooltip>
            </template>
            <a-input-number v-model:value="totalGB" :min="0" :step="0.1" />
          </a-form-item>
          <a-form-item :label="t('pages.inbounds.periodicTrafficResetTitle')">
            <a-select v-model:value="dbForm.trafficReset">
              <a-select-option v-for="r in TRAFFIC_RESETS" :key="r" :value="r">
                {{ t(`pages.inbounds.periodicTrafficReset.${r}`) }}
              </a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip :title="t('pages.inbounds.leaveBlankToNeverExpire')">{{ t('pages.inbounds.expireDate')
              }}</a-tooltip>
            </template>
            <DateTimePicker v-model:value="expiryDate" />
          </a-form-item>
        </a-form>
      </a-tab-pane>

      <!-- ============================== PROTOCOL ============================== -->
      <a-tab-pane v-if="hasProtocolTabContent" key="protocol" :tab="t('pages.inbounds.protocol')">
        <!-- VLess decryption / encryption -->
        <a-form v-if="isVlessLike" :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }"
          class="mt-12">
          <a-form-item :label="t('pages.inbounds.decryption')">
            <a-input v-model:value="inbound.settings.decryption" />
          </a-form-item>
          <a-form-item :label="t('pages.inbounds.encryption')">
            <a-input v-model:value="inbound.settings.encryption" />
          </a-form-item>
          <a-form-item label=" ">
            <a-space :size="8" wrap>
              <a-button type="primary" :loading="saving" @click="getNewVlessEnc('x25519')">
                {{ t('pages.inbounds.vlessAuthX25519') }}
              </a-button>
              <a-button type="primary" :loading="saving" @click="getNewVlessEnc('mlkem768')">
                {{ t('pages.inbounds.vlessAuthMlkem768') }}
              </a-button>
              <a-button danger @click="clearVlessEnc">{{ t('clear') }}</a-button>
            </a-space>
            <a-typography-text type="secondary" class="vless-auth-state">
              {{ t('pages.inbounds.vlessAuthSelected', { auth: selectedVlessAuth }) }}
            </a-typography-text>
          </a-form-item>
        </a-form>

        <a-card v-if="isFallbackHost" size="small" class="mt-12"
          :title="t('pages.inbounds.fallbacks.title') || 'Fallbacks'">
          <a-typography-paragraph type="secondary" style="margin-bottom: 12px">
            {{ t('pages.inbounds.fallbacks.help') || 'When a connection on this inbound does not match any client, route it to another inbound. Pick a child below and the routing fields (SNI / ALPN / path / xver) auto-fill from its transport — most setups need no further tweaking. Each child should listen on 127.0.0.1 with security=none.' }}
          </a-typography-paragraph>
          <template v-if="fallbacks.length === 0">
            <a-empty :description="t('pages.inbounds.fallbacks.empty') || 'No fallbacks yet'" :image-style="{ height: '40px' }" style="margin: 8px 0 12px" />
          </template>
          <div v-for="(record, index) in fallbacks" :key="record.rowKey"
            style="border: 1px solid var(--app-border-tertiary); border-radius: 6px; padding: 10px 12px; margin-bottom: 8px">
            <a-row :gutter="8" align="middle" :wrap="false">
              <a-col flex="none">
                <a-space direction="vertical" :size="2">
                  <a-button size="small" type="text" :disabled="index === 0" @click="moveFallback(index, -1)">
                    <CaretUpOutlined />
                  </a-button>
                  <a-button size="small" type="text" :disabled="index === fallbacks.length - 1" @click="moveFallback(index, 1)">
                    <CaretDownOutlined />
                  </a-button>
                </a-space>
              </a-col>
              <a-col flex="auto">
                <a-select :value="record.childId" :options="fallbackChildOptions" :show-search="true"
                  :placeholder="t('pages.inbounds.fallbacks.pickInbound') || 'Pick an inbound'"
                  :filter-option="(input, option) => (option.label || '').toLowerCase().includes(input.toLowerCase())"
                  style="width: 100%" @change="(v) => onFallbackChildPicked(record, v)" />
                <a-typography-text type="secondary" style="font-size: 12px; display: block; margin-top: 4px">
                  {{ describeFallback(record).condition }}{{ describeFallback(record).proxyTag }}
                </a-typography-text>
              </a-col>
              <a-col flex="none">
                <a-space :size="4">
                  <a-tooltip :title="t('pages.inbounds.fallbacks.rederive') || 'Re-fill from child'">
                    <a-button size="small" type="text" :disabled="!record.childId" @click="rederiveFallback(record)">
                      <SyncOutlined />
                    </a-button>
                  </a-tooltip>
                  <a-tooltip :title="isFallbackEditing(record.rowKey)
                    ? (t('pages.inbounds.fallbacks.hideAdvanced') || 'Hide advanced')
                    : (t('pages.inbounds.fallbacks.editAdvanced') || 'Edit routing fields')">
                    <a-button size="small" type="text" @click="toggleFallbackEdit(record.rowKey)">
                      <SettingOutlined />
                    </a-button>
                  </a-tooltip>
                  <a-button size="small" type="text" danger @click="removeFallback(index)">
                    <DeleteOutlined />
                  </a-button>
                </a-space>
              </a-col>
            </a-row>
            <a-row v-if="isFallbackEditing(record.rowKey)" :gutter="8" style="margin-top: 8px">
              <a-col :xs="24" :md="8">
                <a-input v-model:value="record.name" addon-before="SNI" :placeholder="t('pages.inbounds.fallbacks.matchAny') || 'any'" />
              </a-col>
              <a-col :xs="24" :md="5">
                <a-input v-model:value="record.alpn" addon-before="ALPN" :placeholder="t('pages.inbounds.fallbacks.matchAny') || 'any'" />
              </a-col>
              <a-col :xs="24" :md="7">
                <a-input v-model:value="record.path" addon-before="Path" placeholder="/" />
              </a-col>
              <a-col :xs="24" :md="4">
                <a-input-number v-model:value="record.xver" addon-before="xver" :min="0" :max="2" style="width: 100%" />
              </a-col>
            </a-row>
          </div>
          <a-space :size="8" style="margin-top: 4px" wrap>
            <a-button size="small" @click="addFallback()">
              <PlusOutlined /> {{ t('pages.inbounds.fallbacks.add') || 'Add fallback' }}
            </a-button>
            <a-button size="small" type="primary" ghost @click="quickAddAllFallbacks">
              {{ t('pages.inbounds.fallbacks.quickAddAll') || 'Quick add all eligible' }}
            </a-button>
          </a-space>
        </a-card>

        <!-- Shadowsocks shared fields (method/network/ivCheck) -->
        <a-form v-if="protocol === Protocols.SHADOWSOCKS" :colon="false" :label-col="{ sm: { span: 8 } }"
          :wrapper-col="{ sm: { span: 14 } }" class="mt-12">
          <a-form-item label="Encryption method">
            <a-select v-model:value="inbound.settings.method" @change="onSSMethodChange">
              <a-select-option v-for="(m, k) in SSMethods" :key="k" :value="m">{{ k }}</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item v-if="inbound.isSS2022">
            <template #label>
              Password
              <SyncOutlined class="random-icon" @click="randomSSPassword(inbound.settings)" />
            </template>
            <a-input v-model:value="inbound.settings.password" />
          </a-form-item>
          <a-form-item label="Network">
            <a-select v-model:value="inbound.settings.network" :style="{ width: '120px' }">
              <a-select-option value="tcp,udp">TCP, UDP</a-select-option>
              <a-select-option value="tcp">TCP</a-select-option>
              <a-select-option value="udp">UDP</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="ivCheck">
            <a-switch v-model:checked="inbound.settings.ivCheck" />
          </a-form-item>
        </a-form>

        <!-- HTTP / Mixed accounts -->
        <a-form v-if="protocol === Protocols.HTTP || protocol === Protocols.MIXED" :colon="false"
          :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }" class="mt-12">
          <a-form-item label="Accounts">
            <a-button size="small" @click="protocol === Protocols.HTTP
              ? inbound.settings.addAccount(new Inbound.HttpSettings.HttpAccount())
              : inbound.settings.addAccount(new Inbound.MixedSettings.SocksAccount())">
              <template #icon>
                <PlusOutlined />
              </template>
              Add
            </a-button>
          </a-form-item>
          <a-form-item :wrapper-col="{ span: 24 }">
            <a-input-group v-for="(account, idx) in inbound.settings.accounts" :key="idx" compact class="mb-8">
              <a-input :style="{ width: '45%' }" v-model:value="account.user" placeholder="Username">
                <template #addonBefore>{{ idx + 1 }}</template>
              </a-input>
              <a-input :style="{ width: '45%' }" v-model:value="account.pass" placeholder="Password" />
              <a-button @click="inbound.settings.delAccount(idx)">
                <template #icon>
                  <MinusOutlined />
                </template>
              </a-button>
            </a-input-group>
          </a-form-item>
          <a-form-item v-if="protocol === Protocols.HTTP" label="Allow transparent">
            <a-switch v-model:checked="inbound.settings.allowTransparent" />
          </a-form-item>
          <template v-if="protocol === Protocols.MIXED">
            <a-form-item label="Auth">
              <a-select v-model:value="inbound.settings.auth">
                <a-select-option value="noauth">noauth</a-select-option>
                <a-select-option value="password">password</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="UDP">
              <a-switch v-model:checked="inbound.settings.udp" />
            </a-form-item>
            <a-form-item v-if="inbound.settings.udp" label="UDP IP">
              <a-input v-model:value="inbound.settings.ip" />
            </a-form-item>
          </template>
        </a-form>

        <!-- Tunnel -->
        <a-form v-if="protocol === Protocols.TUNNEL" :colon="false" :label-col="{ sm: { span: 8 } }"
          :wrapper-col="{ sm: { span: 14 } }" class="mt-12">
          <a-form-item label="Rewrite address">
            <a-input v-model:value="inbound.settings.rewriteAddress" />
          </a-form-item>
          <a-form-item label="Rewrite port">
            <a-input-number v-model:value="inbound.settings.rewritePort" :min="0" :max="65535" />
          </a-form-item>
          <a-form-item label="Allowed network">
            <a-select v-model:value="inbound.settings.allowedNetwork">
              <a-select-option value="tcp,udp">TCP, UDP</a-select-option>
              <a-select-option value="tcp">TCP</a-select-option>
              <a-select-option value="udp">UDP</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="Port map">
            <a-button size="small" @click="inbound.settings.addPortMap('', '')">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <a-form-item v-if="inbound.settings.portMap.length > 0" :wrapper-col="{ span: 24 }">
            <a-input-group v-for="(pm, idx) in inbound.settings.portMap" :key="`pm-${idx}`" compact class="mb-8">
              <a-input :style="{ width: '30%' }" v-model:value="pm.name" placeholder="5555">
                <template #addonBefore>{{ idx + 1 }}</template>
              </a-input>
              <a-input :style="{ width: '60%' }" v-model:value="pm.value" placeholder="1.1.1.1:7777" />
              <a-button @click="inbound.settings.removePortMap(idx)">
                <template #icon>
                  <MinusOutlined />
                </template>
              </a-button>
            </a-input-group>
          </a-form-item>
          <a-form-item label="Follow redirect">
            <a-switch v-model:checked="inbound.settings.followRedirect" />
          </a-form-item>
        </a-form>

        <!-- TUN -->
        <a-form v-if="protocol === Protocols.TUN" :colon="false" :label-col="{ sm: { span: 8 } }"
          :wrapper-col="{ sm: { span: 14 } }" class="mt-12">
          <a-form-item label="Interface name">
            <a-input v-model:value="inbound.settings.name" placeholder="xray0" />
          </a-form-item>
          <a-form-item label="MTU">
            <a-input-number v-model:value="inbound.settings.mtu" :min="0" />
          </a-form-item>
          <a-form-item label="Gateway">
            <a-button size="small" @click="inbound.settings.gateway.push('')">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
            <a-input v-for="(_ip, j) in inbound.settings.gateway" :key="`tun-gw-${j}`"
              v-model:value="inbound.settings.gateway[j]" class="mt-4"
              :placeholder="j === 0 ? '10.0.0.1/16' : 'fc00::1/64'">
              <template #addonAfter>
                <a-button size="small" @click="inbound.settings.gateway.splice(j, 1)">
                  <template #icon>
                    <MinusOutlined />
                  </template>
                </a-button>
              </template>
            </a-input>
          </a-form-item>
          <a-form-item label="DNS">
            <a-button size="small" @click="inbound.settings.dns.push('')">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
            <a-input v-for="(_ip, j) in inbound.settings.dns" :key="`tun-dns-${j}`"
              v-model:value="inbound.settings.dns[j]" class="mt-4" :placeholder="j === 0 ? '1.1.1.1' : '8.8.8.8'">
              <template #addonAfter>
                <a-button size="small" @click="inbound.settings.dns.splice(j, 1)">
                  <template #icon>
                    <MinusOutlined />
                  </template>
                </a-button>
              </template>
            </a-input>
          </a-form-item>
          <a-form-item label="User level">
            <a-input-number v-model:value="inbound.settings.userLevel" :min="0" />
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip
                title="Windows-only. CIDRs added to the system routing table automatically so matching traffic goes through TUN.">
                Auto system routes
              </a-tooltip>
            </template>
            <a-button size="small" @click="inbound.settings.autoSystemRoutingTable.push('')">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
            <a-input v-for="(_ip, j) in inbound.settings.autoSystemRoutingTable" :key="`tun-rt-${j}`"
              v-model:value="inbound.settings.autoSystemRoutingTable[j]" class="mt-4"
              :placeholder="j === 0 ? '0.0.0.0/0' : '::/0'">
              <template #addonAfter>
                <a-button size="small" @click="inbound.settings.autoSystemRoutingTable.splice(j, 1)">
                  <template #icon>
                    <MinusOutlined />
                  </template>
                </a-button>
              </template>
            </a-input>
          </a-form-item>
          <a-form-item>
            <template #label>
              <a-tooltip
                title="Physical interface for outbound traffic. Use 'auto' to detect; auto-enabled when Auto system routes is set.">
                Auto outbounds interface
              </a-tooltip>
            </template>
            <a-input v-model:value="inbound.settings.autoOutboundsInterface" placeholder="auto" />
          </a-form-item>
        </a-form>

        <!-- WireGuard -->
        <a-form v-if="protocol === Protocols.WIREGUARD" :colon="false" :label-col="{ sm: { span: 8 } }"
          :wrapper-col="{ sm: { span: 14 } }" class="mt-12">
          <a-form-item>
            <template #label>
              Secret key
              <SyncOutlined class="random-icon" @click="regenInboundWg" />
            </template>
            <a-input v-model:value="inbound.settings.secretKey" />
          </a-form-item>
          <a-form-item label="Public key">
            <a-input v-model:value="inbound.settings.pubKey" disabled />
          </a-form-item>
          <a-form-item label="MTU">
            <a-input-number v-model:value="inbound.settings.mtu" />
          </a-form-item>
          <a-form-item label="No-kernel TUN">
            <a-switch v-model:checked="inbound.settings.noKernelTun" />
          </a-form-item>
          <a-form-item label="Peers">
            <a-button size="small" @click="inbound.settings.addPeer()">
              <template #icon>
                <PlusOutlined />
              </template>
              Add peer
            </a-button>
          </a-form-item>
          <div v-for="(peer, idx) in inbound.settings.peers" :key="idx" class="wg-peer">
            <a-divider style="margin: 8px 0">
              Peer {{ idx + 1 }}
              <DeleteOutlined v-if="inbound.settings.peers.length > 1" class="danger-icon"
                @click="inbound.settings.delPeer(idx)" />
            </a-divider>
            <a-form-item>
              <template #label>
                Secret key
                <SyncOutlined class="random-icon" @click="regenWgKeypair(peer)" />
              </template>
              <a-input v-model:value="peer.privateKey" />
            </a-form-item>
            <a-form-item label="Public key">
              <a-input v-model:value="peer.publicKey" />
            </a-form-item>
            <a-form-item label="PSK">
              <a-input v-model:value="peer.psk" />
            </a-form-item>
            <a-form-item label="Allowed IPs">
              <a-button size="small" @click="peer.allowedIPs.push('')">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
              <a-input v-for="(_ip, j) in peer.allowedIPs" :key="j" v-model:value="peer.allowedIPs[j]" class="mt-4">
                <template #addonAfter>
                  <a-button v-if="peer.allowedIPs.length > 1" size="small" @click="peer.allowedIPs.splice(j, 1)">
                    <template #icon>
                      <MinusOutlined />
                    </template>
                  </a-button>
                </template>
              </a-input>
            </a-form-item>
            <a-form-item label="Keep-alive">
              <a-input-number v-model:value="peer.keepAlive" :min="0" />
            </a-form-item>
          </div>
        </a-form>

      </a-tab-pane>

      <!-- ============================== STREAM ============================== -->
      <a-tab-pane v-if="canEnableStream" key="stream" :tab="t('pages.inbounds.streamTab')">
        <a-form :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }">
          <a-form-item v-if="protocol !== Protocols.HYSTERIA" label="Transmission">
            <a-select v-model:value="network" :style="{ width: '75%' }">
              <a-select-option value="tcp">TCP (RAW)</a-select-option>
              <a-select-option value="kcp">mKCP</a-select-option>
              <a-select-option value="ws">WebSocket</a-select-option>
              <a-select-option value="grpc">gRPC</a-select-option>
              <a-select-option value="httpupgrade">HTTPUpgrade</a-select-option>
              <a-select-option value="xhttp">XHTTP</a-select-option>
            </a-select>
          </a-form-item>

          <!-- TCP (RAW) — proxy-protocol + optional HTTP camouflage with full request/response editor -->
          <template v-if="network === 'tcp'">
            <a-form-item v-if="canEnableTls" label="Proxy Protocol">
              <a-switch v-model:checked="inbound.stream.tcp.acceptProxyProtocol" />
            </a-form-item>
            <a-form-item :label="`HTTP ${t('camouflage')}`">
              <a-switch :checked="inbound.stream.tcp.type === 'http'"
                @change="(v) => (inbound.stream.tcp.type = v ? 'http' : 'none')" />
            </a-form-item>

            <template v-if="inbound.stream.tcp.type === 'http'">
              <!-- Request -->
              <a-divider :style="{ margin: '0' }">{{ t('pages.inbounds.stream.general.request') }}</a-divider>
              <a-form-item :label="t('pages.inbounds.stream.tcp.version')">
                <a-input v-model:value="inbound.stream.tcp.request.version" />
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.stream.tcp.method')">
                <a-input v-model:value="inbound.stream.tcp.request.method" />
              </a-form-item>
              <a-form-item>
                <template #label>
                  {{ t('pages.inbounds.stream.tcp.path') }}
                  <a-button size="small" :style="{ marginLeft: '6px' }"
                    @click="inbound.stream.tcp.request.addPath('/')">
                    <template #icon>
                      <PlusOutlined />
                    </template>
                  </a-button>
                </template>
                <template v-for="(_p, idx) in inbound.stream.tcp.request.path" :key="`tcp-path-${idx}`">
                  <a-input v-model:value="inbound.stream.tcp.request.path[idx]" class="mb-4">
                    <template #addonAfter>
                      <a-button v-if="inbound.stream.tcp.request.path.length > 1" size="small"
                        @click="inbound.stream.tcp.request.removePath(idx)">
                        <template #icon>
                          <MinusOutlined />
                        </template>
                      </a-button>
                    </template>
                  </a-input>
                </template>
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.stream.tcp.requestHeader')">
                <a-button size="small" @click="inbound.stream.tcp.request.addHeader('Host', '')">
                  <template #icon>
                    <PlusOutlined />
                  </template>
                </a-button>
              </a-form-item>
              <a-form-item v-if="inbound.stream.tcp.request.headers.length > 0" :wrapper-col="{ span: 24 }">
                <a-input-group v-for="(h, idx) in inbound.stream.tcp.request.headers" :key="`tcp-rh-${idx}`" compact
                  class="mb-8">
                  <a-input :style="{ width: '45%' }" v-model:value="h.name"
                    :placeholder="t('pages.inbounds.stream.general.name')">
                    <template #addonBefore>{{ idx + 1 }}</template>
                  </a-input>
                  <a-input :style="{ width: '45%' }" v-model:value="h.value"
                    :placeholder="t('pages.inbounds.stream.general.value')" />
                  <a-button @click="inbound.stream.tcp.request.removeHeader(idx)">
                    <template #icon>
                      <MinusOutlined />
                    </template>
                  </a-button>
                </a-input-group>
              </a-form-item>

              <!-- Response -->
              <a-divider :style="{ margin: '0' }">{{ t('pages.inbounds.stream.general.response') }}</a-divider>
              <a-form-item :label="t('pages.inbounds.stream.tcp.version')">
                <a-input v-model:value="inbound.stream.tcp.response.version" />
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.stream.tcp.status')">
                <a-input v-model:value="inbound.stream.tcp.response.status" />
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.stream.tcp.statusDescription')">
                <a-input v-model:value="inbound.stream.tcp.response.reason" />
              </a-form-item>
              <a-form-item :label="t('pages.inbounds.stream.tcp.responseHeader')">
                <a-button size="small"
                  @click="inbound.stream.tcp.response.addHeader('Content-Type', 'application/octet-stream')">
                  <template #icon>
                    <PlusOutlined />
                  </template>
                </a-button>
              </a-form-item>
              <a-form-item v-if="inbound.stream.tcp.response.headers.length > 0" :wrapper-col="{ span: 24 }">
                <a-input-group v-for="(h, idx) in inbound.stream.tcp.response.headers" :key="`tcp-rsh-${idx}`" compact
                  class="mb-8">
                  <a-input :style="{ width: '45%' }" v-model:value="h.name"
                    :placeholder="t('pages.inbounds.stream.general.name')">
                    <template #addonBefore>{{ idx + 1 }}</template>
                  </a-input>
                  <a-input :style="{ width: '45%' }" v-model:value="h.value"
                    :placeholder="t('pages.inbounds.stream.general.value')" />
                  <a-button @click="inbound.stream.tcp.response.removeHeader(idx)">
                    <template #icon>
                      <MinusOutlined />
                    </template>
                  </a-button>
                </a-input-group>
              </a-form-item>
            </template>
          </template>

          <!-- mKCP -->
          <template v-if="network === 'kcp'">
            <a-form-item label="MTU">
              <a-input-number v-model:value="inbound.stream.kcp.mtu" :min="576" :max="1460" />
            </a-form-item>
            <a-form-item label="TTI (ms)">
              <a-input-number v-model:value="inbound.stream.kcp.tti" :min="10" :max="100" />
            </a-form-item>
            <a-form-item label="Uplink (MB/s)">
              <a-input-number v-model:value="inbound.stream.kcp.upCap" :min="0" />
            </a-form-item>
            <a-form-item label="Downlink (MB/s)">
              <a-input-number v-model:value="inbound.stream.kcp.downCap" :min="0" />
            </a-form-item>
            <a-form-item label="CWND Multiplier">
              <a-input-number v-model:value="inbound.stream.kcp.cwndMultiplier" :min="1" />
            </a-form-item>
            <a-form-item label="Max Sending Window">
              <a-input-number v-model:value="inbound.stream.kcp.maxSendingWindow" :min="0" />
            </a-form-item>
          </template>

          <!-- WebSocket -->
          <template v-if="network === 'ws'">
            <a-form-item label="Proxy Protocol">
              <a-switch v-model:checked="inbound.stream.ws.acceptProxyProtocol" />
            </a-form-item>
            <a-form-item :label="t('host')">
              <a-input v-model:value="inbound.stream.ws.host" />
            </a-form-item>
            <a-form-item :label="t('path')">
              <a-input v-model:value="inbound.stream.ws.path" />
            </a-form-item>
            <a-form-item label="Heartbeat Period">
              <a-input-number v-model:value="inbound.stream.ws.heartbeatPeriod" :min="0" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.stream.tcp.requestHeader')">
              <a-button size="small" @click="inbound.stream.ws.addHeader('', '')">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
            </a-form-item>
            <a-form-item v-if="inbound.stream.ws.headers.length > 0" :wrapper-col="{ span: 24 }">
              <a-input-group v-for="(h, idx) in inbound.stream.ws.headers" :key="`ws-h-${idx}`" compact class="mb-8">
                <a-input :style="{ width: '45%' }" v-model:value="h.name"
                  :placeholder="t('pages.inbounds.stream.general.name')">
                  <template #addonBefore>{{ idx + 1 }}</template>
                </a-input>
                <a-input :style="{ width: '45%' }" v-model:value="h.value"
                  :placeholder="t('pages.inbounds.stream.general.value')" />
                <a-button @click="inbound.stream.ws.removeHeader(idx)">
                  <template #icon>
                    <MinusOutlined />
                  </template>
                </a-button>
              </a-input-group>
            </a-form-item>
          </template>

          <!-- gRPC -->
          <template v-if="network === 'grpc'">
            <a-form-item label="Service Name">
              <a-input v-model:value="inbound.stream.grpc.serviceName" />
            </a-form-item>
            <a-form-item label="Authority">
              <a-input v-model:value="inbound.stream.grpc.authority" />
            </a-form-item>
            <a-form-item label="Multi Mode">
              <a-switch v-model:checked="inbound.stream.grpc.multiMode" />
            </a-form-item>
          </template>

          <!-- HTTPUpgrade -->
          <template v-if="network === 'httpupgrade'">
            <a-form-item label="Proxy Protocol">
              <a-switch v-model:checked="inbound.stream.httpupgrade.acceptProxyProtocol" />
            </a-form-item>
            <a-form-item :label="t('host')">
              <a-input v-model:value="inbound.stream.httpupgrade.host" />
            </a-form-item>
            <a-form-item :label="t('path')">
              <a-input v-model:value="inbound.stream.httpupgrade.path" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.stream.tcp.requestHeader')">
              <a-button size="small" @click="inbound.stream.httpupgrade.addHeader('', '')">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
            </a-form-item>
            <a-form-item v-if="inbound.stream.httpupgrade.headers.length > 0" :wrapper-col="{ span: 24 }">
              <a-input-group v-for="(h, idx) in inbound.stream.httpupgrade.headers" :key="`hu-h-${idx}`" compact
                class="mb-8">
                <a-input :style="{ width: '45%' }" v-model:value="h.name"
                  :placeholder="t('pages.inbounds.stream.general.name')">
                  <template #addonBefore>{{ idx + 1 }}</template>
                </a-input>
                <a-input :style="{ width: '45%' }" v-model:value="h.value"
                  :placeholder="t('pages.inbounds.stream.general.value')" />
                <a-button @click="inbound.stream.httpupgrade.removeHeader(idx)">
                  <template #icon>
                    <MinusOutlined />
                  </template>
                </a-button>
              </a-input-group>
            </a-form-item>
          </template>

          <!-- XHTTP -->
          <template v-if="network === 'xhttp'">
            <a-form-item :label="t('host')">
              <a-input v-model:value="inbound.stream.xhttp.host" />
            </a-form-item>
            <a-form-item :label="t('path')">
              <a-input v-model:value="inbound.stream.xhttp.path" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.stream.tcp.requestHeader')">
              <a-button size="small" @click="inbound.stream.xhttp.addHeader('', '')">
                <template #icon>
                  <PlusOutlined />
                </template>
              </a-button>
            </a-form-item>
            <a-form-item v-if="inbound.stream.xhttp.headers.length > 0" :wrapper-col="{ span: 24 }">
              <a-input-group v-for="(h, idx) in inbound.stream.xhttp.headers" :key="`xh-h-${idx}`" compact class="mb-8">
                <a-input :style="{ width: '45%' }" v-model:value="h.name"
                  :placeholder="t('pages.inbounds.stream.general.name')">
                  <template #addonBefore>{{ idx + 1 }}</template>
                </a-input>
                <a-input :style="{ width: '45%' }" v-model:value="h.value"
                  :placeholder="t('pages.inbounds.stream.general.value')" />
                <a-button @click="inbound.stream.xhttp.removeHeader(idx)">
                  <template #icon>
                    <MinusOutlined />
                  </template>
                </a-button>
              </a-input-group>
            </a-form-item>
            <a-form-item label="Mode">
              <a-select v-model:value="inbound.stream.xhttp.mode" :style="{ width: '50%' }">
                <a-select-option v-for="m in MODE_OPTIONS" :key="m" :value="m">{{ m }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item v-if="inbound.stream.xhttp.mode === 'packet-up'" label="Max Buffered Upload">
              <a-input-number v-model:value="inbound.stream.xhttp.scMaxBufferedPosts" />
            </a-form-item>
            <a-form-item v-if="inbound.stream.xhttp.mode === 'packet-up'" label="Max Upload Size (Byte)">
              <a-input v-model:value="inbound.stream.xhttp.scMaxEachPostBytes" />
            </a-form-item>
            <a-form-item v-if="inbound.stream.xhttp.mode === 'stream-up'" label="Stream-Up Server">
              <a-input v-model:value="inbound.stream.xhttp.scStreamUpServerSecs" />
            </a-form-item>
            <a-form-item label="Server Max Header Bytes">
              <a-input-number v-model:value="inbound.stream.xhttp.serverMaxHeaderBytes" :min="0"
                placeholder="0 (default)" />
            </a-form-item>
            <a-form-item label="Padding Bytes">
              <a-input v-model:value="inbound.stream.xhttp.xPaddingBytes" />
            </a-form-item>
            <a-form-item label="Padding Obfs Mode">
              <a-switch v-model:checked="inbound.stream.xhttp.xPaddingObfsMode" />
            </a-form-item>
            <template v-if="inbound.stream.xhttp.xPaddingObfsMode">
              <a-form-item label="Padding Key">
                <a-input v-model:value="inbound.stream.xhttp.xPaddingKey" placeholder="x_padding" />
              </a-form-item>
              <a-form-item label="Padding Header">
                <a-input v-model:value="inbound.stream.xhttp.xPaddingHeader" placeholder="X-Padding" />
              </a-form-item>
              <a-form-item label="Padding Placement">
                <a-select v-model:value="inbound.stream.xhttp.xPaddingPlacement">
                  <a-select-option value="">Default (queryInHeader)</a-select-option>
                  <a-select-option value="queryInHeader">queryInHeader</a-select-option>
                  <a-select-option value="header">header</a-select-option>
                  <a-select-option value="cookie">cookie</a-select-option>
                  <a-select-option value="query">query</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Padding Method">
                <a-select v-model:value="inbound.stream.xhttp.xPaddingMethod">
                  <a-select-option value="">Default (repeat-x)</a-select-option>
                  <a-select-option value="repeat-x">repeat-x</a-select-option>
                  <a-select-option value="tokenish">tokenish</a-select-option>
                </a-select>
              </a-form-item>
            </template>
            <a-form-item label="Session Placement">
              <a-select v-model:value="inbound.stream.xhttp.sessionPlacement">
                <a-select-option value="">Default (path)</a-select-option>
                <a-select-option value="path">path</a-select-option>
                <a-select-option value="header">header</a-select-option>
                <a-select-option value="cookie">cookie</a-select-option>
                <a-select-option value="query">query</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item
              v-if="inbound.stream.xhttp.sessionPlacement && inbound.stream.xhttp.sessionPlacement !== 'path'"
              label="Session Key">
              <a-input v-model:value="inbound.stream.xhttp.sessionKey" placeholder="x_session" />
            </a-form-item>
            <a-form-item label="Sequence Placement">
              <a-select v-model:value="inbound.stream.xhttp.seqPlacement">
                <a-select-option value="">Default (path)</a-select-option>
                <a-select-option value="path">path</a-select-option>
                <a-select-option value="header">header</a-select-option>
                <a-select-option value="cookie">cookie</a-select-option>
                <a-select-option value="query">query</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item v-if="inbound.stream.xhttp.seqPlacement && inbound.stream.xhttp.seqPlacement !== 'path'"
              label="Sequence Key">
              <a-input v-model:value="inbound.stream.xhttp.seqKey" placeholder="x_seq" />
            </a-form-item>
            <a-form-item v-if="inbound.stream.xhttp.mode === 'packet-up'" label="Uplink Data Placement">
              <a-select v-model:value="inbound.stream.xhttp.uplinkDataPlacement">
                <a-select-option value="">Default (body)</a-select-option>
                <a-select-option value="body">body</a-select-option>
                <a-select-option value="header">header</a-select-option>
                <a-select-option value="cookie">cookie</a-select-option>
                <a-select-option value="query">query</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item
              v-if="inbound.stream.xhttp.mode === 'packet-up' && inbound.stream.xhttp.uplinkDataPlacement && inbound.stream.xhttp.uplinkDataPlacement !== 'body'"
              label="Uplink Data Key">
              <a-input v-model:value="inbound.stream.xhttp.uplinkDataKey" placeholder="x_data" />
            </a-form-item>
            <a-form-item label="No SSE Header">
              <a-switch v-model:checked="inbound.stream.xhttp.noSSEHeader" />
            </a-form-item>
          </template>

          <!-- ====== External Proxy ====== -->
          <a-form-item label="External Proxy">
            <a-switch v-model:checked="externalProxy" />
            <a-button v-if="externalProxy" size="small" type="primary" :style="{ marginLeft: '10px' }"
              @click="inbound.stream.externalProxy.push({ forceTls: 'same', dest: '', port: 443, remark: '' })">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <a-form-item v-if="externalProxy" :wrapper-col="{ span: 24 }">
            <a-input-group v-for="(row, idx) in inbound.stream.externalProxy" :key="`ep-${idx}`" compact
              :style="{ margin: '8px 0' }">
              <a-tooltip title="Force TLS">
                <a-select v-model:value="row.forceTls" :style="{ width: '20%' }">
                  <a-select-option value="same">{{ t('pages.inbounds.same') }}</a-select-option>
                  <a-select-option value="none">{{ t('none') }}</a-select-option>
                  <a-select-option value="tls">TLS</a-select-option>
                </a-select>
              </a-tooltip>
              <a-input v-model:value="row.dest" :style="{ width: '30%' }" :placeholder="t('host')" />
              <a-tooltip :title="t('pages.inbounds.port')">
                <a-input-number v-model:value="row.port" :style="{ width: '15%' }" :min="1" :max="65535" />
              </a-tooltip>
              <a-input v-model:value="row.remark" :style="{ width: '35%' }" :placeholder="t('pages.inbounds.remark')">
                <template #addonAfter>
                  <MinusOutlined @click="inbound.stream.externalProxy.splice(idx, 1)" />
                </template>
              </a-input>
            </a-input-group>
          </a-form-item>

          <!-- ====== Sockopt ====== -->
          <a-form-item label="Sockopt">
            <a-switch v-model:checked="inbound.stream.sockoptSwitch" />
          </a-form-item>
          <template v-if="inbound.stream.sockoptSwitch && inbound.stream.sockopt">
            <a-form-item label="Route Mark">
              <a-input-number v-model:value="inbound.stream.sockopt.mark" :min="0" />
            </a-form-item>
            <a-form-item label="TCP Keep Alive Interval">
              <a-input-number v-model:value="inbound.stream.sockopt.tcpKeepAliveInterval" :min="0" />
            </a-form-item>
            <a-form-item label="TCP Keep Alive Idle">
              <a-input-number v-model:value="inbound.stream.sockopt.tcpKeepAliveIdle" :min="0" />
            </a-form-item>
            <a-form-item label="TCP Max Seg">
              <a-input-number v-model:value="inbound.stream.sockopt.tcpMaxSeg" :min="0" />
            </a-form-item>
            <a-form-item label="TCP User Timeout">
              <a-input-number v-model:value="inbound.stream.sockopt.tcpUserTimeout" :min="0" />
            </a-form-item>
            <a-form-item label="TCP Window Clamp">
              <a-input-number v-model:value="inbound.stream.sockopt.tcpWindowClamp" :min="0" />
            </a-form-item>
            <a-form-item label="Proxy Protocol">
              <a-switch v-model:checked="inbound.stream.sockopt.acceptProxyProtocol" />
            </a-form-item>
            <a-form-item label="TCP Fast Open">
              <a-switch v-model:checked="inbound.stream.sockopt.tcpFastOpen" />
            </a-form-item>
            <a-form-item label="Multipath TCP">
              <a-switch v-model:checked="inbound.stream.sockopt.tcpMptcp" />
            </a-form-item>
            <a-form-item label="Penetrate">
              <a-switch v-model:checked="inbound.stream.sockopt.penetrate" />
            </a-form-item>
            <a-form-item label="V6 Only">
              <a-switch v-model:checked="inbound.stream.sockopt.V6Only" />
            </a-form-item>
            <a-form-item label="Domain Strategy">
              <a-select v-model:value="inbound.stream.sockopt.domainStrategy" :style="{ width: '50%' }">
                <a-select-option v-for="d in DOMAIN_STRATEGIES" :key="d" :value="d">{{ d }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="TCP Congestion">
              <a-select v-model:value="inbound.stream.sockopt.tcpcongestion" :style="{ width: '50%' }">
                <a-select-option v-for="c in TCP_CONGESTIONS" :key="c" :value="c">{{ c }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="TProxy">
              <a-select v-model:value="inbound.stream.sockopt.tproxy" :style="{ width: '50%' }">
                <a-select-option value="off">Off</a-select-option>
                <a-select-option value="redirect">Redirect</a-select-option>
                <a-select-option value="tproxy">TProxy</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Dialer Proxy">
              <a-input v-model:value="inbound.stream.sockopt.dialerProxy" />
            </a-form-item>
            <a-form-item label="Interface Name">
              <a-input v-model:value="inbound.stream.sockopt.interfaceName" />
            </a-form-item>
            <a-form-item label="Trusted X-Forwarded-For">
              <a-select v-model:value="inbound.stream.sockopt.trustedXForwardedFor" mode="tags"
                :style="{ width: '100%' }" :token-separators="[',']">
                <a-select-option value="CF-Connecting-IP">CF-Connecting-IP</a-select-option>
                <a-select-option value="X-Real-IP">X-Real-IP</a-select-option>
                <a-select-option value="True-Client-IP">True-Client-IP</a-select-option>
                <a-select-option value="X-Client-IP">X-Client-IP</a-select-option>
              </a-select>
            </a-form-item>
          </template>

          <!-- ====== Hysteria stream settings ====== -->
          <!-- Per https://xtls.github.io/config/transports/hysteria.html -->
          <template v-if="protocol === Protocols.HYSTERIA">
            <a-form-item>
              <template #label>
                <a-tooltip title="Hysteria protocol version. Currently must be 2.">
                  Version
                </a-tooltip>
              </template>
              <a-input-number v-model:value="inbound.stream.hysteria.version" :min="2" :max="2" />
            </a-form-item>
            <a-form-item>
              <template #label>
                <a-tooltip title="Idle timeout (seconds) for a single QUIC native UDP connection.">
                  UDP idle timeout
                </a-tooltip>
              </template>
              <a-input-number v-model:value="inbound.stream.hysteria.udpIdleTimeout" :min="0" />
            </a-form-item>
            <a-form-item label="Masquerade">
              <a-switch v-model:checked="inbound.stream.hysteria.masqueradeSwitch" />
            </a-form-item>
            <template v-if="inbound.stream.hysteria.masqueradeSwitch">
              <a-form-item label="Type">
                <a-select v-model:value="inbound.stream.hysteria.masquerade.type" :style="{ width: '50%' }">
                  <a-select-option value="proxy">Proxy</a-select-option>
                  <a-select-option value="file">File</a-select-option>
                  <a-select-option value="string">String</a-select-option>
                </a-select>
              </a-form-item>

              <!-- Proxy type: url / rewriteHost / insecure -->
              <template v-if="inbound.stream.hysteria.masquerade.type === 'proxy'">
                <a-form-item label="URL">
                  <a-input v-model:value="inbound.stream.hysteria.masquerade.url" placeholder="https://example.com" />
                </a-form-item>
                <a-form-item label="Rewrite Host">
                  <a-switch v-model:checked="inbound.stream.hysteria.masquerade.rewriteHost" />
                </a-form-item>
                <a-form-item label="Insecure">
                  <a-switch v-model:checked="inbound.stream.hysteria.masquerade.insecure" />
                </a-form-item>
              </template>

              <!-- File type: dir -->
              <a-form-item v-if="inbound.stream.hysteria.masquerade.type === 'file'" label="Directory">
                <a-input v-model:value="inbound.stream.hysteria.masquerade.dir" placeholder="/path/to/www" />
              </a-form-item>

              <!-- String type: content / statusCode / headers -->
              <template v-if="inbound.stream.hysteria.masquerade.type === 'string'">
                <a-form-item label="Content">
                  <a-textarea v-model:value="inbound.stream.hysteria.masquerade.content"
                    :auto-size="{ minRows: 2, maxRows: 6 }" />
                </a-form-item>
                <a-form-item label="Status Code">
                  <a-input-number v-model:value="inbound.stream.hysteria.masquerade.statusCode" :min="100" :max="599"
                    placeholder="200" />
                </a-form-item>
                <a-form-item label="Headers">
                  <a-button size="small" @click="inbound.stream.hysteria.masquerade.addHeader('', '')">
                    <template #icon>
                      <PlusOutlined />
                    </template>
                  </a-button>
                </a-form-item>
                <a-form-item v-if="inbound.stream.hysteria.masquerade.headers.length > 0" :wrapper-col="{ span: 24 }">
                  <a-input-group v-for="(h, idx) in inbound.stream.hysteria.masquerade.headers" :key="`mh-${idx}`"
                    compact class="mb-8">
                    <a-input :style="{ width: '45%' }" v-model:value="h.name" placeholder="Name">
                      <template #addonBefore>{{ idx + 1 }}</template>
                    </a-input>
                    <a-input :style="{ width: '45%' }" v-model:value="h.value" placeholder="Value" />
                    <a-button @click="inbound.stream.hysteria.masquerade.removeHeader(idx)">
                      <template #icon>
                        <MinusOutlined />
                      </template>
                    </a-button>
                  </a-input-group>
                </a-form-item>
              </template>
            </template>
          </template>
        </a-form>

        <!-- ====== FinalMask (TCP/UDP masks + QUIC params) ====== -->
        <FinalMaskForm :stream="inbound.stream" :protocol="protocol" />
      </a-tab-pane>

      <!-- ============================== SECURITY ============================== -->
      <a-tab-pane v-if="canEnableStream" key="security" :tab="t('pages.inbounds.securityTab')">
        <a-form :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }">
          <a-form-item :label="t('pages.inbounds.securityTab')">
            <a-radio-group v-model:value="security" button-style="solid" :disabled="!canEnableTls">
              <a-radio-button value="none">none</a-radio-button>
              <a-radio-button value="tls">tls</a-radio-button>
              <a-radio-button v-if="canEnableReality" value="reality">reality</a-radio-button>
            </a-radio-group>
          </a-form-item>

          <template v-if="security === 'tls' && inbound.stream.tls">
            <a-form-item label="SNI">
              <a-input v-model:value="inbound.stream.tls.sni" placeholder="Server Name Indication" />
            </a-form-item>
            <a-form-item label="Cipher Suites">
              <a-select v-model:value="inbound.stream.tls.cipherSuites">
                <a-select-option value="">Auto</a-select-option>
                <a-select-option v-for="[label, val] in CIPHER_SUITES" :key="val" :value="val">{{ label
                }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Min/Max Version">
              <a-input-group compact>
                <a-select v-model:value="inbound.stream.tls.minVersion" :style="{ width: '50%' }">
                  <a-select-option v-for="v in TLS_VERSIONS" :key="v" :value="v">{{ v }}</a-select-option>
                </a-select>
                <a-select v-model:value="inbound.stream.tls.maxVersion" :style="{ width: '50%' }">
                  <a-select-option v-for="v in TLS_VERSIONS" :key="v" :value="v">{{ v }}</a-select-option>
                </a-select>
              </a-input-group>
            </a-form-item>
            <a-form-item label="uTLS">
              <a-select v-model:value="inbound.stream.tls.settings.fingerprint" :style="{ width: '100%' }">
                <a-select-option value="">None</a-select-option>
                <a-select-option v-for="fp in FINGERPRINTS" :key="fp" :value="fp">{{ fp }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="ALPN">
              <a-select v-model:value="inbound.stream.tls.alpn" mode="multiple" :style="{ width: '100%' }"
                :token-separators="[',']">
                <a-select-option v-for="a in ALPNS" :key="a" :value="a">{{ a }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Reject Unknown SNI">
              <a-switch v-model:checked="inbound.stream.tls.rejectUnknownSni" />
            </a-form-item>
            <a-form-item label="Disable System Root">
              <a-switch v-model:checked="inbound.stream.tls.disableSystemRoot" />
            </a-form-item>
            <a-form-item label="Session Resumption">
              <a-switch v-model:checked="inbound.stream.tls.enableSessionResumption" />
            </a-form-item>

            <template v-for="(cert, idx) in inbound.stream.tls.certs" :key="`cert-${idx}`">
              <a-form-item :label="t('certificate')">
                <a-radio-group v-model:value="cert.useFile" button-style="solid">
                  <a-radio-button :value="true">{{ t('pages.inbounds.certificatePath') }}</a-radio-button>
                  <a-radio-button :value="false">{{ t('pages.inbounds.certificateContent') }}</a-radio-button>
                </a-radio-group>
              </a-form-item>
              <a-form-item label=" ">
                <a-space>
                  <a-button v-if="idx === 0" type="primary" size="small" @click="inbound.stream.tls.addCert()">
                    <template #icon>
                      <PlusOutlined />
                    </template>
                  </a-button>
                  <a-button v-if="inbound.stream.tls.certs.length > 1" type="primary" size="small"
                    @click="inbound.stream.tls.removeCert(idx)">
                    <template #icon>
                      <MinusOutlined />
                    </template>
                  </a-button>
                </a-space>
              </a-form-item>
              <template v-if="cert.useFile">
                <a-form-item :label="t('pages.inbounds.publicKey')">
                  <a-input v-model:value="cert.certFile" />
                </a-form-item>
                <a-form-item :label="t('pages.inbounds.privatekey')">
                  <a-input v-model:value="cert.keyFile" />
                </a-form-item>
                <a-form-item label=" ">
                  <a-button type="primary" :disabled="!defaultCert && !defaultKey" @click="setDefaultCertData(idx)">
                    {{ t('pages.inbounds.setDefaultCert') }}
                  </a-button>
                </a-form-item>
              </template>
              <template v-else>
                <a-form-item :label="t('pages.inbounds.publicKey')">
                  <a-textarea v-model:value="cert.cert" :auto-size="{ minRows: 3, maxRows: 8 }" />
                </a-form-item>
                <a-form-item :label="t('pages.inbounds.privatekey')">
                  <a-textarea v-model:value="cert.key" :auto-size="{ minRows: 3, maxRows: 8 }" />
                </a-form-item>
              </template>
              <a-form-item label="One Time Loading">
                <a-switch v-model:checked="cert.oneTimeLoading" />
              </a-form-item>
              <a-form-item label="Usage Option">
                <a-select v-model:value="cert.usage" :style="{ width: '50%' }">
                  <a-select-option v-for="u in USAGES" :key="u" :value="u">{{ u }}</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item v-if="cert.usage === 'issue'" label="Build Chain">
                <a-switch v-model:checked="cert.buildChain" />
              </a-form-item>
            </template>

            <a-form-item label="ECH key">
              <a-input v-model:value="inbound.stream.tls.echServerKeys" />
            </a-form-item>
            <a-form-item label="ECH config">
              <a-input v-model:value="inbound.stream.tls.settings.echConfigList" />
            </a-form-item>
            <a-form-item label=" ">
              <a-space>
                <a-button type="primary" :loading="saving" @click="getNewEchCert">Get New ECH Cert</a-button>
                <a-button danger @click="clearEchCert">Clear</a-button>
              </a-space>
            </a-form-item>
          </template>

          <template v-if="security === 'reality' && inbound.stream.reality">
            <a-form-item label="Show">
              <a-switch v-model:checked="inbound.stream.reality.show" />
            </a-form-item>
            <a-form-item label="Xver">
              <a-input-number v-model:value="inbound.stream.reality.xver" :min="0" />
            </a-form-item>
            <a-form-item label="uTLS">
              <a-select v-model:value="inbound.stream.reality.settings.fingerprint" :style="{ width: '100%' }">
                <a-select-option v-for="fp in FINGERPRINTS" :key="fp" :value="fp">{{ fp }}</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item>
              <template #label>
                Target
                <SyncOutlined class="random-icon" @click="randomizeRealityTarget" />
              </template>
              <a-input v-model:value="inbound.stream.reality.target" />
            </a-form-item>
            <a-form-item>
              <template #label>
                SNI
                <SyncOutlined class="random-icon" @click="randomizeRealityTarget" />
              </template>
              <a-input v-model:value="inbound.stream.reality.serverNames" />
            </a-form-item>
            <a-form-item label="Max Time Diff (ms)">
              <a-input-number v-model:value="inbound.stream.reality.maxTimediff" :min="0" />
            </a-form-item>
            <a-form-item label="Min Client Ver">
              <a-input v-model:value="inbound.stream.reality.minClientVer" placeholder="25.9.11" />
            </a-form-item>
            <a-form-item label="Max Client Ver">
              <a-input v-model:value="inbound.stream.reality.maxClientVer" placeholder="25.9.11" />
            </a-form-item>
            <a-form-item>
              <template #label>
                Short IDs
                <SyncOutlined class="random-icon" @click="randomizeShortIds" />
              </template>
              <a-textarea v-model:value="inbound.stream.reality.shortIds" :auto-size="{ minRows: 1, maxRows: 4 }" />
            </a-form-item>
            <a-form-item label="SpiderX">
              <a-input v-model:value="inbound.stream.reality.settings.spiderX" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.publicKey')">
              <a-textarea v-model:value="inbound.stream.reality.settings.publicKey"
                :auto-size="{ minRows: 1, maxRows: 4 }" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.privatekey')">
              <a-textarea v-model:value="inbound.stream.reality.privateKey" :auto-size="{ minRows: 1, maxRows: 4 }" />
            </a-form-item>
            <a-form-item label=" ">
              <a-space>
                <a-button type="primary" :loading="saving" @click="genRealityKeypair">Get New Cert</a-button>
                <a-button danger @click="clearRealityKeypair">Clear</a-button>
              </a-space>
            </a-form-item>
            <a-form-item label="mldsa65 Seed">
              <a-textarea v-model:value="inbound.stream.reality.mldsa65Seed" :auto-size="{ minRows: 2, maxRows: 6 }" />
            </a-form-item>
            <a-form-item label="mldsa65 Verify">
              <a-textarea v-model:value="inbound.stream.reality.settings.mldsa65Verify"
                :auto-size="{ minRows: 2, maxRows: 6 }" />
            </a-form-item>
            <a-form-item label=" ">
              <a-space>
                <a-button type="primary" :loading="saving" @click="genMldsa65">Get New Seed</a-button>
                <a-button danger @click="clearMldsa65">Clear</a-button>
              </a-space>
            </a-form-item>
          </template>
        </a-form>
      </a-tab-pane>

      <!-- ============================== SNIFFING ============================== -->
      <a-tab-pane key="sniffing" :tab="t('pages.inbounds.sniffingTab')">
        <a-form :colon="false" :label-col="{ sm: { span: 8 } }" :wrapper-col="{ sm: { span: 14 } }">
          <a-form-item :label="t('enable')">
            <a-switch v-model:checked="inbound.sniffing.enabled" />
          </a-form-item>
          <template v-if="inbound.sniffing.enabled">
            <a-form-item :wrapper-col="{ span: 24 }">
              <a-checkbox-group v-model:value="inbound.sniffing.destOverride">
                <a-checkbox v-for="(value, key) in SNIFFING_OPTION" :key="key" :value="value">{{ key }}</a-checkbox>
              </a-checkbox-group>
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.sniffingMetadataOnly')">
              <a-switch v-model:checked="inbound.sniffing.metadataOnly" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.sniffingRouteOnly')">
              <a-switch v-model:checked="inbound.sniffing.routeOnly" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.sniffingIpsExcluded')">
              <a-select v-model:value="inbound.sniffing.ipsExcluded" mode="tags" :token-separators="[',']"
                placeholder="IP/CIDR/geoip:*/ext:*" :style="{ width: '100%' }" />
            </a-form-item>
            <a-form-item :label="t('pages.inbounds.sniffingDomainsExcluded')">
              <a-select v-model:value="inbound.sniffing.domainsExcluded" mode="tags" :token-separators="[',']"
                placeholder="domain:*/ext:*" :style="{ width: '100%' }" />
            </a-form-item>
          </template>
        </a-form>
      </a-tab-pane>

      <!-- ============================== ADVANCED ============================== -->
      <a-tab-pane key="advanced" :tab="t('pages.xray.advancedTemplate')">
        <div class="advanced-shell">
          <div class="advanced-panel">
            <div class="advanced-panel__header">
              <div>
                <div class="advanced-panel__title">{{ t('pages.inbounds.advanced.title') }}</div>
                <div class="advanced-panel__subtitle">
                  {{ t('pages.inbounds.advanced.subtitle') }}
                </div>
              </div>
            </div>

            <a-tabs v-model:active-key="advancedSectionKey" class="advanced-inner-tabs">
              <a-tab-pane key="all" :tab="t('pages.inbounds.advanced.all')">
                <div class="advanced-editor-meta">
                  {{ t('pages.inbounds.advanced.allHelp') }}
                </div>
                <JsonEditor v-model:value="advancedAllConfig" min-height="340px" max-height="560px" />
              </a-tab-pane>
              <a-tab-pane key="settings" :tab="t('pages.inbounds.advanced.settings')">
                <div class="advanced-editor-meta">
                  {{ t('pages.inbounds.advanced.settingsHelp') }}
                  <code>{ settings: { ... } }</code>.
                </div>
                <JsonEditor v-model:value="advancedSettingsConfig" min-height="320px" max-height="540px" />
              </a-tab-pane>
              <a-tab-pane key="sniffingSection" :tab="t('pages.inbounds.advanced.sniffing')">
                <div class="advanced-editor-meta">
                  {{ t('pages.inbounds.advanced.sniffingHelp') }}
                  <code>{ sniffing: { ... } }</code>.
                </div>
                <JsonEditor v-model:value="advancedSniffingConfig" min-height="240px" max-height="420px" />
              </a-tab-pane>
              <a-tab-pane v-if="canEnableStream" key="streamSection" :tab="t('pages.inbounds.advanced.stream')">
                <div class="advanced-editor-meta">
                  {{ t('pages.inbounds.advanced.streamHelp') }}
                  <code>{ streamSettings: { ... } }</code>.
                </div>
                <JsonEditor v-model:value="advancedStreamConfig" min-height="320px" max-height="540px" />
              </a-tab-pane>
            </a-tabs>
          </div>
        </div>
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<style scoped>
.mt-4 {
  margin-top: 4px;
}

.mt-8 {
  margin-top: 8px;
}

.mt-12 {
  margin-top: 12px;
}

.mb-4 {
  margin-bottom: 4px;
}

.mb-8 {
  margin-bottom: 8px;
}

.mb-12 {
  margin-bottom: 12px;
}

.random-icon {
  margin-left: 4px;
  cursor: pointer;
  color: var(--ant-primary-color, #1890ff);
}

.danger-icon {
  margin-left: 6px;
  cursor: pointer;
  color: #ff4d4f;
}

.vless-auth-state {
  display: block;
  margin-top: 6px;
}

.wg-peer {
  margin-top: 4px;
}

.advanced-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.advanced-panel {
  padding: 14px;
  border: 1px solid rgba(128, 128, 128, 0.18);
  border-radius: 12px;
  background: rgba(128, 128, 128, 0.04);
}

.advanced-panel__header {
  margin-bottom: 12px;
}

.advanced-panel__title {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
}

.advanced-panel__subtitle {
  margin-top: 4px;
  opacity: 0.7;
  line-height: 1.5;
}

.advanced-inner-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 12px;
}

.advanced-inner-tabs :deep(.ant-tabs-tab) {
  padding-inline: 14px;
}

.advanced-editor-meta {
  margin-bottom: 10px;
  opacity: 0.75;
  line-height: 1.5;
}

@media (max-width: 768px) {
  .advanced-panel {
    padding: 12px;
    border-radius: 10px;
  }

  .advanced-inner-tabs :deep(.ant-tabs-tab) {
    padding-inline: 10px;
  }
}

:global(body.dark) .advanced-panel,
:global(html[data-theme='ultra-dark']) .advanced-panel {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.03);
}

.section-heading {
  font-weight: 500;
  margin: 12px 0 6px;
  opacity: 0.85;
}
</style>
