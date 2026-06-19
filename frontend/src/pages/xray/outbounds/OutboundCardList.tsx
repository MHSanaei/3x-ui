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
  ExportOutlined,
} from '@ant-design/icons';

import { SizeFormatter } from '@/utils';
import { OutboundProtocols as Protocols } from '@/schemas/primitives';
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

interface OutboundCardListProps {
  rows: OutboundRow[];
  testMode: 'tcp' | 'http';
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  setFirst: (idx: number) => void;
  openEdit: (idx: number) => void;
  onResetTraffic: (tag: string) => void;
  confirmDelete: (idx: number) => void;
  onTest: (index: number, mode: string) => void;
}

export default function OutboundCardList({
  rows,
  testMode,
  outboundsTraffic,
  outboundTestStates,
  setFirst,
  openEdit,
  onResetTraffic,
  confirmDelete,
  onTest,
}: OutboundCardListProps) {
  const { t } = useTranslation();
  if (rows.length === 0) {
    return (
      <div className="card-empty">
        <ExportOutlined style={{ fontSize: 32, marginBottom: 8 }} />
        <div>{t('noData')}</div>
      </div>
    );
  }
  return (
    <>
      {rows.map((record, index) => (
        <div key={record.key} className="outbound-card">
          <div className="card-head">
            <div className="card-identity">
              <span className="card-num">{index + 1}</span>
              <Tooltip title={record.tag}>
                <span className="tag-name">{record.tag}</span>
              </Tooltip>
              <Tag color="green">{record.protocol}</Tag>
              {[Protocols.VMess, Protocols.VLESS, Protocols.Trojan, Protocols.Shadowsocks].includes(record.protocol as never) && (
                <>
                  <Tag>{record.streamSettings?.network}</Tag>
                  {showSecurity(record.streamSettings?.security) && <Tag color="purple">{record.streamSettings?.security}</Tag>}
                </>
              )}
            </div>
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  ...(index > 0
                    ? [{ key: 'top', label: <VerticalAlignTopOutlined />, onClick: () => setFirst(index) }]
                    : []),
                  { key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEdit(index) },
                  { key: 'reset', label: <><RetweetOutlined /> {t('pages.inbounds.resetTraffic')}</>, onClick: () => onResetTraffic(record.tag || '') },
                  { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                ],
              }}
            >
              <Button shape="circle" size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </div>
          {outboundAddresses(record).length > 0 && (
            <div className="address-list">
              {outboundAddresses(record).map((addr) => (
                <Tooltip key={addr} title={addr}>
                  <span className="address-pill">{addr}</span>
                </Tooltip>
              ))}
            </div>
          )}
          <div className="card-foot">
            <span className="traffic-up">↑ {SizeFormatter.sizeFormat(trafficFor(outboundsTraffic, record).up)}</span>
            <span className="traffic-sep" />
            <span className="traffic-down">↓ {SizeFormatter.sizeFormat(trafficFor(outboundsTraffic, record).down)}</span>
            <span className="card-test">
              {testResult(outboundTestStates, index) ? (
                <TestResultPopover result={testResult(outboundTestStates, index)!} />
              ) : isTesting(outboundTestStates, index) ? (
                <LoadingOutlined />
              ) : null}
              <Button
                type="primary"
                shape="circle"
                size="small"
                loading={isTesting(outboundTestStates, index)}
                disabled={isUntestable(record) || isTesting(outboundTestStates, index)}
                icon={<ThunderboltOutlined />}
                onClick={() => onTest(index, testMode)}
              />
            </span>
          </div>
        </div>
      ))}
    </>
  );
}
