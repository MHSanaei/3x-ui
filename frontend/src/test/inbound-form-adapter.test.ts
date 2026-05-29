/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  rawInboundToFormValues,
  formValuesToWirePayload,
  type RawInboundRow,
} from '@/lib/xray/inbound-form-adapter';
import { InboundFormSchema } from '@/schemas/forms/inbound-form';

// Round-trip: raw DB row → InboundFormValues → wire payload, asserting
// that the JSON-stringified settings/streamSettings/sniffing in the
// payload deserialize back to the same data the raw row carried.

interface FixtureCase {
  name: string;
  row: RawInboundRow;
  expectedProtocol: string;
}

const vlessRow: RawInboundRow = {
  id: 7,
  port: 12345,
  listen: '0.0.0.0',
  protocol: 'vless',
  remark: 'edge-1',
  enable: true,
  up: 1024,
  down: 2048,
  total: 1_000_000_000,
  expiryTime: 0,
  trafficReset: 'monthly',
  lastTrafficResetTime: 0,
  tag: 'inbound-1',
  nodeId: null,
  settings: {
    clients: [{
      id: '8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02',
      email: 'alice@example.test',
      flow: '',
      limitIp: 0,
      totalGB: 0,
      expiryTime: 0,
      enable: true,
      tgId: 0,
      subId: 'abc123def',
      comment: '',
      reset: 0,
    }],
    decryption: 'none',
    encryption: 'none',
    fallbacks: [],
  },
  streamSettings: {
    network: 'tcp',
    security: 'none',
    tcpSettings: { header: { type: 'none' } },
  },
  sniffing: {
    enabled: false,
    destOverride: ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: false,
    routeOnly: false,
    ipsExcluded: [],
    domainsExcluded: [],
  },
} as RawInboundRow & { id: number };

const cases: FixtureCase[] = [
  { name: 'vless tcp none', row: vlessRow, expectedProtocol: 'vless' },
  {
    name: 'string-coerced settings',
    row: {
      ...vlessRow,
      settings: JSON.stringify(vlessRow.settings),
      streamSettings: JSON.stringify(vlessRow.streamSettings),
      sniffing: JSON.stringify(vlessRow.sniffing),
    },
    expectedProtocol: 'vless',
  },
  {
    name: 'empty stream settings drop to undefined',
    row: { ...vlessRow, streamSettings: '' },
    expectedProtocol: 'vless',
  },
  {
    name: 'unknown trafficReset coerces to never',
    row: { ...vlessRow, trafficReset: 'totally-fabricated' },
    expectedProtocol: 'vless',
  },
];

describe('rawInboundToFormValues', () => {
  for (const { name, row, expectedProtocol } of cases) {
    it(`maps ${name}`, () => {
      const values = rawInboundToFormValues(row);
      expect(values.protocol).toBe(expectedProtocol);
      expect(values.port).toBe(row.port);
      expect(values.remark).toBe(row.remark ?? '');
      if (name === 'unknown trafficReset coerces to never') {
        expect(values.trafficReset).toBe('never');
      }
      if (name === 'empty stream settings drop to undefined') {
        expect(values.streamSettings).toBeUndefined();
      }
    });
  }

  it('produces values that the InboundFormSchema accepts', () => {
    const values = rawInboundToFormValues(vlessRow);
    const result = InboundFormSchema.safeParse(values);
    expect(result.success).toBe(true);
  });
});

describe('formValuesToWirePayload', () => {
  it('stringifies settings/streamSettings/sniffing with empty-array/default pruning', () => {
    const values = rawInboundToFormValues(vlessRow);
    const payload = formValuesToWirePayload(values);

    expect(typeof payload.settings).toBe('string');
    expect(typeof payload.streamSettings).toBe('string');
    expect(typeof payload.sniffing).toBe('string');

    // Empty arrays like `fallbacks: []` drop out of the payload to match
    // the legacy panel's minimal JSON.
    const parsedSettings = JSON.parse(payload.settings);
    const { fallbacks: _f, ...expectedSettings } = vlessRow.settings as Record<string, unknown>;
    expect(parsedSettings).toEqual(expectedSettings);

    expect(JSON.parse(payload.streamSettings)).toEqual(vlessRow.streamSettings);

    // Disabled sniffing collapses to the bare `{ enabled: false }`
    // regardless of which destOverride/metadataOnly/etc. defaults the
    // form carries.
    expect(JSON.parse(payload.sniffing)).toEqual({ enabled: false });
  });

  it('emits empty string for absent streamSettings', () => {
    const values = rawInboundToFormValues({ ...vlessRow, streamSettings: '' });
    const payload = formValuesToWirePayload(values);
    expect(payload.streamSettings).toBe('');
  });

  it('omits nodeId when null', () => {
    const values = rawInboundToFormValues({ ...vlessRow, nodeId: null });
    const payload = formValuesToWirePayload(values);
    expect('nodeId' in payload).toBe(false);
  });

  it('includes nodeId when set', () => {
    const values = rawInboundToFormValues({ ...vlessRow, nodeId: 42 });
    const payload = formValuesToWirePayload(values);
    expect(payload.nodeId).toBe(42);
  });

  it('round-trips top-level fields through raw → values → payload → values', () => {
    // settings/streamSettings/sniffing don't round-trip byte-equal because
    // the wire payload prunes empty arrays and collapses disabled sniffing
    // to `{ enabled: false }`. Top-level scalars and the protocol picker
    // must still survive the round trip without loss.
    const original = rawInboundToFormValues(vlessRow);
    const payload = formValuesToWirePayload(original);
    const replay = rawInboundToFormValues({
      port: payload.port,
      listen: payload.listen,
      protocol: payload.protocol,
      tag: payload.tag,
      settings: payload.settings,
      streamSettings: payload.streamSettings,
      sniffing: payload.sniffing,
      up: payload.up,
      down: payload.down,
      total: payload.total,
      remark: payload.remark,
      enable: payload.enable,
      expiryTime: payload.expiryTime,
      trafficReset: payload.trafficReset,
      lastTrafficResetTime: payload.lastTrafficResetTime,
      nodeId: payload.nodeId ?? null,
    });
    expect(replay.protocol).toBe(original.protocol);
    expect(replay.port).toBe(original.port);
    expect(replay.tag).toBe(original.tag);
    expect(replay.listen).toBe(original.listen);
    expect(replay.up).toBe(original.up);
    expect(replay.down).toBe(original.down);
    expect(replay.streamSettings).toEqual(original.streamSettings);
  });
});
