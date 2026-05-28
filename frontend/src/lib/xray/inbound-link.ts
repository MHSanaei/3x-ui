import { Base64, Wireguard } from '@/utils';

import type { Inbound } from '@/schemas/api/inbound';
import type { VlessClient } from '@/schemas/protocols/inbound/vless';
import type { VmessSecurity } from '@/schemas/protocols/inbound/vmess';
import type {
  WireguardInboundPeer,
  WireguardInboundSettings,
} from '@/schemas/protocols/inbound/wireguard';
import type { ExternalProxyEntry } from '@/schemas/protocols/stream/external-proxy';
import type { FinalMaskStreamSettings } from '@/schemas/protocols/stream/finalmask';
import type { XHttpStreamSettings } from '@/schemas/protocols/stream/xhttp';

import { getHeaderValue } from './headers';

// Share-link generators. Each per-protocol fn takes a typed inbound plus
// client overrides and returns a URL (or '' when the protocol doesn't
// support shareable links). The helpers below were previously static
// methods on the Inbound class; extracting them removes the
// XrayCommonClass dependency and lets these run against Zod-parsed data
// directly.

type ForceTls = 'same' | 'tls' | 'none';

// xHTTP headers ship as Record<string, string> on the wire (Zod schema)
// rather than the legacy class's HeaderEntry[]. Lookup by case-folded key.
function xhttpHostFallback(xhttp: XHttpStreamSettings | undefined): string {
  return getHeaderValue(xhttp?.headers, 'host');
}

// Pull the bidirectional SplitHTTPConfig fields out of xhttp into a
// compact extra payload. Server-only fields (noSSEHeader, scMaxBufferedPosts,
// scStreamUpServerSecs, serverMaxHeaderBytes) are excluded — the client
// reading the share link wouldn't honor them. Mirrors the legacy
// Inbound.buildXhttpExtra exactly so the shadow link snapshots line up.
function buildXhttpExtra(xhttp: XHttpStreamSettings | undefined): Record<string, unknown> | null {
  if (!xhttp) return null;
  const extra: Record<string, unknown> = {};

  if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
    extra.xPaddingBytes = xhttp.xPaddingBytes;
  }
  if (xhttp.xPaddingObfsMode === true) {
    extra.xPaddingObfsMode = true;
    for (const k of ['xPaddingKey', 'xPaddingHeader', 'xPaddingPlacement', 'xPaddingMethod'] as const) {
      const v = xhttp[k];
      if (typeof v === 'string' && v.length > 0) extra[k] = v;
    }
  }

  const stringFields = [
    'uplinkHTTPMethod',
    'sessionPlacement',
    'sessionKey',
    'seqPlacement',
    'seqKey',
    'uplinkDataPlacement',
    'uplinkDataKey',
    'scMaxEachPostBytes',
  ] as const;
  for (const k of stringFields) {
    const v = xhttp[k];
    if (typeof v === 'string' && v.length > 0) extra[k] = v;
  }

  // Headers on the wire are a record; emit them as a map upstream's
  // SplitHTTPConfig.headers expects, dropping Host (already on the URL).
  if (xhttp.headers && Object.keys(xhttp.headers).length > 0) {
    const headersMap: Record<string, string> = {};
    for (const [name, value] of Object.entries(xhttp.headers)) {
      if (name.toLowerCase() === 'host') continue;
      headersMap[name] = value;
    }
    if (Object.keys(headersMap).length > 0) extra.headers = headersMap;
  }

  return Object.keys(extra).length > 0 ? extra : null;
}

function applyXhttpExtraToObj(xhttp: XHttpStreamSettings | undefined, obj: Record<string, unknown>): void {
  if (!xhttp) return;
  if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
    obj.x_padding_bytes = xhttp.xPaddingBytes;
  }
  const extra = buildXhttpExtra(xhttp);
  if (!extra) return;
  for (const [k, v] of Object.entries(extra)) obj[k] = v;
}

// Recursively checks whether a finalmask payload has any non-empty
// content. Empty arrays / empty objects / empty strings all return false;
// any truthy primitive returns true. Used to decide whether the link
// should carry an `fm` blob at all.
function hasShareableFinalMaskValue(value: unknown): boolean {
  if (value == null) return false;
  if (Array.isArray(value)) return value.some(hasShareableFinalMaskValue);
  if (typeof value === 'object') {
    return Object.values(value as Record<string, unknown>).some(hasShareableFinalMaskValue);
  }
  if (typeof value === 'string') return value.length > 0;
  return true;
}

