type Raw = Record<string, unknown>;

/** The EDNS0 payload the pre-26.9.30 xdns always negotiated; without it answers cap at 512. */
export const XDNS_LEGACY_EDNS0 = 1232;

const LEGACY_RECORD_TYPES: Record<string, number> = { '': 16, txt: 16, a: 1, aaaa: 28 };

function isRaw(value: unknown): value is Raw {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function legacyDomain(spec: string): Raw | null {
  let name = spec.trim();
  let method = '';
  const colon = name.lastIndexOf(':');
  if (colon >= 0) {
    method = name.slice(colon + 1).toLowerCase();
    name = name.slice(0, colon);
  }
  name = name.replace(/^\.+|\.+$/g, '');
  const type = LEGACY_RECORD_TYPES[method];
  if (!name || type === undefined) return null;
  return { names: [name], types: [type], edns0: XDNS_LEGACY_EDNS0 };
}

/** Folds a 26.9.30 domain's single `name` into the `names` list. */
function upgradeDomainObject(domain: Raw): Raw {
  if (!('name' in domain)) return domain;
  const { name: raw, ...rest } = domain;
  const name = typeof raw === 'string' ? raw.trim() : '';
  const names = Array.isArray(rest.names) ? [...(rest.names as unknown[])] : [];
  if (name && !names.includes(name)) names.unshift(name);
  return { ...rest, names };
}

/** A 26.9.30 {type, settings:{addr}} resolver as an addrs URL; that loader took only udp/tcp. */
function legacyResolverAddr(resolver: Raw): string | null {
  const kind = typeof resolver.type === 'string' ? resolver.type.trim().toLowerCase() : '';
  const settings = isRaw(resolver.settings) ? resolver.settings : {};
  const addr = typeof settings.addr === 'string' ? settings.addr.trim() : '';
  if ((kind !== 'udp' && kind !== 'tcp') || !addr) return null;
  return `${kind}://${addr}`;
}

const isLegacyDomain = (v: unknown) => typeof v === 'string' || (isRaw(v) && 'name' in v);
const isLegacyResolver = (v: unknown) => typeof v === 'string' || (isRaw(v) && !('addrs' in v));

// xray-core 26.10.10 (#7090) reads xdns only as names/addrs lists. Mirrors
// internal/util/maskcompat: a bare name becomes TXT, entries the old core refused are dropped.
export function upgradeLegacyXdnsSettings(settings: Raw): { next: Raw; changed: boolean } {
  const rawDomains = Array.isArray(settings.domains) ? (settings.domains as unknown[]) : [];
  const rawResolvers = Array.isArray(settings.resolvers) ? (settings.resolvers as unknown[]) : [];
  if (!rawDomains.some(isLegacyDomain) && !rawResolvers.some(isLegacyResolver)) {
    return { next: settings, changed: false };
  }
  const domains: unknown[] = [];
  const listed = new Set<string>();
  const addDomain = (domain: Raw) => {
    const names = Array.isArray(domain.names) ? (domain.names as unknown[]) : [];
    for (const name of names) if (typeof name === 'string') listed.add(name.toLowerCase());
    domains.push(domain);
  };
  const addLegacyDomain = (domain: Raw) => {
    const [name] = domain.names as string[];
    if (!listed.has(name.toLowerCase())) addDomain(domain);
  };
  for (const entry of rawDomains) {
    if (typeof entry === 'string') {
      const domain = legacyDomain(entry);
      if (domain) addLegacyDomain(domain);
    } else if (isRaw(entry)) {
      addDomain(upgradeDomainObject(entry));
    }
  }
  const resolvers: unknown[] = [];
  for (const entry of rawResolvers) {
    if (typeof entry === 'string') {
      const sep = entry.indexOf('+udp://');
      const addr = sep >= 0 ? entry.slice(sep + '+udp://'.length).trim() : '';
      const domain = sep >= 0 ? legacyDomain(entry.slice(0, sep)) : null;
      if (!addr || !domain) continue;
      addLegacyDomain(domain);
      resolvers.push({ addrs: [`udp://${addr}`] });
    } else if (isRaw(entry) && isLegacyResolver(entry)) {
      const addr = legacyResolverAddr(entry);
      if (addr) resolvers.push({ addrs: [addr] });
    } else {
      resolvers.push(entry);
    }
  }
  const next: Raw = { ...settings, domains };
  if (resolvers.length > 0) next.resolvers = resolvers;
  else delete next.resolvers;
  return { next, changed: true };
}

/** Applies upgradeLegacyXdnsSettings to every xdns entry of a finalmask.udp list. */
export function upgradeLegacyXdnsMasks(udp: unknown[]): { next: unknown[]; changed: boolean } {
  let changed = false;
  const next = udp.map((entry) => {
    const mask = entry as Raw | null;
    if (!mask || typeof mask !== 'object' || String(mask.type).toLowerCase() !== 'xdns') {
      return entry;
    }
    if (!mask.settings || typeof mask.settings !== 'object') return entry;
    const upgraded = upgradeLegacyXdnsSettings(mask.settings as Raw);
    if (!upgraded.changed) return entry;
    changed = true;
    return { ...mask, settings: upgraded.next };
  });
  return { next, changed };
}
