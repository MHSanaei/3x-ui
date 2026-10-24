import type { TFunction } from 'i18next';

/** Xray splits `echConfigList` on the first '+' into ECH target domain and DoH URL. */
export const ECH_DOH_SEPARATOR = '+';

export const DEFAULT_ECH_DOMAIN = 'cloudflare-ech.com';
export const DEFAULT_DOH_ENDPOINT = 'https://dns.alidns.com/dns-query';

interface DohEndpoint {
  value: string;
  brand: string;
  recommended?: boolean;
}

interface DohGroup {
  key: 'china' | 'global' | 'europe' | 'udp';
  endpoints: readonly DohEndpoint[];
}

/* A resolver only works here if it publishes HTTPS RRs (carrying `ech=`). The
 * udp:// entries are plain DNS; Xray dials them the same way, by prefix. */
export const DOH_ENDPOINT_GROUPS: readonly DohGroup[] = [
  {
    key: 'china',
    endpoints: [
      { value: 'https://dns.alidns.com/dns-query', brand: 'AliDNS', recommended: true },
      { value: 'https://sm2.doh.pub/dns-query', brand: 'DNSPod SM2' },
      { value: 'https://doh.360.cn/dns-query', brand: '360' },
      { value: 'https://doh.onedns.net/dns-query', brand: 'OneDNS' },
    ],
  },
  {
    key: 'global',
    endpoints: [
      { value: 'https://cloudflare-dns.com/dns-query', brand: 'Cloudflare' },
      { value: 'https://1.1.1.1/dns-query', brand: 'Cloudflare IP' },
      { value: 'https://dns.google/dns-query', brand: 'Google' },
      { value: 'https://dns.quad9.net/dns-query', brand: 'Quad9' },
    ],
  },
  {
    key: 'europe',
    endpoints: [
      { value: 'https://doh.applied-privacy.net/query', brand: 'Applied Privacy' },
      { value: 'https://odvr.nic.cz/doh', brand: 'CZ.NIC' },
      { value: 'https://dnsforge.de/dns-query', brand: 'DNSforge' },
      { value: 'https://doh.dns.sb/dns-query', brand: 'DNS.SB' },
    ],
  },
  {
    key: 'udp',
    endpoints: [
      { value: 'udp://208.67.220.220:443', brand: 'OpenDNS' },
      { value: 'udp://149.112.112.112:9953', brand: 'Quad9' },
      { value: 'udp://45.90.28.0:5353', brand: 'NextDNS' },
    ],
  },
] as const;

interface EchDomainPreset {
  value: string;
  brand: string;
  recommended?: boolean;
}

/* Targets that publish HTTPS RRs, plus the brand the operator recognises. */
export const ECH_DOMAIN_PRESETS: readonly EchDomainPreset[] = [
  { value: 'cloudflare-ech.com', brand: 'Cloudflare ECH', recommended: true },
  { value: 'crypto.cloudflare.com', brand: 'Cloudflare Crypto' },
  { value: 'encryptedsni.com', brand: 'Cloudflare Encrypted SNI' },
  { value: 'icook.hk', brand: 'iCook HK' },
  { value: 'cm.edu.kg', brand: 'CM University' },
  { value: 'godotengine.org', brand: 'Godot Engine' },
  { value: 'www.britannica.com', brand: 'Britannica' },
  { value: 'www.prometheus.io', brand: 'Prometheus' },
  { value: 'www.kyocera.com', brand: 'Kyocera' },
  { value: 'celestia.org', brand: 'Celestia' },
  { value: 'lido.fi', brand: 'Lido' },
] as const;

const DOMAIN_RE = /^([a-z0-9-]+\.)+[a-z]{2,}$/i;

/* Xray dials the DoH half by case-sensitive prefix match: https:// and h2c://
 * over HTTP, udp:// plain DNS. Anything else never resolves. */
export const SUPPORTED_DOH_SCHEMES = ['https', 'h2c', 'udp'] as const;