function serializeFinalMask(finalmask: FinalMaskStreamSettings | undefined): string {
  if (!finalmask) return '';
  return hasShareableFinalMaskValue(finalmask) ? JSON.stringify(finalmask) : '';
}

function applyFinalMaskToObj(
  finalmask: FinalMaskStreamSettings | undefined,
  obj: Record<string, unknown>,
): void {
  const payload = serializeFinalMask(finalmask);
  if (payload.length > 0) obj.fm = payload;
}

function externalProxyAlpn(value: ExternalProxyEntry['alpn']): string {
  if (Array.isArray(value)) return value.filter(Boolean).join(',');
  return '';
}

function applyExternalProxyTLSObj(
  externalProxy: ExternalProxyEntry | null | undefined,
  obj: Record<string, unknown>,
  security: string,
): void {
  if (!externalProxy || security !== 'tls') return;
  const sni = externalProxy.sni && externalProxy.sni.length > 0 ? externalProxy.sni : externalProxy.dest;
  if (sni && sni.length > 0) obj.sni = sni;
  if (externalProxy.fingerprint && externalProxy.fingerprint.length > 0) obj.fp = externalProxy.fingerprint;
  const alpn = externalProxyAlpn(externalProxy.alpn);
  if (alpn.length > 0) obj.alpn = alpn;
}

export interface GenVmessLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientId: string;
  security?: VmessSecurity;
  externalProxy?: ExternalProxyEntry | null;
}

// VMess share link: `vmess://` followed by base64-encoded JSON. The JSON
// schema is the v2rayN-compatible "v2" shape. Returns '' if the inbound
// is not vmess so dispatcher code can fall through cleanly.
export function genVmessLink(input: GenVmessLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientId,
    security,
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'vmess') return '';

  const stream = inbound.streamSettings;
  if (!stream) return '';

  const tls = forceTls === 'same' ? stream.security : forceTls;
  const obj: Record<string, unknown> = {
    v: '2',
    ps: remark,
    add: address,
    port,
    id: clientId,
    scy: security,
    net: stream.network,
    tls,
  };

  if (stream.network === 'tcp') {
    const tcp = stream.tcpSettings;
    const header = tcp.header;
    if (header) {
      obj.type = header.type;
      if (header.type === 'http') {
        const request = header.request;
        if (request) {
          obj.path = request.path.join(',');
          const host =
            getHeaderValue(header.response?.headers, 'host')
            || getHeaderValue(request.headers, 'host');
          if (host) obj.host = host;
        }
      }
    } else {
      obj.type = 'none';
    }
  } else if (stream.network === 'kcp') {
    const kcp = stream.kcpSettings;
    obj.mtu = kcp.mtu;
    obj.tti = kcp.tti;
  } else if (stream.network === 'ws') {
    const ws = stream.wsSettings;
    obj.path = ws.path;
    obj.host = ws.host.length > 0 ? ws.host : getHeaderValue(ws.headers, 'host');
  } else if (stream.network === 'grpc') {
    const grpc = stream.grpcSettings;
    obj.path = grpc.serviceName;
    obj.authority = grpc.authority;
    if (grpc.multiMode) obj.type = 'multi';
  } else if (stream.network === 'httpupgrade') {
    const hu = stream.httpupgradeSettings;
    obj.path = hu.path;
    obj.host = hu.host.length > 0 ? hu.host : getHeaderValue(hu.headers, 'host');
  } else if (stream.network === 'xhttp') {
    const xhttp = stream.xhttpSettings;
    obj.path = xhttp.path;
    obj.host = xhttp.host.length > 0 ? xhttp.host : xhttpHostFallback(xhttp);
    obj.type = xhttp.mode;
    applyXhttpExtraToObj(xhttp, obj);
  }

  applyFinalMaskToObj(stream.finalmask, obj);

  if (tls === 'tls' && stream.security === 'tls') {
    const tlsSettings = stream.tlsSettings;
    if (tlsSettings.serverName.length > 0) obj.sni = tlsSettings.serverName;
    if (tlsSettings.settings.fingerprint.length > 0) obj.fp = tlsSettings.settings.fingerprint;
    if (tlsSettings.alpn.length > 0) obj.alpn = tlsSettings.alpn.join(',');
    if (tlsSettings.settings.pinnedPeerCertSha256.length > 0) {
      obj.pcs = tlsSettings.settings.pinnedPeerCertSha256.join(',');
    }
  }

  applyExternalProxyTLSObj(externalProxy, obj, tls);

  return 'vmess://' + Base64.encode(JSON.stringify(obj, null, 2));
}

