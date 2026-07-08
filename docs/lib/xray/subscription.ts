// Pure builders for 3x-ui's subscription server: the subscription URLs plus
// previews of the two body formats — Base64 (newline-joined share links,
// standard base64) and JSON (Xray client config, one per client). Grounded in
// internal/sub/{controller,build_urls_test}.go, json_service.go, default.json.
// Reuses links.ts (share-link builders), base64.ts, and outbounds.ts
// (buildStreamSettings). No React/DOM imports.

import { textToBase64 } from './base64';
import { buildVless, buildVmess, buildTrojan, buildShadowsocks } from './links';
import { buildStreamSettings, type Network, type Security } from './outbounds';

export interface SubUrlInput {
  scheme: 'http' | 'https';
  host: string;
  port: number;
  subPath: string; // e.g. '/sub/'
  jsonPath: string; // e.g. '/json/'
  subId: string;
  /** When behind a reverse proxy the public URL omits the sub-server port. */
  behindProxy?: boolean;
}

export interface SubUrls {
  base64: string;
  json: string;
}

export interface SubClient {
  protocol: 'vless' | 'vmess' | 'trojan' | 'ss';
  remark: string;
  address: string;
  port: number;
  // credentials
  id?: string; // vless / vmess uuid
  password?: string; // trojan / ss
  method?: string; // ss cipher
  flow?: string; // vless
  encryption?: string; // vless server encryption, default 'none'
  vmessSecurity?: string; // vmess scy, default 'auto'
  // stream (subset, mirrored into share-link params + JSON streamSettings)
  network?: Network;
  security?: Security;
  sni?: string;
  fingerprint?: string;
  path?: string;
  host?: string;
  serviceName?: string;
  publicKey?: string; // reality
  shortId?: string; // reality
}

function normPath(p: string): string {
  let s = p.trim();
  if (!s.startsWith('/')) s = `/${s}`;
  if (!s.endsWith('/')) s = `${s}/`;
  return s;
}

export function buildSubscriptionUrls(i: SubUrlInput): SubUrls {
  if (!i.subId) return { base64: '', json: '' };
  const origin = i.behindProxy ? `${i.scheme}://${i.host}` : `${i.scheme}://${i.host}:${i.port}`;
  return {
    base64: `${origin}${normPath(i.subPath)}${i.subId}`,
    json: `${origin}${normPath(i.jsonPath)}${i.subId}`,
  };
}

function streamParams(c: SubClient): Record<string, string> {
  const p: Record<string, string> = {
    type: c.network ?? 'tcp',
    security: c.security ?? 'none',
  };
  if (c.sni) p.sni = c.sni;
  if (c.fingerprint) p.fp = c.fingerprint;
  if (c.path) p.path = c.path;
  if (c.host) p.host = c.host;
  if (c.serviceName) p.serviceName = c.serviceName;
  if (c.publicKey) p.pbk = c.publicKey;
  if (c.shortId) p.sid = c.shortId;
  return p;
}

function shareLink(c: SubClient): string {
  switch (c.protocol) {
    case 'vless': {
      const params = streamParams(c);
      if (c.flow) params.flow = c.flow;
      return buildVless({
        credential: c.id ?? '',
        address: c.address,
        port: c.port,
        name: c.remark,
        params,
      });
    }
    case 'trojan':
      return buildTrojan({
        credential: c.password ?? '',
        address: c.address,
        port: c.port,
        name: c.remark,
        params: streamParams(c),
      });
    case 'vmess':
      return buildVmess({
        ps: c.remark,
        add: c.address,
        port: c.port,
        id: c.id ?? '',
        scy: c.vmessSecurity || 'auto',
        net: c.network ?? 'tcp',
        tls: c.security === 'tls' ? 'tls' : '',
        sni: c.sni ?? '',
        host: c.host ?? '',
        path: c.path ?? '',
      });
    case 'ss':
      return buildShadowsocks({
        method: c.method || '',
        password: c.password ?? '',
        address: c.address,
        port: c.port,
        name: c.remark,
      });
  }
}

