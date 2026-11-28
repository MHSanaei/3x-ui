import { useCallback, useSyncExternalStore } from 'react';
import { useTranslation } from 'react-i18next';
import { Tooltip, theme } from 'antd';
import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons';

import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { SizeFormatter } from '@/utils';

const HEARTBEAT_FRESHNESS_MS = 30_000;

export default function NodeNetworkSpeed({
  node,
  receivedAt,
}: {
  node: NodeRecord;
  receivedAt?: number;
}) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const expiration =
    receivedAt && node.heartbeatAgeMillis != null
      ? receivedAt + HEARTBEAT_FRESHNESS_MS - node.heartbeatAgeMillis
      : node.lastHeartbeat
        ? node.lastHeartbeat * 1000 + HEARTBEAT_FRESHNESS_MS
        : 0;
  const subscribe = useCallback(
    (onChange: () => void) => {
      const delay = expiration - Date.now();
      const timer =
        delay > 0 && delay <= HEARTBEAT_FRESHNESS_MS
          ? window.setTimeout(onChange, delay)
          : undefined;
      return () => window.clearTimeout(timer);
    },
    [expiration],
  );
  const isFresh = () => {
    const delay = expiration - Date.now();
    return delay > 0 && delay <= HEARTBEAT_FRESHNESS_MS;
  };
  const fresh = useSyncExternalStore(subscribe, isFresh);
  const available = node.enable && node.status === 'online' && !node.transitive && fresh;
  const directions = [
    {
      key: 'netDown' as const,
      label: t('pages.index.download'),
      Icon: ArrowDownOutlined,
      color: token.colorTextTertiary,
    },
    {
      key: 'netUp' as const,
      label: t('pages.index.upload'),
      Icon: ArrowUpOutlined,
      color: token.colorPrimary,
    },
  ];
  return (
    <span className="node-network-speed">
      {directions.map(({ key, label, Icon, color }) => {
        const rate = node[key];
        const value =
          available &&
          !node.unavailableMetrics?.includes(key) &&
          typeof rate === 'number' &&
          Number.isFinite(rate) &&
          rate >= 0
            ? SizeFormatter.speedFormat(rate)
            : '—';
        return (
          <Tooltip key={key} title={label}>
            <span
              role="img"
              aria-label={label + ' ' + value}
              className="node-network-direction"
              dir="ltr"
            >
              <Icon aria-hidden style={{ color }} />
              <span aria-hidden>{value}</span>
            </span>
          </Tooltip>
        );
      })}
    </span>
  );
}