function normalizeDohUrl(raw: string): string | null {
  const url = raw.trim();
  if (!url || /\s/.test(url)) return null;
  const match = /^([A-Za-z][A-Za-z0-9+.-]*):\/\//.exec(url);
  if (!match) return null;
  const scheme = match[1].toLowerCase();
  if (!(SUPPORTED_DOH_SCHEMES as readonly string[]).includes(scheme)) return null;
  try {
    if (!new URL(url).hostname) return null;
  } catch {
    return null;
  }
  // Lower-case the scheme only: Xray's prefix match rejects "HTTPS://" otherwise.
  return `${scheme}://${url.slice(match[0].length)}`;
}

export function formatDynamicEch(domain: string, dohUrl: string): string | null {
  const d = domain.trim();
  const u = normalizeDohUrl(dohUrl);
  if (!DOMAIN_RE.test(d) || d.includes(ECH_DOH_SEPARATOR)) return null;
  if (!u) return null;
  return `${d}${ECH_DOH_SEPARATOR}${u}`;
}

export function parseDynamicEch(value: string): { domain: string; dohUrl: string } | null {
  const raw = value.trim();
  const split = raw.indexOf(ECH_DOH_SEPARATOR);
  if (split <= 0) return null;
  const domain = raw.slice(0, split).trim();
  const dohUrl = normalizeDohUrl(raw.slice(split + 1));
  if (!DOMAIN_RE.test(domain) || !dohUrl) return null;
  return { domain, dohUrl };
}

export function isSupportedDohUrl(url: string): boolean {
  return normalizeDohUrl(url) !== null;
}

interface PresetOption {
  value: string;
  label: string;
}

export function dohEndpointOptions(t: TFunction): { label: string; options: PresetOption[] }[] {
  const groupLabels: Record<(typeof DOH_ENDPOINT_GROUPS)[number]['key'], string> = {
    china: t('pages.inbounds.form.echDohGroup.china'),
    global: t('pages.inbounds.form.echDohGroup.global'),
    europe: t('pages.inbounds.form.echDohGroup.europe'),
    udp: t('pages.inbounds.form.echDohGroup.udp'),
  };
  const recommended = t('pages.inbounds.form.echRecommended');
  return DOH_ENDPOINT_GROUPS.map((group) => ({
    label: groupLabels[group.key],
    options: group.endpoints.map((e) => ({
      value: e.value,
      label:
        'recommended' in e && e.recommended
          ? `${e.brand} — ${recommended} · ${e.value}`
          : `${e.brand} · ${e.value}`,
    })),
  }));
}

export function echDomainOptions(t: TFunction): PresetOption[] {
  const recommended = t('pages.inbounds.form.echRecommended');
  return ECH_DOMAIN_PRESETS.map((d) => ({
    value: d.value,
    label:
      'recommended' in d && d.recommended
        ? `${d.value} — ${recommended} · ${d.brand}`
        : `${d.value} · ${d.brand}`,
  }));
}

export const DEFAULT_ECH_PAIR = `${DEFAULT_ECH_DOMAIN}${ECH_DOH_SEPARATOR}${DEFAULT_DOH_ENDPOINT}`;

/** Ready-made pairs for the single preset dropdown, grouped by target front. */
export function pairPresetOptions(t: TFunction): { label: string; options: PresetOption[] }[] {
  const recommended = t('pages.inbounds.form.echRecommended');
  const cloudflare = DOH_ENDPOINT_GROUPS.flatMap((g) => g.endpoints).map((e) => ({
    value: `${DEFAULT_ECH_DOMAIN}${ECH_DOH_SEPARATOR}${e.value}`,
    label:
      'recommended' in e && e.recommended
        ? `${e.brand} · ${e.value} — ${recommended}`
        : `${e.brand} · ${e.value}`,
  }));
  const others = ECH_DOMAIN_PRESETS.filter((d) => d.value !== DEFAULT_ECH_DOMAIN).map((d) => ({
    value: `${d.value}${ECH_DOH_SEPARATOR}${DEFAULT_DOH_ENDPOINT}`,
    label: `${d.value} · ${d.brand}`,
  }));
  return [
    { label: t('pages.inbounds.form.echDynamicTitle'), options: cloudflare },
    { label: t('pages.inbounds.form.echPresetOtherDomains'), options: others },
  ];
}
