import { describe, expect, it } from 'vitest';

import {
  DEFAULT_DOH_ENDPOINT,
  DEFAULT_ECH_DOMAIN,
  DOH_ENDPOINT_GROUPS,
  ECH_DOMAIN_PRESETS,
  ECH_DOH_SEPARATOR,
  formatDynamicEch,
  parseDynamicEch,
  resolveAutoEchDomain,
} from '@/lib/xray/ech-presets';

function optionValues(groups: typeof DOH_ENDPOINT_GROUPS): string[] {
  return groups.flatMap((g) => g.endpoints.map((o) => o.value));
}

describe('dynamic ECH presets', () => {
  it('every DoH preset is a scheme Xray can dial for the domain+DoH form', () => {
    // A typo'd scheme ("htps://") or a bare host silently breaks the client's
    // DoH lookup, and nothing else in the panel validates this string.
    const bad = optionValues(DOH_ENDPOINT_GROUPS).filter(
      (v) => !/^(https?|quic|udp|tcp|tls):\/\/[^/\s]+/i.test(v),
    );
    expect(bad).toEqual([]);
  });

  it('every ECH target domain is a bare registrable name, not a URL', () => {
    // Xray splits the string on the first '+', so a scheme in the domain half
    // would be sent to the DNS resolver verbatim and never resolve.
    const bad = ECH_DOMAIN_PRESETS.filter((d) => !/^([a-z0-9-]+\.)+[a-z]{2,}$/.test(d));
    expect(bad).toEqual([]);
  });

  it('carries the mainland-China direct-connect default the issue asks for', () => {
    expect(DEFAULT_ECH_DOMAIN).toBe('cloudflare-ech.com');
    expect(DEFAULT_DOH_ENDPOINT).toBe('https://dns.alidns.com/dns-query');
    expect(formatDynamicEch(DEFAULT_ECH_DOMAIN, DEFAULT_DOH_ENDPOINT)).toBe(
      'cloudflare-ech.com+https://dns.alidns.com/dns-query',
    );
  });

  it('keeps the exact issue example intact', () => {
    expect(formatDynamicEch('cloudflare-ech.com', 'https://1.1.1.1/dns-query')).toBe(
      'cloudflare-ech.com+https://1.1.1.1/dns-query',
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
