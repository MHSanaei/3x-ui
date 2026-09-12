import { isSSMultiUser } from '@/lib/xray/protocol-capabilities';
import { coerceInboundJsonField } from '@/models/dbinbound';
import type { HostRecord } from '@/schemas/api/host';

import type { DBInboundRecord, StreamHints } from './types';

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
    case 'httpupgrade':
      return 'HTTPUpgrade';
    case 'splithttp':
      return 'SplitHTTP';
    case 'xhttp':
      return 'XHTTP';
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
  const parts = (raw || 'tcp')
    .toLowerCase()
    .split(',')
    .map((p) => p.trim())
    .filter(Boolean);
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

export function readSettings(settings: unknown): {
  method?: string;
  network?: string;
  allowedNetwork?: string;
} {
  return coerceInboundJsonField(settings) as {
    method?: string;
    network?: string;
    allowedNetwork?: string;
  };
}

export function isInboundMultiUser(record: { protocol: string; settings: unknown }): boolean {
  switch (record.protocol) {
    case 'vmess':
    case 'vless':
    case 'trojan':
    case 'hysteria':
    case 'mtproto':
    case 'wireguard':
    case 'amneziawg':
    case 'tuic':
      return true;
    case 'shadowsocks':
      return isSSMultiUser({ protocol: 'shadowsocks', settings: readSettings(record.settings) });
    default:
      return false;
  }
}

export function showQrCodeMenu(dbInbound: DBInboundRecord): boolean {
  if (dbInbound.isSS) {
    return !isSSMultiUser({ protocol: 'shadowsocks', settings: readSettings(dbInbound.settings) });
  }
  return false;
}

/** Max host remarks shown inline before truncating with "+N". */
export const HOST_REMARK_VISIBLE_LIMIT = 2;

/** Join Host Group remarks onto inbound ids using existing /hosts/list fields. */
export function buildHostRemarksByInboundId(
  hosts: Pick<HostRecord, 'remark' | 'inboundIds' | 'hosts'>[],
): Map<number, string[]> {
  const map = new Map<number, string[]>();
  for (const host of hosts) {
    const addressFallback = Array.isArray(host.hosts)
      ? host.hosts.map((h) => (h || '').trim()).find(Boolean) || ''
      : '';
    const label = (host.remark || '').trim() || addressFallback;
    if (!label) continue;
    for (const inboundId of host.inboundIds || []) {
      if (!Number.isFinite(inboundId)) continue;
      const list = map.get(inboundId) ?? [];
      if (!list.includes(label)) list.push(label);
      map.set(inboundId, list);
    }
  }
  return map;
}

export function formatHostRemarksLabel(
  remarks: string[],
  visibleLimit = HOST_REMARK_VISIBLE_LIMIT,
): { display: string; full: string; truncated: boolean } {
  const full = remarks.join(', ');
  if (remarks.length <= visibleLimit) {
    return { display: full, full, truncated: false };
  }
  const visible = remarks.slice(0, visibleLimit).join(', ');
  const more = remarks.length - visibleLimit;
  return { display: `${visible}, +${more}`, full, truncated: true };
}
