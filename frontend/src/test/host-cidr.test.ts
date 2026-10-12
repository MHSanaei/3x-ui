import { describe, expect, it } from 'vitest';

import {
  cleanCIDR,
  formatIPv4,
  formatIPv6,
  isCIDR,
  MAX_RANDOM_CIDR_COUNT,
  parseIPv4,
  parseIPv6,
  randomIPsFromCIDR,
} from '@/lib/hosts/host-cidr';

describe('cleanCIDR', () => {
  it('handles standard and bracketed forms', () => {
    expect(cleanCIDR('192.168.1.0/24')).toBe('192.168.1.0/24');
    expect(cleanCIDR('[192.168.1.0/24]')).toBe('192.168.1.0/24');
    expect(cleanCIDR('2001:db8::/64')).toBe('2001:db8::/64');
    expect(cleanCIDR('[2001:db8::/64]')).toBe('2001:db8::/64');
    expect(cleanCIDR('[2001:db8::]/64')).toBe('2001:db8::/64');
  });
});

describe('isCIDR', () => {
  it('validates IPv4 CIDR strings', () => {
    expect(isCIDR('192.168.1.0/24')).toBe(true);
    expect(isCIDR('10.0.0.0/8')).toBe(true);
    expect(isCIDR('0.0.0.0/0')).toBe(true);
    expect(isCIDR('192.168.1.1/32')).toBe(true);
    expect(isCIDR('192.168.1.1/33')).toBe(false);
    expect(isCIDR('192.168.1.1')).toBe(false);
  });

  it('validates IPv6 CIDR strings including bracketed forms', () => {
    expect(isCIDR('2001:db8::/64')).toBe(true);
    expect(isCIDR('[2001:db8::]/64')).toBe(true);
    expect(isCIDR('[2001:db8::/64]')).toBe(true);
    expect(isCIDR('::/0')).toBe(true);
    expect(isCIDR('fe80::/10')).toBe(true);
    expect(isCIDR('2001:db8::1')).toBe(false);
    expect(isCIDR('example.com')).toBe(false);
    expect(isCIDR('')).toBe(false);
  });
});

describe('randomIPsFromCIDR', () => {
  it('generates random usable IPs for IPv4 CIDR', () => {
    const ips = randomIPsFromCIDR('192.168.1.0/24', 5);
    expect(ips).toHaveLength(5);
    const unique = new Set(ips);
    expect(unique.size).toBe(5);
    for (const ip of ips) {
      expect(ip.startsWith('192.168.1.')).toBe(true);
      expect(ip).not.toBe('192.168.1.0');
      expect(ip).not.toBe('192.168.1.255');
    }
  });

  it('generates random usable IPs for bracketed IPv6 CIDR', () => {
    const ips = randomIPsFromCIDR('[2001:db8::]/64', 3);
    expect(ips).toHaveLength(3);
    const unique = new Set(ips);
    expect(unique.size).toBe(3);
    for (const ip of ips) {
      expect(ip.startsWith('2001:db8:')).toBe(true);
      expect(ip).not.toBe('2001:db8::');
    }
  });

  it('caps count at MAX_RANDOM_CIDR_COUNT', () => {
    const ips = randomIPsFromCIDR('2001:db8::/64', 500);
    expect(ips).toHaveLength(MAX_RANDOM_CIDR_COUNT);
  });

  it('respects small subnet bounds', () => {
    const ips = randomIPsFromCIDR('192.0.2.0/30', 10);
    expect(ips).toHaveLength(2);
    expect(ips.sort()).toEqual(['192.0.2.1', '192.0.2.2']);
  });
});

describe('IP parsers and formatters', () => {
  it('handles IPv4 roundtrip', () => {
    const u32 = parseIPv4('10.20.30.40');
    expect(u32).not.toBeNull();
    expect(formatIPv4(u32!)).toBe('10.20.30.40');
  });

  it('handles IPv6 roundtrip and compression', () => {
    const v6 = parseIPv6('2001:db8::1');
    expect(v6).not.toBeNull();
    expect(formatIPv6(v6!)).toBe('2001:db8::1');

    const v6Zero = parseIPv6('::');
    expect(v6Zero).toBe(0n);
    expect(formatIPv6(v6Zero!)).toBe('::');
  });
});
