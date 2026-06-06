import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Col,
  Drawer,
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

interface OutboundsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundsTraffic: OutboundTrafficRow[];
  outboundTestStates: Record<number, OutboundTestState>;
  testingAll: boolean;
  inboundTags: string[];
  subscriptionOutbounds?: unknown[];
  isMobile: boolean;
  onResetTraffic: (tag: string) => void;
  onTest: (index: number, mode: string) => void;
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
  testingAll,
  inboundTags: _inboundTags,
  subscriptionOutbounds,
  isMobile,
  onResetTraffic,
  onTest,
  onTestAll,
  onShowWarp,
  onShowNord,
  onRefreshXrayData,
}: OutboundsTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [testMode, setTestMode] = useState<'tcp' | 'http'>('tcp');
  const [modalOpen, setModalOpen] = useState(false);
  const [editingOutbound, setEditingOutbound] = useState<Record<string, unknown> | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [existingTags, setExistingTags] = useState<string[]>([]);

  // Subscription manager (simple drawer for CRUD + manual refresh)
  const [subDrawerOpen, setSubDrawerOpen] = useState(false);
  const [subs, setSubs] = useState<any[]>([]);
  const [subsLoading, setSubsLoading] = useState(false);
  const [newSub, setNewSub] = useState({ remark: '', url: '', tagPrefix: '', updateInterval: 600, enabled: true });

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
      const r = await HttpUtil.get('/panel/xray/outbound-subs');
      if (r?.success) setSubs(Array.isArray(r.obj) ? r.obj : []);
    } catch (e) {
      message.error('Failed to load subscriptions');
    } finally {
      setSubsLoading(false);
    }
  }
  async function createSub() {
    if (!newSub.url.trim()) {
      message.warning('URL is required');
      return;
    }
    try {
      const body = new URLSearchParams();
      body.set('remark', newSub.remark);
      body.set('url', newSub.url);
      body.set('tagPrefix', newSub.tagPrefix);
      body.set('updateInterval', String(newSub.updateInterval));
      body.set('enabled', newSub.enabled ? 'true' : 'false');
      const r = await HttpUtil.post('/panel/xray/outbound-subs', body, { headers: { 'Content-Type': 'application/x-www-form-urlencoded' } });
      if (r?.success) {
        message.success('Subscription added');
        const createdId = r.obj?.id;
        setNewSub({ remark: '', url: '', tagPrefix: '', updateInterval: 600, enabled: true });
        await loadSubs();
        if (createdId) {
          // First fetch so the user immediately sees the imported outbounds
          await refreshOne(createdId);
        }
        onRefreshXrayData?.();
      } else {
        message.error(r?.msg || 'Failed to add');
      }
    } catch (e) {
      message.error('Failed to add subscription');
    }
  }
  async function refreshOne(id: number) {
    try {
      const r = await HttpUtil.post(`/panel/xray/outbound-subs/${id}/refresh`);
      if (r?.success) {
        message.success('Refreshed');
        await loadSubs();
        onRefreshXrayData?.();
      } else {
        message.error(r?.msg || 'Refresh failed');
      }
    } catch (e) {
      message.error('Refresh failed');
    }
  }
  async function deleteOne(id: number) {
    try {
      const r = await HttpUtil.post(`/panel/xray/outbound-subs/${id}/del`);
      if (r?.success) {
        message.success('Deleted');
        await loadSubs();
      }
    } catch (e) {
      message.error('Delete failed');
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
              <Button icon={<CloudOutlined />} onClick={openSubManager}>
                Subscriptions
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
          <div style={{ marginTop: 16 }}>
            <div style={{ fontWeight: 600, marginBottom: 6 }}>From outbound subscriptions (read-only)</div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {(subscriptionOutbounds as any[]).map((o, i) => (
                <span key={i} style={{ padding: '2px 8px', border: '1px solid #ddd', borderRadius: 4, fontSize: 12 }}>
                  {o?.tag || '(no tag)'} · {o?.protocol || 'unknown'}
                </span>
              ))}
            </div>
            <div style={{ fontSize: 12, opacity: 0.7, marginTop: 4 }}>
              These are injected from active subscriptions. Manage subscriptions via the API or future UI panel.
            </div>
          </div>
        )}
      </Space>

      <Drawer
        title="Outbound Subscriptions"
        open={subDrawerOpen}
        onClose={() => setSubDrawerOpen(false)}
        width={isMobile ? '100%' : 520}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <div>
            <div style={{ fontWeight: 600, marginBottom: 8 }}>Add subscription</div>
            <Form layout="vertical" size="small">
              <Form.Item label="Remark (optional)">
                <Input value={newSub.remark} onChange={(e) => setNewSub({ ...newSub, remark: e.target.value })} placeholder="e.g. HK nodes" />
              </Form.Item>
              <Form.Item label="Subscription URL" required>
                <Input value={newSub.url} onChange={(e) => setNewSub({ ...newSub, url: e.target.value })} placeholder="https://... (base64 list of links)" />
              </Form.Item>
              <Form.Item label="Tag prefix">
                <Input value={newSub.tagPrefix} onChange={(e) => setNewSub({ ...newSub, tagPrefix: e.target.value })} placeholder="hk-" />
              </Form.Item>
              <Form.Item label="Update interval">
                <Space>
                  <InputNumber
                    min={0}
                    value={intervalHours}
                    onChange={(v) => setIntervalHM(Number(v) || 0, intervalMinutes)}
                    style={{ width: 80 }}
                  /> h
                  <InputNumber
                    min={0}
                    max={59}
                    value={intervalMinutes}
                    onChange={(v) => setIntervalHM(intervalHours, Number(v) || 0)}
                    style={{ width: 80 }}
                  /> min
                </Space>
                <div style={{ fontSize: 12, color: '#888', marginTop: 4 }}>
                  Default 10 minutes. The background job checks frequently; each subscription only re-fetches when its own interval has passed.
                </div>
              </Form.Item>
              <Form.Item label="Enabled">
                <Switch checked={newSub.enabled} onChange={(v) => setNewSub({ ...newSub, enabled: v })} />
              </Form.Item>
              <Button type="primary" onClick={createSub} icon={<PlusOutlined />}>Add</Button>
            </Form>
          </div>

          <div>
            <div style={{ fontWeight: 600, marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
              Active subscriptions
              <Button size="small" icon={<ReloadOutlined />} onClick={loadSubs} loading={subsLoading} />
            </div>
            {subs.length === 0 ? (
              <div style={{ color: '#888' }}>No subscriptions yet. Add one above.</div>
            ) : (
              <Table
                size="small"
                dataSource={subs}
                rowKey={(r) => r.id}
                pagination={false}
                columns={[
                  { title: 'Remark', dataIndex: 'remark', key: 'remark' },
                  { title: 'Prefix', dataIndex: 'tagPrefix', key: 'tagPrefix', render: (v) => v || <em>auto</em> },
                  { title: 'Interval', dataIndex: 'updateInterval', key: 'updateInterval', render: (v) => `${Math.floor((v||0)/3600)}h ${Math.floor(((v||0)%3600)/60)}m` },
                  { title: 'Last fetch', dataIndex: 'lastUpdated', key: 'lastUpdated', render: (v: number) => v ? new Date(v * 1000).toLocaleString() : 'never' },
                  { title: 'Enabled', dataIndex: 'enabled', key: 'enabled', render: (v) => (v ? 'Yes' : 'No') },
                  {
                    title: '',
                    key: 'actions',
                    render: (_: any, r: any) => (
                      <Space>
                        <Button size="small" icon={<ReloadOutlined />} onClick={() => refreshOne(r.id)} title={r.lastError ? `Last error: ${r.lastError}` : 'Refresh now'} />
                        <Popconfirm title="Delete this subscription?" onConfirm={() => deleteOne(r.id)}>
                          <Button size="small" danger icon={<DeleteOutlined />} />
                        </Popconfirm>
                      </Space>
                    ),
                  },
                ]}
              />
            )}
            <div style={{ marginTop: 8, fontSize: 12, color: '#666' }}>
              After adding or refreshing, restart Xray (or wait for the next auto-reload) to make the outbounds active.
            </div>
          </div>
        </Space>
      </Drawer>
    </>
  );
}
