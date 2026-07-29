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
  // parseMsg falls back to the original, unvalidated obj on a schema mismatch
  // (see zodValidate.ts) rather than clearing it, so each field is guarded
  // independently here -- same reasoning as useInboundOptions's Array.isArray
  // check, just applied per-field since this response is an object of two
  // arrays rather than one top-level array.
  return {
    domain: Array.isArray(validated.obj?.domain) ? validated.obj.domain : EMPTY_CATEGORIES.domain,
    ip: Array.isArray(validated.obj?.ip) ? validated.obj.ip : EMPTY_CATEGORIES.ip,
  };
}

// Deliberately not staleTime: Infinity like useInboundOptions: geodata .dat
// files can change from xray-core's own unattended geodata-update cron,
// which has no invalidation hook into the panel. Inheriting the app's
// global default staleTime lets a long-open tab pick up newly downloaded
// categories on refocus, at near-zero backend cost thanks to the
// mtime/size cache in GetGeodataCategories.
//
// enabled defaults to true but is meant to be passed as `open` from the rule
// editor modal: the underlying scan/parse is the expensive part of this
// feature (see GetGeodataCategories), so it should run when the editor is
// actually opened, not on every visit to the Routing tab that merely mounts
// this modal closed.
export function useGeodataCategories(enabled = true) {
  return useQuery({
    queryKey: keys.xray.geodataCategories(),
    queryFn: fetchGeodataCategories,
    enabled,
  });
}
