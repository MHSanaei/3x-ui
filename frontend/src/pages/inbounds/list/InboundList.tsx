import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Dropdown,
  Space,
  Switch,
  Table,
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
}: InboundListProps) {
  const { t } = useTranslation();
  const [sortKey, setSortKey] = useState<SortKey | null>(null);
  const [sortOrder, setSortOrder] = useState<SortOrder>(null);
  const [statsRecord, setStatsRecord] = useState<DBInboundRecord | null>(null);

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
              sortedInbounds.map((record) => (
                <div key={record.id} className="inbound-card">
                  <div className="card-head">
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
              ))
            )}
          </div>
        ) : (
          <Table
            columns={columns}
            dataSource={sortedInbounds}
            rowKey={(r) => r.id}
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
