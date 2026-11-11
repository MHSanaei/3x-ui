import { describe, expect, it } from 'vitest';

import { formValuesToWirePayload, rawInboundToFormValues } from '@/lib/xray/inbound-form-adapter';
import { DBInbound, type DBInboundInit } from '@/models/dbinbound';
import { InboundDbFieldsSchema } from '@/schemas/forms/inbound-form';
import {
  ChainProxyFormSchema,
  ChainSelectorFormSchema,
  ChainProxySchema,
} from '@/schemas/chainProxy';

const row = {
  id: 11,
  port: 8443,
  listen: '0.0.0.0',
  protocol: 'vless',
  remark: 'chained',
  enable: true,
  tag: 'chained-8443',
  nodeId: null,
  settings: { clients: [], decryption: 'none', encryption: 'none' },
  streamSettings: { network: 'tcp', security: 'none' },
} satisfies Record<string, unknown>;

/*
 * A chain assignment reaches the wire as two mutually exclusive ids. Losing
 * either on the way through the form adapter is invisible in the panel — the
 * inbound saves fine — and shows up only as a subscription that connects
 * directly around the hop the admin configured.
 */
describe('inbound chain assignment round-trip', () => {
  it('rawInboundToFormValues reads both ids and defaults them to 0', () => {
    const assigned = rawInboundToFormValues({
      ...row,
      chainProxyId: 4,
      chainSelectorId: 0,
    });
    expect(assigned.chainProxyId).toBe(4);
    expect(assigned.chainSelectorId).toBe(0);

    const selectorAssigned = rawInboundToFormValues({
      ...row,
      chainProxyId: 0,
      chainSelectorId: 7,
    });
    expect(selectorAssigned.chainSelectorId).toBe(7);

    const plain = rawInboundToFormValues(row);
    expect(plain.chainProxyId).toBe(0);
    expect(plain.chainSelectorId).toBe(0);
  });

  it('formValuesToWirePayload carries both ids to the API payload', () => {
    const values = rawInboundToFormValues({ ...row, chainProxyId: 4 });
    const payload = formValuesToWirePayload(values);
    expect(payload.chainProxyId).toBe(4);
    expect(payload.chainSelectorId).toBe(0);
  });

  it('the assignment survives raw -> DBInbound -> values -> payload (the edit round-trip)', () => {
    const db = new DBInbound({ ...row, chainSelectorId: 9 } as unknown as DBInboundInit);
    const payload = formValuesToWirePayload(
      rawInboundToFormValues(db as unknown as Record<string, unknown>),
    );
    expect(payload.chainSelectorId).toBe(9);
    expect(payload.chainProxyId).toBe(0);
  });

  it('DBInbound defaults the ids to 0 when the API omits them', () => {
    expect(new DBInbound({ protocol: 'vless' }).chainProxyId).toBe(0);
    expect(new DBInbound().chainSelectorId).toBe(0);
  });

  it('InboundDbFieldsSchema rejects a negative id and defaults to 0', () => {
    const negative = InboundDbFieldsSchema.partial().safeParse({ chainProxyId: -1 });
    expect(negative.success).toBe(false);
    if (!negative.success) expect(negative.error.issues[0]?.path).toContain('chainProxyId');
    expect(InboundDbFieldsSchema.parse({}).chainProxyId).toBe(0);
  });
});

describe('chain proxy schema', () => {
  const valid = {
    remark: 'entry',
    type: 'socks' as const,
    server: 'proxy.example.com',
    port: 1080,
    username: '',
    password: '',
    tls: false,
    allowInsecure: false,
    enabled: true,
  };

  it('accepts an authenticated SOCKS hop', () => {
    const parsed = ChainProxyFormSchema.parse({ ...valid, username: 'u', password: 'p' });
    expect(parsed.username).toBe('u');
    expect(parsed.port).toBe(1080);
  });

  it('rejects an out-of-range port for the right reason', () => {
    for (const port of [0, 65536, 1080.5]) {
      const parsed = ChainProxyFormSchema.safeParse({ ...valid, port });
      expect(parsed.success).toBe(false);
      if (!parsed.success) expect(parsed.error.issues[0]?.path).toContain('port');
    }
  });

  it('rejects a server containing whitespace', () => {
    const parsed = ChainProxyFormSchema.safeParse({ ...valid, server: 'two words' });
    expect(parsed.success).toBe(false);
    if (!parsed.success) expect(parsed.error.issues[0]?.path).toContain('server');
  });

  it('rejects a blank remark', () => {
    const parsed = ChainProxyFormSchema.safeParse({ ...valid, remark: '   ' });
    expect(parsed.success).toBe(false);
    if (!parsed.success) expect(parsed.error.issues[0]?.path).toContain('remark');
  });

  it('keeps an http hop that terminates TLS on the panel side', () => {
    const parsed = ChainProxyFormSchema.parse({ ...valid, type: 'http', port: 3128, tls: true });
    expect(parsed.type).toBe('http');
    expect(parsed.tls).toBe(true);
  });

  it('parses a row served by the API', () => {
    const parsed = ChainProxySchema.parse({
      id: 3,
      ...valid,
      createdAt: 1,
      updatedAt: 2,
    });
    expect(parsed.id).toBe(3);
    expect(parsed.type).toBe('socks');
  });
});

describe('chain selector schema', () => {
  it('rejects an empty member list for the right reason', () => {
    const parsed = ChainSelectorFormSchema.safeParse({
      remark: 'entries',
      strategy: 'random',
      proxyIds: [],
      enabled: true,
    });
    expect(parsed.success).toBe(false);
    if (!parsed.success) expect(parsed.error.issues[0]?.path).toContain('proxyIds');
  });

  it('rejects a strategy the selector API does not accept', () => {
    const parsed = ChainSelectorFormSchema.safeParse({
      remark: 'entries',
      strategy: 'weighted',
      proxyIds: [1],
      enabled: true,
    });
    expect(parsed.success).toBe(false);
  });

  it('accepts every strategy the server accepts', () => {
    for (const strategy of ['leastLoad', 'leastPing', 'random', 'roundRobin'] as const) {
      const parsed = ChainSelectorFormSchema.safeParse({
        remark: 'entries',
        strategy,
        proxyIds: [1, 2],
        enabled: true,
      });
      expect(parsed.success).toBe(true);
    }
  });
});
