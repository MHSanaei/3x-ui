import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { Button, Card, Col, ConfigProvider, Input, Layout, Modal, Result, Row, Spin, Statistic, Typography, message } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { useNodeMutations } from '@/api/queries/useNodeMutations';
import AppSidebar from '@/layouts/AppSidebar';
import NodeList from './NodeList';
import NodeFormModal from './NodeFormModal';
import { setMessageInstance } from '@/utils/messageBus';
import { HttpUtil } from '@/utils';
import type { PanelUpdateInfo } from '../index/PanelUpdateModal';

export default function NodesPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { nodes, loading, fetched, fetchError, refetch, totals } = useNodesQuery();
  const { create, update, remove, setEnable, testConnection, fetchFingerprint, fetchInbounds, probe, updatePanels } = useNodeMutations();

  const { data: latestVersion = '' } = useQuery({
    queryKey: ['server', 'panelUpdateInfo'],
    queryFn: async () => {
      const msg = await HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo');
      return msg?.obj?.latestVersion || '';
    },
    staleTime: 5 * 60 * 1000,
  });

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formNode, setFormNode] = useState<NodeRecord | null>(null);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [mtlsOpen, setMtlsOpen] = useState(false);
  const [trustCa, setTrustCa] = useState('');
  const [copyingCa, setCopyingCa] = useState(false);
  const [savingTrustCa, setSavingTrustCa] = useState(false);

  const onCopyNodeCa = useCallback(async () => {
    setCopyingCa(true);
    try {
      const msg = await HttpUtil.post<{ caCert: string }>('/panel/api/nodes/mtls/ca');
      const ca = msg?.obj?.caCert;
      if (msg?.success && ca) {
        await navigator.clipboard.writeText(ca);
        messageApi.success(t('pages.nodes.mtls.caCopied'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.mtls.caFailed'));
      }
    } catch {
      messageApi.error(t('pages.nodes.mtls.caFailed'));
    } finally {
      setCopyingCa(false);
    }
  }, [messageApi, t]);

  const onSaveTrustCa = useCallback(async () => {
    setSavingTrustCa(true);
    try {
      const msg = await HttpUtil.post('/panel/api/nodes/mtls/trustCA', { caCert: trustCa });
      if (msg?.success) {
        messageApi.success(t('pages.nodes.mtls.saved'));
        setMtlsOpen(false);
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } catch {
      messageApi.error(t('somethingWentWrong'));
    } finally {
      setSavingTrustCa(false);
    }
  }, [trustCa, messageApi, t]);

  const onAdd = useCallback(() => {
    setFormMode('add');
    setFormNode(null);
    setFormOpen(true);
  }, []);

  const onEdit = useCallback((node: NodeRecord) => {
    setFormMode('edit');
    setFormNode({ ...node });
    setFormOpen(true);
  }, []);

  const onSave = useCallback(async (payload: Partial<NodeRecord>) => {
    if (formMode === 'edit' && formNode?.id) {
      return update(formNode.id, payload);
    }
    return create(payload);
  }, [formMode, formNode, update, create]);

  const onDelete = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.deleteConfirmTitle', { name: node.name }),
      content: t('pages.nodes.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.toasts.deleted'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onProbe = useCallback(async (node: NodeRecord) => {
    const msg = await probe(node.id);
    if (msg?.success && msg.obj) {
      if (msg.obj.status === 'online') {
        // Even if xray is in error/stop on the node we still reached its panel API.
        messageApi.success(t('pages.nodes.connectionOk', { ms: msg.obj.latencyMs }));
      } else {
        messageApi.error(msg.obj.error || t('pages.nodes.toasts.probeFailed'));
      }
    }
    // Refresh the list so the new xrayState / xrayError (if any) appears immediately in the row.
    refetch();
  }, [probe, t, messageApi, refetch]);

  const onToggleEnable = useCallback(async (node: NodeRecord, next: boolean) => {
    await setEnable(node.id, next);
  }, [setEnable]);

  const runUpdate = useCallback(async (ids: number[]) => {
    const msg = await updatePanels(ids);
    if (!msg?.success) {
      messageApi.error(msg?.msg || t('somethingWentWrong'));
      return;
    }
    const results = msg.obj ?? [];
    const ok = results.filter((r) => r.ok).length;
    const failed = results.length - ok;
    if (failed === 0) {
      messageApi.success(t('pages.nodes.toasts.updateStarted'));
    } else {
      const firstError = results.find((r) => !r.ok)?.error ?? '';
      const base = t('pages.nodes.toasts.updateResult', { ok, failed });
      messageApi.warning(firstError ? `${base} — ${firstError}` : base);
    }
    setSelectedIds([]);
  }, [updatePanels, messageApi, t]);

  const onUpdateNode = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.updateConfirmTitle', { count: 1 }),
      content: t('pages.nodes.updateConfirmContent'),
      okText: t('update'),
      cancelText: t('cancel'),
      onOk: () => runUpdate([node.id]),
    });
  }, [modal, t, runUpdate]);

  const onUpdateSelected = useCallback(() => {
    const eligible = nodes
      .filter((n) => selectedIds.includes(n.id) && n.enable && n.status === 'online')
      .map((n) => n.id);
    if (eligible.length === 0) {
      messageApi.warning(t('pages.nodes.toasts.updateNoneEligible'));
      return;
    }
    modal.confirm({
      title: t('pages.nodes.updateConfirmTitle', { count: eligible.length }),
      content: t('pages.nodes.updateConfirmContent'),
      okText: t('update'),
      cancelText: t('cancel'),
      onOk: () => runUpdate(eligible),
    });
  }, [modal, t, nodes, selectedIds, runUpdate, messageApi]);

  const pageClass = useMemo(() => {
    const classes = ['nodes-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" loading={loading} onClick={() => refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.totalNodes')}
                            value={String(totals.total)}
                            prefix={<CloudServerOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.onlineNodes')}
                            value={String(totals.online)}
                            prefix={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.offlineNodes')}
                            value={String(totals.offline)}
                            prefix={<CloseCircleOutlined style={{ color: 'var(--ant-color-error)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.avgLatency')}
                            value={totals.avgLatency > 0 ? `${totals.avgLatency} ms` : '-'}
                            prefix={<ThunderboltOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <NodeList
                      nodes={nodes}
                      loading={loading}
                      isMobile={isMobile}
                      latestVersion={latestVersion}
                      selectedIds={selectedIds}
                      onSelectionChange={setSelectedIds}
                      onAdd={onAdd}
                      onMtls={() => setMtlsOpen(true)}
                      onEdit={onEdit}
                      onDelete={onDelete}
                      onProbe={onProbe}
                      onToggleEnable={onToggleEnable}
                      onUpdateNode={onUpdateNode}
                      onUpdateSelected={onUpdateSelected}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <NodeFormModal
          open={formOpen}
          mode={formMode}
          node={formNode}
          testConnection={testConnection}
          fetchFingerprint={fetchFingerprint}
          fetchInbounds={fetchInbounds}
          save={onSave}
          onOpenChange={setFormOpen}
        />

        <Modal
          open={mtlsOpen}
          title={t('pages.nodes.mtls.title')}
          footer={null}
          onCancel={() => setMtlsOpen(false)}
          destroyOnHidden
        >
          <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
            {t('pages.nodes.mtls.intro')}
          </Typography.Paragraph>
          <Button onClick={onCopyNodeCa} loading={copyingCa} style={{ marginBottom: 4 }}>
            {t('pages.nodes.mtls.copyCa')}
          </Button>
          <Typography.Paragraph type="secondary">
            {t('pages.nodes.mtls.copyCaHint')}
          </Typography.Paragraph>
          <Typography.Text strong>{t('pages.nodes.mtls.trustLabel')}</Typography.Text>
          <Input.TextArea
            rows={5}
            value={trustCa}
            onChange={(e) => setTrustCa(e.target.value)}
            placeholder={t('pages.nodes.mtls.trustPlaceholder')}
            style={{ marginTop: 4, fontFamily: 'monospace' }}
          />
          <Typography.Paragraph type="secondary" style={{ marginTop: 4 }}>
            {t('pages.nodes.mtls.trustHint')}
          </Typography.Paragraph>
          <Button type="primary" onClick={onSaveTrustCa} loading={savingTrustCa} block>
            {t('pages.nodes.mtls.save')}
          </Button>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
