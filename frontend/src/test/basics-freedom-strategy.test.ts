import { describe, it, expect } from 'vitest';

import { directFreedomStrategy, setDirectFreedomStrategy } from '@/pages/xray/basics/helpers';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

type Outbound = Record<string, unknown>;

function settingsWithDirect(settings: Outbound, stream?: Outbound): XraySettingsValue {
  return {
    outbounds: [
      {
        protocol: 'freedom',
        tag: 'direct',
        settings,
        ...(stream ? { streamSettings: stream } : {}),
      },
    ],
  } as unknown as XraySettingsValue;
}

function directOutbound(t: XraySettingsValue): Outbound {
  return t.outbounds?.[0] as Outbound;
}

// This select used to write the deprecated settings key (issue #6482), which is
// what made the core warn on every load; it has to write sockopt instead.
describe('BasicsTab freedom strategy', () => {
  it('writes sockopt and clears both legacy placements', () => {
    const t = settingsWithDirect({ domainStrategy: 'UseIPv6', targetStrategy: 'UseIP' });

    setDirectFreedomStrategy(t, 'UseIPv4');

    const outbound = directOutbound(t);
    expect(outbound.settings).toEqual({});
    expect(outbound.streamSettings).toEqual({ sockopt: { domainStrategy: 'UseIPv4' } });
  });

  it('creates the direct outbound when the config has none', () => {
    const t = { outbounds: [] } as unknown as XraySettingsValue;

    setDirectFreedomStrategy(t, 'UseIPv4');

    expect(directOutbound(t)).toEqual({
      protocol: 'freedom',
      tag: 'direct',
      settings: {},
      streamSettings: { sockopt: { domainStrategy: 'UseIPv4' } },
    });
  });

  it('finds the direct outbound when the template spells it "Freedom"', () => {
    const t = {
      outbounds: [
        {
          protocol: 'Freedom',
          tag: 'direct',
          settings: {},
          streamSettings: { sockopt: { domainStrategy: 'UseIPv6' } },
        },
      ],
    } as unknown as XraySettingsValue;

    expect(directFreedomStrategy(t)).toBe('UseIPv6');

    setDirectFreedomStrategy(t, 'UseIPv4');

    expect(t.outbounds).toHaveLength(1);
    expect(directOutbound(t).streamSettings).toEqual({ sockopt: { domainStrategy: 'UseIPv4' } });
  });

  it('never leaves a taken "direct" tag on two outbounds', () => {
    const t = {
      outbounds: [{ protocol: 'socks', tag: 'direct', settings: { servers: [] } }],
    } as unknown as XraySettingsValue;

    setDirectFreedomStrategy(t, 'UseIPv4');

    expect(t.outbounds).toHaveLength(1);
    expect(directOutbound(t)).toEqual({
      protocol: 'socks',
      tag: 'direct',
      settings: { servers: [] },
    });
  });

  it('keeps other sockopt keys the transport form already set', () => {
    const t = settingsWithDirect({}, { sockopt: { tcpFastOpen: true } });

    setDirectFreedomStrategy(t, 'ForceIPv4');

    expect(directOutbound(t).streamSettings).toEqual({
      sockopt: { tcpFastOpen: true, domainStrategy: 'ForceIPv4' },
    });
  });

  it('drops the key again when AsIs is chosen', () => {
    const t = settingsWithDirect({}, { sockopt: { domainStrategy: 'UseIPv4' } });

    setDirectFreedomStrategy(t, 'AsIs');

    expect(directOutbound(t).streamSettings).toBeUndefined();
  });

  it('shows the value the core will run with: legacy settings outrank sockopt', () => {
    expect(
      directFreedomStrategy(
        settingsWithDirect(
          { domainStrategy: 'UseIPv6' },
          { sockopt: { domainStrategy: 'UseIPv4' } },
        ),
      ),
    ).toBe('UseIPv6');
    expect(directFreedomStrategy(settingsWithDirect({ targetStrategy: 'ForceIPv6' }))).toBe(
      'ForceIPv6',
    );
    expect(directFreedomStrategy(settingsWithDirect({ domainStrategy: 'UseIPv4v6' }))).toBe(
      'UseIPv4v6',
    );
    expect(
      directFreedomStrategy(settingsWithDirect({}, { sockopt: { domainStrategy: 'UseIPv4' } })),
    ).toBe('UseIPv4');
    expect(directFreedomStrategy(settingsWithDirect({}))).toBe('AsIs');
    expect(directFreedomStrategy(null)).toBe('AsIs');
  });

  it('does not let an inert AsIs alias mask the sockopt value', () => {
    expect(
      directFreedomStrategy(
        settingsWithDirect({ domainStrategy: 'AsIs' }, { sockopt: { domainStrategy: 'UseIPv4' } }),
      ),
    ).toBe('UseIPv4');
  });
});
