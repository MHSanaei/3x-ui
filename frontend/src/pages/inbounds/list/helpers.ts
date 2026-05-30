import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { isSSMultiUser } from '@/lib/xray/protocol-capabilities';
import { coerceInboundJsonField } from '@/models/dbinbound';

import type { ClientCountEntry, DBInboundRecord, SortKey, StreamHints } from './types';

export function readStreamHints(streamSettings: unknown): StreamHints {
  const stream = coerceInboundJsonField(streamSettings) as { network?: string; security?: string };
  return {
    network: stream.network ?? '',
    isTls: stream.security === 'tls',
    isReality: stream.security === 'reality',
  };
}

// Display label for a network value. All known transports render in
// upper-case for visual consistency with the TCP/UDP/TLS/Reality tags
// already shown alongside; compound names (`httpupgrade`, `splithttp`,
// `xhttp`) get a tiny touch of casing so they don't read as one word.
export function networkLabel(network: string): string {
  const n = (network || '').toLowerCase();
  if (!n) return 'TCP';
  switch (n) {
    case 'httpupgrade': return 'HTTPUpgrade';
    case 'splithttp': return 'SplitHTTP';
    case 'xhttp': return 'XHTTP';
  }
  return n.toUpperCase();
}

// Returns the underlying L4 protocol for transports whose name isn't
// already TCP/UDP. `kcp` and `quic` both ride on UDP; everything else
// (`ws`, `grpc`, `http`, `httpupgrade`, `xhttp`) is TCP-based and gets
// no extra tag (the transport name implies TCP).
export function networkL4(network: string): 'UDP' | '' {
  const n = (network || '').toLowerCase();
  if (n === 'kcp' || n === 'quic') return 'UDP';
  return '';
}

// Shadowsocks settings.network ("tcp" / "udp" / "tcp,udp") and Tunnel
// settings.allowedNetwork (same shape, different field name) both carry
// the L4 transport list independent of streamSettings. Returns a
// comma-separated label.
export function commaNetworkLabel(raw: string): string {
  const parts = (raw || 'tcp').toLowerCase().split(',').map((p) => p.trim()).filter(Boolean);
  if (parts.length === 0) return 'TCP';
  return parts.map(networkLabel).join(',');
}

export function shadowsocksNetworkLabel(settings: unknown): string {
  return commaNetworkLabel(readSettings(settings).network || '');
}

export function tunnelNetworkLabel(settings: unknown): string {
  return commaNetworkLabel(readSettings(settings).allowedNetwork || '');
}

// Mixed (socks+http combo) is always TCP at L4; settings.udp=true adds
// UDP-associate support on the same port (SOCKS5 UDP).
export function mixedNetworkLabel(settings: unknown): string {
  const st = coerceInboundJsonField(settings) as { udp?: boolean };
  return st.udp ? 'TCP,UDP' : 'TCP';
}

export function readSettings(settings: unknown): { method?: string; network?: string; allowedNetwork?: string } {
  return coerceInboundJsonField(settings) as { method?: string; network?: string; allowedNetwork?: string };
}

export function isInboundMultiUser(record: { protocol: string; settings: unknown }): boolean {
  switch (record.protocol) {
    case 'vmess':
    case 'vless':
    case 'trojan':
    case 'hysteria':
      return true;
    case 'shadowsocks':
      return isSSMultiUser({ protocol: 'shadowsocks', settings: readSettings(record.settings) });
    default:
      return false;
  }
}

export function showQrCodeMenu(dbInbound: DBInboundRecord): boolean {
  if (dbInbound.isWireguard) return true;
  if (dbInbound.isSS) {
    return !isSSMultiUser({ protocol: 'shadowsocks', settings: readSettings(dbInbound.settings) });
  }
  return false;
}

export const SORT_FNS: Record<SortKey, (a: DBInboundRecord, b: DBInboundRecord, ctx: { nodesById: Map<number, NodeRecord>; clientCount: Record<number, ClientCountEntry> }) => number> = {
  id: (a, b) => a.id - b.id,
  enable: (a, b) => Number(a.enable) - Number(b.enable),
  remark: (a, b) => (a.remark || '').localeCompare(b.remark || ''),
  port: (a, b) => a.port - b.port,
  protocol: (a, b) => a.protocol.localeCompare(b.protocol),
  traffic: (a, b) => (a.up + a.down) - (b.up + b.down),
  expiryTime: (a, b) => (a.expiryTime || Infinity) - (b.expiryTime || Infinity),
  node: (a, b, ctx) => {
    const nameA = ctx.nodesById.get(a.nodeId ?? -1)?.name ?? (a.nodeId == null ? '￿' : `node #${a.nodeId}`);
    const nameB = ctx.nodesById.get(b.nodeId ?? -1)?.name ?? (b.nodeId == null ? '￿' : `node #${b.nodeId}`);
    return nameA.localeCompare(nameB);
  },
  clients: (a, b, ctx) => (ctx.clientCount[a.id]?.clients || 0) - (ctx.clientCount[b.id]?.clients || 0),
};
