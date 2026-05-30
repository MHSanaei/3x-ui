import { useCallback, useMemo, useState, type Key } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Checkbox,
  Dropdown,
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
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';

import { SORT_FNS } from './helpers';
import { buildRowActionsMenu } from './RowActions';
import { useInboundColumns } from './useInboundColumns';
import InboundStatsModal from './InboundStatsModal';
import type { DBInboundRecord, GeneralAction, InboundListProps, RowAction, SortKey, SortOrder } from './types';
import './InboundList.css';

export default function InboundList({
  dbInbounds,
  clientCount,
  lastOnlineMap: _lastOnlineMap,
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
  const [sortKey, setSortKey] = useState<SortKey | null>(null);
  const [sortOrder, setSortOrder] = useState<SortOrder>(null);
  const [statsRecord, setStatsRecord] = useState<DBInboundRecord | null>(null);
  const [selectedRowKeys, setSelectedRowKeys] = useState<number[]>([]);

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

  const sortedInbounds = useMemo(() => {
    if (!sortKey || !sortOrder) return dbInbounds;
    const fn = SORT_FNS[sortKey];
    if (!fn) return dbInbounds;
    const sorted = [...dbInbounds].sort((a, b) => fn(a, b, { nodesById, clientCount }));
    return sortOrder === 'descend' ? sorted.reverse() : sorted;
  }, [dbInbounds, sortKey, sortOrder, nodesById, clientCount]);

  const hasAnyRemark = useMemo(
    () => dbInbounds.some((i) => typeof i.remark === 'string' && i.remark.trim() !== ''),
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
    setSelectedRowKeys(checked ? sortedInbounds.map((i) => i.id) : []);
  }, [sortedInbounds]);

  const allSelected = sortedInbounds.length > 0 && selectedRowKeys.length === sortedInbounds.length;
  const someSelected = selectedRowKeys.length > 0 && selectedRowKeys.length < sortedInbounds.length;

  const handleBulkDelete = useCallback(async () => {
    const ok = await onBulkDelete(selectedRowKeys);
    if (ok) setSelectedRowKeys([]);
  }, [onBulkDelete, selectedRowKeys]);

  const columns = useInboundColumns({
    hasAnyRemark,
    hasActiveNode,
    nodesById,
    clientCount,
    subEnable,
    expireDiff,
    trafficDiff,
    sortKey,
    sortOrder,
    onRowAction,
    onSwitchEnable,
  });

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
          <Button type="primary" onClick={onAddInbound} icon={<PlusOutlined />}>
            {!isMobile && t('pages.inbounds.addInbound')}
          </Button>
          <Dropdown trigger={['click']} menu={generalActionsMenu}>
            <Button type="primary" icon={<MenuOutlined />}>
              {!isMobile && t('pages.inbounds.generalActions')}
            </Button>
          </Dropdown>
          {selectedRowKeys.length > 0 && (
            <>
              <Tag color="blue" closable onClose={() => setSelectedRowKeys([])} style={{ marginInlineEnd: 0 }}>
                {t('pages.inbounds.selectedCount', { count: selectedRowKeys.length })}
              </Tag>
              <Button danger icon={<DeleteOutlined />} onClick={handleBulkDelete}>
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
            {sortedInbounds.length === 0 ? (
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
              {sortedInbounds.map((record) => (
                <div key={record.id} className={`inbound-card${selectedRowKeys.includes(record.id) ? ' is-selected' : ''}`}>
                  <div className="card-head">
                    <Checkbox
                      checked={selectedRowKeys.includes(record.id)}
                      onChange={(e) => toggleSelect(record.id, e.target.checked)}
                    />
                    <span className="card-id">#{record.id}</span>
                    <span className="tag-name">{record.remark}</span>
                    <div className="card-actions" onClick={(e) => e.stopPropagation()}>
                      <Tooltip title={t('pages.inbounds.inboundInfo')}>
                        <InfoCircleOutlined className="row-action-trigger" onClick={() => setStatsRecord(record)} />
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
                        <MoreOutlined className="row-action-trigger" onClick={(e) => e.preventDefault()} />
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
            dataSource={sortedInbounds}
            rowKey={(r) => r.id}
            rowSelection={{
              selectedRowKeys,
              onChange: (keys: Key[]) => setSelectedRowKeys(keys as number[]),
            }}
            pagination={paginationFor(sortedInbounds)}
            scroll={{ x: 1000 }}
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
            onChange={(_p, _f, sorter) => {
              const single = Array.isArray(sorter) ? sorter[0] : sorter;
              const colKey = (single?.columnKey || single?.field) as SortKey | undefined;
              setSortKey(colKey || null);
              setSortOrder((single?.order as SortOrder) || null);
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
        trafficDiff={trafficDiff}
        expireDiff={expireDiff}
        onClose={() => setStatsRecord(null)}
      />
    </Card>
  );
}
