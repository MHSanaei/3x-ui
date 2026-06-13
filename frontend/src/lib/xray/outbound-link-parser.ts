import { Base64 } from '@/utils';

// Focused share-link parser for the OutboundFormModal's link-import
// helper. Each parser returns a wire-shape outbound record (the same
// shape OutboundsTab.tsx stores in templateSettings.outbounds[]) or
// null when the input doesn't match.
//
// Scope: address + port + auth + remark, plus the network/security
// fields the common vmess:// / vless:// links carry as query params.
// XHTTP advanced fields (xPaddingBytes, scMaxEachPostBytes,
// scMinPostsIntervalMs, uplinkChunkSize, noGRPCHeader) round-trip when
// present in either the JSON or URL params. xmux, reality shortIds,
// padding obfs key/header/placement, hysteria udphop are still left
// to the user to fill in after import — the legacy Outbound.fromLink
// was ~250 lines of dense edge-case handling we don't need to
// replicate verbatim for the common phone-to-panel workflow.

type Raw = Record<string, unknown>;

// XHTTP knob keys grouped by wire type. Used by both the URL query-param
// (vless/trojan) branch and the vmess JSON branch to consistently pull
// the same set of advanced fields when present. Keep order ~stable to
// match the schema's authoring order so diffs read naturally.
const XHTTP_STRING_KEYS = [
  'xPaddingBytes', 'xPaddingKey', 'xPaddingHeader', 'xPaddingPlacement',
  'xPaddingMethod', 'sessionPlacement', 'sessionKey', 'seqPlacement',
  'seqKey', 'uplinkDataPlacement', 'uplinkDataKey', 'scMaxEachPostBytes',
  'scMinPostsIntervalMs', 'scStreamUpServerSecs', 'uplinkHTTPMethod',
] as const;
const XHTTP_NUMBER_KEYS = [
  'scMaxBufferedPosts', 'serverMaxHeaderBytes', 'uplinkChunkSize',
] as const;
const XHTTP_BOOL_KEYS = [
  'xPaddingObfsMode', 'noSSEHeader', 'noGRPCHeader',
] as const;

function asBool(s: string | null): boolean | undefined {
  if (s === null) return undefined;
  return s === 'true' || s === '1';
}

function applyXhttpStringFromParams(xhttp: Raw, params: URLSearchParams): void {
  // Precedence from lowest to highest: stream-init default →
  // x_padding_bytes snake_case alias → extra JSON payload →
  // explicit camelCase URL param. Apply in that order so each tier
  // overwrites the previous when present.
  const padBytesAlt = params.get('x_padding_bytes');
  if (padBytesAlt !== null && padBytesAlt !== '') {
    xhttp.xPaddingBytes = padBytesAlt;
  }
  // The inbound link bundles advanced xhttp knobs into `extra=<json>`.
  // Decode and merge so re-importing a share link round-trips the full
  // xhttp config (xPaddingBytes, scMaxEachPostBytes, sessionKey, etc.).
  const extra = params.get('extra');
  if (extra) {
    try {
      const parsed = JSON.parse(extra) as Record<string, unknown>;
      applyXhttpStringFromJson(xhttp, parsed);
      if (parsed.headers && typeof parsed.headers === 'object') {
        xhttp.headers = parsed.headers;
      }
    } catch {
      // malformed extra — silently ignore, the panel can still operate
      // on the rest of the link
    }
  }
  for (const k of XHTTP_STRING_KEYS) {
    const v = params.get(k);
    if (v !== null && v !== '') xhttp[k] = v;
  }
  for (const k of XHTTP_NUMBER_KEYS) {
    const v = params.get(k);
    if (v !== null && v !== '') xhttp[k] = Number(v) || 0;
  }
  for (const k of XHTTP_BOOL_KEYS) {
    const v = params.get(k);
    if (v !== null && v !== '') xhttp[k] = asBool(v);
  }
}

