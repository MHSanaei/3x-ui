import { describe, expect, it } from 'vitest';

import {
  bindExitPoolRoute,
  listExitPoolBindings,
  unbindExitPoolRoute,
} from '../pages/xray/exit-pool/exit-pool-routing';

describe('exit-pool routing', () => {
  it('binds TCP and UDP without changing unrelated routing rules', () => {
    const original = {
      outbounds: [{ tag: 'fanout-jp-1', protocol: 'socks' }],
      routing: {
        rules: [
          { type: 'field', inboundTag: ['in-a', 'in-b'], outboundTag: 'fanout-us-1' },
          { type: 'field', domain: ['geosite:private'], outboundTag: 'direct' },
        ],
      },
    };

    const next = bindExitPoolRoute(original, 'in-a', 'fanout-jp-1');

    expect(next.routing.rules).toEqual([
      {
        type: 'field',
        network: 'tcp,udp',
        inboundTag: ['in-a'],
        outboundTag: 'fanout-jp-1',
      },
      { type: 'field', inboundTag: ['in-b'], outboundTag: 'fanout-us-1' },
      { type: 'field', domain: ['geosite:private'], outboundTag: 'direct' },
    ]);
    expect(original.routing.rules[0].inboundTag).toEqual(['in-a', 'in-b']);
  });

  it('unbinds only the selected inbound from managed fanout rules', () => {
    const next = unbindExitPoolRoute(
      {
        routing: {
          rules: [
            { type: 'field', inboundTag: ['in-a'], outboundTag: 'fanout-jp-1' },
            { type: 'field', inboundTag: ['in-a'], outboundTag: 'direct' },
          ],
        },
      },
      'in-a',
    );

    expect(next.routing.rules).toEqual([
      { type: 'field', inboundTag: ['in-a'], outboundTag: 'direct' },
    ]);
  });

  it('reports only bindings owned by the exit pool', () => {
    expect(
      listExitPoolBindings({
        routing: {
          rules: [
            { type: 'field', inboundTag: ['in-a', 'in-b'], outboundTag: 'fanout-jp-1' },
            { type: 'field', inboundTag: ['in-c'], outboundTag: 'direct' },
          ],
        },
      }),
    ).toEqual([
      { inboundTag: 'in-a', outboundTag: 'fanout-jp-1' },
      { inboundTag: 'in-b', outboundTag: 'fanout-jp-1' },
    ]);
  });
});
