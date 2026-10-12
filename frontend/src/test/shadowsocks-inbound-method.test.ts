import { describe, expect, it } from 'vitest';

import { ShadowsocksInboundSettingsSchema } from '@/schemas/protocols/inbound/shadowsocks';
import { SSMethodSchema } from '@/schemas/protocols/shared/shadowsocks';

// xray-core refuses 2022-blake3-chacha20-poly1305 on an inbound with users (the
// multi-user identity header is AES-only), and every panel inbound carries users.
describe('shadowsocks inbound method', () => {
  it('rejects 2022-blake3-chacha20-poly1305 on an inbound', () => {
    const parsed = ShadowsocksInboundSettingsSchema.safeParse({
      method: '2022-blake3-chacha20-poly1305',
    });
    expect(parsed.success).toBe(false);
    expect(parsed.error?.issues[0].path).toEqual(['method']);
  });

  it('still accepts the AES 2022 and the legacy chacha methods', () => {
    for (const method of ['2022-blake3-aes-256-gcm', 'chacha20-ietf-poly1305']) {
      expect(ShadowsocksInboundSettingsSchema.parse({ method }).method).toBe(method);
    }
  });

  // A single-key outbound to a 2022-chacha20 server works, so only inbounds lose it.
  it('keeps 2022-blake3-chacha20-poly1305 for outbounds', () => {
    expect(SSMethodSchema.parse('2022-blake3-chacha20-poly1305')).toBe(
      '2022-blake3-chacha20-poly1305',
    );
  });
});
