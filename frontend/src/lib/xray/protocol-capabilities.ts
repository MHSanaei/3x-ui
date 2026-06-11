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
const STREAM_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks', 'hysteria', 'wireguard', 'tunnel'];
const VISION_FLOW = 'xtls-rprx-vision';
const SS_2022_PREFIX = '2022';
const SS_BLAKE3_CHACHA20 = '2022-blake3-chacha20-poly1305';

export interface CapabilityProtocolSlice {
  protocol: string;
  settings?: { encryption?: string; decryption?: string };
  streamSettings?: { network?: string; security?: string };
}

export interface CapabilityVlessSlice extends CapabilityProtocolSlice {
  settings?: { encryption?: string; decryption?: string; clients?: { flow?: string }[] };
}

export interface CapabilityShadowsocksSlice extends CapabilityProtocolSlice {
  settings?: { encryption?: string; method?: string };
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

// VLESS encryption (vlessenc / ML-KEM) is on when encryption or decryption holds
// a generated value (e.g. "mlkem768x25519plus.native.0rtt.<key>") rather than
// the "none"/"" sentinel. The value is never the literal "vlessenc" (that is the
// `xray vlessenc` subcommand). decryption is the server-side value; encryption is
// stored for link generation — either being set means it is on.
function hasVlessEncryption(settings: CapabilityProtocolSlice['settings']): boolean {
  const isSet = (v?: string) => v != null && v !== '' && v !== 'none';
  return isSet(settings?.encryption) || isSet(settings?.decryption);
}

export function canEnableTlsFlow(values: CapabilityProtocolSlice): boolean {
  if (values.protocol !== 'vless') return false;
  const network = values.streamSettings?.network;
  const security = values.streamSettings?.security;

  // Classic XTLS Vision: raw TCP carried over TLS or REALITY.
  if (network === 'tcp' && (security === 'tls' || security === 'reality')) return true;

  // vlessenc carries Vision over XHTTP without transport TLS.
  if (network === 'xhttp' && hasVlessEncryption(values.settings)) return true;

  return false;
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
