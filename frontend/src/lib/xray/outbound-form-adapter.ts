import { Wireguard } from '@/utils';

import type {
  DnsOutboundFormSettings,
  DnsRuleForm,
  FreedomFinalRuleForm,
  FreedomOutboundFormSettings,
  HysteriaOutboundFormSettings,
  LoopbackOutboundFormSettings,
  MuxForm,
  OutboundFormSettings,
  OutboundFormValues,
  OutboundStreamFormValues,
  ReverseSniffingForm,
  ShadowsocksOutboundFormSettings,
  TrojanOutboundFormSettings,
  VlessOutboundFormSettings,
  VmessOutboundFormSettings,
  WireguardOutboundFormPeer,
  WireguardOutboundFormSettings,
} from '@/schemas/forms/outbound-form';

type Raw = Record<string, unknown>;

function asObject(value: unknown): Raw {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Raw) : {};
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function asString(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function asNumber(value: unknown, fallback = 0): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string' && value.trim() !== '') {
    const n = Number(value);
    return Number.isFinite(n) ? n : fallback;
  }
  return fallback;
}

function asBool(value: unknown): boolean {
  return value === true;
}

function asPort(value: unknown, fallback: number): number {
  const n = asNumber(value, fallback);
  if (!Number.isInteger(n) || n < 1 || n > 65535) return fallback;
  return n;
}

const REVERSE_SNIFFING_DEFAULT: ReverseSniffingForm = {
  enabled: false,
  destOverride: ['http', 'tls', 'quic', 'fakedns'],
  metadataOnly: false,
  routeOnly: false,
  ipsExcluded: [],
  domainsExcluded: [],
};

function reverseSniffingFromWire(raw: unknown): ReverseSniffingForm {
  const r = asObject(raw);
  const dest = asArray(r.destOverride).map((x) => asString(x));
  return {
    enabled: asBool(r.enabled),
    destOverride: dest.length > 0 ? dest : ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: asBool(r.metadataOnly),
    routeOnly: asBool(r.routeOnly),
    ipsExcluded: asArray(r.ipsExcluded).map((x) => asString(x)),
    domainsExcluded: asArray(r.domainsExcluded).map((x) => asString(x)),
  };
}

function vmessFromWire(raw: Raw): VmessOutboundFormSettings {
  const vnext = asArray(raw.vnext);
  const v = asObject(vnext[0]);
  const u = asObject(asArray(v.users)[0]);
  return {
    address: asString(v.address),
    port: asPort(v.port, 443),
    id: asString(u.id),
    security: ((): VmessOutboundFormSettings['security'] => {
      const s = asString(u.security);
      const allowed = ['aes-128-gcm', 'chacha20-poly1305', 'auto', 'none', 'zero'];
      return (allowed.includes(s) ? s : 'auto') as VmessOutboundFormSettings['security'];
    })(),
  };
}

function vlessFromWire(raw: Raw): VlessOutboundFormSettings {
  let address = asString(raw.address);
  let port = asPort(raw.port, 443);
  let id = asString(raw.id);
  let flow = asString(raw.flow);
  let encryption = asString(raw.encryption, 'none');
  const vnext = asArray(raw.vnext);
  if (vnext.length > 0) {
    const v = asObject(vnext[0]);
    const u = asObject(asArray(v.users)[0]);
    address = asString(v.address);
    port = asPort(v.port, 443);
    id = asString(u.id);
    flow = asString(u.flow);
    encryption = asString(u.encryption, 'none');
  }
  const reverse = asObject(raw.reverse);
  const reverseTag = asString(reverse.tag);
  const reverseSniffing = reverseTag
    ? reverseSniffingFromWire(reverse.sniffing)
    : REVERSE_SNIFFING_DEFAULT;
  const savedSeed = asArray(raw.testseed);
  const testseed = savedSeed.length === 4
    && savedSeed.every((n) => Number.isInteger(n) && (n as number) > 0)
    ? (savedSeed as number[])
    : [];
  return {
    address,
    port,
    id,
    flow,
    encryption: (encryption === 'none' ? 'none' : 'none') as 'none',
    reverseTag,
    reverseSniffing,
    testpre: asNumber(raw.testpre, 0),
    testseed,
  };
}