// Param-style helpers (vless/trojan/ss/hysteria links). These mirror the
// legacy applyXhttpExtraToParams / applyFinalMaskToParams /
// applyExternalProxyTLSParams but write to a URLSearchParams instance
// directly. Number values get coerced via .toString() on set — same as
// what URLSearchParams does internally so the resulting URL bytes match.

function applyXhttpExtraToParams(xhttp: XHttpStreamSettings | undefined, params: URLSearchParams): void {
  if (!xhttp) return;
  params.set('path', xhttp.path);
  const host = xhttp.host.length > 0 ? xhttp.host : xhttpHostFallback(xhttp);
  params.set('host', host);
  params.set('mode', xhttp.mode);
  if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
    params.set('x_padding_bytes', xhttp.xPaddingBytes);
  }
  const extra = buildXhttpExtra(xhttp);
  if (extra) params.set('extra', JSON.stringify(extra));
}

function applyFinalMaskToParams(finalmask: FinalMaskStreamSettings | undefined, params: URLSearchParams): void {
  const payload = serializeFinalMask(finalmask);
  if (payload.length > 0) params.set('fm', payload);
}

function applyExternalProxyTLSParams(
  externalProxy: ExternalProxyEntry | null | undefined,
  params: URLSearchParams,
  security: string,
): void {
  if (!externalProxy || security !== 'tls') return;
  const sni = externalProxy.sni && externalProxy.sni.length > 0 ? externalProxy.sni : externalProxy.dest;
  if (sni && sni.length > 0) params.set('sni', sni);
  if (externalProxy.fingerprint && externalProxy.fingerprint.length > 0) params.set('fp', externalProxy.fingerprint);
  const alpn = externalProxyAlpn(externalProxy.alpn);
  if (alpn.length > 0) params.set('alpn', alpn);
}

export interface GenVlessLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientId: string;
  flow?: VlessClient['flow'];
  externalProxy?: ExternalProxyEntry | null;
}

