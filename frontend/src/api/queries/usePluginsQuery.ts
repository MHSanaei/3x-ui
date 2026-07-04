import { useQuery } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { PluginCatalogSchema, type PluginCatalog } from '@/schemas/api/plugin';
import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';

async function fetchPluginCatalog(): Promise<PluginCatalog> {
  const msg = await HttpUtil.get('/panel/api/plugins/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch plugins');
  const validated = parseMsg(msg, PluginCatalogSchema, 'plugins/list');
  if (!validated.obj) throw new Error('Plugin catalog response is empty');
  return validated.obj;
}

export function usePluginsQuery() {
  return useQuery({
    queryKey: keys.plugins.list(),
    queryFn: fetchPluginCatalog,
  });
}
