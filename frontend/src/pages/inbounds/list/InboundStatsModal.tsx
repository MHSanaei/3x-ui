import { useTranslation } from 'react-i18next';
import { Modal, Tag } from 'antd';

import { SizeFormatter, IntlUtil, ColorUtils } from '@/utils';
import { InfinityIcon } from '@/components/ui';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

import {
  readStreamHints,
  networkLabel,
  networkL4,
  shadowsocksNetworkLabel,
  tunnelNetworkLabel,
  mixedNetworkLabel,
} from './helpers';
import type { ClientCountEntry, DBInboundRecord } from './types';

interface InboundStatsModalProps {
  open: boolean;
  record: DBInboundRecord | null;
  hasActiveNode: boolean;
  nodesById: Map<number, NodeRecord>;
  clientCount: Record<number, ClientCountEntry>;
  trafficDiff: number;
  expireDiff: number;
  onClose: () => void;
}

export default function InboundStatsModal({
  open,
  record,
  hasActiveNode,
  nodesById,
  clientCount,
  trafficDiff,
  expireDiff,
  onClose,
}: InboundStatsModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      footer={null}
      width={360}
      centered
      title={record ? `#${record.id} ${record.remark || ''}`.trim() : ''}
      onCancel={onClose}
      destroyOnHidden
    >
      {record && (
        <div className="card-stats">
          <div className="stat-row">
            <span className="stat-label">{t('pages.inbounds.protocol')}</span>
            <Tag color="purple">{record.protocol}</Tag>
            {(record.isWireguard || record.isHysteria) && (
              <Tag color="green">UDP</Tag>
            )}
            {record.isSS && (() => {
              const stream = readStreamHints(record.streamSettings);
              return (
                <>
                  <Tag color="green">{shadowsocksNetworkLabel(record.settings)}</Tag>
                  {stream.isTls && <Tag color="blue">TLS</Tag>}
                </>
              );
            })()}
            {record.isTunnel && (
              <Tag color="green">{tunnelNetworkLabel(record.settings)}</Tag>
            )}
            {record.isMixed && (
              <Tag color="green">{mixedNetworkLabel(record.settings)}</Tag>
            )}
            {(record.isVMess || record.isVLess || record.isTrojan) && (() => {
              const stream = readStreamHints(record.streamSettings);
              const l4 = networkL4(stream.network);
              return (
                <>
                  <Tag color="green">{networkLabel(stream.network)}</Tag>
                  {l4 && <Tag color="green">{l4}</Tag>}
                  {stream.isTls && <Tag color="blue">TLS</Tag>}
                  {stream.isReality && <Tag color="blue">Reality</Tag>}
                </>
              );
            })()}
          </div>
          <div className="stat-row">
            <span className="stat-label">{t('pages.inbounds.port')}</span>
            <Tag>{record.port}</Tag>
          </div>
          {hasActiveNode && (
            <div className="stat-row">
              <span className="stat-label">{t('pages.inbounds.node')}</span>
              {record.nodeId == null ? (
                <Tag color="default">{t('pages.inbounds.localPanel')}</Tag>
              ) : nodesById.get(record.nodeId) ? (
                <Tag color={nodesById.get(record.nodeId)!.status === 'online' ? 'blue' : 'red'}>
                  {nodesById.get(record.nodeId)!.name}
                </Tag>
              ) : (
                <Tag color="orange">#{record.nodeId}</Tag>
              )}
            </div>
          )}
          <div className="stat-row">
            <span className="stat-label">{t('pages.inbounds.traffic')}</span>
            <Tag color={ColorUtils.usageColor(record.up + record.down, trafficDiff, record.total)}>
              {SizeFormatter.sizeFormat(record.up + record.down)} /
              {' '}
              {record.total > 0 ? SizeFormatter.sizeFormat(record.total) : <InfinityIcon />}
            </Tag>
          </div>
          {clientCount[record.id] && (
            <div className="stat-row">
              <span className="stat-label">{t('clients')}</span>
              <Tag color="green" className="client-count-tag">{clientCount[record.id].clients}</Tag>
              {clientCount[record.id].online.length > 0 && (
                <Tag color="blue">{clientCount[record.id].online.length} {t('online')}</Tag>
              )}
              {clientCount[record.id].depleted.length > 0 && (
                <Tag color="red">{clientCount[record.id].depleted.length} {t('depleted')}</Tag>
              )}
              {clientCount[record.id].expiring.length > 0 && (
                <Tag color="orange">{clientCount[record.id].expiring.length} {t('depletingSoon')}</Tag>
              )}
            </div>
          )}
          <div className="stat-row">
            <span className="stat-label">{t('pages.inbounds.expireDate')}</span>
            {record.expiryTime > 0 ? (
              <Tag color={ColorUtils.usageColor(Date.now(), expireDiff, record._expiryTime)}>
                {IntlUtil.formatRelativeTime(record.expiryTime)}
              </Tag>
            ) : (
              <Tag color="purple"><InfinityIcon /></Tag>
            )}
          </div>
        </div>
      )}
    </Modal>
  );
}
