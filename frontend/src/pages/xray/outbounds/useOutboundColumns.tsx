import { useMemo, useState } from 'react';
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
  EyeInvisibleOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import { SizeFormatter } from '@/utils';
import { activateOnKey } from '@/utils/a11y';
import { OutboundProtocols as Protocols } from '@/schemas/primitives';
import type { OutboundTestMode, OutboundTestState, OutboundTrafficRow } from '@/hooks/useXraySetting';

import type { OutboundRow } from './outbounds-tab-types';
import CountryPill from './CountryPill';
import TestResultPopover from './TestResultPopover';
import {
  effectiveTestMode,
  countryFlag,
  countryName,
  isTesting,
  isUntestable,
  outboundAddresses,
  showSecurity,
  testModeLabel,
  testResult,
  trafficFor,
} from './outbounds-tab-helpers';

interface OutboundColumnsParams {
  testMode: OutboundTestMode;
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
  const { t, i18n } = useTranslation();
  const [showEgressIp, setShowEgressIp] = useState(false);
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
              <Button shape="circle" size="small" icon={<EditOutlined />} aria-label={t('edit')} onClick={() => openEdit(index)} />
              <Dropdown
                trigger={['click']}
                menu={{
                  items: [
                    ...(index > 0
                      ? [
                          { key: 'top', label: <><VerticalAlignTopOutlined /> {t('pages.xray.outbound.moveToTop')}</>, onClick: () => setFirst(index) },
                        ]
                      : []),
                    { key: 'up', label: <><ArrowUpOutlined /> {t('pages.inbounds.form.moveUp')}</>, disabled: index === 0, onClick: () => moveUp(index) },
                    { key: 'down', label: <><ArrowDownOutlined /> {t('pages.inbounds.form.moveDown')}</>, disabled: index === rows.length - 1, onClick: () => moveDown(index) },
                    { key: 'reset', label: <><RetweetOutlined /> {t('pages.inbounds.resetTraffic')}</>, onClick: () => onResetTraffic(rows[index].tag || '') },
                    { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                  ],
                }}
              >
                <Button shape="circle" size="small" icon={<MoreOutlined />} aria-label={t('more')} />
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
        title: (
          <span className="egress-header">
            {t('pages.xray.outbound.egress')}
            <Tooltip title={t('pages.index.toggleIpVisibility')}>
              {showEgressIp ? (
                <EyeOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setShowEgressIp(false)} onKeyDown={activateOnKey(() => setShowEgressIp(false))} />
              ) : (
                <EyeInvisibleOutlined className="ip-toggle-icon" role="button" tabIndex={0} aria-label={t('pages.index.toggleIpVisibility')} onClick={() => setShowEgressIp(true)} onKeyDown={activateOnKey(() => setShowEgressIp(true))} />
              )}
            </Tooltip>
          </span>
        ),
        key: 'egress',
        align: 'left',
        width: 210,
        render: (_v, record) => {
          const egress = testResult(outboundTestStates, record.key)?.egress;
          const addresses = [
            egress?.ipv4 ? { label: 'v4', value: egress.ipv4 } : null,
            egress?.ipv6 ? { label: 'v6', value: egress.ipv6 } : null,
          ].filter((item): item is { label: string; value: string } => Boolean(item));
          if (addresses.length === 0) {
            return (
              <Tooltip title={t('pages.xray.outbound.egressHint')}>
                <span className="empty">—</span>
              </Tooltip>
            );
          }
          return (
            <div className="egress-stack">
              {addresses.map((addr) => (
                <Tooltip key={addr.label} title={addr.value}>
                  <span className="egress-address">
                    <span className="egress-family">{addr.label}</span>
                    <span className={showEgressIp ? 'address-visible egress-ip' : 'address-hidden egress-ip'}>{addr.value}</span>
                  </span>
                </Tooltip>
              ))}
            </div>
          );
        },
      },
      {
        title: t('pages.xray.outbound.country'),
        key: 'egressCountry',
        align: 'left',
        width: 160,
        render: (_v, record) => {
          const egress = testResult(outboundTestStates, record.key)?.egress;
          if (!egress?.country) {
            return (
              <Tooltip title={t('pages.xray.outbound.egressHint')}>
                <span className="empty">—</span>
              </Tooltip>
            );
          }
          const flag = countryFlag(egress.country);
          const name = countryName(egress.country, i18n.language);
          return (
            <Tooltip title={egress.warp ? `Cloudflare trace · WARP ${egress.warp}` : 'Cloudflare trace'}>
              <CountryPill flag={flag} name={name || egress.country} warp={egress.warp} />
            </Tooltip>
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
        render: (_v, record) => {
          const r = testResult(outboundTestStates, record.key);
          if (!r) return isTesting(outboundTestStates, record.key) ? <LoadingOutlined /> : <span className="empty">—</span>;
          return <TestResultPopover result={r} />;
        },
      },
      {
        title: t('check'),
        key: 'test',
        align: 'center',
        width: 80,
        render: (_v, record) => (
          <Tooltip title={`${t('check')} (${testModeLabel(effectiveTestMode(record, testMode), t)})`}>
            <Button
              type="primary"
              shape="circle"
              loading={isTesting(outboundTestStates, record.key)}
              disabled={isUntestable(record) || isTesting(outboundTestStates, record.key)}
              icon={<ThunderboltOutlined />}
              aria-label={t('check')}
              onClick={() => onTest(record.key, testMode)}
            />
          </Tooltip>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, i18n.language, testMode, rows, outboundTestStates, outboundsTraffic, showEgressIp],
  );
}