// VLESS share link: vless://<uuid>@<host>:<port>?<query>#<remark>. The
// query carries network type, encryption, network-specific knobs, and
// security-specific knobs (TLS fingerprint/alpn/sni or Reality
// pbk/sid/spx). Returns '' if the inbound isn't vless.
export function genVlessLink(input: GenVlessLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientId,
    flow = '',
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'vless') return '';
  const stream = inbound.streamSettings;
  if (!stream) return '';

  const security = forceTls === 'same' ? stream.security : forceTls;
  const params = new URLSearchParams();
  params.set('type', stream.network);
  params.set('encryption', inbound.settings.encryption);

  if (stream.network === 'tcp') {
    const tcp = stream.tcpSettings;
    if (tcp.header?.type === 'http') {
      const request = tcp.header.request;
      if (request) {
        params.set('path', request.path.join(','));
        const host =
          getHeaderValue(tcp.header.response?.headers, 'host')
          || getHeaderValue(request.headers, 'host');
        if (host) params.set('host', host);
        params.set('headerType', 'http');
      }
    }
  } else if (stream.network === 'kcp') {
    const kcp = stream.kcpSettings;
    params.set('mtu', String(kcp.mtu));
    params.set('tti', String(kcp.tti));
  } else if (stream.network === 'ws') {
    const ws = stream.wsSettings;
    params.set('path', ws.path);
    params.set('host', ws.host.length > 0 ? ws.host : getHeaderValue(ws.headers, 'host'));
  } else if (stream.network === 'grpc') {
    const grpc = stream.grpcSettings;
    params.set('serviceName', grpc.serviceName);
    params.set('authority', grpc.authority);
    if (grpc.multiMode) params.set('mode', 'multi');
  } else if (stream.network === 'httpupgrade') {
    const hu = stream.httpupgradeSettings;
    params.set('path', hu.path);
    params.set('host', hu.host.length > 0 ? hu.host : getHeaderValue(hu.headers, 'host'));
  } else if (stream.network === 'xhttp') {
    applyXhttpExtraToParams(stream.xhttpSettings, params);
  }

  applyFinalMaskToParams(stream.finalmask, params);

  if (security === 'tls') {
    params.set('security', 'tls');
    if (stream.security === 'tls') {
      const tls = stream.tlsSettings;
      params.set('fp', tls.settings.fingerprint);
      params.set('alpn', tls.alpn.join(','));
      if (tls.serverName.length > 0) params.set('sni', tls.serverName);
      if (tls.settings.echConfigList.length > 0) params.set('ech', tls.settings.echConfigList);
      if (tls.settings.pinnedPeerCertSha256.length > 0) {
        params.set('pcs', tls.settings.pinnedPeerCertSha256.join(','));
      }
      if (stream.network === 'tcp' && flow.length > 0) params.set('flow', flow);
    }
    applyExternalProxyTLSParams(externalProxy, params, security);
  } else if (security === 'reality') {
    params.set('security', 'reality');
    if (stream.security === 'reality') {
      const reality = stream.realitySettings;
      params.set('pbk', reality.settings.publicKey);
      params.set('fp', reality.settings.fingerprint);

      const sni =
        reality.settings.serverName ||
        reality.serverNames?.[0] ||
        reality.target?.split(':')[0];

      if (sni && sni.length > 0) params.set('sni', sni);

      if (reality.shortIds.length > 0) params.set('sid', reality.shortIds[0]);
      if (reality.settings.spiderX.length > 0) params.set('spx', reality.settings.spiderX);
      if (reality.settings.mldsa65Verify.length > 0) params.set('pqv', reality.settings.mldsa65Verify);
      if (stream.network === 'tcp' && flow.length > 0) params.set('flow', flow);
    }
  } else {
    params.set('security', 'none');
  }

  const url = new URL(`vless://${clientId}@${address}:${port}`);
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

// Shared network-branch writer used by trojan + shadowsocks links.
// VLESS and VMess don't call this because they have minor per-protocol
// quirks inline (vmess maps `multi` differently into obj.type; vless sets
// encryption=none up-front).
function writeNetworkParams(stream: NonNullable<Inbound['streamSettings']>, params: URLSearchParams): void {
  if (stream.network === 'tcp') {
    const tcp = stream.tcpSettings;
    if (tcp.header?.type === 'http') {
      const request = tcp.header.request;
      if (request) {
        params.set('path', request.path.join(','));
        const host =
          getHeaderValue(tcp.header.response?.headers, 'host')
          || getHeaderValue(request.headers, 'host');
        if (host) params.set('host', host);
        params.set('headerType', 'http');
      }
    }
  } else if (stream.network === 'kcp') {
    const kcp = stream.kcpSettings;
    params.set('mtu', String(kcp.mtu));
    params.set('tti', String(kcp.tti));
  } else if (stream.network === 'ws') {
    const ws = stream.wsSettings;
    params.set('path', ws.path);
    params.set('host', ws.host.length > 0 ? ws.host : getHeaderValue(ws.headers, 'host'));
  } else if (stream.network === 'grpc') {
    const grpc = stream.grpcSettings;
    params.set('serviceName', grpc.serviceName);
    params.set('authority', grpc.authority);
    if (grpc.multiMode) params.set('mode', 'multi');
  } else if (stream.network === 'httpupgrade') {
    const hu = stream.httpupgradeSettings;
    params.set('path', hu.path);
    params.set('host', hu.host.length > 0 ? hu.host : getHeaderValue(hu.headers, 'host'));
  } else if (stream.network === 'xhttp') {
    applyXhttpExtraToParams(stream.xhttpSettings, params);
  }
}

