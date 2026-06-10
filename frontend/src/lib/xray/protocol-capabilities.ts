// Pure-function ports of the legacy Inbound class capability predicates
// (canEnableTls, canEnableReality, canEnableTlsFlow, canEnableStream,
// canEnableVisionSeed, isSS2022, isSSMultiUser). Each accepts the minimal
// slice of an InboundFormValues it needs, so the same predicate can be
// called against a partial-row, a full form value, or a hand-built test
// fixture without the caller projecting a whole object.

const TLS_ELIGIBLE_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks'];
const TLS_NETWORKS = ['tcp', 'ws', 'http', 'grpc', 'httpupgrade', 'xhttp'];
const REALITY_ELIGIBLE_PROTOCOLS = ['vless', 'trojan'];
const REALITY_NETWORKS = ['tcp', 'http', 'grpc', 'xhttp'];
const STREAM_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks', 'hysteria', 'wireguard'];
const VISION_FLOW = 'xtls-rprx-vision';
const SS_2022_PREFIX = '2022';
const SS_BLAKE3_CHACHA20 = '2022-blake3-chacha20-poly1305';

export interface CapabilityProtocolSlice {
  protocol: string;
  streamSettings?: { network?: string; security?: string };
}

export interface CapabilityVlessSlice extends CapabilityProtocolSlice {
  settings?: { clients?: { flow?: string }[] };
}

export interface CapabilityShadowsocksSlice {
  protocol: string;
  settings?: { method?: string };
}

export function canEnableTls(values: CapabilityProtocolSlice): boolean {
  if (values.protocol === 'hysteria') return true;
  if (!TLS_ELIGIBLE_PROTOCOLS.includes(values.protocol)) return false;
  return TLS_NETWORKS.includes(values.streamSettings?.network ?? '');
}

export function canEnableReality(values: CapabilityProtocolSlice): boolean {
  if (!REALITY_ELIGIBLE_PROTOCOLS.includes(values.protocol)) return false;
  return REALITY_NETWORKS.includes(values.streamSettings?.network ?? '');
}

export function canEnableTlsFlow(values: CapabilityProtocolSlice): boolean {
  const security = values.streamSettings?.security;
  if (security !== 'tls' && security !== 'reality') return false;
  if (values.streamSettings?.network !== 'tcp') return false;
  return values.protocol === 'vless';
}

export function canEnableStream(values: { protocol: string }): boolean {
  return STREAM_PROTOCOLS.includes(values.protocol);
}

// mtproto is served by an external mtg process, not Xray, so the Xray sniffing
// block does not apply to it. Every other inbound supports sniffing.
export function canEnableSniffing(values: { protocol: string }): boolean {
  return values.protocol !== 'mtproto';
}

// Vision seed applies only when XTLS Vision (TCP/TLS) flow is selected
// AND at least one VLESS client uses the vision flow. Excludes UDP variant.
export function canEnableVisionSeed(values: CapabilityVlessSlice): boolean {
  if (!canEnableTlsFlow(values)) return false;
  const clients = values.settings?.clients;
  if (!Array.isArray(clients)) return false;
  return clients.some((c) => c?.flow === VISION_FLOW);
}

// Why: legacy returns true on non-SS protocols too (the method getter
// resolves to "" and "" !== blake3-chacha20-poly1305). Preserved for
// parity with the legacy class; in practice the callers all narrow on
// protocol === shadowsocks before checking.
export function isSSMultiUser(values: CapabilityShadowsocksSlice): boolean {
  const method = values.protocol === 'shadowsocks' ? (values.settings?.method ?? '') : '';
  return method !== SS_BLAKE3_CHACHA20;
}

export function isSS2022(values: CapabilityShadowsocksSlice): boolean {
  const method = values.protocol === 'shadowsocks' ? (values.settings?.method ?? '') : '';
  return method.substring(0, 4) === SS_2022_PREFIX;
}