function trojanFromWire(raw: Raw): TrojanOutboundFormSettings {
  const s = asObject(asArray(raw.servers)[0]);
  return {
    address: asString(s.address),
    port: asPort(s.port, 443),
    password: asString(s.password),
  };
}

function shadowsocksFromWire(raw: Raw): ShadowsocksOutboundFormSettings {
  const s = asObject(asArray(raw.servers)[0]);
  return {
    address: asString(s.address),
    port: asPort(s.port, 443),
    password: asString(s.password),
    method: asString(s.method, '2022-blake3-aes-128-gcm') as ShadowsocksOutboundFormSettings['method'],
    uot: asBool(s.uot),
    UoTVersion: asNumber(s.UoTVersion, 1),
  };
}

interface SimpleAuthFormSettings {
  address: string;
  port: number;
  user: string;
  pass: string;
}

function simpleAuthFromWire(raw: Raw, defaultPort: number): SimpleAuthFormSettings {
  const s = asObject(asArray(raw.servers)[0]);
  const u = asObject(asArray(s.users)[0]);
  return {
    address: asString(s.address),
    port: asPort(s.port, defaultPort),
    user: asString(u.user),
    pass: asString(u.pass),
  };
}

function wireguardFromWire(raw: Raw): WireguardOutboundFormSettings {
  const secretKey = asString(raw.secretKey);
  const pubKey = secretKey.length > 0
    ? Wireguard.generateKeypair(secretKey).publicKey
    : '';
  const addressArr = asArray(raw.address).map((x) =>
    typeof x === 'number' ? String(x) : asString(x),
  );
  const reservedArr = asArray(raw.reserved).map((x) =>
    typeof x === 'number' ? String(x) : asString(x),
  );
  const peers: WireguardOutboundFormPeer[] = asArray(raw.peers).map((p) => {
    const pp = asObject(p);
    const allowed = asArray(pp.allowedIPs).map((x) => asString(x));
    return {
      publicKey: asString(pp.publicKey),
      psk: asString(pp.preSharedKey),
      allowedIPs: allowed.length > 0 ? allowed : ['0.0.0.0/0', '::/0'],
      endpoint: asString(pp.endpoint),
      keepAlive: asNumber(pp.keepAlive, 0),
    };
  });
  return {
    mtu: asNumber(raw.mtu, 1420),
    secretKey,
    pubKey,
    address: addressArr.join(','),
    workers: asNumber(raw.workers, 2),
    domainStrategy: ((): WireguardOutboundFormSettings['domainStrategy'] => {
      const allowed = ['ForceIP', 'ForceIPv4', 'ForceIPv4v6', 'ForceIPv6', 'ForceIPv6v4'];
      const s = asString(raw.domainStrategy);
      return (allowed.includes(s) ? s : '') as WireguardOutboundFormSettings['domainStrategy'];
    })(),
    reserved: reservedArr.join(','),
    peers,
    noKernelTun: asBool(raw.noKernelTun),
  };
}

function hysteriaFromWire(raw: Raw): HysteriaOutboundFormSettings {
  return {
    address: asString(raw.address),
    port: asPort(raw.port, 443),
    version: 2,
  };
}

