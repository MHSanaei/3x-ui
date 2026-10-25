import type { HostRecord } from '@/schemas/api/host';

// https://www.cloudflare.com/ips-v4/
export const CLOUDFLARE_RANGES = [
  '173.245.48.0/20',
  '103.21.244.0/22',
  '103.22.200.0/22',
  '103.31.4.0/22',
  '141.101.64.0/18',
  '108.162.192.0/18',
  '190.93.240.0/20',
  '188.114.96.0/20',
  '197.234.240.0/22',
  '198.41.128.0/17',
  '162.158.0.0/15',
  '104.16.0.0/13',
  '104.24.0.0/14',
  '172.64.0.0/13',
  '131.0.72.0/22',
];

export function randomIps(cidr: string, count: number): string[] {
  const [base, bits] = cidr.split('/');
  const start = base.split('.').reduce((acc, part) => acc * 256 + Number(part), 0);
  const size = 2 ** (32 - Number(bits));
  const out = new Set<string>();
  for (let tries = 0; out.size < Math.min(count, size - 2) && tries < count * 20; tries++) {
    const n = start + 1 + Math.floor(Math.random() * (size - 2));
    out.add([24, 16, 8, 0].map((shift) => Math.floor(n / 2 ** shift) % 256).join('.'));
  }
  return [...out];
}

export function scanTargets(perRange: number): string[] {
  const ips = CLOUDFLARE_RANGES.flatMap((cidr) => randomIps(cidr, perRange));
  for (let i = ips.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [ips[i], ips[j]] = [ips[j], ips[i]];
  }
  return ips;
}

// The panel's CSP allows wss: to any host, and a WebSocket to an IP runs the same TLS
// handshake as the real client: a fast failure means the IP answers, silence means blocked.
export function probeIp(ip: string, timeoutMs: number): Promise<{ ok: boolean; ms: number }> {
  return new Promise((resolve) => {
    const t0 = performance.now();
    let settled = false;
    let ws: WebSocket | null = null;
    const finish = (answered: boolean) => {
      if (settled) return;
      settled = true;
      window.clearTimeout(timer);
      try {
        ws?.close();
      } catch {
        // already closed
      }
      const ms = Math.round(performance.now() - t0);
      resolve({ ok: answered && ms < timeoutMs, ms });
    };
    const timer = window.setTimeout(() => finish(false), timeoutMs);
    try {
      ws = new WebSocket(`wss://${ip}/cdn-cgi/trace`);
      ws.onerror = () => finish(true);
      ws.onclose = () => finish(true);
      ws.onopen = () => finish(true);
    } catch {
      finish(false);
    }
  });
}

export const ISP_SEP = ' · ';

function sameInbounds(a: HostRecord, b: HostRecord): boolean {
  const x = [...a.inboundIds].sort((p, q) => p - q);
  const y = [...b.inboundIds].sort((p, q) => p - q);
  return x.length === y.length && x.every((v, i) => v === y[i]);
}

// Per-ISP groups are named "<main remark> · <ISP>" and serve the same inbounds.
export function ispGroupsOf(main: HostRecord, all: HostRecord[]): Map<string, HostRecord> {
  const prefix = `${main.remark ?? ''}${ISP_SEP}`;
  const out = new Map<string, HostRecord>();
  for (const g of all) {
    if (g.groupId === main.groupId || !sameInbounds(g, main)) continue;
    if ((g.remark ?? '').startsWith(prefix)) out.set((g.remark ?? '').slice(prefix.length), g);
  }
  return out;
}

export function mainGroups(all: HostRecord[]): HostRecord[] {
  const children = new Set<string>();
  for (const g of all)
    for (const child of ispGroupsOf(g, all).values()) children.add(child.groupId);
  return all.filter((g) => !children.has(g.groupId));
}

export function hostAddress(entry: string): string {
  const e = entry.trim();
  if (e.startsWith('[')) return e.slice(1, e.indexOf(']'));
  return e.split(':').length === 2 ? e.split(':')[0] : e;
}

export function joinHost(ip: string, port: number): string {
  return ip.includes(':') ? `[${ip}]:${port}` : `${ip}:${port}`;
}

export interface FragmentSetting {
  packets: string;
  length: string;
  delay: string;
}

// A host group's Final Mask (JSON) with its fragment mask replaced by `frag`, or removed for
// null; the other masks stay. xray-core #6334 takes lengths/delays as range arrays.
export function withFragment(finalMask: string | undefined, frag: FragmentSetting | null): string {
  let mask: Record<string, unknown> = {};
  try {
    const parsed: unknown = JSON.parse(finalMask || '{}');
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      mask = parsed as Record<string, unknown>;
    }
  } catch {
    mask = {};
  }
  const tcp = (Array.isArray(mask.tcp) ? mask.tcp : []).filter(
    (m: unknown) => !(m && typeof m === 'object' && (m as { type?: unknown }).type === 'fragment'),
  );
  if (frag) {
    tcp.push({
      type: 'fragment',
      settings: { packets: frag.packets, lengths: [frag.length], delays: [frag.delay] },
    });
  }
  const next: Record<string, unknown> = { ...mask };
  if (tcp.length) next.tcp = tcp;
  else delete next.tcp;
  return Object.keys(next).length ? JSON.stringify(next) : '';
}

// The group's fields as the add/update endpoints take them, with new hosts.
export function groupPayload(group: HostRecord, remark: string, hosts: string[]) {
  const { groupId: _groupId, sortOrder: _sortOrder, ...fields } = group;
  return { ...fields, remark, hosts };
}
