import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { ClientPageResponseSchema } from '@/schemas/client';
import { fetchXrayConfig } from '@/hooks/useXraySetting';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('parseMsg', () => {
  it('rejects a successful response whose payload violates its schema', () => {
    const msg = new Msg(true, '', { id: 'not-a-number' });

    expect(() => parseMsg(msg, z.object({ id: z.number() }), 'test/value', { strict: true })).toThrow(
      'test/value response failed validation',
    );
  });

  it('preserves a missing successful payload for callers that handle empty values', () => {
    expect(parseMsg(new Msg(true, '', null), z.object({ id: z.number() }), 'test/value').obj).toBeNull();
  });

  it('rejects malformed paged-client payloads', () => {
    const payload = { items: [], total: 'one', filtered: 1, page: 1, pageSize: 20 };

    expect(() => parseMsg(new Msg(true, '', payload), ClientPageResponseSchema, 'clients/list/paged', { strict: true })).toThrow(
      'clients/list/paged response failed validation',
    );
  });
});

describe('fetchXrayConfig', () => {
  it('keeps a malformed xray payload available for repair', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', JSON.stringify({ xraySetting: 'not-an-object' })));

    await expect(fetchXrayConfig()).resolves.toEqual({ xraySetting: 'not-an-object' });
  });
});
