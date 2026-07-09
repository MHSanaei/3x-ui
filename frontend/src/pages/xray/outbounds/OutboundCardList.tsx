import { useState } from 'react';
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
  EyeInvisibleOutlined,
  EyeOutlined,
} from '@ant-design/icons';

import { SizeFormatter } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { OutboundProtocols as Protocols } from '@/schemas/primitives';
import type { OutboundTestMode, OutboundTestState, OutboundTrafficRow } from '@/hooks/useXraySetting';

import type { OutboundRow } from './outbounds-tab-types';
import CountryPill from './CountryPill';
import TestResultPopover from './TestResultPopover';
import {
  countryFlag,
  countryName,
  isTesting,
  isUntestable,
  outboundAddresses,
  showSecurity,
  testResult,
  trafficFor,
} from './outbounds-tab-helpers';

interface OutboundCardListProps {
  rows: OutboundRow[];
  testMode: OutboundTestMode;
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
  const { t, i18n } = useTranslation();
  const [showEgressIp, setShowEgressIp] = useState<Record<string, boolean>>({});

  const setCardEgressVisible = (key: string, visible: boolean) => {
    setShowEgressIp((prev) => ({ ...prev, [key]: visible }));
  };

  const renderEgress = (index: number, rowKey: string) => {
    const result = testResult(outboundTestStates, index);
    const egress = result?.egress;
    const isEgressVisible = !!showEgressIp[rowKey];
    const flag = countryFlag(egress?.country);
    const name = countryName(egress?.country, i18n.language);
    const addresses = [
      egress?.ipv4 ? { label: 'v4', value: egress.ipv4 } : null,
      egress?.ipv6 ? { label: 'v6', value: egress.ipv6 } : null,
    ].filter((item): item is { label: string; value: string } => Boolean(item));

    if (!egress || (addresses.length === 0 && !egress.country)) {
      return null;
    }

    return (
      <div className="card-egress">
        <div className="card-egress-row">
          <span>{t('pages.xray.outbound.egress')}:</span>
          <Tooltip title={t('pages.index.toggleIpVisibility')}>
            {isEgressVisible ? (
              <EyeOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setCardEgressVisible(rowKey, false)} onKeyDown={activateOnKey(() => setCardEgressVisible(rowKey, false))} />
            ) : (
              <EyeInvisibleOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setCardEgressVisible(rowKey, true)} onKeyDown={activateOnKey(() => setCardEgressVisible(rowKey, true))} />
            )}
          </Tooltip>
          {egress.country && (
            <CountryPill flag={flag} name={name || egress.country} warp={egress.warp} />
          )}
        </div>
        {addresses.map((addr) => (
          <Tooltip key={addr.label} title={addr.value}>
            <div className="card-egress-row">
              <span className="egress-family">{addr.label}:</span>
              <span className={isEgressVisible ? 'address-visible egress-ip' : 'address-hidden egress-ip'}>{addr.value}</span>
            </div>
          </Tooltip>
        ))}
      </div>
    );
  };

  if (rows.length === 0) {
    return (
      <div className="card-empty">
        <ExportOutlined style={{ fontSize: 32, marginBottom: 8 }} aria-hidden="true" />
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
                    ? [{ key: 'top', label: <><VerticalAlignTopOutlined /> {t('pages.xray.outbound.moveToTop')}</>, onClick: () => setFirst(index) }]
                    : []),
                  { key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEdit(index) },
                  { key: 'reset', label: <><RetweetOutlined /> {t('pages.inbounds.resetTraffic')}</>, onClick: () => onResetTraffic(record.tag || '') },
                  { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                ],
              }}
            >
              <Button shape="circle" size="small" icon={<MoreOutlined />} aria-label={t('more')} />
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
          {renderEgress(index, String(record.key))}
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
                aria-label={t('check')}
                onClick={() => onTest(index, testMode)}
              />
            </span>
          </div>
        </div>
      ))}
    </>
  );
}
