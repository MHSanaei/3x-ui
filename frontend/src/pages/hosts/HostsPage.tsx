import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Col, ConfigProvider, Layout, Modal, Result, Row, Spin, Statistic, message } from 'antd';
import { CheckCircleOutlined, GlobalOutlined, StopOutlined } from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useHostsQuery, type HostRecord } from '@/api/queries/useHostsQuery';
import { useHostMutations } from '@/api/queries/useHostMutations';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import AppSidebar from '@/layouts/AppSidebar';
import { setMessageInstance } from '@/utils/messageBus';
import type { HostFormValues } from '@/schemas/api/host';
import HostList from './HostList';
import HostFormModal from './HostFormModal';

// Hosts for one inbound in render order — used to compute a reorder payload.
function inboundHostsInOrder(hosts: HostRecord[], inboundId: number): HostRecord[] {
  return hosts
    .filter((h) => h.inboundId === inboundId)
    .sort((a, b) => {
      const sa = a.sortOrder ?? 0;
      const sb = b.sortOrder ?? 0;
      if (sa !== sb) return sa - sb;
      return a.id - b.id;
    });
}

export default function HostsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { hosts, loading, fetched, fetchError, refetch } = useHostsQuery();
  const { create, update, remove, setEnable, reorder, bulkSetEnable, bulkDel } = useHostMutations();
  const { data: inboundOptions = [] } = useInboundOptions();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formHost, setFormHost] = useState<HostRecord | null>(null);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);

  const onAdd = useCallback(() => {
    setFormMode('add');
    setFormHost(null);
    setFormOpen(true);
  }, []);

  const onEdit = useCallback((host: HostRecord) => {
    setFormMode('edit');
    setFormHost({ ...host });
    setFormOpen(true);
  }, []);

  const onSave = useCallback(async (payload: Partial<HostFormValues>) => {
    if (formMode === 'edit' && formHost?.id) {
      return update(formHost.id, payload);
    }
    return create(payload);
  }, [formMode, formHost, update, create]);

  const onDelete = useCallback((host: HostRecord) => {
    modal.confirm({
      title: t('pages.hosts.deleteConfirmTitle', { name: host.remark }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(host.id);
        if (msg?.success) messageApi.success(t('pages.hosts.toasts.delete'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onToggleEnable = useCallback(async (host: HostRecord, next: boolean) => {
    await setEnable(host.id, next);
  }, [setEnable]);

  const onMove = useCallback(async (host: HostRecord, dir: 'up' | 'down') => {
    const group = inboundHostsInOrder(hosts, host.inboundId);
    const idx = group.findIndex((h) => h.id === host.id);
    const swapWith = dir === 'up' ? idx - 1 : idx + 1;
    if (idx < 0 || swapWith < 0 || swapWith >= group.length) return;
    const ids = group.map((h) => h.id);
    [ids[idx], ids[swapWith]] = [ids[swapWith], ids[idx]];
    await reorder(ids);
  }, [hosts, reorder]);

  const onBulkEnable = useCallback(async (enable: boolean) => {
    if (selectedIds.length === 0) return;
    const msg = await bulkSetEnable(selectedIds, enable);
    if (msg?.success) setSelectedIds([]);
  }, [selectedIds, bulkSetEnable]);

  const onBulkDelete = useCallback(() => {
    if (selectedIds.length === 0) return;
    modal.confirm({
      title: t('pages.hosts.bulkDeleteConfirm', { count: selectedIds.length }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await bulkDel(selectedIds);
        if (msg?.success) {
          messageApi.success(t('pages.hosts.toasts.delete'));
          setSelectedIds([]);
        }
      },
    });
  }, [selectedIds, modal, t, bulkDel, messageApi]);

  const summary = useMemo(() => {
    const total = hosts.length;
    const enabled = hosts.filter((h) => !h.isDisabled).length;
    return { total, enabled, disabled: total - enabled };
  }, [hosts]);

  const pageClass = useMemo(() => {
    const classes = ['hosts-page'];
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
            <Spin spinning={!fetched} delay={200} size="large">
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
                      <Row gutter={[16, 12]}>
                        <Col xs={8} sm={8} md={8}>
                          <Statistic
                            title={t('pages.hosts.summary.total')}
                            value={String(summary.total)}
                            prefix={<GlobalOutlined />}
                          />
                        </Col>
                        <Col xs={8} sm={8} md={8}>
                          <Statistic
                            title={t('pages.hosts.summary.enabled')}
                            value={String(summary.enabled)}
                            prefix={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={8} sm={8} md={8}>
                          <Statistic
                            title={t('pages.hosts.summary.disabled')}
                            value={String(summary.disabled)}
                            prefix={<StopOutlined style={{ color: 'var(--ant-color-text-quaternary)' }} />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <HostList
                      hosts={hosts}
                      inboundOptions={inboundOptions}
                      loading={loading}
                      isMobile={isMobile}
                      selectedIds={selectedIds}
                      onSelectionChange={setSelectedIds}
                      onAdd={onAdd}
                      onEdit={onEdit}
                      onDelete={onDelete}
                      onToggleEnable={onToggleEnable}
                      onMove={onMove}
                      onBulkEnable={onBulkEnable}
                      onBulkDelete={onBulkDelete}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <HostFormModal
          open={formOpen}
          mode={formMode}
          host={formHost}
          inboundOptions={inboundOptions}
          save={onSave}
          onOpenChange={setFormOpen}
        />
      </Layout>
    </ConfigProvider>
  );
}
