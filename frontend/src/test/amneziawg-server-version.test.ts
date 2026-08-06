import { describe, expect, it } from 'vitest';

import {
  AWG_VERSION_2,
  AWG_VERSION_3,
  AmneziawgServerSchema,
  effectiveAwgVersion,
} from '@/schemas/protocols/inbound/amneziawg';

describe('AmneziawgServerSchema — awgVersion default', () => {
  it('defaults to AWG_VERSION_2 when absent', () => {
    const parsed = AmneziawgServerSchema.parse({});
    expect(parsed.awgVersion).toBe(AWG_VERSION_2);
  });
});

describe('effectiveAwgVersion', () => {
  it('promotes a legacy record (blank awgVersion) with headerProtectionKey set', () => {
    expect(effectiveAwgVersion('', 'some-key', '')).toBe(AWG_VERSION_3);
  });

  it('promotes a legacy record (blank awgVersion) with contentPaddingAddition set', () => {
    expect(effectiveAwgVersion('', '', '50-100')).toBe(AWG_VERSION_3);
  });

  it('promotes a legacy record with both AWG3 fields set', () => {
    expect(effectiveAwgVersion(undefined, 'some-key', '50-100')).toBe(AWG_VERSION_3);
  });

  it('leaves a brand new blank record at AWG_VERSION_2', () => {
    expect(effectiveAwgVersion('', '', '')).toBe(AWG_VERSION_2);
    expect(effectiveAwgVersion(undefined, undefined, undefined)).toBe(AWG_VERSION_2);
  });

  it('never overrides an explicit awgVersion, even if inconsistent with the AWG3 fields', () => {
    expect(effectiveAwgVersion(AWG_VERSION_2, 'some-key', '')).toBe(AWG_VERSION_2);
    expect(effectiveAwgVersion(AWG_VERSION_2, '', '50-100')).toBe(AWG_VERSION_2);
    expect(effectiveAwgVersion(AWG_VERSION_3, '', '')).toBe(AWG_VERSION_3);
  });
});
