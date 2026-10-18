import { describe, expect, it } from 'vitest';

import {
  DEFAULT_DOH_ENDPOINT,
  DEFAULT_ECH_DOMAIN,
  DOH_ENDPOINT_GROUPS,
  ECH_DOMAIN_PRESETS,
  ECH_DOH_SEPARATOR,
  formatDynamicEch,
  isSupportedDohUrl,
  parseDynamicEch,
  resolveAutoEchDomain,
  SUPPORTED_DOH_SCHEMES,
} from '@/lib/xray/ech-presets';

function optionValues(groups: typeof DOH_ENDPOINT_GROUPS): string[] {
  return groups.flatMap((g) => g.endpoints.map((o) => o.value));
}

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
    const bad = ECH_DOMAIN_PRESETS.filter((d) => !/^([a-z0-9-]+\.)+[a-z]{2,}$/.test(d));
    expect(bad).toEqual([]);
  });

  it.each(['https', 'h2c', 'udp'])('builds a value for the %s scheme Xray supports', (scheme) => {
    const url = `${scheme}://dns.example.com/dns-query`;
    expect(formatDynamicEch('cloudflare-ech.com', url)).toBe(`cloudflare-ech.com+${url}`);
  });

  it.each(['http', 'quic', 'tcp', 'tls', 'doq', 'ftp'])(
    'rejects the %s scheme, which Xray never dials for ECH',
    (scheme) => {
      // Accepting these would ship a config that fails at the client, where the
      // operator cannot see the panel's advice any more.
      expect(formatDynamicEch('cloudflare-ech.com', `${scheme}://1.1.1.1/dns-query`)).toBeNull();
    },
  );

  it('exposes exactly the three schemes its own parser accepts', () => {
    expect([...SUPPORTED_DOH_SCHEMES]).toEqual(['https', 'h2c', 'udp']);
    for (const scheme of ['http', 'quic', 'tcp', 'tls']) {
      expect(SUPPORTED_DOH_SCHEMES).not.toContain(scheme);
    }
  });

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

  it('falls back to the node SNI for automatically resolved ECH', () => {
    expect(resolveAutoEchDomain('t.example.com', false)).toBe('t.example.com');
    expect(resolveAutoEchDomain(' t.example.com ', false)).toBe('t.example.com');
  });

  it('ignores an SNI that is an IP literal, which can never carry ECH', () => {
    expect(resolveAutoEchDomain('203.0.113.10', false)).toBeNull();
    expect(resolveAutoEchDomain('2001:db8::1', false)).toBeNull();
  });

  it('never auto-resolves while the inbound only accepts unknown SNIs', () => {
    // rejectUnknownSni means the target name is not the SNI; reusing it would
    // advertise an ECH domain the server does not serve.
    expect(resolveAutoEchDomain('t.example.com', true)).toBeNull();
  });
});
