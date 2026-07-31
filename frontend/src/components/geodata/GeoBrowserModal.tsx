import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Empty, Input, Modal, Pagination, Select, Space, Table, Tag, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { useGeodataCategories, useGeodataEntries, useGeodataFiles } from '@/api/queries/useGeodata';
import { canonicalToken, mergeSelection, selectionFromValue, tokenFor } from '@/lib/xray/geoTokens';
import { SizeFormatter } from '@/utils';
import type { GeoCategory, GeoEntry, GeoFile, GeoKind } from '@/generated/types';

import './GeoBrowserModal.css';

const ENTRY_PAGE_SIZE = 100;
const CATEGORY_SCROLL_HEIGHT = 438;
const ENTRY_FILTER_DELAY = 500;

export interface GeoBrowserModalProps {
  open: boolean;
  kind: GeoKind;
  value: string;
  onApply: (value: string) => void;
  onClose: () => void;
}

// A geosite category inside an ip rule (or the reverse) is a config Xray will
// reject, so a field only ever offers databases of its own kind.
function databasesFor(files: GeoFile[], kind: GeoKind): GeoFile[] {
  return files.filter((file) => file.kind === kind || (file.error && namePrefersKind(file.name, kind)));
}

function namePrefersKind(name: string, kind: GeoKind): boolean {
  return name.toLowerCase().includes('ip') === (kind === 'ip');
}

function preferredFile(files: GeoFile[], kind: GeoKind): string | undefined {
  const usable = databasesFor(files, kind).filter((file) => !file.error);
  const preferredName = kind === 'ip' ? 'geoip.dat' : 'geosite.dat';
  return usable.find((file) => file.name === preferredName)?.name ?? usable[0]?.name;
}

