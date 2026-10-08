import { describe, expect, it } from 'vitest';

import { genAllLinks, genInboundLinks } from '@/lib/xray/inbound-link';
import { InboundSchema } from '@/schemas/api/inbound';

// Multi-client MTProto renders one tg://proxy deep link per entry in
// settings.clients, each carrying that client's own FakeTLS secret.
function mtprotoInbound(extra: Record<string, unknown> = {}) {
  return InboundSchema.parse({
    id: 70,
    remark: 'mt-mc',
    port: 8443,
    protocol: 'mtproto',
    settings: {
      ...extra,
      fakeTlsDomain: 'www.cloudflare.com',
      clients: [
        {
          email: 'alice',
          secret: 'ee0123456789abcdef0123456789abcdef7777772e636c6f7564666c6172652e636f6d',
          enable: true,
        },
        {
          email: 'bob',
          secret: 'eeabcdefabcdefabcdefabcdefabcdef01676f6f676c652e636f6d',
          enable: true,
        },
      ],
    },
  });
}

describe('mtproto multi-client link fan-out', () => {
  it('emits one tg://proxy per client from settings.clients', () => {
    const out = genInboundLinks({
      inbound: mtprotoInbound(),
      remark: 'mt-mc',
      fallbackHostname: 'mt.example.test',
    });
    const links = out.split('\r\n').filter(Boolean);
    expect(links).toHaveLength(2);
    expect(links[0]).toContain('tg://proxy');
    expect(links[0]).toContain(
      'secret=ee0123456789abcdef0123456789abcdef7777772e636c6f7564666c6172652e636f6d',
    );
    expect(links[1]).toContain('secret=eeabcdefabcdefabcdefabcdefabcdef01676f6f676c652e636f6d');
    expect(links[0]).not.toContain('#');
    expect(links[1]).not.toContain('#');
  });
});

// A secured inbound also serves dd clients on each client's FakeTLS key, so
// every ee link gets a dd sibling with the same server and port.
describe('mtproto secured (dd) links', () => {
  it('adds a dd link after each ee link when the inbound is secured', () => {
    const out = genInboundLinks({
      inbound: mtprotoInbound({ secured: true }),
      remark: 'mt-mc',
      fallbackHostname: 'mt.example.test',
    });
    expect(out.split('\r\n')).toEqual([
      'tg://proxy?server=mt.example.test&port=8443&secret=ee0123456789abcdef0123456789abcdef7777772e636c6f7564666c6172652e636f6d',
      'tg://proxy?server=mt.example.test&port=8443&secret=dd0123456789abcdef0123456789abcdef',
      'tg://proxy?server=mt.example.test&port=8443&secret=eeabcdefabcdefabcdefabcdefabcdef01676f6f676c652e636f6d',
      'tg://proxy?server=mt.example.test&port=8443&secret=ddabcdefabcdefabcdefabcdefabcdef01',
    ]);
  });

  it('labels the dd entry so the QR and info panels can tell it apart', () => {
    const entries = genAllLinks({
      inbound: mtprotoInbound({ secured: true }),
      remark: 'mt-mc',
      client: { secret: 'ee0123456789abcdef0123456789abcdef7777772e636c6f7564666c6172652e636f6d' },
      fallbackHostname: 'mt.example.test',
    });
    expect(entries.map((e) => e.remark)).toEqual(['mt-mc', 'mt-mc-dd']);
  });

  it('emits no dd link when secured is off', () => {
    const out = genInboundLinks({
      inbound: mtprotoInbound({ secured: false }),
      remark: 'mt-mc',
      fallbackHostname: 'mt.example.test',
    });
    expect(out).not.toContain('secret=dd');
  });
});
