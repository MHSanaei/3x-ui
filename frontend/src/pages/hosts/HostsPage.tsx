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
import type { BulkAddHostValues } from '@/schemas/api/host';
import HostList, { sortHosts } from './HostList';
import HostFormModal from './HostFormModal';

export default function HostsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { hosts, loading, fetched, fetchError, refetch } = useHostsQuery();
  const { bulkCreate, update, remove, setEnable, reorder, bulkSetEnable, bulkDel } = useHostMutations();
  const { data: inboundOptions = [] } = useInboundOptions();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formHost, setFormHost] = useState<HostRecord | null>(null);
  const [selectedGroupIds, setSelectedGroupIds] = useState<string[]>([]);

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

  const onSave = useCallback(async (payload: BulkAddHostValues) => {
    if (formMode === 'edit' && formHost?.groupId) {
      return update(formHost.groupId, payload);
    }
    return bulkCreate(payload);
  }, [formMode, formHost, update, bulkCreate]);

  const onDelete = useCallback((host: HostRecord) => {
    modal.confirm({
      title: t('pages.hosts.deleteConfirmTitle', { name: host.remark }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(host.groupId);
        if (msg?.success) messageApi.success(t('pages.hosts.toasts.delete'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onToggleEnable = useCallback(async (host: HostRecord, next: boolean) => {
    await setEnable(host.groupId, next);
  }, [setEnable]);

  const onMove = useCallback(async (host: HostRecord, dir: 'up' | 'down') => {
    const sorted = sortHosts(hosts);
    const idx = sorted.findIndex((h) => h.groupId === host.groupId);
    const swapWith = dir === 'up' ? idx - 1 : idx + 1;
    if (idx < 0 || swapWith < 0 || swapWith >= sorted.length) return;
    const groupIds = sorted.map((h) => h.groupId);
    [groupIds[idx], groupIds[swapWith]] = [groupIds[swapWith], groupIds[idx]];
    await reorder(groupIds);
  }, [hosts, reorder]);

  const onBulkEnable = useCallback(async (enable: boolean) => {
    if (selectedGroupIds.length === 0) return;
    const msg = await bulkSetEnable(selectedGroupIds, enable);
    if (msg?.success) setSelectedGroupIds([]);
  }, [selectedGroupIds, bulkSetEnable]);

  const onBulkDelete = useCallback(() => {
    if (selectedGroupIds.length === 0) return;
    modal.confirm({
      title: t('pages.hosts.bulkDeleteConfirm', { count: selectedGroupIds.length }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await bulkDel(selectedGroupIds);
        if (msg?.success) {
          messageApi.success(t('pages.hosts.toasts.delete'));
          setSelectedGroupIds([]);
        }
      },
    });
  }, [selectedGroupIds, modal, t, bulkDel, messageApi]);

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
                      selectedGroupIds={selectedGroupIds}
                      onSelectionChange={setSelectedGroupIds}
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
          existingHosts={hosts}
          save={onSave}
          onOpenChange={setFormOpen}
        />
      </Layout>
    </ConfigProvider>
  );
}
