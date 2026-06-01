import { OutboundProtocols as Protocols } from '@/schemas/primitives';
import type { OutboundTestState, OutboundTrafficRow } from '@/hooks/useXraySetting';

import type { OutboundRow } from './outbounds-tab-types';

export function outboundAddresses(o: OutboundRow): string[] {
  const settings = o.settings as Record<string, unknown> | undefined;
  switch (o.protocol) {
    case Protocols.VMess: {
      const serverObj = settings?.vnext as Array<{ address: string; port: number }> | undefined;
      return serverObj ? serverObj.map((s) => `${s.address}:${s.port}`) : [];
    }
    case Protocols.VLESS:
      return [`${settings?.address || ''}:${settings?.port || ''}`];
    case Protocols.HTTP:
    case Protocols.Socks:
    case Protocols.Shadowsocks:
    case Protocols.Trojan: {
      const serverObj = settings?.servers as Array<{ address: string; port: number }> | undefined;
      return serverObj ? serverObj.map((s) => `${s.address}:${s.port}`) : [];
    }
    case Protocols.DNS: {
      const addr = (settings?.rewriteAddress as string) || (settings?.address as string) || '';
      const port = (settings?.rewritePort as string | number) || (settings?.port as string | number) || '';
      return addr || port ? [`${addr}:${port}`] : [];
    }
    case Protocols.Wireguard:
      return (((settings?.peers as Array<{ endpoint?: string }>) || []).map((p) => p.endpoint || '').filter(Boolean));
    default:
      return [];
  }
}

export function isUntestable(o: OutboundRow, mode: string): boolean {
  if (!o) return true;
  if (o.protocol === Protocols.Blackhole || o.protocol === Protocols.Loopback || o.tag === 'blocked') return true;
  if (mode === 'tcp' && (o.protocol === Protocols.Freedom || o.protocol === Protocols.DNS)) return true;
  return false;
}

export function showSecurity(security?: string): boolean {
  return security === 'tls' || security === 'reality';
}

export function hasBreakdown(r: { endpoints?: unknown[]; error?: string } | null | undefined): boolean {
  if (!r) return false;
  if (r.endpoints?.length) return true;
  return !!r.error;
}

export function trafficFor(outboundsTraffic: OutboundTrafficRow[], o: OutboundRow): { up: number; down: number } {
  const tr = outboundsTraffic.find((x) => x.tag === o.tag);
  return { up: tr?.up || 0, down: tr?.down || 0 };
}

export function isTesting(states: Record<number, OutboundTestState>, idx: number): boolean {
  return !!states?.[idx]?.testing;
}

export function testResult(states: Record<number, OutboundTestState>, idx: number) {
  return states?.[idx]?.result || null;
}
