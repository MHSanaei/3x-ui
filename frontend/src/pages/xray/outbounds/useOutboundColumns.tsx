import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Tag, Tooltip } from 'antd';
import {
  RetweetOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  VerticalAlignTopOutlined,
  ThunderboltOutlined,
  LoadingOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from '@ant-design/icons';
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

interface OutboundColumnsParams {
  testMode: 'tcp' | 'http';
  rows: OutboundRow[];
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  openEdit: (idx: number) => void;
  setFirst: (idx: number) => void;
  moveUp: (idx: number) => void;
  moveDown: (idx: number) => void;
  confirmDelete: (idx: number) => void;
  onResetTraffic: (tag: string) => void;
  onTest: (index: number, mode: string) => void;
}

export function useOutboundColumns({
  testMode,
  rows,
  outboundsTraffic,
  outboundTestStates,
  openEdit,
  setFirst,
  moveUp,
  moveDown,
  confirmDelete,
  onResetTraffic,
  onTest,
}: OutboundColumnsParams): ColumnsType<OutboundRow> {
  const { t } = useTranslation();
  return useMemo(
    () => [
      {
        title: '#',
        key: 'action',
        align: 'center',
        width: 100,
        render: (_v, _record, index) => (
          <div className="action-cell">
            <span className="row-index">{index + 1}</span>
            <div className="action-buttons">
              <Button shape="circle" size="small" icon={<EditOutlined />} onClick={() => openEdit(index)} />
              <Dropdown
                trigger={['click']}
                menu={{
                  items: [
                    ...(index > 0
                      ? [
                          { key: 'top', label: <><VerticalAlignTopOutlined /> Move to top</>, onClick: () => setFirst(index) },
                        ]
                      : []),
                    { key: 'up', label: <ArrowUpOutlined />, disabled: index === 0, onClick: () => moveUp(index) },
                    { key: 'down', label: <ArrowDownOutlined />, disabled: index === rows.length - 1, onClick: () => moveDown(index) },
                    { key: 'reset', label: <><RetweetOutlined /> Reset traffic</>, onClick: () => onResetTraffic(rows[index].tag || '') },
                    { key: 'del', danger: true, label: <><DeleteOutlined /> Delete</>, onClick: () => confirmDelete(index) },
                  ],
                }}
              >
                <Button shape="circle" size="small" icon={<MoreOutlined />} />
              </Dropdown>
            </div>
          </div>
        ),
      },
      {
        title: t('pages.xray.outbound.tag'),
        key: 'identity',
        align: 'left',
        render: (_v, record) => (
          <div className="identity-cell">
            <Tooltip title={record.tag}>
              <span className="tag-name">{record.tag}</span>
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
        ),
      },
      {
        title: t('pages.inbounds.address'),
        key: 'address',
        align: 'left',
        render: (_v, record) => {
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
        },
      },
      {
        title: t('pages.inbounds.traffic'),
        key: 'traffic',
        align: 'left',
        width: 200,
        render: (_v, record) => {
          const tr = trafficFor(outboundsTraffic, record);
          return (
            <>
              <span className="traffic-up">↑ {SizeFormatter.sizeFormat(tr.up)}</span>
              <span className="traffic-sep" />
              <span className="traffic-down">↓ {SizeFormatter.sizeFormat(tr.down)}</span>
            </>
          );
        },
      },
      {
        title: t('pages.nodes.latency'),
        key: 'testResult',
        align: 'left',
        width: 140,
        render: (_v, _record, index) => {
          const r = testResult(outboundTestStates, index);
          if (!r) return isTesting(outboundTestStates, index) ? <LoadingOutlined /> : <span className="empty">—</span>;
          return <TestResultPopover result={r} />;
        },
      },
      {
        title: t('check'),
        key: 'test',
        align: 'center',
        width: 80,
        render: (_v, record, index) => (
          <Tooltip title={`${t('check')} (${(isUdpOutbound(record) ? 'http' : testMode).toUpperCase()})`}>
            <Button
              type="primary"
              shape="circle"
              loading={isTesting(outboundTestStates, index)}
              disabled={isUntestable(record, testMode) || isTesting(outboundTestStates, index)}
              icon={<ThunderboltOutlined />}
              onClick={() => onTest(index, testMode)}
            />
          </Tooltip>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, testMode, rows, outboundTestStates, outboundsTraffic],
  );
}