function freedomFromWire(raw: Raw): FreedomOutboundFormSettings {
  const fragment = asObject(raw.fragment);
  const noises = asArray(raw.noises).map((n) => {
    const nn = asObject(n);
    return {
      type: (asString(nn.type, 'rand') as FreedomOutboundFormSettings['noises'][number]['type']),
      packet: asString(nn.packet, '10-20'),
      delay: asString(nn.delay, '10-16'),
      applyTo: (asString(nn.applyTo, 'ip') as FreedomOutboundFormSettings['noises'][number]['applyTo']),
    };
  });
  const finalRulesRaw = asArray(raw.finalRules);
  const finalRules: FreedomFinalRuleForm[] = finalRulesRaw.map((r) => {
    const rr = asObject(r);
    const network = Array.isArray(rr.network)
      ? rr.network.map((x) => asString(x)).join(',')
      : asString(rr.network);
    return {
      action: (asString(rr.action, 'block') === 'allow' ? 'allow' : 'block') as FreedomFinalRuleForm['action'],
      network,
      port: asString(rr.port),
      ip: asArray(rr.ip).map((x) => asString(x)),
      blockDelay: asString(rr.blockDelay),
    };
  });
  // Legacy ipsBlocked → finalRule(block) backfill
  if (finalRules.length === 0) {
    const ipsBlocked = asArray(raw.ipsBlocked).map((x) => asString(x));
    if (ipsBlocked.length > 0) {
      finalRules.push({ action: 'block', network: '', port: '', ip: ipsBlocked, blockDelay: '' });
    }
  }
  // Wire fragment is either missing or a populated object. Mirror the
  // legacy behavior: when the wire omits fragment, leave all four fields
  // empty so the modal's "Fragment" Switch starts off. When present,
  // surface whatever the wire holds verbatim.
  const wireHasFragment = raw.fragment != null
    && typeof raw.fragment === 'object'
    && Object.keys(fragment).length > 0;
  return {
    domainStrategy: ((): FreedomOutboundFormSettings['domainStrategy'] => {
      const allowed = [
        'AsIs', 'UseIP', 'UseIPv4', 'UseIPv6', 'UseIPv6v4', 'UseIPv4v6',
        'ForceIP', 'ForceIPv6v4', 'ForceIPv6', 'ForceIPv4v6', 'ForceIPv4',
      ];
      const s = asString(raw.domainStrategy);
      return (allowed.includes(s) ? s : '') as FreedomOutboundFormSettings['domainStrategy'];
    })(),
    redirect: asString(raw.redirect),
    fragment: wireHasFragment
      ? {
          packets: asString(fragment.packets, '1-3'),
          length: asString(fragment.length),
          interval: asString(fragment.interval),
          maxSplit: asString(fragment.maxSplit),
        }
      : { packets: '', length: '', interval: '', maxSplit: '' },
    noises,
    finalRules,
  };
}

function blackholeFromWire(raw: Raw) {
  const response = asObject(raw.response);
  const t = asString(response.type);
  return { type: (t === 'none' || t === 'http' ? t : '') as '' | 'none' | 'http' };
}

function dnsRuleFromWire(raw: unknown): DnsRuleForm {
  const r = asObject(raw);
  const qtype = Array.isArray(r.qtype)
    ? r.qtype.map((x) => String(x)).join(',')
    : typeof r.qtype === 'number'
      ? String(r.qtype)
      : asString(r.qtype);
  const domain = Array.isArray(r.domain)
    ? r.domain.map((x) => asString(x)).join(',')
    : asString(r.domain);
  const action = asString(r.action, 'direct');
  const validAction = ['direct', 'reject', 'rejectIPv4', 'rejectIPv6'].includes(action)
    ? action
    : 'direct';
  return { action: validAction as DnsRuleForm['action'], qtype, domain };
}

function dnsFromWire(raw: Raw): DnsOutboundFormSettings {
  const rules = asArray(raw.rules).map(dnsRuleFromWire);
  return {
    rewriteNetwork: ((): DnsOutboundFormSettings['rewriteNetwork'] => {
      const s = asString(raw.rewriteNetwork ?? raw.network);
      return (s === 'udp' || s === 'tcp') ? s : '';
    })(),
    rewriteAddress: asString(raw.rewriteAddress ?? raw.address),
    rewritePort: asPort(raw.rewritePort ?? raw.port, 53),
    userLevel: asNumber(raw.userLevel, 0),
    rules,
  };
}

function loopbackFromWire(raw: Raw): LoopbackOutboundFormSettings {
  return { inboundTag: asString(raw.inboundTag) };
}

