import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import { GeodataCategoriesSchema, type GeodataCategories } from '@/schemas/routing';

const EMPTY_CATEGORIES: GeodataCategories = { domain: [], ip: [] };

async function fetchGeodataCategories(): Promise<GeodataCategories> {
  const msg = await HttpUtil.get('/panel/api/xray/getGeodataCategories', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch geodata categories');
  const validated = parseMsg(msg, GeodataCategoriesSchema, 'xray/getGeodataCategories');
  return validated.obj ?? EMPTY_CATEGORIES;
}

// Deliberately not staleTime: Infinity like useInboundOptions: geodata .dat
// files can change from xray-core's own unattended geodata-update cron,
// which has no invalidation hook into the panel. Inheriting the app's
// global default staleTime lets a long-open tab pick up newly downloaded
// categories on refocus, at near-zero backend cost thanks to the
// mtime/size cache in GetGeodataCategories.
export function useGeodataCategories() {
  return useQuery({
    queryKey: keys.xray.geodataCategories(),
    queryFn: fetchGeodataCategories,
  });
}
