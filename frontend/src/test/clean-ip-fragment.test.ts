import { describe, expect, it } from 'vitest';

import { withFragment } from '@/pages/clean-ip/scan';

// The ISP group copies the main group's Final Mask; saving a fragment found by the full
// test must replace only the fragment mask and keep every other mask the admin set.
describe('withFragment', () => {
  const tlshello = { packets: 'tlshello', length: '10-20', delay: '1-2' };

  it('replaces an existing fragment mask and keeps the other masks', () => {
    const current = JSON.stringify({
      tcp: [
        { type: 'fragment', settings: { packets: '1-3', lengths: ['100-200'], delays: [] } },
        { type: 'header-custom', settings: { clients: [] } },
      ],
      udp: [{ type: 'salamander', settings: { password: 'x' } }],
    });
    expect(JSON.parse(withFragment(current, tlshello))).toEqual({
      tcp: [
        { type: 'header-custom', settings: { clients: [] } },
        {
          type: 'fragment',
          settings: { packets: 'tlshello', lengths: ['10-20'], delays: ['1-2'] },
        },
      ],
      udp: [{ type: 'salamander', settings: { password: 'x' } }],
    });
  });

  it('removes the fragment mask for "no fragment" and empties a mask left with nothing', () => {
    const current = JSON.stringify({ tcp: [{ type: 'fragment', settings: { packets: '1-1' } }] });
    expect(withFragment(current, null)).toBe('');
  });

  it('starts a mask when the group has none (or an unreadable one)', () => {
    const want = {
      tcp: [{ type: 'fragment', settings: { packets: '1-1', lengths: ['1-5'], delays: ['1-2'] } }],
    };
    const tcpSegments = { packets: '1-1', length: '1-5', delay: '1-2' };
    expect(JSON.parse(withFragment('', tcpSegments))).toEqual(want);
    expect(JSON.parse(withFragment('{not json', tcpSegments))).toEqual(want);
  });
});