function muxFromWire(raw: unknown): MuxForm {
  const m = asObject(raw);
  return {
    enabled: asBool(m.enabled),
    concurrency: asNumber(m.concurrency, 8),
    xudpConcurrency: asNumber(m.xudpConcurrency, 16),
    xudpProxyUDP443: ((): MuxForm['xudpProxyUDP443'] => {
      const s = asString(m.xudpProxyUDP443, 'reject');
      return (['reject', 'allow', 'skip'].includes(s) ? s : 'reject') as MuxForm['xudpProxyUDP443'];
    })(),
  };
}

export interface RawOutboundRow {
  tag?: string;
  protocol?: string;
  sendThrough?: string;
  settings?: unknown;
  streamSettings?: unknown;
  mux?: unknown;
}

export function rawOutboundToFormValues(raw: RawOutboundRow): OutboundFormValues {
  const protocol = asString(raw.protocol, 'vless');
  const settings = asObject(raw.settings);
  const tag = asString(raw.tag);
  const sendThrough = asString(raw.sendThrough);
  const mux = muxFromWire(raw.mux);
  const hasStream = raw.streamSettings
    && typeof raw.streamSettings === 'object'
    && Object.keys(raw.streamSettings as Raw).length > 0;
  const streamSettings = hasStream
    ? (raw.streamSettings as unknown as OutboundStreamFormValues)
    : undefined;

  let typed: OutboundFormSettings;
  switch (protocol) {
    case 'vmess':       typed = { protocol: 'vmess',       settings: vmessFromWire(settings) }; break;
    case 'vless':       typed = { protocol: 'vless',       settings: vlessFromWire(settings) }; break;
    case 'trojan':      typed = { protocol: 'trojan',      settings: trojanFromWire(settings) }; break;
    case 'shadowsocks': typed = { protocol: 'shadowsocks', settings: shadowsocksFromWire(settings) }; break;
    case 'socks':       typed = { protocol: 'socks',       settings: simpleAuthFromWire(settings, 1080) }; break;
    case 'http':        typed = { protocol: 'http',        settings: simpleAuthFromWire(settings, 8080) }; break;
    case 'wireguard':   typed = { protocol: 'wireguard',   settings: wireguardFromWire(settings) }; break;
    case 'hysteria':    typed = { protocol: 'hysteria',    settings: hysteriaFromWire(settings) }; break;
    case 'freedom':     typed = { protocol: 'freedom',     settings: freedomFromWire(settings) }; break;
    case 'blackhole':   typed = { protocol: 'blackhole',   settings: blackholeFromWire(settings) }; break;
    case 'dns':         typed = { protocol: 'dns',         settings: dnsFromWire(settings) }; break;
    case 'loopback':    typed = { protocol: 'loopback',    settings: loopbackFromWire(settings) }; break;
    default:            typed = { protocol: 'vless',       settings: vlessFromWire(settings) };
  }

  return {
    ...typed,
    tag,
    sendThrough,
    mux,
    streamSettings,
  };
}

// --- Form values -> wire payload --------------------------------------

function vmessToWire(s: VmessOutboundFormSettings) {
  return {
    vnext: [{
      address: s.address,
      port: s.port,
      users: [{ id: s.id, security: s.security }],
    }],
  };
}

function reverseSniffingToWire(s: ReverseSniffingForm) {
  return {
    enabled: s.enabled,
    destOverride: s.destOverride,
    metadataOnly: s.metadataOnly,
    routeOnly: s.routeOnly,
    ipsExcluded: s.ipsExcluded.length > 0 ? s.ipsExcluded : undefined,
    domainsExcluded: s.domainsExcluded.length > 0 ? s.domainsExcluded : undefined,
  };
}

