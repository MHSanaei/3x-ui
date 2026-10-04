import { describe, expect, it } from 'vitest';

import { isPostQuantumLink } from '@/lib/xray/inbound-link';

const BASE = 'vless://22222222-3333-4444-9555-666666666666@example.test:443';

describe('post-quantum QR restrictions', () => {
  it.each([
    '?security=reality&support-x25519mlkem768=true&encryption=none',
    '?security=reality#mlkem768-mldsa65-ML-KEM-768',
    '?security=reality&sni=mlkem768.example.test',
    '?security=reality&pqv=',
  ])('does not hide QR for a link without post-quantum keys: %s', (suffix) => {
    expect(isPostQuantumLink(BASE + suffix)).toBe(false);
  });

  it.each([
    '?security=reality&pqv=' + 'A'.repeat(2603),
    '?security=reality&support-x25519mlkem768=true&pqv=' + 'A'.repeat(2603),
    '?encryption=mlkem768x25519plus.native.0rtt.' + 'A'.repeat(1590),
    '?encryption=ML-KEM-768.native.0rtt.' + 'A'.repeat(1590),
    '?encryption=%6Dlkem768x25519plus.native.0rtt.' + 'A'.repeat(1590),
  ])('keeps QR restricted for post-quantum key payloads %#', (suffix) => {
    expect(isPostQuantumLink(BASE + suffix)).toBe(true);
  });

  it('does not throw on non-URL config text', () => {
    expect(isPostQuantumLink('[Interface]\nAddress = 10.0.0.2/32')).toBe(false);
  });
});
