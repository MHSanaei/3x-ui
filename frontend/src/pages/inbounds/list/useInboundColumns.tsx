import { useMemo, type ReactElement } from 'react';
import { useTranslation } from 'react-i18next';
import { Popover, Switch, Tag, type TableColumnType } from 'antd';
import { TeamOutlined } from '@ant-design/icons';

import { SizeFormatter, IntlUtil, ColorUtils } from '@/utils';
import { InfinityIcon } from '@/components/ui';
import { useDatepicker } from '@/hooks/useDatepicker';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

import { RowActionsCell } from './RowActions';
import {
  readStreamHints,
  networkLabel,
  networkL4,
  shadowsocksNetworkLabel,
  tunnelNetworkLabel,
  mixedNetworkLabel,
} from './helpers';
import type { ClientCountEntry, DBInboundRecord, RowAction } from './types';

interface UseInboundColumnsParams {
  hasAnyRemark: boolean;
  hasActiveNode: boolean;
  nodesById: Map<number, NodeRecord>;
  clientCount: Record<number, ClientCountEntry>;
  subEnable: boolean;
  expireDiff: number;
  trafficDiff: number;
  onRowAction: (action: { key: RowAction; dbInbound: DBInboundRecord }) => void;
  onSwitchEnable: (dbInbound: DBInboundRecord, next: boolean) => void;
}

export function useInboundColumns({
  hasAnyRemark,
  hasActiveNode,
  nodesById,
  clientCount,
  subEnable,
  expireDiff,
  trafficDiff,
  onRowAction,
  onSwitchEnable,
}: UseInboundColumnsParams): TableColumnType<DBInboundRecord>[] {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();

  return useMemo(() => {
    const cols: TableColumnType<DBInboundRecord>[] = [
      {
        title: 'ID',
        dataIndex: 'id',
        key: 'id',
        align: 'right',
        width: 30,
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
            hasClients={(clientCount[record.id]?.clients || 0) > 0}
            onClick={(key) => onRowAction({ key, dbInbound: record })}
          />
        ),
      },
      {
        title: t('pages.inbounds.enable'),
        key: 'enable',
        align: 'center',
        width: 35,
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
      });
    }

    if (hasActiveNode) {
      cols.push({
        title: t('pages.inbounds.node'),
        key: 'node',
        align: 'center',
        width: 60,
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
      },
      {
        title: t('pages.inbounds.protocol'),
        key: 'protocol',
        align: 'left',
        width: 130,
        render: (_, record) => {
          const tags: ReactElement[] = [<Tag key="p" color="purple">{record.protocol}</Tag>];
          if (record.isWireguard || record.isHysteria) {
            tags.push(<Tag key="n" color="green">UDP</Tag>);
          } else if (record.isSS) {
            const stream = readStreamHints(record.streamSettings);
            tags.push(<Tag key="n" color="green">{shadowsocksNetworkLabel(record.settings)}</Tag>);
            if (stream.isTls) tags.push(<Tag key="tls" color="blue">TLS</Tag>);
          } else if (record.isTunnel) {
            tags.push(<Tag key="n" color="green">{tunnelNetworkLabel(record.settings)}</Tag>);
          } else if (record.isMixed) {
            tags.push(<Tag key="n" color="green">{mixedNetworkLabel(record.settings)}</Tag>);
          } else if (record.isVMess || record.isVLess || record.isTrojan) {
            const stream = readStreamHints(record.streamSettings);
            tags.push(<Tag key="n" color="green">{networkLabel(stream.network)}</Tag>);
            const l4 = networkL4(stream.network);
            if (l4) tags.push(<Tag key="l4" color="green">{l4}</Tag>);
            if (stream.isTls) tags.push(<Tag key="tls" color="blue">TLS</Tag>);
            if (stream.isReality) tags.push(<Tag key="reality" color="blue">Reality</Tag>);
          }
          return <div className="protocol-tags">{tags}</div>;
        },
      },
      {
        title: t('clients'),
        key: 'clients',
        align: 'left',
        width: 110,
        render: (_, record) => {
          const cc = clientCount[record.id];
          if (!cc) return null;
          return (
            <>
              <Tag className="client-count-tag" style={{ margin: 0, padding: '0 2px' }}>
                <TeamOutlined /> {cc.clients}
              </Tag>
              {cc.active.length > 0 && (
                <Popover
                  title={t('subscription.active')}
                  content={(
                    <div className="client-email-list">
                      {cc.active.map((e) => <div key={e}>{e}</div>)}
                    </div>
                  )}
                >
                  <Tag color="green" className="client-count-tag" style={{ margin: 0, marginRight: 4, padding: '0 2px' }}>{cc.active.length}</Tag>
                </Popover>
              )}
              {cc.deactive.length > 0 && (
                <Popover
                  title={t('disabled')}
                  content={(
                    <div className="client-email-list">
                      {cc.deactive.map((e) => <div key={e}>{e}</div>)}
                    </div>
                  )}
                >
                  <Tag className="client-count-tag" style={{ margin: 0, marginRight: 4, padding: '0 2px' }}>{cc.deactive.length}</Tag>
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
                  <Tag color="red" className="client-count-tag" style={{ margin: 0, marginRight: 4, padding: '0 2px' }}>{cc.depleted.length}</Tag>
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
  }, [t, hasAnyRemark, hasActiveNode, nodesById, clientCount, subEnable, expireDiff, trafficDiff, datepicker, onRowAction, onSwitchEnable]);
}
