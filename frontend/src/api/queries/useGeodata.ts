import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { keys } from '@/api/queryKeys';
import { GeoCategoryPageSchema, GeoEntryPageSchema, GeoFileSchema, GeodataTokenIssueSchema } from '@/generated/zod';
import type { GeoCategoryPage, GeoEntryPage, GeoFile, GeodataTokenIssue } from '@/generated/types';
import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';

const GeoFileListSchema = z.array(GeoFileSchema);
const GeodataTokenIssueListSchema = z.array(GeodataTokenIssueSchema);

const EMPTY_CATEGORY_PAGE: GeoCategoryPage = { total: 0, items: [] };
const EMPTY_ENTRY_PAGE: GeoEntryPage = { total: 0, items: [] };

export type GeoTokenKind = 'ip' | 'domain';

export interface ValidateGeoTokensInput {
  tokens: string[];
  kind: GeoTokenKind;
}

async function fetchGeodataFiles(): Promise<GeoFile[]> {
  const msg = await HttpUtil.get('/panel/api/xray/geodata/files', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch geodata files');
  const validated = parseMsg(msg, GeoFileListSchema, 'xray/geodata/files');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

async function fetchGeodataCategories(file: string, query: string): Promise<GeoCategoryPage> {
  const msg = await HttpUtil.get('/panel/api/xray/geodata/categories', { file, q: query }, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch geodata categories');
  const validated = parseMsg(msg, GeoCategoryPageSchema, 'xray/geodata/categories');
  return validated.obj ?? EMPTY_CATEGORY_PAGE;
}

async function fetchGeodataEntries(
  file: string,
  code: string,
  query: string,
  offset: number,
  limit: number,
): Promise<GeoEntryPage> {
  const msg = await HttpUtil.get(
    '/panel/api/xray/geodata/entries',
    { file, code, q: query, offset, limit },
    { silent: true },
  );
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch geodata entries');
  const validated = parseMsg(msg, GeoEntryPageSchema, 'xray/geodata/entries');
  return validated.obj ?? EMPTY_ENTRY_PAGE;
}

export function useGeodataFiles(enabled: boolean) {
  return useQuery({
    queryKey: keys.xray.geodata.files(),
    queryFn: fetchGeodataFiles,
    enabled,
    staleTime: 5 * 60 * 1000,
  });
}

export function useGeodataCategories(file: string | undefined, query: string, enabled: boolean) {
  return useQuery({
    queryKey: keys.xray.geodata.categories(file ?? '', query),
    queryFn: () => fetchGeodataCategories(file ?? '', query),
    enabled: enabled && !!file,
    staleTime: 5 * 60 * 1000,
    placeholderData: keepPreviousData,
  });
}

export function useGeodataEntries(
  file: string | undefined,
  code: string | undefined,
  query: string,
  offset: number,
  limit: number,
  enabled: boolean,
) {
  return useQuery({
    queryKey: keys.xray.geodata.entries(file ?? '', code ?? '', query, offset, limit),
    queryFn: () => fetchGeodataEntries(file ?? '', code ?? '', query, offset, limit),
    enabled: enabled && !!file && !!code,
    placeholderData: keepPreviousData,
  });
}

export function useValidateGeoTokens() {
  return useMutation<GeodataTokenIssue[], Error, ValidateGeoTokensInput>({
    mutationFn: async ({ tokens, kind }) => {
      const msg = await HttpUtil.post(
        '/panel/api/xray/geodata/validate',
        { tokens: tokens.join(','), kind },
        { silent: true },
      );
      if (!msg?.success) throw new Error(msg?.msg || 'Failed to validate geodata tokens');
      const validated = parseMsg(msg, GeodataTokenIssueListSchema, 'xray/geodata/validate');
      return Array.isArray(validated.obj) ? validated.obj : [];
    },
  });
}
