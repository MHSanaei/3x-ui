type Raw = Record<string, unknown>;

/** The EDNS0 payload the pre-26.9.30 xdns always negotiated; without it answers cap at 512. */
export const XDNS_LEGACY_EDNS0 = 1232;

const LEGACY_RECORD_TYPES: Record<string, number> = { '': 16, txt: 16, a: 1, aaaa: 28 };

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
  return { name, types: [type], edns0: XDNS_LEGACY_EDNS0 };
}

// xray-core 26.9.30 (#6718) parses xdns domains/resolvers only as objects. Mirrors
// internal/util/maskcompat: a bare name becomes TXT, entries the old core refused are dropped.
export function upgradeLegacyXdnsSettings(settings: Raw): { next: Raw; changed: boolean } {
  const rawDomains = Array.isArray(settings.domains) ? (settings.domains as unknown[]) : [];
  const rawResolvers = Array.isArray(settings.resolvers) ? (settings.resolvers as unknown[]) : [];
  const isLegacy = (v: unknown) => typeof v === 'string';
  if (!rawDomains.some(isLegacy) && !rawResolvers.some(isLegacy)) {
    return { next: settings, changed: false };
  }
  const domains: unknown[] = [];
  const listed = new Set<string>();
  const addDomain = (domain: Raw) => {
    const key = String(domain.name ?? '').toLowerCase();
    if (key && listed.has(key)) return;
    listed.add(key);
    domains.push(domain);
  };
  for (const entry of rawDomains) {
    if (typeof entry === 'string') {
      const domain = legacyDomain(entry);
      if (domain) addDomain(domain);
    } else if (entry && typeof entry === 'object') {
      addDomain(entry as Raw);
    }
  }
  const resolvers: unknown[] = [];
  for (const entry of rawResolvers) {
    if (typeof entry !== 'string') {
      resolvers.push(entry);
      continue;
    }
    const sep = entry.indexOf('+udp://');
    const addr = sep >= 0 ? entry.slice(sep + '+udp://'.length).trim() : '';
    const domain = sep >= 0 ? legacyDomain(entry.slice(0, sep)) : null;
    if (!addr || !domain) continue;
    addDomain(domain);
    resolvers.push({ type: 'udp', settings: { addr } });
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
