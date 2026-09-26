import { describe, it, expect } from 'vitest';

import { formValuesToWirePayload, rawOutboundToFormValues } from '@/lib/xray/outbound-form-adapter';

// A freedom outbound resolves through sockopt.domainStrategy, and the core warns
// on every load for both legacy placements it migrates there (infra/conf/xray.go).
describe('freedom domain strategy placement', () => {
  it('emits the freedom card strategy into sockopt instead of settings', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'freedom',
        tag: 'direct',
        settings: { domainStrategy: 'UseIPv4' },
      }),
    );

    expect((wire.settings as Record<string, unknown>).domainStrategy).toBeUndefined();
    expect(wire.targetStrategy).toBeUndefined();
    expect(wire.streamSettings).toEqual({ sockopt: { domainStrategy: 'UseIPv4' } });
  });

  it('migrates a legacy settings key into sockopt on the next emit', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'freedom',
        tag: 'direct',
        settings: { domainStrategy: 'UseIPv6' },
      }),
    );

    expect(wire.streamSettings).toEqual({ sockopt: { domainStrategy: 'UseIPv6' } });
    expect((wire.settings as Record<string, unknown>).domainStrategy).toBeUndefined();
  });

  it('migrates a legacy outbound-root targetStrategy into sockopt', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'freedom',
        tag: 'direct',
        targetStrategy: 'ForceIPv4',
        settings: {},
      }),
    );

    expect(wire.targetStrategy).toBeUndefined();
    expect(wire.streamSettings).toEqual({ sockopt: { domainStrategy: 'ForceIPv4' } });
  });

  it('reads the sockopt strategy back into the freedom card', () => {
    const values = rawOutboundToFormValues({
      protocol: 'freedom',
      tag: 'direct',
      streamSettings: { sockopt: { domainStrategy: 'UseIPv4v6' } },
      settings: {},
    });

    expect((values.settings as { domainStrategy?: string }).domainStrategy).toBe('UseIPv4v6');
  });

  it('keeps the freedom card empty rather than showing a stale legacy value twice', () => {
    const values = rawOutboundToFormValues({
      protocol: 'freedom',
      tag: 'direct',
      targetStrategy: 'UseIPv4',
      settings: {},
    });

    expect(values.targetStrategy).toBe('');
    expect((values.settings as { domainStrategy?: string }).domainStrategy).toBe('UseIPv4');
  });

  it('shows the strategy the core will run with when a legacy key outranks sockopt', () => {
    const values = rawOutboundToFormValues({
      protocol: 'freedom',
      tag: 'direct',
      targetStrategy: 'UseIPv4',
      streamSettings: { sockopt: { domainStrategy: 'UseIPv6' } },
      settings: {},
    });

    expect((values.settings as { domainStrategy?: string }).domainStrategy).toBe('UseIPv4');
    const wire = formValuesToWirePayload(values);
    expect(wire.targetStrategy).toBeUndefined();
    expect(wire.streamSettings).toEqual({ sockopt: { domainStrategy: 'UseIPv4' } });
  });

  it('drops the sockopt key when the card is cleared', () => {
    const values = rawOutboundToFormValues({
      protocol: 'freedom',
      tag: 'direct',
      streamSettings: { sockopt: { domainStrategy: 'UseIPv4', tcpFastOpen: true } },
      settings: {},
    });
    (values.settings as { domainStrategy?: string }).domainStrategy = '';

    const wire = formValuesToWirePayload(values);

    expect(wire.streamSettings).toEqual({ sockopt: { tcpFastOpen: true } });
  });

  it('normalizes the sockopt spelling the core matches case-insensitively', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'freedom',
        tag: 'direct',
        streamSettings: { sockopt: { domainStrategy: 'useipv4v6' } },
        settings: {},
      }),
    );

    expect(wire.streamSettings).toEqual({ sockopt: { domainStrategy: 'UseIPv4v6' } });
  });

  it('leaves AsIs out of the wire entirely', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'freedom',
        tag: 'direct',
        settings: { domainStrategy: 'AsIs' },
      }),
    );

    expect(wire.streamSettings).toBeUndefined();
    expect((wire.settings as Record<string, unknown>).domainStrategy).toBeUndefined();
  });

  it('merges into a sockopt the transport form already carries', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'freedom',
        tag: 'direct',
        settings: { domainStrategy: 'UseIPv4' },
        streamSettings: { sockopt: { tcpFastOpen: true } },
      }),
    );

    expect(wire.streamSettings).toEqual({
      sockopt: { tcpFastOpen: true, domainStrategy: 'UseIPv4' },
    });
  });

  it('still emits the root targetStrategy for protocols that use it there', () => {
    const wire = formValuesToWirePayload(
      rawOutboundToFormValues({
        protocol: 'vless',
        tag: 'proxy',
        targetStrategy: 'UseIPv4',
        settings: { address: 'example.com', port: 443, id: 'x', encryption: 'none' },
      }),
    );

    expect(wire.targetStrategy).toBe('UseIPv4');
  });
});
