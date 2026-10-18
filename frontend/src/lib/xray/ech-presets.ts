import type { TFunction } from 'i18next';

/** Xray splits `echConfigList` on the first '+' into ECH target domain and DoH URL. */
export const ECH_DOH_SEPARATOR = '+';

export const DEFAULT_ECH_DOMAIN = 'cloudflare-ech.com';
export const DEFAULT_DOH_ENDPOINT = 'https://dns.alidns.com/dns-query';

/* A resolver only works here if it publishes HTTPS RRs (carrying `ech=`); the
 * issue's remaining hosts did not answer when this list was assembled. */
export const DOH_ENDPOINT_GROUPS = [
  {
    key: 'china',
    endpoints: [
      { value: 'https://dns.alidns.com/dns-query', recommended: true },
      { value: 'https://sm2.doh.pub/dns-query' },
      { value: 'https://doh.360.cn/dns-query' },
      { value: 'https://doh.onedns.net/dns-query' },
    ],
  },
  {
    key: 'global',
    endpoints: [
      { value: 'https://cloudflare-dns.com/dns-query' },
      { value: 'https://1.1.1.1/dns-query' },
      { value: 'https://dns.google/dns-query' },
      { value: 'https://dns.quad9.net/dns-query' },
    ],
  },
  {
    key: 'europe',
    endpoints: [
      { value: 'https://doh.applied-privacy.net/query' },
      { value: 'https://odvr.nic.cz/doh' },
      { value: 'https://dnsforge.de/dns-query' },
      { value: 'https://doh.dns.sb/dns-query' },
    ],
  },
] as const;

/* Targets that publish HTTPS RRs: a name without one yields no ECH at all. */
export const ECH_DOMAIN_PRESETS = [
  'cloudflare-ech.com',
  'crypto.cloudflare.com',
  'encryptedsni.com',
  'icook.hk',
  'cm.edu.kg',
  'godotengine.org',
  'www.britannica.com',
  'www.prometheus.io',
  'www.kyocera.com',
  'celestia.org',
  'lido.fi',
] as const;

const DOMAIN_RE = /^([a-z0-9-]+\.)+[a-z]{2,}$/i;

/* Xray's ECH resolver dials the DoH half by literal prefix match: https:// and
 * h2c:// over HTTP, udp:// plain DNS. Any other scheme silently never resolves. */
export const SUPPORTED_DOH_SCHEMES = ['https', 'h2c', 'udp'] as const;
const DOH_URI_RE = /^(https|h2c|udp):\/\/[^\s/+]+/i;

export function formatDynamicEch(domain: string, dohUrl: string): string | null {
  const d = domain.trim();
  const u = dohUrl.trim();
  if (!DOMAIN_RE.test(d) || d.includes(ECH_DOH_SEPARATOR)) return null;
  if (!DOH_URI_RE.test(u)) return null;
  return `${d}${ECH_DOH_SEPARATOR}${u}`;
}

export function parseDynamicEch(value: string): { domain: string; dohUrl: string } | null {
  const raw = value.trim();
  const split = raw.indexOf(ECH_DOH_SEPARATOR);
  if (split <= 0) return null;
  const domain = raw.slice(0, split).trim();
  const dohUrl = raw.slice(split + 1).trim();
  if (!DOMAIN_RE.test(domain) || !DOH_URI_RE.test(dohUrl)) return null;
  return { domain, dohUrl };
}

export function isSupportedDohUrl(url: string): boolean {
  return DOH_URI_RE.test(url.trim());
}

/** null = leave the name to the client, which resolves whatever SNI it dials. */
export function resolveAutoEchDomain(sni: string, rejectUnknownSni: boolean): string | null {
  const name = sni.trim();
  if (rejectUnknownSni || !DOMAIN_RE.test(name)) return null;
  return name;
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
  };
  return DOH_ENDPOINT_GROUPS.map((group) => ({
    label: groupLabels[group.key],
    options: group.endpoints.map((e) => ({
      value: e.value,
      label:
        'recommended' in e && e.recommended
          ? `${e.value} — ${t('pages.inbounds.form.echRecommended')}`
          : e.value,
    })),
  }));
}

export function echDomainOptions(): PresetOption[] {
  return ECH_DOMAIN_PRESETS.map((d) => ({ value: d, label: d }));
}
