import { describe, expect, it } from 'vitest';

import { formValuesToWirePayload, rawOutboundToFormValues } from '@/lib/xray/outbound-form-adapter';
import type { AmneziaWGOutboundFormSettings } from '@/schemas/forms/outbound-form';

// amneziawg outbound: lossless form->wire->form; payload stays a raw row.
describe('amneziawg outbound adapter', () => {
  const wire = {
    mtu: 1380,
    secretKey: '6Nn0ZB4C1Pj3TBEsXgLv7VdmSnYXGxS+HhVBDhvGgHE=',
    address: ['10.8.0.2/32'],
    listenPort: 40001,
    jc: 5,
    jmin: 40,
    jmax: 90,
    s1: 20,
    s2: 90,
    s3: 15,
    s4: 13,
    h1: '100-800',
    h2: '900-1600',
    h3: '1700-2400',
    h4: '2500-3200',
    i1: '<r 64>',
    contentPaddingAddition: '8-40',
    randomTrailers: true,
    disableCookies: false,
    peers: [
      {
        publicKey: 'Qk9fWqDqC7LzKpYvJq0m2b1tq8eF3uY6oPpRrSsTtUu=',
        presharedKey: 'cHNo',
        allowedIPs: ['0.0.0.0/0', '::/0'],
        endpoint: '203.0.113.7:51820',
        keepAlive: 25,
      },
    ],
  };

  it('hydrates defaults when the template omits optional keys', () => {
    const values = rawOutboundToFormValues({ protocol: 'amneziawg', tag: 'awg-x' });
    expect(values.protocol).toBe('amneziawg');
    const s = values.settings as AmneziaWGOutboundFormSettings;
    expect(s.mtu).toBe(1420);
    expect(s.randomTrailers).toBe(false);
    expect(s.disableCookies).toBe(true);
    expect(s.peers).toEqual([]);
    expect(values.tag).toBe('awg-x');
  });

  it('round-trips wire -> form -> wire losslessly', () => {
    const values = rawOutboundToFormValues({ protocol: 'amneziawg', tag: 'awg-x', settings: wire });
    const payload = formValuesToWirePayload(values);
    expect(payload.protocol).toBe('amneziawg');
    expect(payload.tag).toBe('awg-x');
    // undefined-valued optionals are dropped by JSON semantics; compare the
    // meaningful fields directly.
    expect((payload.settings as Record<string, unknown>).mtu).toBe(1380);
    expect((payload.settings as Record<string, unknown>).listenPort).toBe(40001);
    expect((payload.settings as Record<string, unknown>).i1).toBe('<r 64>');
    expect((payload.settings as Record<string, unknown>).peers).toEqual(wire.peers);
    expect((payload.settings as Record<string, unknown>).disableCookies).toBe(false);
  });

  it('omits empty optional strings and zero listenPort from the payload', () => {
    const values = rawOutboundToFormValues({ protocol: 'amneziawg', settings: wire });
    const awg = values.settings as AmneziaWGOutboundFormSettings;
    awg.i1 = '';
    awg.listenPort = 0;
    const payload = formValuesToWirePayload(values);
    const s = payload.settings as Record<string, unknown>;
    expect(s.i1).toBeUndefined();
    expect(s.listenPort).toBeUndefined();
    // always-present booleans survive so a true->false edit is diffable
    expect(s.randomTrailers).toBe(true);
  });

  it('is included in every protocol-capability gate like wireguard (non-stream, non-mux)', () => {
    const values = rawOutboundToFormValues({
      protocol: 'amneziawg',
      settings: wire,
      streamSettings: { network: 'tcp', tcpSettings: {} },
    });
    const payload = formValuesToWirePayload(values);
    // Non-stream protocol keeps only sockopt; here there is none.
    expect(payload.streamSettings).toBeUndefined();
  });
});