function applyXhttpStringFromJson(xhttp: Raw, json: Record<string, unknown>): void {
  for (const k of XHTTP_STRING_KEYS) {
    if (typeof json[k] === 'string') xhttp[k] = json[k];
  }
  for (const k of XHTTP_NUMBER_KEYS) {
    if (typeof json[k] === 'number') xhttp[k] = json[k];
  }
  for (const k of XHTTP_BOOL_KEYS) {
    if (typeof json[k] === 'boolean') xhttp[k] = json[k];
  }
}

function buildStream(network: string, security: string): Raw {
  const stream: Raw = { network, security };
  switch (network) {
    case 'tcp':
      stream.tcpSettings = { header: { type: 'none' } };
      break;
    case 'kcp':
      stream.kcpSettings = {
        mtu: 1350, tti: 20, uplinkCapacity: 5, downlinkCapacity: 20,
        cwndMultiplier: 1, maxSendingWindow: 2097152,
      };
      break;
    case 'ws':
      stream.wsSettings = { path: '/', host: '', headers: {}, heartbeatPeriod: 0 };
      break;
    case 'grpc':
      stream.grpcSettings = { serviceName: '', authority: '', multiMode: false };
      break;
    case 'httpupgrade':
      stream.httpupgradeSettings = { path: '/', host: '', headers: {} };
      break;
    case 'xhttp':
      stream.xhttpSettings = {
        path: '/', host: '', mode: 'auto', headers: {},
        xPaddingBytes: '100-1000',
      };
      break;
    default:
      stream.tcpSettings = { header: { type: 'none' } };
  }
  if (security === 'tls') {
    stream.tlsSettings = {
      serverName: '', alpn: [], fingerprint: '',
      echConfigList: '', verifyPeerCertByName: '', pinnedPeerCertSha256: '',
    };
  } else if (security === 'reality') {
    stream.realitySettings = {
      publicKey: '', fingerprint: 'chrome', serverName: '',
      shortId: '', spiderX: '', mldsa65Verify: '',
    };
  }
  return stream;
}

function applyTransportParams(stream: Raw, params: URLSearchParams): void {
  const network = stream.network as string;
  const host = params.get('host') ?? '';
  const path = params.get('path') ?? '/';
  switch (network) {
    case 'ws':
      (stream.wsSettings as Raw).host = host;
      (stream.wsSettings as Raw).path = path;
      break;
    case 'grpc': {
      const grpc = stream.grpcSettings as Raw;
      const serviceName = params.get('serviceName') ?? params.get('path') ?? '';
      grpc.serviceName = serviceName;
      grpc.authority = params.get('authority') ?? '';
      grpc.multiMode = params.get('mode') === 'multi';
      break;
    }
    case 'httpupgrade':
      (stream.httpupgradeSettings as Raw).host = host;
      (stream.httpupgradeSettings as Raw).path = path;
      break;
    case 'xhttp': {
      const xhttp = stream.xhttpSettings as Raw;
      xhttp.host = host;
      xhttp.path = path;
      if (params.get('mode')) xhttp.mode = params.get('mode');
      applyXhttpStringFromParams(xhttp, params);
      break;
    }
    case 'tcp':
      // vless/trojan TCP HTTP camouflage rides on header=http+host+path
      if (params.get('headerType') === 'http' || params.get('type') === 'http') {
        (stream.tcpSettings as Raw).header = {
          type: 'http',
          request: {
            version: '1.1',
            method: 'GET',
            path: path.split(',').filter(Boolean),
            headers: host ? { Host: host.split(',').filter(Boolean) } : {},
          },
        };
      }
      break;
  }
}

// The inbound link emits the entire finalmask object as a JSON-encoded
// `fm` query param. Decode and attach to streamSettings so udpHop /
// quicParams / tcp+udp masks round-trip on outbound import.
function applyFinalMaskParam(stream: Raw, params: URLSearchParams): void {
  const fm = params.get('fm');
  if (!fm) return;
  try {
    const parsed = JSON.parse(fm) as Record<string, unknown>;
    if (parsed && typeof parsed === 'object') {
      stream.finalmask = parsed;
    }
  } catch {
    // malformed fm — leave streamSettings.finalmask absent
  }
}

