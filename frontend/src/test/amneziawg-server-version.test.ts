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

  it('defaults the 5 AWG3 timer fields to an empty string', () => {
    const parsed = AmneziawgServerSchema.parse({});
    expect(parsed.rekeyAfterTime).toBe('');
    expect(parsed.rekeyTimeout).toBe('');
    expect(parsed.rejectAfterTime).toBe('');
    expect(parsed.keepaliveTimeout).toBe('');
    expect(parsed.maxHandshakeAttempts).toBe('');
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

  // The 5 AWG3 timer fields go through the same gate as headerProtectionKey/
  // contentPaddingAddition -- accepted via the rest-parameter awg3Fields, in
  // any position.
  it('promotes a legacy record with only a timer field set', () => {
    expect(effectiveAwgVersion('', '', '', '118-135', '', '', '', '')).toBe(AWG_VERSION_3);
    expect(effectiveAwgVersion(undefined, undefined, undefined, undefined, undefined, undefined, '15-22')).toBe(AWG_VERSION_3);
  });

  it('never overrides an explicit awgVersion even with a timer field set', () => {
    expect(effectiveAwgVersion(AWG_VERSION_2, '', '', '', '', '175-190', '', '')).toBe(AWG_VERSION_2);
  });
});
