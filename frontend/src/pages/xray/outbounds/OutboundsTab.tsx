import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Col,
  Dropdown,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Radio,
  Row,
  Space,
  Switch,
  Table,
  Tooltip,
  message,
} from 'antd';
import {
  PlusOutlined,
  CloudOutlined,
  ApiOutlined,
  MoreOutlined,
  RetweetOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  DeleteOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';

import OutboundFormModal from './OutboundFormModal';
import type { XraySettingsValue, SetTemplate, OutboundTestState, OutboundTrafficRow } from '@/hooks/useXraySetting';
import './OutboundsTab.css';

import type { OutboundRow } from './outbounds-tab-types';
import { useOutboundColumns } from './useOutboundColumns';
import OutboundCardList from './OutboundCardList';
import SubscriptionOutbounds from './SubscriptionOutbounds';

interface OutboundSub {
  id: number;
  remark?: string;
  url?: string;
  enabled?: boolean;
  allowPrivate?: boolean;
  tagPrefix?: string;
  updateInterval?: number;
  lastUpdated?: number;
  lastError?: string;
}

interface OutboundsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  subscriptionTestStates: Record<string, OutboundTestState>;
  testingAll: boolean;
  inboundTags: string[];
  subscriptionOutbounds?: unknown[];
  isMobile: boolean;
  onResetTraffic: (tag: string) => void;
  onTest: (index: number, mode: string) => void;
  onTestSubscription: (outbound: Record<string, unknown>, mode: string) => void;
  onTestAll: (mode: string) => void;
  onShowWarp: () => void;
  onShowNord: () => void;
  onRefreshXrayData?: () => void;
}