function vlessToWire(s: VlessOutboundFormSettings) {
  const result: Raw = {
    address: s.address,
    port: s.port,
    id: s.id,
    flow: s.flow,
    encryption: s.encryption || 'none',
  };
  if (s.reverseTag) {
    const sn = reverseSniffingToWire(s.reverseSniffing);
    const defaultSn = reverseSniffingToWire(REVERSE_SNIFFING_DEFAULT);
    result.reverse = {
      tag: s.reverseTag,
      sniffing: JSON.stringify(sn) === JSON.stringify(defaultSn) ? {} : sn,
    };
  }
  if (s.flow === 'xtls-rprx-vision') {
    if (s.testpre > 0) result.testpre = s.testpre;
    if (s.testseed.length === 4 && s.testseed.every((v) => Number.isInteger(v) && v > 0)) {
      result.testseed = s.testseed;
    }
  }
  return result;
}

function trojanToWire(s: TrojanOutboundFormSettings) {
  return { servers: [{ address: s.address, port: s.port, password: s.password }] };
}

function shadowsocksToWire(s: ShadowsocksOutboundFormSettings) {
  return {
    servers: [{
      address: s.address,
      port: s.port,
      password: s.password,
      method: s.method,
      uot: s.uot,
      UoTVersion: s.UoTVersion,
    }],
  };
}

function simpleAuthToWire(s: SimpleAuthFormSettings) {
  return {
    servers: [{
      address: s.address,
      port: s.port,
      users: s.user ? [{ user: s.user, pass: s.pass }] : [],
    }],
  };
}

function wireguardToWire(s: WireguardOutboundFormSettings) {
  return {
    mtu: s.mtu || undefined,
    secretKey: s.secretKey,
    address: s.address ? s.address.split(',').map((x) => x.trim()).filter(Boolean) : [],
    workers: s.workers || undefined,
    domainStrategy: s.domainStrategy || undefined,
    reserved: s.reserved
      ? s.reserved.split(',').map((x) => Number(x.trim())).filter((n) => Number.isFinite(n))
      : undefined,
    peers: s.peers.map((p) => ({
      publicKey: p.publicKey,
      preSharedKey: p.psk.length > 0 ? p.psk : undefined,
      allowedIPs: p.allowedIPs.length > 0 ? p.allowedIPs : undefined,
      endpoint: p.endpoint,
      keepAlive: p.keepAlive || undefined,
    })),
    noKernelTun: s.noKernelTun,
  };
}

function hysteriaToWire(s: HysteriaOutboundFormSettings) {
  return { address: s.address, port: s.port, version: s.version };
}

function freedomToWire(s: FreedomOutboundFormSettings) {
  // Legacy semantics: emit fragment only when the user actually populated
  // at least one of the four sub-fields. Defaults like packets='1-3' alone
  // are not enough — the modal's Fragment Switch sets all four together.
  const fragmentEntries = Object.entries(s.fragment).filter(([, v]) => v !== '' && v != null);
  const fragmentEnabled = !!s.fragment.length || !!s.fragment.interval || !!s.fragment.maxSplit;
  return {
    domainStrategy: s.domainStrategy || undefined,
    redirect: s.redirect || undefined,
    fragment: fragmentEnabled ? Object.fromEntries(fragmentEntries) : undefined,
    noises: s.noises.length > 0 ? s.noises : undefined,
    finalRules: s.finalRules.length > 0
      ? s.finalRules.map((r) => ({
          action: r.action,
          network: r.network || undefined,
          port: r.port || undefined,
          ip: r.ip.length > 0 ? r.ip : undefined,
          blockDelay: r.action === 'block' && r.blockDelay ? r.blockDelay : undefined,
        }))
      : undefined,
  };
}

function blackholeToWire(s: { type: '' | 'none' | 'http' }) {
  return { response: s.type ? { type: s.type } : undefined };
}

function dnsRuleToWire(r: DnsRuleForm) {
  const action = ['direct', 'reject', 'rejectIPv4', 'rejectIPv6'].includes(r.action)
    ? r.action
    : 'direct';
  const result: Raw = { action };
  const qtype = r.qtype.trim();
  if (qtype) {
    result.qtype = /^\d+$/.test(qtype) ? Number(qtype) : qtype;
  }
  const domains = r.domain.split(',').map((d) => d.trim()).filter(Boolean);
  if (domains.length > 0) result.domain = domains;
  return result;
}

