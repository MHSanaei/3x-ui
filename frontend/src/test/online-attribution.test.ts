import { describe, expect, it } from 'vitest';

import { isClientOnlineOnInbound } from '@/lib/online-attribution';

describe('isClientOnlineOnInbound', () => {
  const base = { tag: 'in-a', protocol: 'vless', inboundActive: true };

  it('without attribution falls back to the active-tag signal', () => {
    expect(isClientOnlineOnInbound({ ...base, attributed: undefined })).toBe(true);
    expect(isClientOnlineOnInbound({ ...base, attributed: undefined, inboundActive: false })).toBe(
      false,
    );
  });

  it('a client attributed only to B is not online on a busy inbound A', () => {
    expect(isClientOnlineOnInbound({ ...base, attributed: new Set(['in-b']) })).toBe(false);
  });

  it('a client attributed to both inbounds is online on each', () => {
    expect(isClientOnlineOnInbound({ ...base, attributed: new Set(['in-a', 'in-b']) })).toBe(true);
  });

  it('tuic and mtproto inbounds keep the traffic signal because they never log an email', () => {
    const attributed = new Set(['in-vless']);
    expect(isClientOnlineOnInbound({ ...base, protocol: 'tuic', attributed })).toBe(true);
    expect(isClientOnlineOnInbound({ ...base, protocol: 'mtproto', attributed })).toBe(true);
    expect(
      isClientOnlineOnInbound({ ...base, protocol: 'tuic', attributed, inboundActive: false }),
    ).toBe(false);
  });
});
