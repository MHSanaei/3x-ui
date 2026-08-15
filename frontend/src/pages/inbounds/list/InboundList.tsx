import { useCallback, useMemo, useState, type Key } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Checkbox,
  Dropdown,
  Input,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  type MenuProps,
} from 'antd';
import {
  PlusOutlined,
  MenuOutlined,
  MoreOutlined,
  ExportOutlined,
  ImportOutlined,
  ReloadOutlined,
  InfoCircleOutlined,
  DeleteOutlined,
  SearchOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { activateOnKey } from '@/utils/a11y';

import { buildRowActionsMenu } from './RowActions';
import { useInboundColumns } from './useInboundColumns';
import InboundStatsModal from './InboundStatsModal';
import type { DBInboundRecord, GeneralAction, InboundListProps, RowAction } from './types';
import './InboundList.css';

export default function InboundList({
  dbInbounds,
  clientCount,
  lastOnlineMap: _lastOnlineMap,
  inboundSpeed,
  expireDiff,
  trafficDiff,
  pageSize,
  isMobile,
  subEnable,
  nodesById,
  hasActiveNode,
  onAddInbound,
  onGeneralAction,
  onRowAction,
  onBulkDelete,
}: InboundListProps) {
  const { t } = useTranslation();
  const [statsRecord, setStatsRecord] = useState<DBInboundRecord | null>(null);
  const [selectedRowKeys, setSelectedRowKeys] = useState<number[]>([]);
  // Node filter (#4997): 'all' shows everything, 0 is the local-panel
  // sentinel (inbounds without a nodeId), otherwise a node id. Session-only.
  const [nodeFilter, setNodeFilter] = useState<number | 'all'>('all');
  const [searchKey, setSearchKey] = useState('');

  const showNodeFilter = useMemo(
    () => nodesById.size > 0 || dbInbounds.some((ib) => ib.nodeId != null),
    [nodesById, dbInbounds],
  );

  const nodeFilterOptions = useMemo(
    () => [
      { value: 'all' as const, label: t('pages.clients.filters.nodes') },
      { value: 0, label: t('pages.clients.filters.localPanel') },
      ...Array.from(nodesById.values()).map((n) => ({ value: n.id, label: n.name || `#${n.id}` })),
    ],
    [nodesById, t],
  );

  const visibleInbounds = useMemo(() => {
    let list = dbInbounds;
    if (nodeFilter === 0) list = list.filter((ib) => ib.nodeId == null);
    else if (nodeFilter !== 'all') list = list.filter((ib) => ib.nodeId === nodeFilter);
    const q = searchKey.trim().toLowerCase();
    if (!q) return list;
    return list.filter((ib) => (
      (ib.remark || '').toLowerCase().includes(q)
      || String(ib.port).includes(q)
      || (ib.protocol || '').toLowerCase().includes(q)
    ));
  }, [dbInbounds, nodeFilter, searchKey]);

  const onSwitchEnable = useCallback(async (dbInbound: DBInboundRecord, next: boolean) => {
    const previous = dbInbound.enable;
    dbInbound.enable = next;
    try {
      const formData = new FormData();
      formData.append('enable', String(next));
      const msg = await HttpUtil.post(`/panel/api/inbounds/setEnable/${dbInbound.id}`, formData);
      if (!msg?.success) dbInbound.enable = previous;
    } catch {
      dbInbound.enable = previous;
    }
  }, []);

  const hasAnyRemark = useMemo(
    () => dbInbounds.some((i) => typeof i.remark === 'string' && i.remark.trim() !== ''),
    [dbInbounds],
  );

  const hasAnySubSortIndex = useMemo(
    () => dbInbounds.some((i) => (i.subSortIndex ?? 1) > 1),
    [dbInbounds],
  );

  const toggleSelect = useCallback((id: number, checked: boolean) => {
    setSelectedRowKeys((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id); else next.delete(id);
      return Array.from(next);
    });
  }, []);

  const selectAll = useCallback((checked: boolean) => {
    setSelectedRowKeys(checked ? visibleInbounds.map((i) => i.id) : []);
  }, [visibleInbounds]);

  const allSelected = visibleInbounds.length > 0 && selectedRowKeys.length === visibleInbounds.length;
  const someSelected = selectedRowKeys.length > 0 && selectedRowKeys.length < visibleInbounds.length;

  const handleBulkDelete = useCallback(async () => {
    const ok = await onBulkDelete(selectedRowKeys);
    if (ok) setSelectedRowKeys([]);
  }, [onBulkDelete, selectedRowKeys]);

  const columns = useInboundColumns({
    hasAnyRemark,
    hasAnySubSortIndex,
    hasActiveNode,
    nodesById,
    clientCount,
    inboundSpeed,
    subEnable,
    expireDiff,
    trafficDiff,
    onRowAction,
    onSwitchEnable,
  });

  const tableScrollX = useMemo(
    () => columns.reduce((sum, c) => sum + (typeof c.width === 'number' ? c.width : 0), 0),
    [columns],
  );

  const paginationFor = (rows: DBInboundRecord[]) => {
    const size = pageSize > 0 ? pageSize : rows.length || 1;
    return { pageSize: size, showSizeChanger: false, hideOnSinglePage: true };
  };

  const generalActionsMenu: MenuProps = {
    items: [
      { key: 'import', icon: <ImportOutlined />, label: t('pages.inbounds.importInbound') },
      { key: 'export', icon: <ExportOutlined />, label: t('pages.inbounds.export') },
      ...(subEnable
        ? [{ key: 'subs', icon: <ExportOutlined />, label: `${t('pages.inbounds.export')} — ${t('pages.settings.subSettings')}` }]
        : []),
      { key: 'resetInbounds', icon: <ReloadOutlined />, label: t('pages.inbounds.resetAllTraffic') },
    ],
    onClick: ({ key }) => onGeneralAction(key as GeneralAction),
  };

  return (
    <Card
      hoverable
      title={(
        <Space>
          <Button type="primary" onClick={onAddInbound} icon={<PlusOutlined />} aria-label={t('pages.inbounds.addInbound')}>
            {!isMobile && t('pages.inbounds.addInbound')}
          </Button>
          <Dropdown trigger={['click']} menu={generalActionsMenu}>
            <Button type="primary" icon={<MenuOutlined />} aria-label={t('pages.inbounds.generalActions')}>
              {!isMobile && t('pages.inbounds.generalActions')}
            </Button>
          </Dropdown>
          {showNodeFilter && (
            <Select
              value={nodeFilter}
              onChange={(v) => setNodeFilter(v)}
              options={nodeFilterOptions}
              showSearch
              popupMatchSelectWidth={false}
              style={{ minWidth: isMobile ? 90 : 140 }}
              aria-label={t('pages.clients.filters.nodes')}
            />
          )}
          <Input
            value={searchKey}
            onChange={(e) => setSearchKey(e.target.value)}
            placeholder={t('search')}
            allowClear
            prefix={<SearchOutlined />}
            style={{ maxWidth: isMobile ? 110 : 200 }}
            aria-label={t('search')}
          />
          {selectedRowKeys.length > 0 && (
            <>
              <Tag color="blue" closable onClose={() => setSelectedRowKeys([])} style={{ marginInlineEnd: 0 }}>
                {t('pages.inbounds.selectedCount', { count: selectedRowKeys.length })}
              </Tag>
              <Button danger icon={<DeleteOutlined />} onClick={handleBulkDelete} aria-label={t('delete')}>
                {!isMobile && t('delete')}
              </Button>
            </>
          )}
        </Space>
      )}
    >
      <Space orientation="vertical" style={{ width: '100%' }}>
        {isMobile ? (
          <div className="inbound-cards">
            {visibleInbounds.length === 0 ? (
              <div className="card-empty">
                <ImportOutlined style={{ fontSize: 28, opacity: 0.5 }} />
                <div>{t('noData')}</div>
              </div>
            ) : (
              <>
              <div className="card-bulk-bar">
                <Checkbox
                  checked={allSelected}
                  indeterminate={someSelected}
                  onChange={(e) => selectAll(e.target.checked)}
                >
                  {t('pages.inbounds.selectAll')}
                </Checkbox>
                {selectedRowKeys.length > 0 && (
                  <span className="bulk-count">{selectedRowKeys.length}</span>
                )}
              </div>
              {visibleInbounds.map((record) => (
                <div key={record.id} className={`inbound-card${selectedRowKeys.includes(record.id) ? ' is-selected' : ''}`}>
                  <div className="card-head">
                    <Checkbox
                      checked={selectedRowKeys.includes(record.id)}
                      onChange={(e) => toggleSelect(record.id, e.target.checked)}
                    />
                    <span className="card-id">#{record.id}</span>
                    <span className="tag-name">{record.remark}</span>
                    <div className="card-actions">
                      <Tooltip title={t('pages.inbounds.inboundInfo')}>
                        <InfoCircleOutlined
                          className="row-action-trigger"
                          role="button"
                          tabIndex={0}
                          aria-label={t('pages.inbounds.inboundInfo')}
                          onClick={() => setStatsRecord(record)}
                          onKeyDown={activateOnKey(() => setStatsRecord(record))}
                        />
                      </Tooltip>
                      <Switch
                        checked={record.enable}
                        size="small"
                        onChange={(next) => onSwitchEnable(record, next)}
                      />
                      <Dropdown
                        trigger={['click']}
                        placement="bottomRight"
                        menu={{
                          items: buildRowActionsMenu({ record, subEnable, t, isMobile: true, hasClients: (clientCount[record.id]?.clients || 0) > 0 }),
                          onClick: ({ key }) => onRowAction({ key: key as RowAction, dbInbound: record }),
                        }}
                      >
                        <Button type="text" size="small" className="row-action-trigger" icon={<MoreOutlined />} aria-label={t('more')} />
                      </Dropdown>
                    </div>
                  </div>
                </div>
              ))}
              </>
            )}
          </div>
        ) : (
          <Table
            columns={columns}
            dataSource={visibleInbounds}
            rowKey={(r) => r.id}
            rowSelection={{
              selectedRowKeys,
              onChange: (keys: Key[]) => setSelectedRowKeys(keys as number[]),
            }}
            pagination={paginationFor(visibleInbounds)}
            scroll={{ x: tableScrollX }}
            style={{ marginTop: 10 }}
            size="small"
            locale={{
              emptyText: (
                <div className="card-empty">
                  <ImportOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                  <div>{t('noData')}</div>
                </div>
              ),
            }}
          />
        )}
      </Space>

      <InboundStatsModal
        open={isMobile && !!statsRecord}
        record={statsRecord}
        hasActiveNode={hasActiveNode}
        nodesById={nodesById}
        clientCount={clientCount}
        inboundSpeed={inboundSpeed}
        trafficDiff={trafficDiff}
        expireDiff={expireDiff}
        onClose={() => setStatsRecord(null)}
      />
    </Card>
  );
}
