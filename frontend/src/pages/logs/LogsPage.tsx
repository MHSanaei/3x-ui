import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Card, ConfigProvider, Empty, Layout, Result, Spin, Tabs } from 'antd';
import { useQueryClient } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { useLogFilesQuery, useLogLinesQuery, useLogStatsQuery } from '@/api/queries/useLogs';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { EMPTY_LOG_FILTERS, type LogFilters, type LogTextField } from '@/lib/logs/logQuery';
import LogsFilterBar from './LogsFilterBar';
import LogsOverview from './LogsOverview';
import LogsTable from './LogsTable';
import './LogsPage.css';

const TEXT_FILTER_DELAY = 300;
type LogsTab = 'overview' | 'raw';

export default function LogsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const queryClient = useQueryClient();

  const [tab, setTab] = useState<LogsTab>('overview');
  const [chosenFile, setChosenFile] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState<number | false>(false);
  // The inputs edit `draft` at typing speed; the scans only ever see `filters`.
  const [draft, setDraft] = useState<LogFilters>(EMPTY_LOG_FILTERS);
  const [filters, setFilters] = useState<LogFilters>(EMPTY_LOG_FILTERS);

  useEffect(() => {
    if (draft === filters) return;
    const handle = window.setTimeout(() => setFilters(draft), TEXT_FILTER_DELAY);
    return () => window.clearTimeout(handle);
  }, [draft, filters]);

  const applyNow = useCallback((next: LogFilters) => {
    setDraft(next);
    setFilters(next);
  }, []);

  const toggleField = useCallback(
    (field: LogTextField, value: string) => {
      applyNow({ ...draft, [field]: draft[field] === value ? '' : value });
    },
    [applyNow, draft],
  );

  const selectTimeSpan = useCallback(
    (since: number, until: number) => applyNow({ ...draft, range: 'custom', since, until }),
    [applyNow, draft],
  );

  const filesQuery = useLogFilesQuery(autoRefresh);
  const files = useMemo(() => filesQuery.data?.files ?? [], [filesQuery.data]);
  // A chosen file can rotate away between refreshes, so the choice is re-validated here.
  const file = files.some((f) => f.name === chosenFile) ? chosenFile : (files[0]?.name ?? null);

  const statsQuery = useLogStatsQuery({
    file,
    filters,
    refetchInterval: autoRefresh,
    enabled: tab === 'overview',
  });
  const linesQuery = useLogLinesQuery({
    file,
    filters,
    refetchInterval: autoRefresh,
    enabled: tab === 'raw',
  });

  const pageClass = useMemo(() => {
    const classes = ['logs-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const external = filesQuery.data?.external ?? [];
  const activeQuery = tab === 'overview' ? statsQuery : linesQuery;
  const firstLoad = filesQuery.isPending;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={firstLoad} delay={200} description={t('loading')} size="large">
              {firstLoad ? (
                <div className="loading-spacer" />
              ) : filesQuery.isError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={filesQuery.error.message}
                  extra={
                    <Button type="primary" onClick={() => void filesQuery.refetch()}>
                      {t('refresh')}
                    </Button>
                  }
                />
              ) : (
                <div className="logs-inner">
                  {external.length > 0 && (
                    <Alert
                      type="info"
                      showIcon
                      closable
                      title={t('pages.logs.externalTitle')}
                      description={
                        <>
                          <div>{t('pages.logs.externalDesc')}</div>
                          {external.map((entry) => (
                            <div key={entry.kind} className="logs-external-row">
                              <span>{t(`pages.logs.kinds.${entry.kind}`)}:</span>
                              <code dir="ltr">{entry.path}</code>
                              <span aria-hidden>→</span>
                              <code dir="ltr">{entry.suggested}</code>
                            </div>
                          ))}
                        </>
                      }
                    />
                  )}
                  {files.length === 0 ? (
                    <Card>
                      <Empty description={t('pages.logs.noFiles')}>
                        <div>{t('pages.logs.noFilesHint')}</div>
                        <code dir="ltr">{filesQuery.data?.folder}</code>
                      </Empty>
                    </Card>
                  ) : (
                    <>
                      <LogsFilterBar
                        files={files}
                        file={file}
                        onFileChange={setChosenFile}
                        draft={draft}
                        onDraftChange={setDraft}
                        onApply={applyNow}
                        autoRefresh={autoRefresh}
                        onAutoRefreshChange={setAutoRefresh}
                        refreshing={filesQuery.isFetching || activeQuery.isFetching}
                        onRefresh={() =>
                          void queryClient.invalidateQueries({ queryKey: keys.logs.root() })
                        }
                      />
                      <Tabs
                        activeKey={tab}
                        onChange={(key) => setTab(key as LogsTab)}
                        items={[
                          {
                            key: 'overview',
                            label: t('pages.logs.tabs.overview'),
                            children: (
                              <LogsOverview
                                query={statsQuery}
                                filters={draft}
                                onToggleField={toggleField}
                                onSelectTimeSpan={selectTimeSpan}
                              />
                            ),
                          },
                          {
                            key: 'raw',
                            label: t('pages.logs.tabs.raw'),
                            children: (
                              <LogsTable
                                key={JSON.stringify([file, filters])}
                                query={linesQuery}
                                filters={filters}
                                onToggleField={toggleField}
                              />
                            ),
                          },
                        ]}
                      />
                    </>
                  )}
                </div>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