export default function OutboundsTab({
  templateSettings,
  setTemplateSettings,
  outboundsTraffic,
  outboundTestStates,
  subscriptionTestStates,
  testingAll,
  inboundTags: _inboundTags,
  subscriptionOutbounds,
  isMobile,
  onResetTraffic,
  onTest,
  onTestSubscription,
  onTestAll,
  onShowWarp,
  onShowNord,
  onRefreshXrayData,
}: OutboundsTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [testMode, setTestMode] = useState<'tcp' | 'http'>('tcp');
  const [modalOpen, setModalOpen] = useState(false);
  const [editingOutbound, setEditingOutbound] = useState<Record<string, unknown> | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [existingTags, setExistingTags] = useState<string[]>([]);

  // Subscription manager (simple drawer for CRUD + manual refresh)
  const [subDrawerOpen, setSubDrawerOpen] = useState(false);
  const [subs, setSubs] = useState<OutboundSub[]>([]);
  const [subsLoading, setSubsLoading] = useState(false);
  const [newSub, setNewSub] = useState({ remark: '', url: '', tagPrefix: '', updateInterval: 600, enabled: true, allowPrivate: false });

  // Convenience: expose hours/minutes for the interval input
  const intervalHours = Math.floor((newSub.updateInterval || 600) / 3600);
  const intervalMinutes = Math.floor(((newSub.updateInterval || 600) % 3600) / 60);
  function setIntervalHM(h: number, m: number) {
    const secs = Math.max(60, (h || 0) * 3600 + (m || 0) * 60);
    setNewSub((prev) => ({ ...prev, updateInterval: secs }));
  }

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

  function openSubManager() {
    setSubDrawerOpen(true);
    loadSubs();
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

  // --- Subscription management (minimal inline UI) ---
  async function loadSubs() {
    setSubsLoading(true);
    try {
      const r = await HttpUtil.get('/panel/api/xray/outbound-subs');
      if (r?.success) setSubs(Array.isArray(r.obj) ? r.obj : []);
    } catch {
      messageApi.error(t('pages.xray.outboundSub.toastLoadFailed'));
    } finally {
      setSubsLoading(false);
    }
  }
  async function createSub() {
    if (!newSub.url.trim()) {
      messageApi.warning(t('pages.xray.outboundSub.toastUrlRequired'));
      return;
    }
    try {
      const r = await HttpUtil.post<OutboundSub>('/panel/api/xray/outbound-subs', {
        remark: newSub.remark,
        url: newSub.url,
        tagPrefix: newSub.tagPrefix,
        updateInterval: newSub.updateInterval,
        enabled: newSub.enabled,
        allowPrivate: newSub.allowPrivate,
      });
      if (r?.success) {
        messageApi.success(t('pages.xray.outboundSub.toastAdded'));
        const createdId = r.obj?.id;
        setNewSub({ remark: '', url: '', tagPrefix: '', updateInterval: 600, enabled: true, allowPrivate: false });
        await loadSubs();
        if (createdId) {
          // First fetch so the user immediately sees the imported outbounds
          await refreshOne(createdId);
        }
        onRefreshXrayData?.();
      } else {
        messageApi.error(r?.msg || t('pages.xray.outboundSub.toastAddFailed'));
      }
    } catch {
      messageApi.error(t('pages.xray.outboundSub.toastAddFailed'));
    }
  }
  async function refreshOne(id: number) {
    try {
      const r = await HttpUtil.post(`/panel/api/xray/outbound-subs/${id}/refresh`);
      if (r?.success) {
        messageApi.success(t('pages.xray.outboundSub.toastRefreshed'));
        await loadSubs();
        onRefreshXrayData?.();
      } else {
        messageApi.error(r?.msg || t('pages.xray.outboundSub.toastRefreshFailed'));
      }
    } catch {
      messageApi.error(t('pages.xray.outboundSub.toastRefreshFailed'));
    }
  }
  async function deleteOne(id: number) {
    try {
      const r = await HttpUtil.post(`/panel/api/xray/outbound-subs/${id}/del`);
      if (r?.success) {
        messageApi.success(t('pages.xray.outboundSub.toastDeleted'));
        await loadSubs();
        onRefreshXrayData?.();
      }
    } catch {
      messageApi.error(t('pages.xray.outboundSub.toastDeleteFailed'));
    }
  }

  const columns = useOutboundColumns({
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
  });

  return (
    <>
      {modalContextHolder}
      {messageContextHolder}
      <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
        <Row gutter={[12, 12]} align="middle" justify="space-between">
          <Col xs={24} sm={12}>
            <Space size="small" wrap>
              <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
                {!isMobile && t('pages.xray.Outbounds')}
              </Button>
              <Button icon={<CloudOutlined />} onClick={openSubManager}>
                {t('pages.xray.outboundSub.manage')}
              </Button>
              <Dropdown
                trigger={['click']}
                menu={{
                  items: [
                    { key: 'warp', icon: <CloudOutlined />, label: 'WARP', onClick: onShowWarp },
                    { key: 'nord', icon: <ApiOutlined />, label: 'NordVPN', onClick: onShowNord },
                  ],
                }}
              >
                <Button icon={<MoreOutlined />}>{t('more')}</Button>
              </Dropdown>
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
          <OutboundCardList
            rows={rows}
            testMode={testMode}
            outboundsTraffic={outboundsTraffic}
            outboundTestStates={outboundTestStates}
            setFirst={setFirst}
            openEdit={openEdit}
            onResetTraffic={onResetTraffic}
            confirmDelete={confirmDelete}
            onTest={onTest}
          />
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

        {/* Subscription outbounds (read-only, merged at runtime) */}
        {Array.isArray(subscriptionOutbounds) && subscriptionOutbounds.length > 0 && (
          <SubscriptionOutbounds
            subscriptionOutbounds={subscriptionOutbounds}
            outboundsTraffic={outboundsTraffic}
            subscriptionTestStates={subscriptionTestStates}
            testMode={testMode}
            isMobile={isMobile}
            onTestSubscription={onTestSubscription}
          />
        )}
      </Space>

      <Modal
        title={t('pages.xray.outboundSub.title')}
        open={subDrawerOpen}
        onCancel={() => setSubDrawerOpen(false)}
        footer={null}
        width={isMobile ? '100%' : 640}
        destroyOnHidden
      >
        <Space orientation="vertical" style={{ width: '100%' }} size="large">
          <div>
            <Form layout="vertical" size="small">
              <Form.Item label={t('pages.xray.outboundSub.remark')}>
                <Input value={newSub.remark} onChange={(e) => setNewSub({ ...newSub, remark: e.target.value })} placeholder={t('pages.xray.outboundSub.remarkPlaceholder')} />
              </Form.Item>
              <Form.Item label={t('pages.xray.outboundSub.url')} required>
                <Input value={newSub.url} onChange={(e) => setNewSub({ ...newSub, url: e.target.value })} placeholder={t('pages.xray.outboundSub.urlPlaceholder')} />
              </Form.Item>
              <Form.Item label={t('pages.xray.outboundSub.tagPrefix')}>
                <Input value={newSub.tagPrefix} onChange={(e) => setNewSub({ ...newSub, tagPrefix: e.target.value })} placeholder={t('pages.xray.outboundSub.tagPrefixPlaceholder')} />
              </Form.Item>
              <Form.Item label={t('pages.xray.outboundSub.interval')}>
                <Space>
                  <InputNumber
                    min={0}
                    value={intervalHours}
                    onChange={(v) => setIntervalHM(Number(v) || 0, intervalMinutes)}
                    style={{ width: 80 }}
                  /> {t('pages.xray.outboundSub.hours')}
                  <InputNumber
                    min={0}
                    max={59}
                    value={intervalMinutes}
                    onChange={(v) => setIntervalHM(intervalHours, Number(v) || 0)}
                    style={{ width: 80 }}
                  /> {t('pages.xray.outboundSub.minutes')}
                </Space>
                <div style={{ fontSize: 12, color: '#888', marginTop: 4 }}>
                  {t('pages.xray.outboundSub.intervalHint')}
                </div>
              </Form.Item>
              <Form.Item label={t('pages.xray.outboundSub.enabled')}>
                <Switch checked={newSub.enabled} onChange={(v) => setNewSub({ ...newSub, enabled: v })} />
              </Form.Item>
              <Form.Item label={t('pages.xray.outboundSub.allowPrivate')}>
                <Switch checked={newSub.allowPrivate} onChange={(v) => setNewSub({ ...newSub, allowPrivate: v })} />
                <div style={{ fontSize: 12, color: '#888', marginTop: 4 }}>
                  {t('pages.xray.outboundSub.allowPrivateHint')}
                </div>
              </Form.Item>
              <Button type="primary" onClick={createSub} icon={<PlusOutlined />}>{t('pages.xray.outboundSub.addButton')}</Button>
            </Form>
          </div>

          <div>
            <div style={{ fontWeight: 600, marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
              {t('pages.xray.outboundSub.active')}
              <Button size="small" icon={<ReloadOutlined />} onClick={loadSubs} loading={subsLoading} />
            </div>
            {subs.length === 0 ? (
              <div style={{ color: '#888' }}>{t('pages.xray.outboundSub.empty')}</div>
            ) : (
              <Table
                size="small"
                dataSource={subs}
                rowKey={(r) => r.id}
                pagination={false}
                columns={[
                  { title: t('pages.xray.outboundSub.colRemark'), dataIndex: 'remark', key: 'remark' },
                  { title: t('pages.xray.outboundSub.colPrefix'), dataIndex: 'tagPrefix', key: 'tagPrefix', render: (v) => v || <em>{t('pages.xray.outboundSub.auto')}</em> },
                  { title: t('pages.xray.outboundSub.colInterval'), dataIndex: 'updateInterval', key: 'updateInterval', render: (v) => `${Math.floor((v || 0) / 3600)}h ${Math.floor(((v || 0) % 3600) / 60)}m` },
                  { title: t('pages.xray.outboundSub.colLastFetch'), dataIndex: 'lastUpdated', key: 'lastUpdated', render: (v: number) => v ? new Date(v * 1000).toLocaleString() : t('pages.xray.outboundSub.never') },
                  { title: t('pages.xray.outboundSub.colEnabled'), dataIndex: 'enabled', key: 'enabled', render: (v) => (v ? t('pages.xray.outboundSub.yes') : t('pages.xray.outboundSub.no')) },
                  {
                    title: '',
                    key: 'actions',
                    render: (_: unknown, r: OutboundSub) => (
                      <Space>
                        <Button size="small" icon={<ReloadOutlined />} onClick={() => refreshOne(r.id)} title={r.lastError ? `${t('pages.xray.outboundSub.lastError')}: ${r.lastError}` : t('pages.xray.outboundSub.refreshNow')} />
                        <Popconfirm title={t('pages.xray.outboundSub.deleteConfirm')} okText={t('delete')} cancelText={t('cancel')} onConfirm={() => deleteOne(r.id)}>
                          <Button size="small" danger icon={<DeleteOutlined />} />
                        </Popconfirm>
                      </Space>
                    ),
                  },
                ]}
              />
            )}
            <div style={{ marginTop: 8, fontSize: 12, color: '#666' }}>
              {t('pages.xray.outboundSub.restartHint')}
            </div>
          </div>
        </Space>
      </Modal>
    </>
  );
}
