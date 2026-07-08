'use client';

import { create } from '@orama/orama';
import { useDocsSearch } from 'fumadocs-core/search/client';
import { oramaStaticClient } from 'fumadocs-core/search/client/orama-static';
import {
  SearchDialog,
  SearchDialogClose,
  SearchDialogContent,
  SearchDialogHeader,
  SearchDialogIcon,
  SearchDialogInput,
  SearchDialogList,
  SearchDialogOverlay,
} from 'fumadocs-ui/components/dialog/search';
import { useI18n } from 'fumadocs-ui/contexts/i18n';
import { useMemo } from 'react';

interface SharedProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// The static search index is keyed by locale code (en/fa/ru/zh). Fumadocs'
// default static dialog feeds those codes to Orama as a tokenizer language, but
// Orama only accepts full names ("english") and throws on "en" — which silently
// breaks search entirely. All docs content is English (other locales fall back
// to it), so re-create the dialog — the documented escape hatch for custom Orama
// setups — with an initOrama that always builds an English index.
export default function SearchDialogClient(props: SharedProps) {
  const { locale } = useI18n();
  const client = useMemo(
    () =>
      oramaStaticClient({
        from: '/api/search',
        locale,
        initOrama: () => create({ schema: { _: 'string' }, language: 'english' }),
      }),
    [locale],
  );
  const { search, setSearch, query } = useDocsSearch({ client });

  return (
    <SearchDialog search={search} onSearchChange={setSearch} isLoading={query.isLoading} {...props}>
      <SearchDialogOverlay />
      <SearchDialogContent>
        <SearchDialogHeader>
          <SearchDialogIcon />
          <SearchDialogInput />
          <SearchDialogClose />
        </SearchDialogHeader>
        <SearchDialogList items={query.data !== 'empty' ? query.data : null} />
      </SearchDialogContent>
    </SearchDialog>
  );
}
