import { useCallback, useMemo, useState, type ReactElement } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Dropdown,
  Modal,
  Popover,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  type TableColumnType,
  type MenuProps,
} from 'antd';
import {
  PlusOutlined,
  MenuOutlined,
  MoreOutlined,
  EditOutlined,
  QrcodeOutlined,
  CopyOutlined,
  ExportOutlined,
  ImportOutlined,
  ReloadOutlined,
  RetweetOutlined,
  BlockOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';

import { HttpUtil, SizeFormatter, IntlUtil, ColorUtils } from '@/utils';
import InfinityIcon from '@/components/InfinityIcon';
import { useDatepicker } from '@/hooks/useDatepicker';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import './InboundList.css';

type ProtocolFlags = {
  isVMess?: boolean;
  isVLess?: boolean;
  isTrojan?: boolean;
  isSS?: boolean;
  isHysteria?: boolean;
  isMixed?: boolean;
  isHTTP?: boolean;
  isWireguard?: boolean;
};

interface DBInboundRecord extends ProtocolFlags {
  id: number;
  enable: boolean;
  remark: string;
  port: number;
  protocol: string;
  up: number;
  down: number;
  total: number;
  expiryTime: number;
  _expiryTime: { valueOf(): number } | null;
  nodeId?: number | null;
  toInbound: () => {
    stream?: { network?: string; isTls?: boolean; isReality?: boolean };
    isSSMultiUser?: boolean;
  };
  isMultiUser: () => boolean;
}

export interface ClientCountEntry {
  clients: number;
  active: string[];
  deactive: string[];
  depleted: string[];
  expiring: string[];
  online: string[];
}

export type RowAction =
  | 'edit'
  | 'showInfo'
  | 'qrcode'
  | 'export'
  | 'subs'
  | 'clipboard'
  | 'delete'
  | 'resetTraffic'
  | 'clone';

export type GeneralAction = 'import' | 'export' | 'subs' | 'resetInbounds';

interface InboundListProps {
  dbInbounds: DBInboundRecord[];
  clientCount: Record<number, ClientCountEntry>;
  onlineClients: string[];
  lastOnlineMap: Record<string, number>;
  expireDiff: number;
  trafficDiff: number;
  pageSize: number;
  isMobile: boolean;
  subEnable: boolean;
  nodesById: Map<number, NodeRecord>;
  hasActiveNode: boolean;
  onAddInbound: () => void;
  onGeneralAction: (key: GeneralAction) => void;
  onRowAction: (action: { key: RowAction; dbInbound: DBInboundRecord }) => void;
}

type SortKey =
  | 'id'
  | 'enable'
  | 'remark'
  | 'port'
  | 'protocol'
  | 'traffic'
  | 'expiryTime'
  | 'node'
  | 'clients';

type SortOrder = 'ascend' | 'descend' | null;

const SORT_FNS: Record<SortKey, (a: DBInboundRecord, b: DBInboundRecord, ctx: { nodesById: Map<number, NodeRecord>; clientCount: Record<number, ClientCountEntry> }) => number> = {
  id: (a, b) => a.id - b.id,
  enable: (a, b) => Number(a.enable) - Number(b.enable),
  remark: (a, b) => (a.remark || '').localeCompare(b.remark || ''),
  port: (a, b) => a.port - b.port,
  protocol: (a, b) => a.protocol.localeCompare(b.protocol),
  traffic: (a, b) => (a.up + a.down) - (b.up + b.down),
  expiryTime: (a, b) => (a.expiryTime || Infinity) - (b.expiryTime || Infinity),
  node: (a, b, ctx) => {
    const nameA = ctx.nodesById.get(a.nodeId ?? -1)?.name ?? (a.nodeId == null ? '￿' : `node #${a.nodeId}`);
    const nameB = ctx.nodesById.get(b.nodeId ?? -1)?.name ?? (b.nodeId == null ? '￿' : `node #${b.nodeId}`);
    return nameA.localeCompare(nameB);
  },
  clients: (a, b, ctx) => (ctx.clientCount[a.id]?.clients || 0) - (ctx.clientCount[b.id]?.clients || 0),
};

function showQrCodeMenu(dbInbound: DBInboundRecord): boolean {
  if (dbInbound.isWireguard) return true;
  if (dbInbound.isSS) {
    try {
      return !dbInbound.toInbound().isSSMultiUser;
    } catch {
      return false;
    }
  }
  return false;
}

interface RowActionsMenuProps {
  record: DBInboundRecord;
  subEnable: boolean;
  onClick: (key: RowAction) => void;
  isMobile?: boolean;
}

function buildRowActionsMenu({ record, subEnable, t, isMobile }: { record: DBInboundRecord; subEnable: boolean; t: (k: string) => string; isMobile?: boolean }): MenuProps['items'] {
  const items: MenuProps['items'] = [];
  if (isMobile) {
    items.push({ key: 'edit', icon: <EditOutlined />, label: t('edit') });
  }
  if (showQrCodeMenu(record)) {
    items.push({ key: 'qrcode', icon: <QrcodeOutlined />, label: t('qrCode') });
  }
  if (record.isMultiUser()) {
    items.push({ key: 'export', icon: <ExportOutlined />, label: t('pages.inbounds.export') });
    if (subEnable) {
      items.push({
        key: 'subs',
        icon: <ExportOutlined />,
        label: `${t('pages.inbounds.export')} — ${t('pages.settings.subSettings')}`,
      });
    }
  } else {
    items.push({ key: 'showInfo', icon: <InfoCircleOutlined />, label: t('info') });
  }
  items.push({ key: 'clipboard', icon: <CopyOutlined />, label: t('pages.inbounds.exportInbound') });
  items.push({ key: 'resetTraffic', icon: <RetweetOutlined />, label: t('pages.inbounds.resetTraffic') });
  items.push({ key: 'clone', icon: <BlockOutlined />, label: t('pages.inbounds.clone') });
  items.push({ key: 'delete', icon: <DeleteOutlined />, danger: true, label: t('delete') });
  return items;
}

function RowActionsCell({ record, subEnable, onClick }: RowActionsMenuProps) {
  const { t } = useTranslation();
  return (
    <div className="action-buttons">
      <Button type="text" size="small" icon={<EditOutlined />} onClick={() => onClick('edit')} />
      <Dropdown
        trigger={['click']}
        menu={{
          items: buildRowActionsMenu({ record, subEnable, t }),
          onClick: ({ key }) => onClick(key as RowAction),
        }}
      >
        <Button type="text" size="small" icon={<MoreOutlined />} />
      </Dropdown>
    </div>
  );
}

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
  const { datepicker } = useDatepicker();
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

  const sorterFor = useCallback((key: SortKey) => ({
    sorter: true as const,
    showSorterTooltip: false,
    sortOrder: sortKey === key ? sortOrder : null,
    sortDirections: ['ascend' as const, 'descend' as const],
  }), [sortKey, sortOrder]);

  const columns: TableColumnType<DBInboundRecord>[] = useMemo(() => {
    const cols: TableColumnType<DBInboundRecord>[] = [
      {
        title: 'ID',
        dataIndex: 'id',
        key: 'id',
        align: 'right',
        width: 30,
        ...sorterFor('id'),
      },
      {
        title: t('pages.inbounds.operate'),
        key: 'action',
        align: 'center',
        width: 60,
        render: (_, record) => (
          <RowActionsCell
            record={record}
            subEnable={subEnable}
            onClick={(key) => onRowAction({ key, dbInbound: record })}
          />
        ),
      },
      {
        title: t('pages.inbounds.enable'),
        key: 'enable',
        align: 'center',
        width: 35,
        ...sorterFor('enable'),
        render: (_, record) => (
          <Switch
            checked={record.enable}
            onChange={(next) => onSwitchEnable(record, next)}
          />
        ),
      },
    ];

    if (hasAnyRemark) {
      cols.push({
        title: t('pages.inbounds.remark'),
        dataIndex: 'remark',
        key: 'remark',
        align: 'center',
        width: 60,
        ...sorterFor('remark'),
      });
    }

    if (hasActiveNode) {
      cols.push({
        title: t('pages.inbounds.node'),
        key: 'node',
        align: 'center',
        width: 60,
        ...sorterFor('node'),
        render: (_, record) => {
          if (record.nodeId == null) {
            return <Tag color="default">{t('pages.inbounds.localPanel')}</Tag>;
          }
          const node = nodesById.get(record.nodeId);
          if (!node) {
            return <Tag color="orange">node #{record.nodeId}</Tag>;
          }
          return (
            <Tag color={node.status === 'online' ? 'blue' : 'red'}>{node.name}</Tag>
          );
        },
      });
    }

    cols.push(
      {
        title: t('pages.inbounds.port'),
        dataIndex: 'port',
        key: 'port',
        align: 'center',
        width: 40,
        ...sorterFor('port'),
      },
      {
        title: t('pages.inbounds.protocol'),
        key: 'protocol',
        align: 'left',
        width: 130,
        ...sorterFor('protocol'),
        render: (_, record) => {
          const tags: ReactElement[] = [<Tag key="p" color="purple">{record.protocol}</Tag>];
          if (record.isVMess || record.isVLess || record.isTrojan || record.isSS || record.isHysteria) {
            const stream = record.toInbound().stream;
            tags.push(
              <Tag key="n" color="green">
                {record.isHysteria ? 'UDP' : stream?.network}
              </Tag>,
            );
            if (stream?.isTls) tags.push(<Tag key="tls" color="blue">TLS</Tag>);
            if (stream?.isReality) tags.push(<Tag key="reality" color="blue">Reality</Tag>);
          }
          return <div className="protocol-tags">{tags}</div>;
        },
      },
      {
        title: t('clients'),
        key: 'clients',
        align: 'left',
        width: 50,
        ...sorterFor('clients'),
        render: (_, record) => {
          const cc = clientCount[record.id];
          if (!cc) return null;
          return (
            <>
              <Tag color="green" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>
                {cc.clients}
              </Tag>
              {cc.deactive.length > 0 && (
                <Popover
                  title={t('disabled')}
                  content={(
                    <div className="client-email-list">
                      {cc.deactive.map((e) => <div key={e}>{e}</div>)}
                    </div>
                  )}
                >
                  <Tag className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{cc.deactive.length}</Tag>
                </Popover>
              )}
              {cc.depleted.length > 0 && (
                <Popover
                  title={t('depleted')}
                  content={(
                    <div className="client-email-list">
                      {cc.depleted.map((e) => <div key={e}>{e}</div>)}
                    </div>
                  )}
                >
                  <Tag color="red" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{cc.depleted.length}</Tag>
                </Popover>
              )}
              {cc.expiring.length > 0 && (
                <Popover
                  title={t('depletingSoon')}
                  content={(
                    <div className="client-email-list">
                      {cc.expiring.map((e) => <div key={e}>{e}</div>)}
                    </div>
                  )}
                >
                  <Tag color="orange" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{cc.expiring.length}</Tag>
                </Popover>
              )}
              {cc.online.length > 0 && (
                <Popover
                  title={t('online')}
                  content={(
                    <div className="client-email-list">
                      {cc.online.map((e) => <div key={e}>{e}</div>)}
                    </div>
                  )}
                >
                  <Tag color="blue" className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>{cc.online.length}</Tag>
                </Popover>
              )}
            </>
          );
        },
      },
      {
        title: t('pages.inbounds.traffic'),
        key: 'traffic',
        align: 'center',
        width: 90,
        ...sorterFor('traffic'),
        render: (_, record) => (
          <Popover
            content={(
              <table cellPadding={2}>
                <tbody>
                  <tr>
                    <td>↑ {SizeFormatter.sizeFormat(record.up)}</td>
                    <td>↓ {SizeFormatter.sizeFormat(record.down)}</td>
                  </tr>
                  {record.total > 0 && record.up + record.down < record.total && (
                    <tr>
                      <td>{t('remained')}</td>
                      <td>{SizeFormatter.sizeFormat(record.total - record.up - record.down)}</td>
                    </tr>
                  )}
                </tbody>
              </table>
            )}
          >
            <Tag color={ColorUtils.usageColor(record.up + record.down, trafficDiff, record.total)}>
              {SizeFormatter.sizeFormat(record.up + record.down)} /
              {' '}
              {record.total > 0 ? SizeFormatter.sizeFormat(record.total) : <InfinityIcon />}
            </Tag>
          </Popover>
        ),
      },
      {
        title: t('pages.inbounds.expireDate'),
        key: 'expiryTime',
        align: 'center',
        width: 40,
        ...sorterFor('expiryTime'),
        render: (_, record) => {
          if (record.expiryTime > 0) {
            return (
              <Popover content={IntlUtil.formatDate(record.expiryTime, datepicker)}>
                <Tag color={ColorUtils.usageColor(Date.now(), expireDiff, record._expiryTime)} style={{ minWidth: 50 }}>
                  {IntlUtil.formatRelativeTime(record.expiryTime)}
                </Tag>
              </Popover>
            );
          }
          return <Tag color="purple"><InfinityIcon /></Tag>;
        },
      },
    );

    return cols;
  }, [t, hasAnyRemark, hasActiveNode, nodesById, clientCount, subEnable, expireDiff, trafficDiff, datepicker, onRowAction, onSwitchEnable, sorterFor]);

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
              <div className="card-empty">—</div>
            ) : (
              sortedInbounds.map((record) => (
                <div key={record.id} className="inbound-card">
                  <div className="card-head">
                    <span className="card-id">#{record.id}</span>
                    <span className="tag-name">{record.remark}</span>
                    <div className="card-actions" onClick={(e) => e.stopPropagation()}>
                      <Tooltip title={t('info')}>
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
                          items: buildRowActionsMenu({ record, subEnable, t, isMobile: true }),
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
            onChange={(_p, _f, sorter) => {
              const single = Array.isArray(sorter) ? sorter[0] : sorter;
              const colKey = (single?.columnKey || single?.field) as SortKey | undefined;
              setSortKey(colKey || null);
              setSortOrder((single?.order as SortOrder) || null);
            }}
          />
        )}
      </Space>

      <Modal
        open={isMobile && !!statsRecord}
        footer={null}
        width={360}
        centered
        title={statsRecord ? `#${statsRecord.id} ${statsRecord.remark || ''}`.trim() : ''}
        onCancel={() => setStatsRecord(null)}
        destroyOnHidden
      >
        {statsRecord && (
          <div className="card-stats">
            <div className="stat-row">
              <span className="stat-label">{t('pages.inbounds.protocol')}</span>
              <Tag color="purple">{statsRecord.protocol}</Tag>
              {(statsRecord.isVMess || statsRecord.isVLess || statsRecord.isTrojan || statsRecord.isSS || statsRecord.isHysteria) && (
                <>
                  <Tag color="green">
                    {statsRecord.isHysteria ? 'UDP' : statsRecord.toInbound().stream?.network}
                  </Tag>
                  {statsRecord.toInbound().stream?.isTls && <Tag color="blue">TLS</Tag>}
                  {statsRecord.toInbound().stream?.isReality && <Tag color="blue">Reality</Tag>}
                </>
              )}
            </div>
            <div className="stat-row">
              <span className="stat-label">{t('pages.inbounds.port')}</span>
              <Tag>{statsRecord.port}</Tag>
            </div>
            {hasActiveNode && (
              <div className="stat-row">
                <span className="stat-label">{t('pages.inbounds.node')}</span>
                {statsRecord.nodeId == null ? (
                  <Tag color="default">{t('pages.inbounds.localPanel')}</Tag>
                ) : nodesById.get(statsRecord.nodeId) ? (
                  <Tag color={nodesById.get(statsRecord.nodeId)!.status === 'online' ? 'blue' : 'red'}>
                    {nodesById.get(statsRecord.nodeId)!.name}
                  </Tag>
                ) : (
                  <Tag color="orange">#{statsRecord.nodeId}</Tag>
                )}
              </div>
            )}
            <div className="stat-row">
              <span className="stat-label">{t('pages.inbounds.traffic')}</span>
              <Tag color={ColorUtils.usageColor(statsRecord.up + statsRecord.down, trafficDiff, statsRecord.total)}>
                {SizeFormatter.sizeFormat(statsRecord.up + statsRecord.down)} /
                {' '}
                {statsRecord.total > 0 ? SizeFormatter.sizeFormat(statsRecord.total) : <InfinityIcon />}
              </Tag>
            </div>
            {clientCount[statsRecord.id] && (
              <div className="stat-row">
                <span className="stat-label">{t('clients')}</span>
                <Tag color="green" className="client-count-tag">{clientCount[statsRecord.id].clients}</Tag>
                {clientCount[statsRecord.id].online.length > 0 && (
                  <Tag color="blue">{clientCount[statsRecord.id].online.length} {t('online')}</Tag>
                )}
                {clientCount[statsRecord.id].depleted.length > 0 && (
                  <Tag color="red">{clientCount[statsRecord.id].depleted.length} {t('depleted')}</Tag>
                )}
                {clientCount[statsRecord.id].expiring.length > 0 && (
                  <Tag color="orange">{clientCount[statsRecord.id].expiring.length} {t('depletingSoon')}</Tag>
                )}
              </div>
            )}
            <div className="stat-row">
              <span className="stat-label">{t('pages.inbounds.expireDate')}</span>
              {statsRecord.expiryTime > 0 ? (
                <Tag color={ColorUtils.usageColor(Date.now(), expireDiff, statsRecord._expiryTime)}>
                  {IntlUtil.formatRelativeTime(statsRecord.expiryTime)}
                </Tag>
              ) : (
                <Tag color="purple"><InfinityIcon /></Tag>
              )}
            </div>
          </div>
        )}
      </Modal>
    </Card>
  );
}
