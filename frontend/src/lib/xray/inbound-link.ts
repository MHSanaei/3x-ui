import { Base64 } from '@/utils';

import type { Inbound } from '@/schemas/api/inbound';
import type { VlessClient } from '@/schemas/protocols/inbound/vless';
import type { VmessSecurity } from '@/schemas/protocols/inbound/vmess';
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
          const host = getHeaderValue(request.headers, 'host');
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
        const host = getHeaderValue(request.headers, 'host');
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
      if (stream.network === 'tcp' && flow.length > 0) params.set('flow', flow);
    }
    applyExternalProxyTLSParams(externalProxy, params, security);
  } else if (security === 'reality') {
    params.set('security', 'reality');
    if (stream.security === 'reality') {
      const reality = stream.realitySettings;
      params.set('pbk', reality.settings.publicKey);
      params.set('fp', reality.settings.fingerprint);
      // Legacy parity quirk: the old class stored realitySettings.serverNames
      // as a comma-joined string and gated SNI on `!ObjectUtil.isArrEmpty(s)`
      // — which returns true for any string, so SNI was never written into
      // Reality share links. Existing deployed clients rely on receiving
      // the SNI from realitySettings.target instead; we keep the omission
      // here so this extraction stays byte-stable with the legacy URL.
      // Fixing the bug is a separate intentional commit.
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
