import { describe, expect, it } from 'vitest';
import type { TFunction } from 'i18next';

import {
  DEFAULT_DOH_ENDPOINT,
  DEFAULT_ECH_DOMAIN,
  DEFAULT_ECH_PAIR,
  DOH_ENDPOINT_GROUPS,
  ECH_DOMAIN_PRESETS,
  ECH_DOH_SEPARATOR,
  formatDynamicEch,
  isSupportedDohUrl,
  pairPresetOptions,
  parseDynamicEch,
} from '@/lib/xray/ech-presets';

function optionValues(groups: typeof DOH_ENDPOINT_GROUPS): string[] {
  return groups.flatMap((g) => g.endpoints.map((o) => o.value));
}

const t = ((key: string) => key) as unknown as TFunction;

describe('dynamic ECH presets', () => {
  it('every shipped DoH preset is one Xray can actually resolve', () => {
    // A preset with any other scheme passes review but never resolves, because
    // Xray matches the scheme by literal prefix before dialing.
    const bad = optionValues(DOH_ENDPOINT_GROUPS).filter((v) => !isSupportedDohUrl(v));
    expect(bad).toEqual([]);
  });

  it('every ECH target domain is a bare registrable name, not a URL', () => {
    // Xray splits the string on the first '+', so a scheme in the domain half
    // would be sent to the DNS resolver verbatim and never resolve.
    const bad = ECH_DOMAIN_PRESETS.filter((d) => !/^([a-z0-9-]+\.)+[a-z]{2,}$/.test(d.value));
    expect(bad.map((d) => d.value)).toEqual([]);
  });

  it('every preset dropdown pair round-trips through the Xray parser', () => {
    // The dropdown is a single click away from echConfigList; a pair the parser
    // rejects here would ship a value every client fails to resolve.
    const pairs = pairPresetOptions(t).flatMap((g) => g.options.map((o) => o.value));
    expect(pairs.length).toBeGreaterThan(0);
    expect(pairs.filter((p) => parseDynamicEch(p) === null)).toEqual([]);
    expect(pairs).toContain(DEFAULT_ECH_PAIR);
  });

  it('offers every resolver under the Cloudflare front, udp ones included', () => {
    const [cloudflare] = pairPresetOptions(t);
    expect(cloudflare.options).toHaveLength(optionValues(DOH_ENDPOINT_GROUPS).length);
    expect(cloudflare.options.some((o) => o.value.includes('udp://'))).toBe(true);
  });

  it.each(['https', 'h2c', 'udp'])('builds a value for the %s scheme Xray supports', (scheme) => {
    const url = `${scheme}://dns.example.com/dns-query`;
    expect(formatDynamicEch('cloudflare-ech.com', url)).toBe(`cloudflare-ech.com+${url}`);
  });

  it.each(['http', 'quic', 'tcp', 'tls', 'doq', 'ftp'])(
    'rejects the %s scheme at every entry point',
    (scheme) => {
      // Accepting these would ship a config that fails at the client, where the
      // operator cannot see the panel's advice any more.
      const url = `${scheme}://1.1.1.1/dns-query`;
      expect(isSupportedDohUrl(url)).toBe(false);
      expect(formatDynamicEch('cloudflare-ech.com', url)).toBeNull();
      expect(parseDynamicEch(`cloudflare-ech.com+${url}`)).toBeNull();
    },
  );

  it('preselects Cloudflare over AliDNS, the mainland-China reachable pair', () => {
    expect(formatDynamicEch(DEFAULT_ECH_DOMAIN, DEFAULT_DOH_ENDPOINT)).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );
  });

  it('trims pasted whitespace instead of emitting an unresolvable string', () => {
    expect(formatDynamicEch('  cloudflare-ech.com ', ' https://dns.google/dns-query ')).toBe(
      'cloudflare-ech.com+https://dns.google/dns-query',
    );
  });

  it('rejects a domain that already embeds a DoH URL', () => {
    // Pasting a full existing value into the domain box would otherwise produce
    // "...+https://a+https://b", which no client can parse.
    expect(
      formatDynamicEch(
        'cloudflare-ech.com+https://dns.alidns.com/dns-query',
        'https://dns.google/dns-query',
      ),
    ).toBeNull();
  });

  it('rejects a DoH half that is not a URI', () => {
    expect(formatDynamicEch('cloudflare-ech.com', 'dns.google')).toBeNull();
    expect(formatDynamicEch('cloudflare-ech.com', '')).toBeNull();
    expect(formatDynamicEch('', 'https://dns.google/dns-query')).toBeNull();
  });

  it('splits an existing dynamic value back into its two halves', () => {
    expect(
      parseDynamicEch(`cloudflare-ech.com${ECH_DOH_SEPARATOR}https://dns.google/dns-query`),
    ).toEqual({ domain: 'cloudflare-ech.com', dohUrl: 'https://dns.google/dns-query' });
  });

  it('refuses to split a value whose DoH half uses an unsupported scheme', () => {
    // Otherwise pasting a quic:// value back into the helper would round-trip an
    // endpoint Xray can never reach.
    expect(parseDynamicEch('cloudflare-ech.com+quic://1.1.1.1/dns-query')).toBeNull();
  });

  it('does not mistake a local ECHConfigList (base64) for a dynamic value', () => {
    // "Get New ECH Cert" writes a base64 blob; the helper must not try to
    // reverse-engineer it.
    expect(parseDynamicEch('AEX+DQBB')).toBeNull();
    expect(parseDynamicEch('')).toBeNull();
  });

  it('lower-cases a pasted scheme so Xray prefix-matches it', () => {
    // Xray compares with strings.HasPrefix(server, "https://") — case-sensitive.
    // Passing "HTTPS://" through unchanged ships a config every client fails on.
    expect(formatDynamicEch('cloudflare-ech.com', 'HTTPS://dns.google/dns-query')).toBe(
      'cloudflare-ech.com+https://dns.google/dns-query',
    );
    expect(formatDynamicEch('cloudflare-ech.com', 'UDP://1.1.1.1')).toBe(
      'cloudflare-ech.com+udp://1.1.1.1',
    );
    expect(isSupportedDohUrl('H2C://dns.google/dns-query')).toBe(true);
  });

  it('rejects a resolver URL carrying whitespace Xray would put on the wire', () => {
    // The old prefix-only regex accepted these and the junk went into the value.
    expect(isSupportedDohUrl('https://dns.google/dns-query extra')).toBe(false);
    expect(isSupportedDohUrl('https://dns.google /dns-query')).toBe(false);
    expect(formatDynamicEch('cloudflare-ech.com', 'https://dns.google/dns-query extra')).toBeNull();
  });

  it('rejects a resolver URL with no host', () => {
    // new URL("https://?x") parses but carries an empty hostname: Xray would
    // dial nothing and fail the handshake.
    expect(isSupportedDohUrl('https://?x')).toBe(false);
    expect(formatDynamicEch('cloudflare-ech.com', 'https://?x')).toBeNull();
  });

  it('defaults the target to Cloudflare, not the inbound SNI', () => {
    // Behind Cloudflare the ECH record belongs to Cloudflare's ECH front, so the
    // helper must not derive it from the proxied hostname.
    expect(DEFAULT_ECH_DOMAIN).toBe('cloudflare-ech.com');
    expect(formatDynamicEch(DEFAULT_ECH_DOMAIN, DEFAULT_DOH_ENDPOINT)).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );
  });
});