function writeTlsParams(stream: NonNullable<Inbound['streamSettings']>, params: URLSearchParams): void {
  if (stream.security !== 'tls') return;
  const tls = stream.tlsSettings;
  params.set('fp', tls.settings.fingerprint);
  params.set('alpn', tls.alpn.join(','));
  if (tls.settings.echConfigList.length > 0) params.set('ech', tls.settings.echConfigList);
  if (tls.serverName.length > 0) params.set('sni', tls.serverName);
  if (tls.settings.pinnedPeerCertSha256.length > 0) {
    params.set('pcs', tls.settings.pinnedPeerCertSha256.join(','));
  }
}

// Reality query-string writer shared by VLESS and Trojan. Preserves the
// legacy SNI-omission quirk (see genVlessLink for the full story).
function writeRealityParams(stream: NonNullable<Inbound['streamSettings']>, params: URLSearchParams): void {
  if (stream.security !== 'reality') return;
  const reality = stream.realitySettings;
  params.set('pbk', reality.settings.publicKey);
  params.set('fp', reality.settings.fingerprint);

  const sni =
    reality.settings.serverName ||
    reality.serverNames?.[0] ||
    reality.target?.split(':')[0];

  if (sni && sni.length > 0) params.set('sni', sni);

  if (reality.shortIds.length > 0) params.set('sid', reality.shortIds[0]);
  if (reality.settings.spiderX.length > 0) params.set('spx', reality.settings.spiderX);
  if (reality.settings.mldsa65Verify.length > 0) params.set('pqv', reality.settings.mldsa65Verify);
}

export interface GenTrojanLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientPassword: string;
  externalProxy?: ExternalProxyEntry | null;
}

// Trojan share link: trojan://<password>@<host>:<port>?<query>#<remark>.
// Same query-string shape as VLESS minus the `encryption` and `flow`
// fields. Returns '' if the inbound isn't trojan.
export function genTrojanLink(input: GenTrojanLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientPassword,
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'trojan') return '';
  const stream = inbound.streamSettings;
  if (!stream) return '';

  const security = forceTls === 'same' ? stream.security : forceTls;
  const params = new URLSearchParams();
  params.set('type', stream.network);

  writeNetworkParams(stream, params);
  applyFinalMaskToParams(stream.finalmask, params);

  if (security === 'tls') {
    params.set('security', 'tls');
    writeTlsParams(stream, params);
    applyExternalProxyTLSParams(externalProxy, params, security);
  } else if (security === 'reality') {
    params.set('security', 'reality');
    writeRealityParams(stream, params);
  } else {
    params.set('security', 'none');
  }

  const url = new URL(`trojan://${encodeURIComponent(clientPassword)}@${address}:${port}`);
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export interface GenShadowsocksLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientPassword?: string;
  externalProxy?: ExternalProxyEntry | null;
}

// Shadowsocks 2022 share link. The userinfo portion is base64(method:pw)
// for single-user and base64(method:settingsPw:clientPw) for multi-user
// 2022-blake3. Legacy SS (non-2022) leaves the password out of the
// userinfo entirely — matches the legacy class's password-array logic.
// Note: legacy `isSSMultiUser` returns true for everything except
// 2022-blake3-chacha20-poly1305 (a curious classification, but we
// preserve it for byte-stable parity).
export function genShadowsocksLink(input: GenShadowsocksLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientPassword = '',
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'shadowsocks') return '';
  const stream = inbound.streamSettings;
  if (!stream) return '';
  const settings = inbound.settings;

  const security = forceTls === 'same' ? stream.security : forceTls;
  const params = new URLSearchParams();
  params.set('type', stream.network);

  writeNetworkParams(stream, params);
  applyFinalMaskToParams(stream.finalmask, params);

  if (security === 'tls') {
    params.set('security', 'tls');
    writeTlsParams(stream, params);
    applyExternalProxyTLSParams(externalProxy, params, security);
  }

  const isSS2022 = settings.method.substring(0, 4) === '2022';
  const isSSMultiUser = settings.method !== '2022-blake3-chacha20-poly1305';
  const passwords: string[] = [];
  if (isSS2022) passwords.push(settings.password);
  if (isSSMultiUser) passwords.push(clientPassword);

  const userinfo = Base64.encode(`${settings.method}:${passwords.join(':')}`, true);
  const url = new URL(`ss://${userinfo}@${address}:${port}`);
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export interface GenHysteriaLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  remark?: string;
  clientAuth: string;
}

