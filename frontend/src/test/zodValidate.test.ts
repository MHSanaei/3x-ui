import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { ClientPageResponseSchema } from '@/schemas/client';
import { AllSettingSchema } from '@/schemas/setting';
import { fetchXrayConfig } from '@/hooks/useXraySetting';

describe('parseMsg', () => {
  it('rejects a successful response whose payload violates its schema', () => {
    const msg = new Msg(true, '', { id: 'not-a-number' });

    expect(() => parseMsg(msg, z.object({ id: z.number() }), 'test/value')).toThrow(
      'test/value response failed validation',
    );
  });

  it('rejects a missing successful payload when the schema requires an object', () => {
    expect(() => parseMsg(new Msg(true, '', null), z.object({ id: z.number() }), 'test/value')).toThrow(
      'test/value response failed validation',
    );
  });

  it.each([
    ['clients/list/paged', ClientPageResponseSchema, { items: [], total: 'one', filtered: 1, page: 1, pageSize: 20 }],
    ['setting/all', AllSettingSchema, { webPort: 'not-a-port' }],
  ])('rejects malformed %s payloads', (context, schema, payload) => {
    expect(() => parseMsg(new Msg(true, '', payload), schema, context)).toThrow(`${context} response failed validation`);
  });

  it('rejects a malformed xray payload before it reaches the query cache', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(true, '', JSON.stringify({ xraySetting: 'not-an-object' })));

    await expect(fetchXrayConfig()).rejects.toThrow('xray/ config payload failed validation');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });
});
