import { describe, expect, it } from 'vitest';
import { ClientHydrateSchema, ClientTrafficSchema } from '@/schemas/client';

describe('client depletion response parsing', () => {
  it('accepts both current causes without a known historical timestamp', () => {
    const depletion = { depletedAt: 0, depletedReason: 'both' };
    expect(ClientTrafficSchema.parse(depletion)).toEqual(depletion);
    expect(
      ClientHydrateSchema.parse({ client: { email: 'unknown' }, inboundIds: [], ...depletion }),
    ).toMatchObject(depletion);
  });
  it('preserves depletion on traffic and client detail responses', () => {
    const depletion = { depletedAt: 1700000000000, depletedReason: 'traffic' };
    expect(ClientTrafficSchema.parse({ up: 100, total: 100, ...depletion })).toEqual({
      up: 100,
      total: 100,
      ...depletion,
    });
    const detail = {
      client: { email: 'spent' },
      inboundIds: [],
      externalLinks: [],
      ...depletion,
    };
    expect(ClientHydrateSchema.parse(detail)).toMatchObject(detail);
  });

  it('continues accepting responses from older panels', () => {
    expect(ClientTrafficSchema.parse({ up: 1, down: 2 })).toEqual({ up: 1, down: 2 });
    expect(ClientHydrateSchema.parse({ client: { email: 'old' }, inboundIds: [] })).toMatchObject({
      client: { email: 'old' },
      inboundIds: [],
    });
  });
});
