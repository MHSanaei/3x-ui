import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Table, Tag, Tooltip } from 'antd';
import { ThunderboltOutlined, LoadingOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import { SizeFormatter } from '@/utils';
import { OutboundProtocols as Protocols } from '@/schemas/primitives';
import { isUdpOutbound } from '@/hooks/useXraySetting';
import type { OutboundTestState, OutboundTrafficRow } from '@/hooks/useXraySetting';

import type { OutboundRow } from './outbounds-tab-types';
import TestResultPopover from './TestResultPopover';
import {
  isTesting,
  isUntestable,
  outboundAddresses,
  showSecurity,
  testResult,
  trafficFor,
} from './outbounds-tab-helpers';

interface SubscriptionOutboundsProps {
  subscriptionOutbounds: unknown[];
  outboundsTraffic: OutboundTrafficRow[];
  subscriptionTestStates: Record<string, OutboundTestState>;
  testMode: 'tcp' | 'http';
  isMobile: boolean;
  onTestSubscription: (outbound: Record<string, unknown>, mode: string) => void;
}

// Read-only view of outbounds imported from active subscriptions. They are not
// part of the editable template (so no edit/delete/move), but traffic is matched
// by tag and they can be latency-tested via the same backend endpoint.
export default function SubscriptionOutbounds({
  subscriptionOutbounds,
  outboundsTraffic,
  subscriptionTestStates,
  testMode,
  isMobile,
  onTestSubscription,
}: SubscriptionOutboundsProps) {
  const { t } = useTranslation();

  const rows = useMemo<OutboundRow[]>(
    () => (subscriptionOutbounds || []).map((o, i) => ({ ...(o as object), key: i }) as OutboundRow),
    [subscriptionOutbounds],
  );

  if (rows.length === 0) return null;

  const identityCell = (record: OutboundRow) => (
    <div className="identity-cell">
      <Tooltip title={record.tag}>
        <span className="tag-name">{record.tag || '—'}</span>
      </Tooltip>
      <div className="protocol-line">
        <Tag color="green">{record.protocol}</Tag>
        {[Protocols.VMess, Protocols.VLESS, Protocols.Trojan, Protocols.Shadowsocks].includes(record.protocol as never) && (
          <>
            <Tag>{record.streamSettings?.network}</Tag>
            {showSecurity(record.streamSettings?.security) && <Tag color="purple">{record.streamSettings?.security}</Tag>}
          </>
        )}
      </div>
    </div>
  );

  const addressCell = (record: OutboundRow) => {
    const addrs = outboundAddresses(record);
    return (
      <div className="address-list">
        {addrs.length === 0 ? (
          <span className="empty">—</span>
        ) : (
          addrs.map((addr) => (
            <Tooltip key={addr} title={addr}>
              <span className="address-pill">{addr}</span>
            </Tooltip>
          ))
        )}
      </div>
    );
  };

  const trafficCell = (record: OutboundRow) => {
    const tr = trafficFor(outboundsTraffic, record);
    return (
      <>
        <span className="traffic-up">↑ {SizeFormatter.sizeFormat(tr.up)}</span>
        <span className="traffic-sep" />
        <span className="traffic-down">↓ {SizeFormatter.sizeFormat(tr.down)}</span>
      </>
    );
  };

  const latencyCell = (record: OutboundRow) => {
    const key = record.tag || '';
    const r = testResult(subscriptionTestStates, key);
    if (!r) return isTesting(subscriptionTestStates, key) ? <LoadingOutlined /> : <span className="empty">—</span>;
    return <TestResultPopover result={r} />;
  };

  const testButton = (record: OutboundRow) => {
    const key = record.tag || '';
    return (
      <Tooltip title={`${t('check')} (${(isUdpOutbound(record) ? 'http' : testMode).toUpperCase()})`}>
        <Button
          type="primary"
          shape="circle"
          size={isMobile ? 'small' : undefined}
          loading={isTesting(subscriptionTestStates, key)}
          disabled={!record.tag || isUntestable(record, testMode) || isTesting(subscriptionTestStates, key)}
          icon={<ThunderboltOutlined />}
          onClick={() => onTestSubscription(record as unknown as Record<string, unknown>, testMode)}
        />
      </Tooltip>
    );
  };

  const header = (
    <div className="subscription-outbounds-head">
      <div className="subscription-outbounds-title">{t('pages.xray.outboundSub.fromSubsTitle')}</div>
      <div className="subscription-outbounds-desc">{t('pages.xray.outboundSub.fromSubsDesc')}</div>
    </div>
  );

  if (isMobile) {
    return (
      <div className="subscription-outbounds" style={{ marginTop: 16 }}>
        {header}
        {rows.map((record, index) => (
          <div key={record.key} className="outbound-card">
            <div className="card-head">
              <div className="card-identity">
                <span className="card-num">{index + 1}</span>
                {identityCell(record)}
              </div>
              {testButton(record)}
            </div>
            {outboundAddresses(record).length > 0 && addressCell(record)}
            <div className="card-foot">
              {trafficCell(record)}
              <span className="card-test">{latencyCell(record)}</span>
            </div>
          </div>
        ))}
      </div>
    );
  }

  const columns: ColumnsType<OutboundRow> = [
    {
      title: '#',
      key: 'num',
      align: 'center',
      width: 60,
      render: (_v, _record, index) => <span className="row-index">{index + 1}</span>,
    },
    { title: t('pages.xray.outbound.tag'), key: 'identity', align: 'left', render: (_v, record) => identityCell(record) },
    { title: t('pages.inbounds.address'), key: 'address', align: 'left', render: (_v, record) => addressCell(record) },
    { title: t('pages.inbounds.traffic'), key: 'traffic', align: 'left', width: 200, render: (_v, record) => trafficCell(record) },
    { title: t('pages.nodes.latency'), key: 'testResult', align: 'left', width: 140, render: (_v, record) => latencyCell(record) },
    { title: t('check'), key: 'test', align: 'center', width: 80, render: (_v, record) => testButton(record) },
  ];

  return (
    <div className="subscription-outbounds" style={{ marginTop: 16 }}>
      {header}
      <Table columns={columns} dataSource={rows} rowKey={(r) => r.key} pagination={false} size="small" />
    </div>
  );
}