function dnsToWire(s: DnsOutboundFormSettings) {
  const result: Raw = {};
  if (s.rewriteNetwork) result.rewriteNetwork = s.rewriteNetwork;
  if (s.rewriteAddress) result.rewriteAddress = s.rewriteAddress;
  if (s.rewritePort) result.rewritePort = s.rewritePort;
  if (s.userLevel) result.userLevel = s.userLevel;
  if (s.rules.length > 0) result.rules = s.rules.map(dnsRuleToWire);
  return result;
}

function loopbackToWire(s: LoopbackOutboundFormSettings) {
  return { inboundTag: s.inboundTag || undefined };
}

// canEnableMux mirrors the legacy Outbound.canEnableMux().
const MUX_PROTOCOLS = new Set(['vmess', 'vless', 'trojan', 'shadowsocks', 'http', 'socks']);
const STREAM_PROTOCOLS = new Set(['vmess', 'vless', 'trojan', 'shadowsocks', 'hysteria']);

function dropEmptyStrings(obj: Raw): Raw {
  const out: Raw = {};
  for (const [k, v] of Object.entries(obj)) {
    if (v === '') continue;
    out[k] = v;
  }
  return out;
}

function stripUiOnlyStreamFields(stream: unknown): Raw {
  const next = { ...(stream as Raw) };
  const xh = next.xhttpSettings;
  if (xh && typeof xh === 'object') {
    const cleaned = { ...(xh as Raw) };
    delete cleaned.enableXmux;
    next.xhttpSettings = dropEmptyStrings(cleaned);
  }
  return next;
}

function muxAllowed(values: OutboundFormValues): boolean {
  if (!MUX_PROTOCOLS.has(values.protocol)) return false;
  const flow = values.protocol === 'vless'
    ? (values.settings as VlessOutboundFormSettings).flow
    : '';
  if (flow) return false;
  const network = values.streamSettings && 'network' in values.streamSettings
    ? values.streamSettings.network
    : undefined;
  if (network === 'xhttp') return false;
  return true;
}

export type WireOutboundPayload = Raw;

export function formValuesToWirePayload(values: OutboundFormValues): WireOutboundPayload {
  let settings: Raw;
  switch (values.protocol) {
    case 'vmess':       settings = vmessToWire(values.settings); break;
    case 'vless':       settings = vlessToWire(values.settings); break;
    case 'trojan':      settings = trojanToWire(values.settings); break;
    case 'shadowsocks': settings = shadowsocksToWire(values.settings); break;
    case 'socks':       settings = simpleAuthToWire(values.settings); break;
    case 'http':        settings = simpleAuthToWire(values.settings); break;
    case 'wireguard':   settings = wireguardToWire(values.settings); break;
    case 'hysteria':    settings = hysteriaToWire(values.settings); break;
    case 'freedom':     settings = freedomToWire(values.settings); break;
    case 'blackhole':   settings = blackholeToWire(values.settings); break;
    case 'dns':         settings = dnsToWire(values.settings); break;
    case 'loopback':    settings = loopbackToWire(values.settings); break;
  }

  const result: Raw = {
    protocol: values.protocol,
    settings,
  };
  if (values.tag) result.tag = values.tag;

  // streamSettings emission gates on canEnableStream — non-stream protocols
  // still emit just `sockopt` if that key is present (legacy behavior).
  if (values.streamSettings) {
    if (STREAM_PROTOCOLS.has(values.protocol)) {
      result.streamSettings = stripUiOnlyStreamFields(values.streamSettings);
    } else {
      const sockopt = (values.streamSettings as { sockopt?: unknown }).sockopt;
      if (sockopt) result.streamSettings = { sockopt };
    }
  }

  if (values.sendThrough) result.sendThrough = values.sendThrough;
  // mux may be absent when the modal didn't render the Mux switch (non-
  // stream protocols or when isMuxAllowed gated it out). validateFields()
  // only returns registered fields, so values.mux can be undefined.
  if (values.mux?.enabled && muxAllowed(values)) {
    result.mux = values.mux;
  }
  return result;
}
