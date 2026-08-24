import { describe, expect, it } from 'vitest';

import { fitsInQrCode } from '@/lib/xray/inbound-link';

const I_LINE =
  'I1 = <b 0x1603010152><b 0x0100014e><b 0x0303><r 32><b 0x20><r 32><b 0x0022130113031302c02bc02fcca9cca8c02cc030c00ac009c013c014009c009d002f0035><b 0x0100><b 0x00e3><b 0x0000000e000c0000097275747562652e7275><b 0x00170000><b 0xff01000100><b 0x000a000e000c001d00170018001901000101><b 0x000b00020100><b 0x00230000><b 0x0010000e000c02683208687474702f312e31><b 0x000500050100000000><b 0x0022000a00080403050306030203><b 0x0033004a0048><b 0x001d0020><r 32><b 0x00170020><r 32><b 0x002b00050403040303><b 0x000d0018001604030503060308040805080604010501060102030201><b 0x002d00020101><b 0x001c00024001><b 0x00150000>';

describe('fitsInQrCode', () => {
  it('accepts an ordinary share link', () => {
    expect(fitsInQrCode('vless://uuid@example.com:443?type=tcp&security=reality#node')).toBe(true);
  });

  it('rejects an AmneziaWG config carrying I1-I5', () => {
    const conf = ['[Interface]', 'PrivateKey = k', ...Array.from({ length: 5 }, () => I_LINE)].join(
      '\n',
    );
    expect(conf.length).toBeGreaterThan(2331);
    expect(fitsInQrCode(conf)).toBe(false);
  });

  it('counts UTF-8 bytes, not code points', () => {
    expect(fitsInQrCode('я'.repeat(1200))).toBe(false);
    expect(fitsInQrCode('a'.repeat(1200))).toBe(true);
  });

  it('is exact at the capacity boundary', () => {
    expect(fitsInQrCode('a'.repeat(2331))).toBe(true);
    expect(fitsInQrCode('a'.repeat(2332))).toBe(false);
  });
});
