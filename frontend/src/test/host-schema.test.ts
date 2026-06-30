/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { HostFormSchema } from '@/schemas/api/host';

describe('HostFormSchema', () => {
  const valid = {
    inboundId: 1,
    remark: 'cdn-front',
    address: 'cdn.example.com',
    port: 8443,
    security: 'tls',
    tags: ['CDN', 'EU'],
    mihomoIpVersion: 'dual',
    excludeFromSubTypes: ['clash'],
  };

  it('parses a valid host', () => {
    const parsed = HostFormSchema.parse(valid);
    expect(parsed.remark).toBe('cdn-front');
    expect(parsed.security).toBe('tls');
    expect(parsed.tags).toEqual(['CDN', 'EU']);
    expect(parsed.excludeFromSubTypes).toEqual(['clash']);
  });

  it('rejects an empty remark', () => {
    expect(() => HostFormSchema.parse({ ...valid, remark: '' })).toThrow();
  });

  it('accepts a templated remark up to 256 chars and rejects beyond', () => {
    expect(() => HostFormSchema.parse({ ...valid, remark: 'x'.repeat(256) })).not.toThrow();
    expect(() => HostFormSchema.parse({ ...valid, remark: 'x'.repeat(257) })).toThrow();
  });

  it('rejects an out-of-range port', () => {
    expect(() => HostFormSchema.parse({ ...valid, port: 70000 })).toThrow();
  });

  it('accepts a single vlessRoute 0-65535 and rejects specs/out-of-range', () => {
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '443' })).not.toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '0' })).not.toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '65535' })).not.toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '' })).not.toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '53,443' })).toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '1000-2000' })).toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: '70000' })).toThrow();
    expect(() => HostFormSchema.parse({ ...valid, vlessRoute: 'abc' })).toThrow();
  });

  it('rejects a bad security enum', () => {
    expect(() => HostFormSchema.parse({ ...valid, security: 'bogus' })).toThrow();
  });

  it('rejects a tag with invalid characters', () => {
    expect(() => HostFormSchema.parse({ ...valid, tags: ['lower-case'] })).toThrow();
  });

  it('rejects more than 10 tags', () => {
    expect(() =>
      HostFormSchema.parse({ ...valid, tags: Array.from({ length: 11 }, (_, i) => `T${i}`) }),
    ).toThrow();
  });

  it('rejects a bad mihomoIpVersion enum', () => {
    expect(() => HostFormSchema.parse({ ...valid, mihomoIpVersion: 'nope' })).toThrow();
  });

  it('rejects a bad excludeFromSubTypes value', () => {
    expect(() => HostFormSchema.parse({ ...valid, excludeFromSubTypes: ['xml'] })).toThrow();
  });

  it('defaults security to "same" and port to 0', () => {
    const parsed = HostFormSchema.parse({ inboundId: 1, remark: 'r' });
    expect(parsed.security).toBe('same');
    expect(parsed.port).toBe(0);
    expect(parsed.tags).toEqual([]);
  });
});
