import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Col,
  Dropdown,
  Modal,
  Popconfirm,
  Popover,
  Radio,
  Row,
  Space,
  Table,
  Tag,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  CloudOutlined,
  ApiOutlined,
  RetweetOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  VerticalAlignTopOutlined,
  ThunderboltOutlined,
  CheckCircleFilled,
  CloseCircleFilled,
  LoadingOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import { SizeFormatter } from '@/utils';
import { OutboundProtocols as Protocols } from '@/schemas/primitives';
import OutboundFormModal from './OutboundFormModal';
import { isUdpOutbound } from '@/hooks/useXraySetting';
import type { XraySettingsValue, SetTemplate, OutboundTestState, OutboundTrafficRow } from '@/hooks/useXraySetting';
import './OutboundsTab.css';

interface OutboundsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  testingAll: boolean;
  inboundTags: string[];
  isMobile: boolean;
  onResetTraffic: (tag: string) => void;
  onTest: (index: number, mode: string) => void;
  onTestAll: (mode: string) => void;
  onShowWarp: () => void;
  onShowNord: () => void;
}

interface OutboundRow {
  key: number;
  tag?: string;
  protocol?: string;
  streamSettings?: { network?: string; security?: string };
  settings?: Record<string, unknown>;
}

function outboundAddresses(o: OutboundRow): string[] {
  const settings = o.settings as Record<string, unknown> | undefined;
  switch (o.protocol) {
    case Protocols.VMess: {
      const serverObj = settings?.vnext as Array<{ address: string; port: number }> | undefined;
      return serverObj ? serverObj.map((s) => `${s.address}:${s.port}`) : [];
    }
    case Protocols.VLESS:
      return [`${settings?.address || ''}:${settings?.port || ''}`];
    case Protocols.HTTP:
    case Protocols.Socks:
    case Protocols.Shadowsocks:
    case Protocols.Trojan: {
      const serverObj = settings?.servers as Array<{ address: string; port: number }> | undefined;
      return serverObj ? serverObj.map((s) => `${s.address}:${s.port}`) : [];
    }
    case Protocols.DNS: {
      const addr = (settings?.rewriteAddress as string) || (settings?.address as string) || '';
      const port = (settings?.rewritePort as string | number) || (settings?.port as string | number) || '';
      return addr || port ? [`${addr}:${port}`] : [];
    }
    case Protocols.Wireguard:
      return (((settings?.peers as Array<{ endpoint?: string }>) || []).map((p) => p.endpoint || '').filter(Boolean));
    default:
      return [];
  }
}

function isUntestable(o: OutboundRow, mode: string): boolean {
  if (!o) return true;
  if (o.protocol === Protocols.Blackhole || o.protocol === Protocols.Loopback || o.tag === 'blocked') return true;
  if (mode === 'tcp' && (o.protocol === Protocols.Freedom || o.protocol === Protocols.DNS)) return true;
  return false;
}

function showSecurity(security?: string): boolean {
  return security === 'tls' || security === 'reality';
}

function hasBreakdown(r: { endpoints?: unknown[]; error?: string } | null | undefined): boolean {
  if (!r) return false;
  if (r.endpoints?.length) return true;
  return !!r.error;
}

