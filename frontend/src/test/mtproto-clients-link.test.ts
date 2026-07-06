import { describe, expect, it } from 'vitest';

import { genInboundLinks } from '@/lib/xray/inbound-link';
import { InboundSchema } from '@/schemas/api/inbound';

// Multi-client MTProto renders one tg://proxy deep link per entry in
// settings.clients, each carrying that client's own FakeTLS secret.
function mtprotoInbound() {
  return InboundSchema.parse({
    id: 70,
    remark: 'mt-mc',
    port: 8443,
    protocol: 'mtproto',
    settings: {
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
    const out = genInboundLinks({ inbound: mtprotoInbound(), remark: 'mt-mc', fallbackHostname: 'mt.example.test' });
    const links = out.split('\r\n').filter(Boolean);
    expect(links).toHaveLength(2);
    expect(links[0]).toContain('tg://proxy');
    expect(links[0]).toContain('secret=ee0123456789abcdef0123456789abcdef7777772e636c6f7564666c6172652e636f6d');
    expect(links[1]).toContain('secret=eeabcdefabcdefabcdefabcdefabcdef01676f6f676c652e636f6d');
    expect(links[0]).not.toContain('#');
    expect(links[1]).not.toContain('#');
  });
});