function applySecurityParams(stream: Raw, params: URLSearchParams): void {
  if (stream.security === 'tls') {
    const tls = stream.tlsSettings as Raw;
    tls.serverName = params.get('sni') ?? '';
    tls.fingerprint = params.get('fp') ?? '';
    const alpn = params.get('alpn');
    if (alpn) tls.alpn = alpn.split(',');
    tls.echConfigList = params.get('ech') ?? '';
    tls.pinnedPeerCertSha256 = params.get('pcs') ?? '';
  } else if (stream.security === 'reality') {
    const reality = stream.realitySettings as Raw;
    reality.serverName = params.get('sni') ?? '';
    reality.fingerprint = params.get('fp') ?? 'chrome';
    reality.publicKey = params.get('pbk') ?? '';
    reality.shortId = params.get('sid') ?? '';
    reality.spiderX = params.get('spx') ?? '';
    reality.mldsa65Verify = params.get('pqv') ?? '';
  }
}

function decodeRemark(url: URL): string {
  try {
    return decodeURIComponent(url.hash.replace(/^#/, ''));
  } catch {
    return url.hash.replace(/^#/, '');
  }
}

export function parseVmessLink(link: string): Raw | null {
  if (!link.startsWith('vmess://')) return null;
  try {
    const decoded = Base64.decode(link.slice('vmess://'.length));
    const json = JSON.parse(decoded) as Record<string, unknown>;
    const network = (json.net as string) || 'tcp';
    const security = json.tls === 'tls' ? 'tls' : 'none';
    const stream = buildStream(network, security);
    // Map the vmess JSON's net-specific keys onto the stream branch.
    if (network === 'tcp' && json.type === 'http') {
      (stream.tcpSettings as Raw).header = {
        type: 'http',
        request: {
          version: '1.1', method: 'GET',
          path: (json.path as string ?? '/').split(',').filter(Boolean),
          headers: json.host ? { Host: (json.host as string).split(',').filter(Boolean) } : {},
        },
      };
    } else if (network === 'ws') {
      (stream.wsSettings as Raw).host = json.host ?? '';
      (stream.wsSettings as Raw).path = json.path ?? '/';
    } else if (network === 'grpc') {
      (stream.grpcSettings as Raw).serviceName = json.path ?? '';
      (stream.grpcSettings as Raw).authority = json.authority ?? '';
      (stream.grpcSettings as Raw).multiMode = json.type === 'multi';
    } else if (network === 'httpupgrade') {
      (stream.httpupgradeSettings as Raw).host = json.host ?? '';
      (stream.httpupgradeSettings as Raw).path = json.path ?? '/';
    } else if (network === 'xhttp') {
      const xhttp = stream.xhttpSettings as Raw;
      xhttp.host = json.host ?? '';
      xhttp.path = json.path ?? '/';
      if (json.mode) xhttp.mode = json.mode;
      applyXhttpStringFromJson(xhttp, json);
    }
    if (security === 'tls') {
      const tls = stream.tlsSettings as Raw;
      tls.serverName = json.sni ?? '';
      tls.fingerprint = json.fp ?? '';
      if (json.alpn) tls.alpn = (json.alpn as string).split(',');
    }

    const port = Number(json.port) || 443;
    return {
      protocol: 'vmess',
      tag: typeof json.ps === 'string' ? json.ps : '',
      settings: {
        vnext: [{
          address: json.add ?? '',
          port,
          users: [{ id: json.id ?? '', security: (json.scy as string) || 'auto' }],
        }],
      },
      streamSettings: stream,
    };
  } catch {
    return null;
  }
}

function parseUrlLink(link: string, expectedProto: string): URL | null {
  try {
    const url = new URL(link);
    if (url.protocol.replace(/:$/, '') !== expectedProto) return null;
    return url;
  } catch {
    return null;
  }
}

export function parseVlessLink(link: string): Raw | null {
  const url = parseUrlLink(link, 'vless');
  if (!url) return null;
  const id = url.username;
  const address = url.hostname;
  const port = Number(url.port) || 443;
  const params = url.searchParams;
  const network = params.get('type') ?? 'tcp';
  const security = (params.get('security') ?? 'none') as string;
  const stream = buildStream(network, security);
  applyTransportParams(stream, params);
  applySecurityParams(stream, params);
  applyFinalMaskParam(stream, params);
  return {
    protocol: 'vless',
    tag: decodeRemark(url),
    settings: {
      address,
      port,
      id,
      flow: params.get('flow') ?? '',
      encryption: params.get('encryption') ?? 'none',
    },
    streamSettings: stream,
  };
}

export function parseTrojanLink(link: string): Raw | null {
  const url = parseUrlLink(link, 'trojan');
  if (!url) return null;
  const password = url.username;
  const address = url.hostname;
  const port = Number(url.port) || 443;
  const params = url.searchParams;
  const network = params.get('type') ?? 'tcp';
  const security = (params.get('security') ?? 'tls') as string;
  const stream = buildStream(network, security);
  applyTransportParams(stream, params);
  applySecurityParams(stream, params);
  applyFinalMaskParam(stream, params);
  return {
    protocol: 'trojan',
    tag: decodeRemark(url),
    settings: {
      servers: [{ address, port, password }],
    },
    streamSettings: stream,
  };
}

export function parseShadowsocksLink(link: string): Raw | null {
  if (!link.startsWith('ss://')) return null;
  // Two link shapes coexist:
  //   modern:  ss://base64(method:password)@host:port#remark
  //   legacy:  ss://base64(method:password@host:port)#remark
  // Try modern first; fall back to legacy decode of the whole userinfo+host.
  let userInfo: string;
  let host: string;
  let port: number;
  let remark = '';
  const hashIndex = link.indexOf('#');
  const linkNoHash = hashIndex >= 0 ? link.slice(0, hashIndex) : link;
  if (hashIndex >= 0) {
    try { remark = decodeURIComponent(link.slice(hashIndex + 1)); } catch { remark = ''; }
  }
  const queryIndex = linkNoHash.indexOf('?');
  const core = queryIndex >= 0 ? linkNoHash.slice(0, queryIndex) : linkNoHash;
  const atIndex = core.indexOf('@');
  if (atIndex >= 0) {
    try { userInfo = Base64.decode(core.slice('ss://'.length, atIndex)); }
    catch { userInfo = core.slice('ss://'.length, atIndex); }
    const hostPort = core.slice(atIndex + 1);
    const colon = hostPort.lastIndexOf(':');
    if (colon < 0) return null;
    host = hostPort.slice(0, colon);
    port = Number(hostPort.slice(colon + 1)) || 443;
  } else {
    let decoded: string;
    try { decoded = Base64.decode(core.slice('ss://'.length)); }
    catch { return null; }
    const at = decoded.indexOf('@');
    if (at < 0) return null;
    userInfo = decoded.slice(0, at);
    const hostPort = decoded.slice(at + 1);
    const colon = hostPort.lastIndexOf(':');
    if (colon < 0) return null;
    host = hostPort.slice(0, colon);
    port = Number(hostPort.slice(colon + 1)) || 443;
  }
  const sep = userInfo.indexOf(':');
  const method = sep < 0 ? '2022-blake3-aes-128-gcm' : userInfo.slice(0, sep);
  const password = sep < 0 ? userInfo : userInfo.slice(sep + 1);
  return {
    protocol: 'shadowsocks',
    tag: remark,
    settings: {
      servers: [{ address: host, port, password, method }],
    },
  };
}

export function parseHysteria2Link(link: string): Raw | null {
  const url = parseUrlLink(link, 'hysteria2') ?? parseUrlLink(link, 'hy2');
  if (!url) return null;
  // hysteria2's auth rides as the URL userinfo. The streamSettings
  // network branch is the dedicated 'hysteria' transport — the modal's
  // newStreamSlice('hysteria') initializer fills in receive-window
  // defaults; we override the user-set fields here.
  const auth = url.username;
  const address = url.hostname;
  const port = Number(url.port) || 443;
  const params = url.searchParams;
  const alpn = params.get('alpn');
  const stream: Raw = {
    network: 'hysteria',
    security: 'tls',
    hysteriaSettings: {
      version: 2, auth, udpIdleTimeout: 60,
    },
    tlsSettings: {
      serverName: params.get('sni') ?? '',
      alpn: alpn ? alpn.split(',') : ['h3'],
      fingerprint: params.get('fp') ?? '',
      echConfigList: params.get('ech') ?? '',
      verifyPeerCertByName: '',
      pinnedPeerCertSha256: params.get('pinSHA256') ?? '',
    },
  };
  applyFinalMaskParam(stream, params);
  return {
    protocol: 'hysteria',
    tag: decodeRemark(url),
    settings: { address, port, version: 2 },
    streamSettings: stream,
  };
}

function firstParam(params: URLSearchParams, ...keys: string[]): string | null {
  for (const k of keys) {
    const v = params.get(k);
    if (v !== null && v !== '') return v;
  }
  return null;
}

export function parseWireguardLink(link: string): Raw | null {
  const url = parseUrlLink(link, 'wireguard') ?? parseUrlLink(link, 'wg');
  if (!url) return null;
  let secretKey: string;
  try {
    secretKey = decodeURIComponent(url.username);
  } catch {
    secretKey = url.username;
  }
  const params = url.searchParams;
  const host = url.hostname;
  const port = url.port;
  const endpoint = host ? (port ? `${host}:${port}` : host) : '';

  const addressRaw = firstParam(params, 'address', 'ip') ?? '';
  const address = addressRaw.split(',').map((s) => s.trim()).filter(Boolean);

  const allowedRaw = firstParam(params, 'allowedips', 'allowed_ips');
  const allowedIPs = allowedRaw
    ? allowedRaw.split(',').map((s) => s.trim()).filter(Boolean)
    : ['0.0.0.0/0', '::/0'];

  const peer: Raw = {
    publicKey: firstParam(params, 'publickey', 'publicKey', 'public_key', 'peerPublicKey') ?? '',
    endpoint,
    allowedIPs,
  };
  const psk = firstParam(params, 'presharedkey', 'preshared_key', 'pre-shared-key', 'psk');
  if (psk) peer.preSharedKey = psk;
  const keepAliveRaw = firstParam(params, 'keepalive', 'persistentkeepalive', 'persistent_keepalive');
  if (keepAliveRaw !== null) {
    const k = Number(keepAliveRaw);
    if (Number.isFinite(k)) peer.keepAlive = k;
  }

  const settings: Raw = { secretKey, address, peers: [peer] };
  const mtuRaw = firstParam(params, 'mtu');
  if (mtuRaw !== null) {
    const m = Number(mtuRaw);
    if (Number.isFinite(m)) settings.mtu = m;
  }
  const reservedRaw = firstParam(params, 'reserved');
  if (reservedRaw) {
    const reserved = reservedRaw.split(',')
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n));
    if (reserved.length > 0) settings.reserved = reserved;
  }

  return {
    protocol: 'wireguard',
    tag: decodeRemark(url),
    settings,
  };
}

// Dispatcher — first non-null parser wins. Returns null when no parser
// recognizes the link's protocol scheme.
export function parseOutboundLink(link: string): Raw | null {
  const trimmed = link.trim();
  if (!trimmed) return null;
  return (
    parseVmessLink(trimmed)
    ?? parseVlessLink(trimmed)
    ?? parseTrojanLink(trimmed)
    ?? parseShadowsocksLink(trimmed)
    ?? parseHysteria2Link(trimmed)
    ?? parseWireguardLink(trimmed)
  );
}