export default function OutboundsTab({
  templateSettings,
  setTemplateSettings,
  outboundsTraffic,
  outboundTestStates,
  testingAll,
  inboundTags: _inboundTags,
  isMobile,
  onResetTraffic,
  onTest,
  onTestAll,
  onShowWarp,
  onShowNord,
}: OutboundsTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [testMode, setTestMode] = useState<'tcp' | 'http'>('tcp');
  const [modalOpen, setModalOpen] = useState(false);
  const [editingOutbound, setEditingOutbound] = useState<Record<string, unknown> | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [existingTags, setExistingTags] = useState<string[]>([]);

  const outbounds = useMemo(
    () => (templateSettings?.outbounds || []) as unknown as OutboundRow[],
    [templateSettings?.outbounds],
  );

  const rows = useMemo(() => outbounds.map((o, i) => ({ ...o, key: i })), [outbounds]);

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  function openAdd() {
    setEditingOutbound(null);
    setEditingIndex(null);
    setExistingTags((templateSettings?.outbounds || []).map((o) => o?.tag).filter((tg): tg is string => !!tg));
    setModalOpen(true);
  }
  function openEdit(idx: number) {
    setEditingOutbound((templateSettings?.outbounds || [])[idx] as Record<string, unknown>);
    setEditingIndex(idx);
    setExistingTags(
      (templateSettings?.outbounds || [])
        .filter((_, i) => i !== idx)
        .map((o) => o?.tag)
        .filter((tg): tg is string => !!tg),
    );
    setModalOpen(true);
  }
  function onConfirm(outbound: Record<string, unknown>) {
    mutate((tt) => {
      if (!Array.isArray(tt.outbounds)) tt.outbounds = [];
      if (editingIndex == null) {
        if (!outbound.tag) return;
        tt.outbounds.push(outbound as never);
      } else {
        tt.outbounds[editingIndex] = outbound as never;
      }
    });
    setModalOpen(false);
  }

  function confirmDelete(idx: number) {
    modal.confirm({
      title: `${t('delete')} ${t('pages.xray.Outbounds')} #${idx + 1}?`,
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: () => {
        mutate((tt) => {
          tt.outbounds?.splice(idx, 1);
        });
      },
    });
  }
  function setFirst(idx: number) {
    mutate((tt) => {
      if (!tt.outbounds) return;
      const [moved] = tt.outbounds.splice(idx, 1);
      tt.outbounds.unshift(moved);
    });
  }
  function moveUp(idx: number) {
    if (idx <= 0) return;
    mutate((tt) => {
      if (!tt.outbounds) return;
      [tt.outbounds[idx - 1], tt.outbounds[idx]] = [tt.outbounds[idx], tt.outbounds[idx - 1]];
    });
  }
  function moveDown(idx: number) {
    mutate((tt) => {
      if (!tt.outbounds || idx >= tt.outbounds.length - 1) return;
      [tt.outbounds[idx + 1], tt.outbounds[idx]] = [tt.outbounds[idx], tt.outbounds[idx + 1]];
    });
  }

  function trafficFor(o: OutboundRow): { up: number; down: number } {
    const tr = outboundsTraffic.find((x) => x.tag === o.tag);
    return { up: tr?.up || 0, down: tr?.down || 0 };
  }
  function isTesting(idx: number): boolean {
    return !!outboundTestStates?.[idx]?.testing;
  }
  function testResult(idx: number) {
    return outboundTestStates?.[idx]?.result || null;
  }

  const columns: ColumnsType<OutboundRow> = useMemo(
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
          const tr = trafficFor(record);
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
          const r = testResult(index);
          if (!r) return isTesting(index) ? <LoadingOutlined /> : <span className="empty">—</span>;
          return (
            <Popover
              placement="topLeft"
              rootClassName="outbound-test-popover"
              content={
                <div className="timing-breakdown">
                  <div className={`td-head ${r.success ? 'ok' : 'fail'}`}>
                    {r.success ? <span>{r.delay} ms</span> : <span>{r.error || 'failed'}</span>}
                    {r.mode && <span className="mode-badge">{String(r.mode).toUpperCase()}</span>}
                  </div>
                  {hasBreakdown(r) && (
                    <>
                      {(r.endpoints || []).map((ep) => (
                        <div key={ep.address} className="endpoint-row">
                          <span className={ep.success ? 'dot-ok' : 'dot-fail'}>●</span>
                          <span className="ep-addr">{ep.address}</span>
                          <span className="ep-meta">{ep.success ? `${ep.delay} ms` : ep.error || 'failed'}</span>
                        </div>
                      ))}
                    </>
                  )}
                </div>
              }
            >
              <span className={r.success ? 'pill-ok' : 'pill-fail'}>
                {r.success ? <CheckCircleFilled /> : <CloseCircleFilled />}
                {r.success ? <span>{r.delay}&nbsp;ms</span> : <span>failed</span>}
              </span>
            </Popover>
          );
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
              loading={isTesting(index)}
              disabled={isUntestable(record, testMode) || isTesting(index)}
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

  return (
    <>
      {modalContextHolder}
      <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
        <Row gutter={[12, 12]} align="middle" justify="space-between">
          <Col xs={24} sm={12}>
            <Space size="small" wrap>
              <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
                {!isMobile && t('pages.xray.Outbounds')}
              </Button>
              <Button type="primary" icon={<CloudOutlined />} onClick={onShowWarp}>
                WARP
              </Button>
              <Button type="primary" icon={<ApiOutlined />} onClick={onShowNord}>
                NordVPN
              </Button>
            </Space>
          </Col>
          <Col xs={24} sm={12} className="toolbar-right">
            <Space size="small" wrap>
              <Tooltip title={t('pages.xray.outbound.testModeTooltip')}>
                <Radio.Group value={testMode} onChange={(e) => setTestMode(e.target.value)} buttonStyle="solid" size="small">
                  <Radio.Button value="tcp">TCP</Radio.Button>
                  <Radio.Button value="http">HTTP</Radio.Button>
                </Radio.Group>
              </Tooltip>
              <Button type="primary" loading={testingAll} icon={<PlayCircleOutlined />} onClick={() => onTestAll(testMode)}>
                {!isMobile && t('pages.xray.outbound.testAll')}
              </Button>
              <Popconfirm
                placement="topRight"
                okText={t('reset')}
                cancelText={t('cancel')}
                title={t('pages.inbounds.resetAllTrafficContent')}
                onConfirm={() => onResetTraffic('-alltags-')}
              >
                <Button icon={<RetweetOutlined />} />
              </Popconfirm>
            </Space>
          </Col>
        </Row>

        {isMobile ? (
          rows.length === 0 ? (
            <div className="card-empty">—</div>
          ) : (
            rows.map((record, index) => (
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
                  <span className="traffic-up">↑ {SizeFormatter.sizeFormat(trafficFor(record).up)}</span>
                  <span className="traffic-sep" />
                  <span className="traffic-down">↓ {SizeFormatter.sizeFormat(trafficFor(record).down)}</span>
                  <span className="card-test">
                    {testResult(index) ? (
                      <span className={testResult(index)!.success ? 'pill-ok' : 'pill-fail'}>
                        {testResult(index)!.success ? <CheckCircleFilled /> : <CloseCircleFilled />}
                        {testResult(index)!.success ? <span>{testResult(index)!.delay}&nbsp;ms</span> : <span>failed</span>}
                      </span>
                    ) : isTesting(index) ? (
                      <LoadingOutlined />
                    ) : null}
                    <Button
                      type="primary"
                      shape="circle"
                      size="small"
                      loading={isTesting(index)}
                      disabled={isUntestable(record, testMode) || isTesting(index)}
                      icon={<ThunderboltOutlined />}
                      onClick={() => onTest(index, testMode)}
                    />
                  </span>
                </div>
              </div>
            ))
          )
        ) : (
          <Table
            columns={columns}
            dataSource={rows}
            rowKey={(r) => r.key}
            pagination={false}
            size="small"
          />
        )}

        <OutboundFormModal
          open={modalOpen}
          outbound={editingOutbound}
          existingTags={existingTags}
          onClose={() => setModalOpen(false)}
          onConfirm={onConfirm}
        />
      </Space>
    </>
  );
}
