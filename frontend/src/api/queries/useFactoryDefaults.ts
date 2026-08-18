import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { FactoryDefaultsSchema, type FactoryDefaults } from '@/schemas/setting';
import { keys } from '@/api/queryKeys';

async function fetchFactoryDefaults(): Promise<FactoryDefaults> {
  const msg = await HttpUtil.post('/panel/api/setting/factoryDefaults', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch factory defaults');
  const validated = parseMsg(msg, FactoryDefaultsSchema, 'setting/factoryDefaults');
  const parsed = FactoryDefaultsSchema.safeParse(validated.obj);
  return parsed.success ? parsed.data : {};
}

export function useFactoryDefaults() {
  return useQuery({
    queryKey: keys.settings.factoryDefaults(),
    queryFn: fetchFactoryDefaults,
    staleTime: Infinity,
  });
}