// Hysteria share link: hysteria://<auth>@<host>:<port>?<query>#<remark>.
// The URL scheme is "hysteria2" when settings.version === 2 (hysteria v2
// AKA hysteria2), "hysteria" otherwise. Salamander obfuscation pulls its
// password from finalmask.udp[type=salamander] when present; the broader
// finalmask payload still rides under `fm` like the other links.
//
// Note: legacy genHysteriaLink reads stream.tls.settings.allowInsecure,
// which isn't a field on TlsStreamSettings.Settings — the guard is always
// false. We omit the `insecure` param here to stay byte-stable.
export function genHysteriaLink(input: GenHysteriaLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    remark = '',
    clientAuth,
  } = input;

  if (inbound.protocol !== 'hysteria') return '';
  const stream = inbound.streamSettings;
  if (!stream || stream.security !== 'tls') return '';

  const settings = inbound.settings;
  const scheme = settings.version === 2 ? 'hysteria2' : 'hysteria';

  const params = new URLSearchParams();
  params.set('security', 'tls');
  const tls = stream.tlsSettings;
  if (tls.settings.fingerprint.length > 0) params.set('fp', tls.settings.fingerprint);
  if (tls.alpn.length > 0) params.set('alpn', tls.alpn.join(','));
  if (tls.settings.echConfigList.length > 0) params.set('ech', tls.settings.echConfigList);
  if (tls.serverName.length > 0) params.set('sni', tls.serverName);

  const udpMasks = stream.finalmask?.udp;
  if (Array.isArray(udpMasks)) {
    const salamander = udpMasks.find((m) => m?.type === 'salamander');
    const obfsPassword = salamander?.settings?.password;
    if (typeof obfsPassword === 'string' && obfsPassword.length > 0) {
      params.set('obfs', 'salamander');
      params.set('obfs-password', obfsPassword);
    }
  }

  applyFinalMaskToParams(stream.finalmask, params);

  const url = new URL(`${scheme}://${clientAuth}@${address}:${port}`);
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export interface GenWireguardLinkInput {
  settings: WireguardInboundSettings;
  address: string;
  port: number;
  remark?: string;
  peerIndex: number;
}

// Wireguard share link: wireguard://<peerPrivKey>@<host>:<port>
//   ?publickey=<serverPub>&address=<peerAllowedIP>&mtu=<mtu>#<remark>
// pubKey is derived from the server's secretKey via Wireguard.generateKeypair
// at call time (Zod's schema stores secretKey only — pubKey isn't on the
// wire). Returns '' when the peer index is out of bounds.
export function genWireguardLink(input: GenWireguardLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const peer = settings.peers[peerIndex];
  if (!peer) return '';

  const url = new URL(`wireguard://${address}:${port}`);
  url.username = peer.privateKey ?? '';

  const pubKey = settings.secretKey.length > 0
    ? Wireguard.generateKeypair(settings.secretKey).publicKey
    : '';
  if (pubKey.length > 0) url.searchParams.set('publickey', pubKey);
  if (peer.allowedIPs.length > 0 && peer.allowedIPs[0]) {
    url.searchParams.set('address', peer.allowedIPs[0]);
  }
  if (typeof settings.mtu === 'number' && settings.mtu > 0) {
    url.searchParams.set('mtu', String(settings.mtu));
  }

  url.hash = encodeURIComponent(remark);
  return url.toString();
}

// Plain-text WireGuard client config (.conf format). Mirrors the legacy
// getWireguardTxt — same DNS defaults (1.1.1.1, 1.0.0.1), MTU optional,
// presharedKey + keepAlive only emitted when present on the peer. The
// final newline structure follows the legacy: no newline after Endpoint,
// optional preSharedKey appended with leading \n, keepAlive appended
// with leading \n AND trailing \n.
export function genWireguardConfig(input: GenWireguardLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const peer = settings.peers[peerIndex];
  if (!peer) return '';

  const pubKey = settings.secretKey.length > 0
    ? Wireguard.generateKeypair(settings.secretKey).publicKey
    : '';

  let txt = `[Interface]\n`;
  txt += `PrivateKey = ${peer.privateKey ?? ''}\n`;
  txt += `Address = ${peer.allowedIPs[0] ?? ''}\n`;
  txt += `DNS = 1.1.1.1, 1.0.0.1\n`;
  if (typeof settings.mtu === 'number' && settings.mtu > 0) {
    txt += `MTU = ${settings.mtu}\n`;
  }
  txt += `\n# ${remark}\n`;
  txt += `[Peer]\n`;
  txt += `PublicKey = ${pubKey}\n`;
  txt += `AllowedIPs = 0.0.0.0/0, ::/0\n`;
  txt += `Endpoint = ${address}:${port}`;
  if (peer.preSharedKey && peer.preSharedKey.length > 0) {
    txt += `\nPresharedKey = ${peer.preSharedKey}`;
  }
  if (typeof peer.keepAlive === 'number' && peer.keepAlive > 0) {
    txt += `\nPersistentKeepalive = ${peer.keepAlive}\n`;
  }
  return txt;
}

