import { getMessage } from '@/utils/messageBus';
import { ColorUtils, ClipboardManager, FileManager } from '@/utils';
import { Protocols } from '@/schemas/primitives';
import { coerceInboundJsonField } from '@/models/dbinbound';
import {
  canEnableTlsFlow,
  isSS2022 as isSS2022Helper,
  isSSMultiUser as isSSMultiUserHelper,
} from '@/lib/xray/protocol-capabilities';

import type { ClientSetting, ClientStats, DBInboundLike, InboundInfo } from './types';

const LINK_PROTOCOLS: ReadonlySet<string> = new Set([
  Protocols.VMESS,
  Protocols.VLESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
]);

export function hasShareLink(protocol: string): boolean {
  return LINK_PROTOCOLS.has(protocol);
}

function readHeader(headers: unknown, name: string): string {
  const needle = name.toLowerCase();
  if (Array.isArray(headers)) {
    for (const h of headers) {
      if (h && typeof h === 'object' && String((h as { name?: string }).name ?? '').toLowerCase() === needle) {
        return String((h as { value?: unknown }).value ?? '');
      }
    }
    return '';
  }
  if (headers && typeof headers === 'object') {
    for (const [k, v] of Object.entries(headers as Record<string, unknown>)) {
      if (k.toLowerCase() === needle) {
        return Array.isArray(v) ? String(v[0] ?? '') : String(v ?? '');
      }
    }
  }
  return '';
}

function readNetworkHost(stream: Record<string, unknown>, network: string): string | null {
  switch (network) {
    case 'tcp': {
      const tcp = stream.tcpSettings as { header?: { request?: { headers?: unknown } } } | undefined;
      return readHeader(tcp?.header?.request?.headers, 'host');
    }
    case 'ws': {
      const ws = stream.wsSettings as { host?: string; headers?: unknown } | undefined;
      return (ws?.host && ws.host.length > 0) ? ws.host : readHeader(ws?.headers, 'host');
    }
    case 'httpupgrade': {
      const hu = stream.httpupgradeSettings as { host?: string; headers?: unknown } | undefined;
      return (hu?.host && hu.host.length > 0) ? hu.host : readHeader(hu?.headers, 'host');
    }
    case 'xhttp': {
      const xh = stream.xhttpSettings as { host?: string; headers?: unknown } | undefined;
      return (xh?.host && xh.host.length > 0) ? xh.host : readHeader(xh?.headers, 'host');
    }
    default:
      return null;
  }
}

function readNetworkPath(stream: Record<string, unknown>, network: string): string | null {
  switch (network) {
    case 'tcp': {
      const tcp = stream.tcpSettings as { header?: { request?: { path?: string[] } } } | undefined;
      return tcp?.header?.request?.path?.[0] ?? null;
    }
    case 'ws':
      return (stream.wsSettings as { path?: string } | undefined)?.path ?? null;
    case 'httpupgrade':
      return (stream.httpupgradeSettings as { path?: string } | undefined)?.path ?? null;
    case 'xhttp':
      return (stream.xhttpSettings as { path?: string } | undefined)?.path ?? null;
    default:
      return null;
  }
}

export function buildInboundInfo(dbInbound: DBInboundLike): InboundInfo {
  const settings = coerceInboundJsonField(dbInbound.settings) as Record<string, unknown>;
  const stream = coerceInboundJsonField(dbInbound.streamSettings) as Record<string, unknown>;
  const network = (stream.network as string | undefined) ?? '';
  const security = (stream.security as string | undefined) ?? 'none';
  const clients = Array.isArray(settings.clients) ? (settings.clients as ClientSetting[]) : [];
  const xhttpSettings = stream.xhttpSettings as { mode?: string } | undefined;
  const grpcSettings = stream.grpcSettings as { multiMode?: boolean; serviceName?: string } | undefined;
  let serverName = '';
  if (security === 'tls') {
    const tls = stream.tlsSettings as { sni?: string; serverName?: string } | undefined;
    serverName = tls?.sni ?? tls?.serverName ?? '';
  } else if (security === 'reality') {
    const reality = stream.realitySettings as { serverNames?: string[]; serverName?: string } | undefined;
    if (Array.isArray(reality?.serverNames)) {
      serverName = reality.serverNames.join(', ');
    } else if (reality?.serverName) {
      serverName = reality.serverName;
    }
  }
  return {
    protocol: dbInbound.protocol,
    clients,
    settings,
    isTcp: network === 'tcp',
    isWs: network === 'ws',
    isHttpupgrade: network === 'httpupgrade',
    isXHTTP: network === 'xhttp',
    isGrpc: network === 'grpc',
    isSSMultiUser: isSSMultiUserHelper({
      protocol: dbInbound.protocol,
      settings: settings as { method?: string },
    }),
    isSS2022: isSS2022Helper({
      protocol: dbInbound.protocol,
      settings: settings as { method?: string },
    }),
    isVlessTlsFlow: canEnableTlsFlow({
      protocol: dbInbound.protocol,
      settings: {
        encryption: settings.encryption as string | undefined,
        decryption: settings.decryption as string | undefined,
      },
      streamSettings: { network, security },
    }),
    host: readNetworkHost(stream, network),
    path: readNetworkPath(stream, network),
    serviceName: grpcSettings?.serviceName ?? '',
    serverName,
    stream: {
      network,
      security,
      xhttp: xhttpSettings ? { mode: xhttpSettings.mode } : undefined,
      grpc: grpcSettings ? { multiMode: grpcSettings.multiMode } : undefined,
    },
  };
}

export function copyText(value: unknown, t: (k: string) => string) {
  ClipboardManager.copyText(String(value ?? '')).then((ok) => {
    if (ok) getMessage().success(t('copied'));
  });
}

export function downloadText(content: string, filename: string) {
  FileManager.downloadTextFile(content, filename);
}

export function statsColor(stats: ClientStats, trafficDiff: number) {
  return ColorUtils.usageColor(stats.up + stats.down, trafficDiff, stats.total);
}

export function formatIpInfo(record: unknown) {
  if (record == null) return '';
  if (typeof record === 'string' || typeof record === 'number') return String(record);
  const r = record as { ip?: string; IP?: string; timestamp?: number | string; Timestamp?: number | string };
  const ip = r.ip || r.IP || '';
  const ts = r.timestamp || r.Timestamp || 0;
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
