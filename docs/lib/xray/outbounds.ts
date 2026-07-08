// Pure builders for Xray outbound objects, matching the wire shapes 3x-ui
// emits (internal/util/link/outbound.go + the panel's outbound-defaults.ts):
//   - VLESS uses the FLAT settings form {address,port,id,flow,encryption}.
//   - VMess uses the vnext form {vnext:[{address,port,users:[...]}]}.
//   - trojan/ss/socks/http use {servers:[...]}.
//   - freedom/blackhole use empty settings.
//   - wireguard/warp use {secretKey,address,peers:[...],mtu}.
// No React/DOM imports — unit-tested in Node.

export type OutboundKind =
  | 'freedom'
  | 'blackhole'
  | 'vless'
  | 'vmess'
  | 'trojan'
  | 'shadowsocks'
  | 'socks'
  | 'http'
  | 'wireguard'
  | 'warp';

export type Network = 'tcp' | 'kcp' | 'ws' | 'grpc' | 'httpupgrade' | 'xhttp';
export type Security = 'none' | 'tls' | 'reality';

export interface StreamInput {
  network: Network;
  security: Security;
  host?: string;
  path?: string;
  serviceName?: string; // grpc
  sni?: string; // tls/reality serverName
  fingerprint?: string; // utls fingerprint
  publicKey?: string; // reality
  shortId?: string; // reality
  spiderX?: string; // reality
}

export interface ProxyServerInput {
  address: string;
  port: number;
  id?: string; // vless / vmess uuid
  password?: string; // trojan / ss / socks / http
  method?: string; // ss cipher
  flow?: string; // vless xtls flow
  encryption?: string; // vless, default 'none'
  vmessSecurity?: string; // vmess scy, default 'auto'
  username?: string; // socks / http
}

export interface WireguardInput {
  secretKey: string;
  address: string[]; // local interface addresses
  publicKey: string; // peer public key
  endpoint: string; // peer endpoint host:port
  preSharedKey?: string;
  reserved?: number[];
  mtu?: number;
  keepAlive?: number;
}

export interface OutboundInput {
  kind: OutboundKind;
  tag: string;
  server?: ProxyServerInput;
  wireguard?: WireguardInput;
  stream?: StreamInput;
  domainStrategy?: string; // freedom
}

// The well-known Cloudflare WARP WireGuard endpoint. The panel fills the real
// keys/reserved bytes when you register WARP; this is the template default.
const WARP_ENDPOINT = 'engage.cloudflareclient.com:2408';

// Protocols that carry a streamSettings transport in the panel.
const STREAM_KINDS = new Set<OutboundKind>(['vless', 'vmess', 'trojan', 'shadowsocks']);

function toPort(port: number | undefined): number {
  const n = Number(port);
  return Number.isFinite(n) && n > 0 ? n : 443;
}

export function buildStreamSettings(s: StreamInput): Record<string, unknown> {
  const out: Record<string, unknown> = { network: s.network, security: s.security };

  switch (s.network) {
    case 'tcp':
      out.tcpSettings = { header: { type: 'none' } };
      break;
    case 'kcp':
      out.kcpSettings = {
        mtu: 1350,
        tti: 20,
        uplinkCapacity: 5,
        downlinkCapacity: 20,
        cwndMultiplier: 1,
        maxSendingWindow: 2097152,
      };
      break;
    case 'ws': {
      const ws: Record<string, unknown> = { path: s.path || '/' };
      if (s.host) ws.host = s.host;
      out.wsSettings = ws;
      break;
    }
    case 'grpc':
      out.grpcSettings = { serviceName: s.serviceName || '', multiMode: false };
      break;
    case 'httpupgrade': {
      const hu: Record<string, unknown> = { path: s.path || '/' };
      if (s.host) hu.host = s.host;
      out.httpupgradeSettings = hu;
      break;
    }
    case 'xhttp': {
      const xh: Record<string, unknown> = { path: s.path || '/', mode: 'auto' };
      if (s.host) xh.host = s.host;
      out.xhttpSettings = xh;
      break;
    }
  }

  if (s.security === 'tls') {
    const tls: Record<string, unknown> = {};
    if (s.sni) tls.serverName = s.sni;
    if (s.fingerprint) tls.fingerprint = s.fingerprint;
    out.tlsSettings = tls;
  } else if (s.security === 'reality') {
    const reality: Record<string, unknown> = { fingerprint: s.fingerprint || 'chrome' };
    if (s.publicKey) reality.publicKey = s.publicKey;
    if (s.sni) reality.serverName = s.sni;
    if (s.shortId) reality.shortId = s.shortId;
    if (s.spiderX) reality.spiderX = s.spiderX;
    out.realitySettings = reality;
  }

  return out;
}