export type { WireguardInboundPeer };

// Orchestrators.
// resolveAddr picks the host that goes into share/sub links. Order:
//   1. hostOverride (caller supplies node address for node-managed inbounds)
//   2. inbound's bind listen (when explicit, not 0.0.0.0)
//   3. fallbackHostname (caller-supplied — typically window.location.hostname
//      in the browser; tests pass a fixed value)
export function resolveAddr(inbound: Inbound, hostOverride: string, fallbackHostname: string): string {
  if (hostOverride.length > 0) return hostOverride;
  if (inbound.listen.length > 0 && inbound.listen !== '0.0.0.0') return inbound.listen;
  return fallbackHostname;
}

// Returns the client array for protocols that have one. SS returns its
// clients only in 2022-blake3 multi-user mode (matches the legacy
// `this.clients` getter, which used isSSMultiUser to gate). Returns null
// for SS single-user, http, mixed, tunnel, wireguard, hysteria2-without-
// clients, and any protocol without a clients array.
type ClientShape = { id?: string; security?: VmessSecurity; flow?: VlessClient['flow']; password?: string; auth?: string; email?: string };

export function getInboundClients(inbound: Inbound): ClientShape[] | null {
  switch (inbound.protocol) {
    case 'vmess':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'vless':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'trojan':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'hysteria':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'shadowsocks': {
      const isMultiUser = inbound.settings.method !== '2022-blake3-chacha20-poly1305';
      return isMultiUser ? ((inbound.settings.clients ?? []) as ClientShape[]) : null;
    }
    default:
      return null;
  }
}

export interface GenLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  client: ClientShape;
  externalProxy?: ExternalProxyEntry | null;
}

// Per-protocol dispatcher matching the legacy `genLink` switch. Returns
// '' for protocols that don't have client-based share links (wireguard
// goes through genWireguardLinks/Configs separately, http/mixed/tunnel
// don't have share URLs).
export function genLink(input: GenLinkInput): string {
  const { inbound, address, port = inbound.port, forceTls = 'same', remark = '', client, externalProxy = null } = input;
  switch (inbound.protocol) {
    case 'vmess':
      return genVmessLink({
        inbound, address, port, forceTls, remark,
        clientId: client.id ?? '',
        security: client.security,
        externalProxy,
      });
    case 'vless':
      return genVlessLink({
        inbound, address, port, forceTls, remark,
        clientId: client.id ?? '',
        flow: client.flow,
        externalProxy,
      });
    case 'shadowsocks': {
      const isMultiUser = inbound.settings.method !== '2022-blake3-chacha20-poly1305';
      return genShadowsocksLink({
        inbound, address, port, forceTls, remark,
        clientPassword: isMultiUser ? (client.password ?? '') : '',
        externalProxy,
      });
    }
    case 'trojan':
      return genTrojanLink({
        inbound, address, port, forceTls, remark,
        clientPassword: client.password ?? '',
        externalProxy,
      });
    case 'hysteria':
      return genHysteriaLink({
        inbound, address, port, remark,
        clientAuth: client.auth ?? '',
      });
    default:
      return '';
  }
}

export interface GenAllLinksEntry {
  remark: string;
  link: string;
}

export interface GenAllLinksInput {
  inbound: Inbound;
  remark?: string;
  remarkModel?: string;
  client: ClientShape;
  hostOverride?: string;
  fallbackHostname: string;
}

