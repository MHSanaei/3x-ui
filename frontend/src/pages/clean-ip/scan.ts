import type { HostRecord } from '@/schemas/api/host';
import type { InboundOption } from '@/schemas/client';

import { coloPlace } from './colos';

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

export function scanTargets(ranges: string[], perRange: number): string[] {
  const ips = ranges.flatMap((cidr) => randomIps(cidr, perRange));
  for (let i = ips.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [ips[i], ips[j]] = [ips[j], ips[i]];
  }
  return ips;
}

function ipv4(text: string): string | null {
  const parts = text.split('.');
  if (parts.length !== 4 || parts.some((p) => !/^\d{1,3}$/.test(p) || Number(p) > 255)) return null;
  return parts.map(Number).join('.');
}

export interface CleanIpCdn {
  id: string;
  name: string;
  groups: { id: string; on: boolean; ranges: string[] }[];
}

export function defaultSelection(cdn: CleanIpCdn): string[] {
  return cdn.groups.filter((g) => g.on).flatMap((g) => g.ranges);
}

// What both tests sample: the ticked ranges plus the admin's extra ones (IPv4 /8 to /30).
export function scanRanges(ticked: string[], extraText: string): string[] {
  const out = new Set(ticked);
  for (const word of extraText.split(/[\s,;]+/)) {
    const [base, bits, rest] = word.split('/');
    const ip = ipv4(base);
    const n = Number(bits);
    if (ip && rest === undefined && /^\d{1,2}$/.test(bits ?? '') && n >= 8 && n <= 30)
      out.add(`${ip}/${n}`);
  }
  return [...out];
}

// The admin's own list: single IPs are tested as given, each range gives perRange random IPs.
export function parseTargets(text: string, perRange: number): string[] {
  const out = new Set<string>();
  const words = text.replace(/#[^\n]*/g, ' ').split(/[\s,;]+/);
  for (const word of words) {
    const [base, bits, extra] = word.split('/');
    const ip = ipv4(base);
    if (!ip || extra !== undefined) continue;
    if (bits === undefined) {
      out.add(ip);
      continue;
    }
    const n = Number(bits);
    if (!/^\d{1,2}$/.test(bits) || n > 32) continue;
    if (n >= 31) out.add(ip);
    else for (const hit of randomIps(`${ip}/${n}`, perRange)) out.add(hit);
  }
  return [...out];
}

// Edges next to a working one usually sit behind the same route, so its /24 is worth a look.
export function nearbyIps(working: string[], tested: Set<string>, perIp: number): string[] {
  const out = new Set<string>();
  for (const ip of working) {
    const prefix = ip.split('.').slice(0, 3).join('.');
    const free: string[] = [];
    for (let last = 1; last <= 254; last++) {
      const candidate = `${prefix}.${last}`;
      if (!tested.has(candidate) && !out.has(candidate)) free.push(candidate);
    }
    for (let i = 0; i < perIp && free.length; i++) {
      out.add(free.splice(Math.floor(Math.random() * free.length), 1)[0]);
    }
  }
  return [...out];
}

export function resultsCsv(rows: { ip: string; ms: number; colo: string }[], lang: string): string {
  const lines = rows.map((r) => {
    const place = coloPlace(r.colo, lang);
    return [r.ip, r.ms, r.colo, place?.city ?? '', place?.country ?? ''].join(',');
  });
  return ['ip,ms,colo,city,country', ...lines].join('\n') + '\n';
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

// The CDN domain the tests use as SNI: what the group's configs present to Cloudflare.
export function groupDomain(group: HostRecord): string {
  const named = group.hosts.map(hostAddress).find((h) => !/^[\d.]+$/.test(h) && !h.includes(':'));
  return group.sni || group.hostHeader || named || '';
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

const INBOUND_TARGET = 'inbound:';
// transports a CDN can carry; raw TCP / REALITY inbounds cannot sit behind one
const CDN_NETWORKS = new Set(['ws', 'httpupgrade', 'xhttp', 'splithttp', 'grpc']);

// Inbounds no host group serves yet, as unsaved groups: saving the scan creates their host.
export function inboundTargets(options: InboundOption[], all: HostRecord[]): HostRecord[] {
  const served = new Set(all.flatMap((g) => g.inboundIds));
  return options
    .filter((o) => CDN_NETWORKS.has(o.network ?? '') && !served.has(o.id))
    .map((o) => ({
      groupId: `${INBOUND_TARGET}${o.id}`,
      inboundIds: [o.id],
      hosts: [],
      remark: o.remark || `#${o.id}`,
      port: 443,
      security: 'tls',
    }));
}

// The domain an inbound's clients present: transport host, Host header, then TLS server name.
export function inboundDomain(streamSettings: unknown): string {
  let s: Record<string, unknown> = {};
  try {
    const parsed: unknown =
      typeof streamSettings === 'string' ? JSON.parse(streamSettings) : streamSettings;
    if (parsed && typeof parsed === 'object') s = parsed as Record<string, unknown>;
  } catch {
    return '';
  }
  const obj = (v: unknown) => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});
  const str = (v: unknown) => (typeof v === 'string' ? v.trim() : '');
  const transport = obj(s[`${str(s.network)}Settings`]);
  const headers = obj(transport.headers);
  return (
    str(transport.host) ||
    str(headers.Host) ||
    str(headers.host) ||
    str(obj(s.tlsSettings).serverName)
  );
}

export function isInboundTarget(group: HostRecord): boolean {
  return group.groupId.startsWith(INBOUND_TARGET);
}

export type SaveTo = 'same' | 'isp';

// Where the picked IPs go: the chosen host itself, or its "<remark> · <ISP>" group.
export function planSave(o: {
  group: HostRecord;
  ispGroups: Map<string, HostRecord>;
  isp: string;
  to: SaveTo;
  entries: string[];
  domain: string;
  fragment: FragmentSetting | null | undefined;
}) {
  const target =
    o.to === 'isp' ? o.ispGroups.get(o.isp) : isInboundTarget(o.group) ? undefined : o.group;
  const remark =
    o.to === 'isp'
      ? `${o.group.remark ?? ''}${ISP_SEP}${o.isp}`.slice(0, 256)
      : (o.group.remark ?? '');
  const values = groupPayload(target ?? o.group, remark, o.entries);
  values.isDisabled = false;
  if (o.fragment !== undefined) values.finalMask = withFragment(values.finalMask, o.fragment);
  // the IPs only work with the domain they were tested under as SNI
  if (!values.sni && !values.snis?.length && !values.keepSniBlank && o.domain) {
    values.sni = o.domain;
    values.snis = [o.domain];
  }
  return { groupId: target?.groupId, remark, values };
}