function buildSettings(o: OutboundInput): Record<string, unknown> {
  const s = o.server;
  switch (o.kind) {
    case 'freedom':
      return o.domainStrategy ? { domainStrategy: o.domainStrategy } : {};
    case 'blackhole':
      return {};
    case 'vless':
      return {
        address: s?.address ?? '',
        port: toPort(s?.port),
        id: s?.id ?? '',
        flow: s?.flow ?? '',
        encryption: s?.encryption || 'none',
      };
    case 'vmess':
      return {
        vnext: [
          {
            address: s?.address ?? '',
            port: toPort(s?.port),
            users: [{ id: s?.id ?? '', security: s?.vmessSecurity || 'auto' }],
          },
        ],
      };
    case 'trojan':
      return { servers: [{ address: s?.address ?? '', port: toPort(s?.port), password: s?.password ?? '' }] };
    case 'shadowsocks':
      return {
        servers: [
          {
            address: s?.address ?? '',
            port: toPort(s?.port),
            password: s?.password ?? '',
            method: s?.method || '2022-blake3-aes-128-gcm',
          },
        ],
      };
    case 'socks':
    case 'http': {
      const server: Record<string, unknown> = { address: s?.address ?? '', port: toPort(s?.port) };
      server.users = s?.username ? [{ user: s.username, pass: s.password ?? '' }] : [];
      return { servers: [server] };
    }
    case 'wireguard':
    case 'warp':
      return buildWireguardSettings(o);
  }
}

function buildWireguardSettings(o: OutboundInput): Record<string, unknown> {
  const w = o.wireguard;
  const isWarp = o.kind === 'warp';
  const peer: Record<string, unknown> = {
    publicKey: w?.publicKey ?? '',
    endpoint: w?.endpoint || (isWarp ? WARP_ENDPOINT : ''),
    allowedIPs: ['0.0.0.0/0', '::/0'],
  };
  if (w?.preSharedKey) peer.preSharedKey = w.preSharedKey;
  if (w?.keepAlive) peer.keepAlive = w.keepAlive;

  const settings: Record<string, unknown> = {
    secretKey: w?.secretKey ?? '',
    address: w?.address ?? [],
    peers: [peer],
    mtu: w?.mtu ?? 1420,
  };
  if (w?.reserved && w.reserved.length > 0) settings.reserved = w.reserved;
  return settings;
}

function protocolFor(kind: OutboundKind): string {
  return kind === 'warp' ? 'wireguard' : kind;
}

export function buildOutbound(o: OutboundInput): Record<string, unknown> {
  const tag = o.kind === 'warp' ? 'warp' : o.tag;
  const ob: Record<string, unknown> = {
    tag,
    protocol: protocolFor(o.kind),
    settings: buildSettings(o),
  };
  if (o.stream && STREAM_KINDS.has(o.kind)) {
    ob.streamSettings = buildStreamSettings(o.stream);
  }
  return ob;
}

export function buildOutboundJson(o: OutboundInput): string {
  return JSON.stringify(buildOutbound(o), null, 2);
}
