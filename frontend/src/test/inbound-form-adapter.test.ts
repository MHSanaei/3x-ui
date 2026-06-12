/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  rawInboundToFormValues,
  formValuesToWirePayload,
  type RawInboundRow,
} from '@/lib/xray/inbound-form-adapter';
import { InboundDbFieldsSchema, InboundFormSchema } from '@/schemas/forms/inbound-form';
import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';

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
      expect(values.shareAddrStrategy).toBe('node');
      expect(values.shareAddr).toBe('');
    });
  }

  it('produces values that the InboundFormSchema accepts', () => {
    const values = rawInboundToFormValues(vlessRow);
    const result = InboundFormSchema.safeParse(values);
    expect(result.success).toBe(true);
  });
});

// Regression: wireguard (UDP-only) and tunnel (dokodemo-door) have no
// user-selectable transport, so the modal submits streamSettings WITHOUT a
// `network` key — just `security`, plus `sockopt` for tunnel's TProxy. The
// network schema must accept that transportless shape; before the transportless
// union branch landed it failed with "Invalid discriminator value. Expected
// 'tcp' | ..." and blocked every wireguard/tunnel save.
describe('transportless streamSettings (wireguard / tunnel)', () => {
  it('accepts wireguard with a network-less streamSettings', () => {
    const result = InboundFormSchema.safeParse({
      port: 51820,
      protocol: 'wireguard',
      settings: { secretKey: 'cE9mYWtlLXNlY3JldC1rZXktZm9yLXVuaXQtdGVzdA==', peers: [] },
      streamSettings: { security: 'none' },
    });
    expect(result.success).toBe(true);
  });

  it('accepts tunnel with sockopt.tproxy and no network', () => {
    const result = InboundFormSchema.safeParse({
      port: 12345,
      protocol: 'tunnel',
      settings: { allowedNetwork: 'tcp,udp', followRedirect: true, portMap: {} },
      streamSettings: {
        security: 'none',
        sockopt: SockoptStreamSettingsSchema.parse({ tproxy: 'tproxy' }),
      },
    });
    expect(result.success).toBe(true);
    if (result.success) {
      const stream = result.data.streamSettings as {
        network?: unknown;
        sockopt?: { tproxy?: string };
      };
      expect(stream.network).toBeUndefined();
      expect(stream.sockopt?.tproxy).toBe('tproxy');
    }
  });

  it('still rejects a present-but-invalid network value', () => {
    const result = InboundFormSchema.safeParse({
      port: 12345,
      protocol: 'tunnel',
      settings: { allowedNetwork: 'tcp,udp', followRedirect: true, portMap: {} },
      streamSettings: { network: 'bogus', security: 'none' },
    });
    expect(result.success).toBe(false);
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

  it('emits empty sniffing for mtproto (mtg-served, not Xray)', () => {
    const values = rawInboundToFormValues({
      ...vlessRow,
      protocol: 'mtproto',
      settings: { fakeTlsDomain: 'www.cloudflare.com', secret: 'ee00' },
    });
    const payload = formValuesToWirePayload(values);
    expect(payload.protocol).toBe('mtproto');
    expect(payload.sniffing).toBe('');
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

  it('round-trips share address strategy fields', () => {
    const values = rawInboundToFormValues({
      ...vlessRow,
      shareAddrStrategy: 'custom',
      shareAddr: 'edge.example.test',
    });
    const payload = formValuesToWirePayload(values);
    expect(payload.shareAddrStrategy).toBe('custom');
    expect(payload.shareAddr).toBe('edge.example.test');
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

describe('subSortIndex', () => {
  it('rawInboundToFormValues defaults to 1 when field is absent', () => {
    const values = rawInboundToFormValues({ ...vlessRow, subSortIndex: undefined });
    expect(values.subSortIndex).toBe(1);
  });

  it('rawInboundToFormValues preserves valid values and clamps below-minimum ones to 1', () => {
    expect(rawInboundToFormValues({ ...vlessRow, subSortIndex: 5 }).subSortIndex).toBe(5);
    expect(rawInboundToFormValues({ ...vlessRow, subSortIndex: 0 }).subSortIndex).toBe(1);
    expect(rawInboundToFormValues({ ...vlessRow, subSortIndex: -10 }).subSortIndex).toBe(1);
  });

  it('formValuesToWirePayload includes subSortIndex in the payload', () => {
    const values = rawInboundToFormValues({ ...vlessRow, subSortIndex: 3 });
    const payload = formValuesToWirePayload(values);
    expect(payload.subSortIndex).toBe(3);
  });

  it('subSortIndex round-trips through raw → values → payload', () => {
    const values = rawInboundToFormValues({ ...vlessRow, subSortIndex: 42 });
    const payload = formValuesToWirePayload(values);
    const replay = rawInboundToFormValues({ ...vlessRow, subSortIndex: payload.subSortIndex });
    expect(replay.subSortIndex).toBe(42);
  });

  it('InboundDbFieldsSchema enforces an integer minimum of 1 and defaults to 1', () => {
    expect(InboundDbFieldsSchema.partial().safeParse({ subSortIndex: 1.5 }).success).toBe(false);
    expect(InboundDbFieldsSchema.partial().safeParse({ subSortIndex: 0 }).success).toBe(false);
    expect(InboundDbFieldsSchema.parse({}).subSortIndex).toBe(1);
  });
});
