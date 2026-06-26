import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { Button, Card, Col, ConfigProvider, Form, Layout, Modal, Result, Row, Select, Space, Spin, Statistic, Switch, Table, Tag, Tooltip, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  ApiOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  LinkOutlined,
  PlusOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { useLinkMutations } from '@/api/queries/useLinkMutations';
import { useLinksQuery, type ManagedLinkRecord } from '@/api/queries/useLinksQuery';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { ClientRecordSchema, type ClientRecord } from '@/schemas/client';
import type { ManagedLinkFormValues } from '@/schemas/api/link';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { parseMsg } from '@/utils/zodValidate';
import LinkFormModal from './LinkFormModal';
import '../hosts/HostList.css';

const ClientListSchema = ClientRecordSchema.array();

async function fetchAllClients(): Promise<ClientRecord[]> {
  const msg = await HttpUtil.get('/panel/api/clients/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch clients');
  const validated = parseMsg(msg, ClientListSchema, 'clients/list');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

function sortLinks(links: ManagedLinkRecord[]): ManagedLinkRecord[] {
  return [...links].sort((a, b) => {
    const sa = a.sortIndex ?? 0;
    const sb = b.sortIndex ?? 0;
    if (sa !== sb) return sa - sb;
    return a.id - b.id;
  });
}

function shortValue(value: string): string {
  if (!value) return '';
  if (value.length <= 96) return value;
  return `${value.slice(0, 72)}...${value.slice(-18)}`;
}

export default function LinksPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { links, loading, fetched, fetchError, refetch } = useLinksQuery();
  const { create, update, remove, setEnable, reorder, assign, bulkSetEnable, bulkDel } = useLinkMutations();
  const clientsQuery = useQuery({
    queryKey: keys.clients.all(),
    queryFn: fetchAllClients,
    staleTime: 5000,
  });

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formLink, setFormLink] = useState<ManagedLinkRecord | null>(null);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [assignOpen, setAssignOpen] = useState(false);
  const [assignForm] = Form.useForm<{ emails: string[] }>();
  const [assigning, setAssigning] = useState(false);

  const sorted = useMemo(() => sortLinks(links), [links]);
  const clients = clientsQuery.data ?? [];
  const clientOptions = useMemo(
    () => clients
      .filter((c) => c.email)
      .map((c) => ({ value: c.email, label: c.email })),
    [clients],
  );

  const summary = useMemo(() => {
    const total = links.length;
    const enabled = links.filter((link) => !link.isDisabled).length;
    const subscriptions = links.filter((link) => link.kind === 'subscription').length;
    return { total, enabled, subscriptions };
  }, [links]);

  const pageClass = useMemo(() => {
    const classes = ['hosts-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const onAdd = useCallback(() => {
    setFormMode('add');
    setFormLink(null);
    setFormOpen(true);
  }, []);

  const onEdit = useCallback((link: ManagedLinkRecord) => {
    setFormMode('edit');
    setFormLink({ ...link });
    setFormOpen(true);
  }, []);

  const onSave = useCallback(async (payload: ManagedLinkFormValues) => {
    if (formMode === 'edit' && formLink?.id) return update(formLink.id, payload);
    return create(payload);
  }, [formMode, formLink, update, create]);

  const onDelete = useCallback((link: ManagedLinkRecord) => {
    modal.confirm({
      title: t('pages.links.deleteConfirmTitle', { name: link.remark || shortValue(link.value) }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(link.id);
        if (msg?.success) messageApi.success(t('pages.links.toasts.delete'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onMove = useCallback(async (link: ManagedLinkRecord, dir: 'up' | 'down') => {
    const idx = sorted.findIndex((row) => row.id === link.id);
    const swapWith = dir === 'up' ? idx - 1 : idx + 1;
    if (idx < 0 || swapWith < 0 || swapWith >= sorted.length) return;
    const ids = sorted.map((row) => row.id);
    [ids[idx], ids[swapWith]] = [ids[swapWith], ids[idx]];
    await reorder(ids);
  }, [sorted, reorder]);

  const onBulkEnable = useCallback(async (enable: boolean) => {
    if (selectedIds.length === 0) return;
    const msg = await bulkSetEnable(selectedIds, enable);
    if (msg?.success) setSelectedIds([]);
  }, [selectedIds, bulkSetEnable]);

  const onBulkDelete = useCallback(() => {
    if (selectedIds.length === 0) return;
    modal.confirm({
      title: t('pages.links.bulkDeleteConfirm', { count: selectedIds.length }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await bulkDel(selectedIds);
        if (msg?.success) {
          messageApi.success(t('pages.links.toasts.delete'));
          setSelectedIds([]);
        }
      },
    });
  }, [selectedIds, modal, t, bulkDel, messageApi]);

  const openAssign = useCallback(() => {
    if (selectedIds.length === 0) return;
    assignForm.setFieldsValue({ emails: [] });
    setAssignOpen(true);
  }, [selectedIds, assignForm]);

  const onAssign = useCallback(async () => {
    let values: { emails: string[] };
    try {
      values = await assignForm.validateFields();
    } catch {
      return;
    }
    setAssigning(true);
    try {
      const msg = await assign(selectedIds, values.emails || []);
      if (msg?.success) {
        const obj = msg.obj;
        messageApi.success(t('pages.links.toasts.assignResult', {
          attached: obj?.attached ?? 0,
          skipped: obj?.skipped ?? 0,
        }));
        setAssignOpen(false);
        setSelectedIds([]);
      } else if (msg?.msg) {
        messageApi.error(msg.msg);
      }
    } finally {
      setAssigning(false);
    }
  }, [assignForm, assign, selectedIds, messageApi, t]);

  const movable = useMemo(() => {
    const idx = new Map<number, number>();
    sorted.forEach((row, i) => idx.set(row.id, i));
    return idx;
  }, [sorted]);

  const columns: ColumnsType<ManagedLinkRecord> = [
    {
      title: t('pages.links.fields.actions'),
      key: 'actions',
      width: 168,
      render: (_, link) => {
        const idx = movable.get(link.id) ?? 0;
        return (
          <Space size={2}>
            <Tooltip title={t('pages.links.moveUp')}>
              <Button size="small" type="text" icon={<ArrowUpOutlined />} disabled={idx === 0} onClick={() => onMove(link, 'up')} />
            </Tooltip>
            <Tooltip title={t('pages.links.moveDown')}>
              <Button size="small" type="text" icon={<ArrowDownOutlined />} disabled={idx >= sorted.length - 1} onClick={() => onMove(link, 'down')} />
            </Tooltip>
            <Tooltip title={t('edit')}>
              <Button size="small" type="text" icon={<EditOutlined />} onClick={() => onEdit(link)} />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={() => onDelete(link)} />
            </Tooltip>
          </Space>
        );
      },
    },
    {
      title: t('pages.links.fields.enable'),
      key: 'enable',
      width: 90,
      render: (_, link) => (
        <Switch size="small" checked={!link.isDisabled} onChange={(next) => setEnable(link.id, next)} />
      ),
    },
    {
      title: t('pages.links.fields.remark'),
      key: 'remark',
      width: 220,
      render: (_, link) => link.remark || <Typography.Text type="secondary">-</Typography.Text>,
    },
    {
      title: t('pages.links.fields.kind'),
      key: 'kind',
      width: 150,
      render: (_, link) => (
        <Tag color={link.kind === 'subscription' ? 'purple' : 'blue'}>
          {t(`pages.links.kind.${link.kind}`)}
        </Tag>
      ),
    },
    {
      title: t('pages.links.fields.value'),
      key: 'value',
      render: (_, link) => (
        <Typography.Text code copyable={{ text: link.value }}>
          {shortValue(link.value)}
        </Typography.Text>
      ),
    },
  ];

  const toolbar = (
    <div className="card-toolbar">
      {selectedIds.length === 0 ? (
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {!isMobile && t('pages.links.addLink')}
        </Button>
      ) : (
        <>
          <Tag
            color="blue"
            closable
            onClose={() => setSelectedIds([])}
            style={{ marginInlineEnd: 0, padding: '4px 8px', fontSize: 13 }}
          >
            {t('pages.links.selectedCount', { count: selectedIds.length })}
          </Tag>
          <Button icon={<TeamOutlined />} onClick={openAssign}>{t('pages.links.assign')}</Button>
          <Button onClick={() => onBulkEnable(true)}>{t('pages.links.bulkEnable')}</Button>
          <Button onClick={() => onBulkEnable(false)}>{t('pages.links.bulkDisable')}</Button>
          <Button danger icon={<DeleteOutlined />} onClick={onBulkDelete}>{t('pages.links.bulkDelete')}</Button>
        </>
      )}
    </div>
  );

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
                        <Col xs={8}>
                          <Statistic title={t('pages.links.summary.total')} value={String(summary.total)} prefix={<LinkOutlined />} />
                        </Col>
                        <Col xs={8}>
                          <Statistic title={t('pages.links.summary.enabled')} value={String(summary.enabled)} prefix={<ApiOutlined style={{ color: 'var(--ant-color-success)' }} />} />
                        </Col>
                        <Col xs={8}>
                          <Statistic title={t('pages.links.summary.subscriptions')} value={String(summary.subscriptions)} prefix={<TeamOutlined />} />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card size="small" hoverable title={toolbar} className="hosts-card">
                      <Table<ManagedLinkRecord>
                        rowKey="id"
                        size="small"
                        loading={loading}
                        columns={columns}
                        dataSource={sorted}
                        pagination={false}
                        scroll={{ x: 'max-content' }}
                        rowSelection={{
                          selectedRowKeys: selectedIds,
                          onChange: (keys) => setSelectedIds(keys as number[]),
                        }}
                        locale={{
                          emptyText: (
                            <div className="card-empty">
                              <LinkOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                              <div>{t('noData')}</div>
                            </div>
                          ),
                        }}
                      />
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <LinkFormModal
          open={formOpen}
          mode={formMode}
          link={formLink}
          save={onSave}
          onOpenChange={setFormOpen}
        />

        <Modal
          open={assignOpen}
          title={t('pages.links.assignTitle', { count: selectedIds.length })}
          okText={t('pages.links.assign')}
          cancelText={t('cancel')}
          confirmLoading={assigning}
          onOk={onAssign}
          onCancel={() => setAssignOpen(false)}
          destroyOnHidden
        >
          <Form form={assignForm} layout="vertical" preserve={false}>
            <Form.Item
              name="emails"
              label={t('pages.links.assignClients')}
              rules={[{ required: true, type: 'array', min: 1 }]}
            >
              <Select
                mode="multiple"
                options={clientOptions}
                loading={clientsQuery.isFetching}
                placeholder={t('pages.links.selectClients')}
                maxTagCount="responsive"
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>
          </Form>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