export function buildShareLinks(clients: SubClient[]): string[] {
  return clients.map(shareLink);
}

export function buildBase64Subscription(clients: SubClient[]): string {
  if (clients.length === 0) return '';
  return textToBase64(buildShareLinks(clients).join('\n'));
}

// The non-outbound skeleton of internal/sub/default.json. A factory so every
// call returns a fresh object (pure, no shared mutation).
function subJsonSkeleton(): Record<string, unknown> {
  return {
    dns: {
      tag: 'dns_out',
      queryStrategy: 'UseIP',
      servers: [{ address: '8.8.8.8', skipFallback: false }],
    },
    inbounds: [
      {
        port: 10808,
        protocol: 'mixed',
        settings: { auth: 'noauth', udp: true, userLevel: 8 },
        sniffing: { destOverride: ['http', 'tls', 'quic', 'fakedns'], enabled: true },
        tag: 'mixed',
      },
      { port: 10809, protocol: 'http', settings: { userLevel: 8 }, tag: 'http' },
    ],
    log: { loglevel: 'warning' },
    policy: {
      levels: { '8': { connIdle: 300, downlinkOnly: 1, handshake: 4, uplinkOnly: 1 } },
      system: { statsOutboundUplink: true, statsOutboundDownlink: true },
    },
    routing: {
      domainStrategy: 'AsIs',
      rules: [{ type: 'field', network: 'tcp,udp', outboundTag: 'proxy' }],
    },
    stats: {},
  };
}

function skeletonOutbounds(): Record<string, unknown>[] {
  return [
    {
      tag: 'direct',
      protocol: 'freedom',
      settings: { domainStrategy: 'AsIs', redirect: '', noises: [] },
    },
    { tag: 'block', protocol: 'blackhole', settings: { response: { type: 'http' } } },
  ];
}

function proxyOutbound(c: SubClient): Record<string, unknown> {
  const streamSettings = buildStreamSettings({
    network: c.network ?? 'tcp',
    security: c.security ?? 'none',
    sni: c.sni,
    fingerprint: c.fingerprint,
    path: c.path,
    host: c.host,
    serviceName: c.serviceName,
    publicKey: c.publicKey,
    shortId: c.shortId,
  });

  let settings: Record<string, unknown>;
  switch (c.protocol) {
    case 'vless': {
      const s: Record<string, unknown> = {
        address: c.address,
        port: c.port,
        id: c.id ?? '',
        encryption: c.encryption || 'none',
        level: 8,
      };
      if (c.flow) s.flow = c.flow;
      settings = s;
      break;
    }
    case 'vmess':
      settings = {
        address: c.address,
        port: c.port,
        id: c.id ?? '',
        security: c.vmessSecurity || 'auto',
        level: 8,
      };
      break;
    case 'trojan':
      settings = { servers: [{ address: c.address, port: c.port, password: c.password ?? '', level: 8 }] };
      break;
    case 'ss':
      settings = {
        servers: [
          { address: c.address, port: c.port, password: c.password ?? '', level: 8, method: c.method || '' },
        ],
      };
      break;
  }

  return {
    protocol: c.protocol === 'ss' ? 'shadowsocks' : c.protocol,
    tag: 'proxy',
    streamSettings,
    settings,
  };
}

function jsonConfig(c: SubClient): Record<string, unknown> {
  return {
    remarks: c.remark,
    ...subJsonSkeleton(),
    outbounds: [proxyOutbound(c), ...skeletonOutbounds()],
  };
}

export function buildJsonSubscription(clients: SubClient[]): string {
  if (clients.length === 0) return '';
  const configs = clients.map(jsonConfig);
  // 3x-ui returns a single object for one client, an array for several.
  return JSON.stringify(configs.length === 1 ? configs[0] : configs, null, 2);
}