export default function GeoBrowserModal({ open, kind, value, onApply, onClose }: GeoBrowserModalProps) {
  const { t } = useTranslation();
  const [file, setFile] = useState<string | undefined>(undefined);
  const [categoryQuery, setCategoryQuery] = useState('');
  const [activeCode, setActiveCode] = useState<string | undefined>(undefined);
  const [entryQuery, setEntryQuery] = useState('');
  const [entryFilter, setEntryFilter] = useState('');
  const [entryPage, setEntryPage] = useState(1);
  const [selected, setSelected] = useState<string[]>([]);

  const knownRef = useRef<Set<string>>(new Set());
  const seededFilesRef = useRef<Set<string>>(new Set());

  const filesQuery = useGeodataFiles(open);
  const files = useMemo(() => databasesFor(filesQuery.data ?? [], kind), [filesQuery.data, kind]);
  const activeFile = files.find((candidate) => candidate.name === file);
  const fileKind: GeoKind = activeFile?.kind ?? kind;

  const categoriesQuery = useGeodataCategories(file, '', open && !!file);
  // While a newly picked database loads, the query still serves the previous
  // one's categories; seeding or filtering against those would attribute one
  // database's codes to another.
  const categoriesLoaded = !categoriesQuery.isPlaceholderData && !categoriesQuery.isLoading;
  const categories = useMemo(
    () => (categoriesLoaded ? (categoriesQuery.data?.items ?? []) : []),
    [categoriesLoaded, categoriesQuery.data],
  );

  // Only the settled filter reaches the query key: every request rescans the
  // whole .dat file server-side, so a per-keystroke fetch would be one full
  // scan per character while the box itself stays instant.
  const entriesQuery = useGeodataEntries(
    file,
    activeCode,
    entryFilter,
    (entryPage - 1) * ENTRY_PAGE_SIZE,
    ENTRY_PAGE_SIZE,
    open && !!file && !!activeCode,
  );

  // Resets clear both halves at once so a switch of database or category never
  // renders with the previous filter still in the key, which would fire the
  // very request the debounce exists to avoid.
  const clearEntryFilter = useCallback(() => {
    setEntryQuery('');
    setEntryFilter('');
    setEntryPage(1);
  }, []);

  useEffect(() => {
    if (entryQuery === entryFilter) return;
    const handle = window.setTimeout(() => {
      setEntryFilter(entryQuery);
      setEntryPage(1);
    }, ENTRY_FILTER_DELAY);
    return () => window.clearTimeout(handle);
  }, [entryQuery, entryFilter]);

  useEffect(() => {
    if (!open) return;
    knownRef.current = new Set();
    seededFilesRef.current = new Set();
    setCategoryQuery('');
    setEntryQuery('');
    setEntryFilter('');
    setActiveCode(undefined);
    setEntryPage(1);
    setSelected([]);
  }, [open]);

  useEffect(() => {
    if (!open || file || files.length === 0) return;
    setFile(preferredFile(files, kind));
  }, [open, file, files, kind]);

  useEffect(() => {
    if (!open || !file || categories.length === 0 || seededFilesRef.current.has(file)) return;
    const tokens = categories.map((category) => tokenFor(file, category.code, fileKind));
    for (const token of tokens) knownRef.current.add(token);
    seededFilesRef.current.add(file);
    const fromValue = selectionFromValue(value, new Set(tokens));
    if (fromValue.length > 0) {
      setSelected((previous) => [...previous, ...fromValue.filter((token) => !previous.includes(token))]);
    }
  }, [open, file, categories, fileKind, value]);

  const visibleCategories = useMemo(() => {
    const query = categoryQuery.trim().toLowerCase();
    if (!query) return categories;
    return categories.filter((category) => category.code.includes(query));
  }, [categories, categoryQuery]);

  // Comparisons run through the canonical form: a field may hold the long
  // ext:geosite.dat:cn spelling or a different case, and those name the same
  // category as the geosite:cn this modal generates.
  const selectedCodes = useMemo(() => {
    if (!file) return [];
    const chosen = new Set(selected.map(canonicalToken));
    return categories
      .filter((category) => chosen.has(canonicalToken(tokenFor(file, category.code, fileKind))))
      .map((category) => category.code);
  }, [categories, file, fileKind, selected]);

  const toggle = useCallback(
    (codes: string[]) => {
      if (!file) return;
      const chosen = new Set(codes.map((code) => tokenFor(file, code, fileKind)));
      const chosenCanonical = new Set([...chosen].map(canonicalToken));
      // The table reports keys for the rows it currently shows, so a selection
      // made before the search box was narrowed must survive untouched.
      const shown = new Set(
        visibleCategories.map((category) => canonicalToken(tokenFor(file, category.code, fileKind))),
      );
      setSelected((previous) => {
        const kept = previous.filter((token) => {
          const canonical = canonicalToken(token);
          return !shown.has(canonical) || chosenCanonical.has(canonical);
        });
        const keptCanonical = new Set(kept.map(canonicalToken));
        return [...kept, ...[...chosen].filter((token) => !keptCanonical.has(canonicalToken(token)))];
      });
    },
    [visibleCategories, file, fileKind],
  );

  const categoryColumns: ColumnsType<GeoCategory> = useMemo(
    () => [
      {
        title: t('pages.xray.geoBrowser.searchCategory'),
        dataIndex: 'code',
        render: (code: string, category: GeoCategory) => (
          <span className="geo-category">
            <span className="geo-code">{code}</span>
            {category.attributes?.length > 0 && (
              <span className="geo-attrs">
                {category.attributes.map((attribute) => (
                  <Tag key={attribute} bordered={false}>
                    @{attribute}
                  </Tag>
                ))}
              </span>
            )}
          </span>
        ),
      },
      {
        dataIndex: 'entries',
        align: 'right',
        width: 90,
        render: (entries: number) => <span className="geo-count">{entries.toLocaleString()}</span>,
      },
    ],
    [t],
  );

  const entryColumns: ColumnsType<GeoEntry> = useMemo(
    () => [
      {
        dataIndex: 'kind',
        width: 88,
        render: (entryKind: string) => (
          <Tag bordered={false} className={`geo-kind geo-kind-${entryKind}`}>
            {entryKind}
          </Tag>
        ),
      },
      {
        dataIndex: 'value',
        render: (entryValue: string) => <span className="geo-entry-value">{entryValue}</span>,
      },
    ],
    [],
  );

  const fileOptions = files.map((candidate) => ({
    value: candidate.name,
    label: candidate.error ? `${candidate.name} — ${describeFileError(candidate.error, t)}` : candidate.name,
    disabled: !!candidate.error,
  }));

  const meta = activeFile
    ? t('pages.xray.geoBrowser.fileMeta', {
        count: activeFile.categories.toLocaleString(),
        size: SizeFormatter.sizeFormat(activeFile.size),
        date: new Date(activeFile.modifiedAt).toLocaleString(),
      })
    : '';

  const entriesTotal = entriesQuery.data?.total ?? 0;
  const activeCategory = categories.find((category) => category.code === activeCode);
  const countLabel = activeCategory
    ? t(fileKind === 'ip' ? 'pages.xray.geoBrowser.subnetsCount' : 'pages.xray.geoBrowser.entriesCount', {
        count: activeCategory.entries.toLocaleString(),
      })
    : '';

  return (
    <Modal
      open={open}
      title={t('pages.xray.geoBrowser.title')}
      width={880}
      onCancel={onClose}
      onOk={() => onApply(mergeSelection(value, selected, knownRef.current))}
      okText={t('pages.xray.geoBrowser.apply')}
      cancelText={t('close')}
      className="geo-browser-modal"
    >
      {filesQuery.isError && <Alert type="error" showIcon title={t('pages.xray.geoBrowser.loadFailed')} className="mb-12" />}

      {!filesQuery.isError && !filesQuery.isLoading && files.length === 0 ? (
        <Empty
          description={
            <span>
              {t('pages.xray.geoBrowser.noFiles')}
              <br />
              <Typography.Text type="secondary">{t('pages.xray.geoBrowser.noFilesHint')}</Typography.Text>
            </span>
          }
        />
      ) : (
        <>
          <div className="geo-toolbar">
            <Select
              value={file}
              options={fileOptions}
              onChange={(next) => {
                setFile(next);
                setActiveCode(undefined);
                setCategoryQuery('');
                clearEntryFilter();
              }}
              style={{ minWidth: 200 }}
              aria-label={t('pages.xray.geoBrowser.database')}
            />
            <Input.Search
              value={categoryQuery}
              onChange={(event) => setCategoryQuery(event.target.value)}
              placeholder={t('pages.xray.geoBrowser.searchCategory')}
              allowClear
            />
            <Button
              onClick={() => toggle([...new Set([...selectedCodes, ...visibleCategories.map((c) => c.code)])])}
              disabled={visibleCategories.length === 0}
            >
              {`${t('pages.xray.geoBrowser.selectFound')} (${visibleCategories.length.toLocaleString()})`}
            </Button>
            <span className="geo-meta">{meta}</span>
          </div>

          <div className="geo-columns">
            <div className="geo-panel geo-categories">
              <Table
                size="small"
                virtual
                showHeader={false}
                rowKey="code"
                columns={categoryColumns}
                dataSource={visibleCategories}
                loading={filesQuery.isLoading || categoriesQuery.isLoading || categoriesQuery.isPlaceholderData}
                pagination={false}
                scroll={{ y: CATEGORY_SCROLL_HEIGHT }}
                locale={{ emptyText: t('pages.xray.geoBrowser.noMatches') }}
                rowSelection={{
                  columnWidth: 42,
                  preserveSelectedRowKeys: true,
                  selectedRowKeys: selectedCodes,
                  onChange: (keys) => toggle(keys as string[]),
                }}
                onRow={(category) => ({
                  onClick: (event) => {
                    if ((event.target as HTMLElement).closest('.ant-table-selection-column')) return;
                    setActiveCode(category.code);
                    clearEntryFilter();
                  },
                })}
                rowClassName={(category) => (category.code === activeCode ? 'geo-row-active' : '')}
              />
            </div>

            <div className="geo-panel geo-preview">
              {activeCode ? (
                <>
                  <div className="geo-preview-head">
                    <Tooltip title={file ? tokenFor(file, activeCode, fileKind) : activeCode}>
                      <span className="geo-preview-title">{activeCode}</span>
                    </Tooltip>
                    <Typography.Text type="secondary">{countLabel}</Typography.Text>
                    <Input
                      value={entryQuery}
                      onChange={(event) => setEntryQuery(event.target.value)}
                      placeholder={t('pages.xray.geoBrowser.searchEntries')}
                      allowClear
                      className="geo-entry-filter"
                    />
                  </div>
                  <div className="geo-preview-body">
                    <Table
                      size="small"
                      showHeader={false}
                      rowKey={(entry, index) => `${entry.value}-${index}`}
                      columns={entryColumns}
                      dataSource={entriesQuery.data?.items ?? []}
                      loading={entriesQuery.isLoading}
                      locale={{
                        emptyText: entriesQuery.isError
                          ? t('pages.xray.geoBrowser.loadFailed')
                          : t('pages.xray.geoBrowser.noMatches'),
                      }}
                      pagination={false}
                    />
                  </div>
                  <div className="geo-pager">
                    <Pagination
                      current={entryPage}
                      pageSize={ENTRY_PAGE_SIZE}
                      total={entriesTotal}
                      size="small"
                      showSizeChanger={false}
                      onChange={setEntryPage}
                      showTotal={(total, range) =>
                        t('pages.xray.geoBrowser.shownRange', {
                          from: range[0].toLocaleString(),
                          to: range[1].toLocaleString(),
                          total: total.toLocaleString(),
                        })
                      }
                    />
                  </div>
                </>
              ) : (
                <div className="geo-placeholder">
                  <Typography.Text type="secondary">{t('pages.xray.geoBrowser.pickCategory')}</Typography.Text>
                </div>
              )}
            </div>
          </div>

          <div className="geo-footer">
            {selected.length === 0 ? (
              <Typography.Text type="secondary">{t('pages.xray.geoBrowser.emptySelection')}</Typography.Text>
            ) : (
              <>
                <Space size={4} wrap className="geo-chips">
                  {selected.map((token) => (
                    <Tag
                      key={token}
                      closable
                      color="processing"
                      onClose={() => setSelected((previous) => previous.filter((item) => item !== token))}
                    >
                      {token}
                    </Tag>
                  ))}
                </Space>
                <span className="geo-selected-count">
                  {t('pages.xray.geoBrowser.selected', { count: selected.length })}
                </span>
                <Button type="link" size="small" onClick={() => setSelected([])}>
                  {t('pages.xray.geoBrowser.clearAll')}
                </Button>
              </>
            )}
          </div>
        </>
      )}
    </Modal>
  );
}

function describeFileError(error: string, t: (key: string) => string): string {
  if (error.includes('too large')) return t('pages.xray.geoBrowser.tooLarge');
  return t('pages.xray.geoBrowser.parseFailed');
}