// Fans out a single client's link per externalProxy entry, or just one
// link when there are no external proxies. remarkModel is a 4-char
// string: first char is the separator, remaining chars pick which
// pieces to compose into the per-link remark — 'i' = inbound remark,
// 'e' = client email, 'o' = externalProxy remark. Defaults to '-io'
// (dash-separated, inbound + email + proxy).
export function genAllLinks(input: GenAllLinksInput): GenAllLinksEntry[] {
  const {
    inbound,
    remark = '',
    remarkModel = '-io',
    client,
    hostOverride = '',
    fallbackHostname,
  } = input;

  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const port = inbound.port;
  const separationChar = remarkModel.charAt(0);
  const orderChars = remarkModel.slice(1);
  const email = client.email ?? '';

  const composeRemark = (proxyRemark: string): string => {
    const orders: Record<string, string> = { i: remark, e: email, o: proxyRemark };
    return orderChars.split('')
      .map((c) => orders[c] ?? '')
      .filter((x) => x.length > 0)
      .join(separationChar);
  };

  const externals = inbound.streamSettings?.externalProxy;
  if (!externals || externals.length === 0) {
    const r = composeRemark('');
    return [{ remark: r, link: genLink({ inbound, address: addr, port, forceTls: 'same', remark: r, client }) }];
  }
  return externals.map((ep) => {
    const r = composeRemark(ep.remark);
    return {
      remark: r,
      link: genLink({
        inbound,
        address: ep.dest,
        port: ep.port,
        forceTls: ep.forceTls,
        remark: r,
        client,
        externalProxy: ep,
      }),
    };
  });
}

export interface GenInboundLinksInput {
  inbound: Inbound;
  remark?: string;
  remarkModel?: string;
  hostOverride?: string;
  fallbackHostname: string;
}

// Top-level entrypoint that produces the full \r\n-joined block a user
// pastes into a client. Iterates per-client for protocols with clients,
// falls back to a single SS link for single-user 2022-blake3-chacha20,
// and emits per-peer .conf blocks for wireguard. Returns '' for the
// other clientless protocols (http, mixed, tunnel).
export function genInboundLinks(input: GenInboundLinksInput): string {
  const {
    inbound,
    remark = '',
    remarkModel = '-io',
    hostOverride = '',
    fallbackHostname,
  } = input;
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const clients = getInboundClients(inbound);
  if (clients) {
    const links: string[] = [];
    for (const client of clients) {
      const entries = genAllLinks({ inbound, remark, remarkModel, client, hostOverride, fallbackHostname });
      for (const e of entries) links.push(e.link);
    }
    return links.join('\r\n');
  }
  if (inbound.protocol === 'shadowsocks') {
    return genShadowsocksLink({ inbound, address: addr, port: inbound.port, forceTls: 'same', remark });
  }
  if (inbound.protocol === 'wireguard') {
    return genWireguardConfigs({ inbound, remark, remarkModel, hostOverride, fallbackHostname });
  }
  return '';
}

// Per-peer wireguard fanout. Each peer gets its own link (or .conf
// block) with an index-suffixed remark, joined by \r\n. Matches the
// legacy genWireguardLinks / genWireguardConfigs exactly.
export interface GenWireguardFanoutInput {
  inbound: Inbound;
  remark?: string;
  remarkModel?: string;
  hostOverride?: string;
  fallbackHostname: string;
}

export function genWireguardLinks(input: GenWireguardFanoutInput): string {
  const { inbound, remark = '', remarkModel = '-io', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'wireguard') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = remarkModel.charAt(0);
  return inbound.settings.peers
    .map((_p, i) => genWireguardLink({
      settings: inbound.settings as WireguardInboundSettings,
      address: addr,
      port: inbound.port,
      remark: `${remark}${sep}${i + 1}`,
      peerIndex: i,
    }))
    .join('\r\n');
}

export function genWireguardConfigs(input: GenWireguardFanoutInput): string {
  const { inbound, remark = '', remarkModel = '-io', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'wireguard') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = remarkModel.charAt(0);
  return inbound.settings.peers
    .map((_p, i) => genWireguardConfig({
      settings: inbound.settings as WireguardInboundSettings,
      address: addr,
      port: inbound.port,
      remark: `${remark}${sep}${i + 1}`,
      peerIndex: i,
    }))
    .join('\r\n');
}
