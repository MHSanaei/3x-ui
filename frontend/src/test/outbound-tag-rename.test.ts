import { describe, it, expect } from 'vitest';

import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { propagateOutboundTagRename } from '@/pages/xray/basics/helpers';

function baseTemplate(): XraySettingsValue {
  return {
    outbounds: [
      { tag: 'To-External-Proxy', protocol: 'vless' },
      { tag: 'direct', protocol: 'freedom' },
    ],
    routing: {
      rules: [
        {
          type: 'field',
          inboundTag: ['iran-in'],
          outboundTag: 'To-External-Proxy',
        },
      ],
      balancers: [
        {
          tag: 'lb-1',
          selector: ['To-External-Proxy', 'direct'],
          fallbackTag: 'To-External-Proxy',
        },
      ],
    },
  } as XraySettingsValue;
}

describe('propagateOutboundTagRename', () => {
  it('updates routing rule outboundTag when outbound is renamed', () => {
    const t = baseTemplate();
    propagateOutboundTagRename(t, 'To-External-Proxy', 'external-vps');
    expect(t.routing?.rules?.[0]?.outboundTag).toBe('external-vps');
  });

  it('updates balancer selector and fallbackTag', () => {
    const t = baseTemplate();
    propagateOutboundTagRename(t, 'To-External-Proxy', 'external-vps');
    expect(t.routing?.balancers?.[0]?.selector).toEqual(['external-vps', 'direct']);
    expect(t.routing?.balancers?.[0]?.fallbackTag).toBe('external-vps');
  });

  it('updates sockopt dialerProxy references in other outbounds', () => {
    const t = baseTemplate();
    (t.outbounds![1] as { streamSettings?: { sockopt?: { dialerProxy?: string } } }).streamSettings = {
      sockopt: { dialerProxy: 'To-External-Proxy' },
    };
    propagateOutboundTagRename(t, 'To-External-Proxy', 'external-vps');
    const dialerProxy = (t.outbounds![1] as { streamSettings?: { sockopt?: { dialerProxy?: string } } })
      .streamSettings?.sockopt?.dialerProxy;
    expect(dialerProxy).toBe('external-vps');
  });

  it('is a no-op when old and new tags are equal', () => {
    const t = baseTemplate();
    propagateOutboundTagRename(t, 'To-External-Proxy', 'To-External-Proxy');
    expect(t.routing?.rules?.[0]?.outboundTag).toBe('To-External-Proxy');
  });
});
