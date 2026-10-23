import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  ConfigProvider,
  Form,
  Input,
  InputNumber,
  Layout,
  Modal,
  message,
  Popconfirm,
  Select,
  Spin,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PlusOutlined, SyncOutlined, UserOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';

const { Title, Text } = Typography;
const { Option } = Select;

interface InboundOption {
  id: number;
  remark: string;
  tag: string;
  port?: number;
  enable?: boolean;
}

interface ResellerUsage {
  clientCount: number;
  usedGB: number; // bytes
}

interface ClientItem {
  id: number;
  email: string;
  userId: number;
  enable: boolean;
  totalGB: number;
  expiryTime: number;
}

interface Reseller {
  id: number;
  username: string;
  role: string;
  parentId: number;
  totalGB: number;
  usedGB: number;
  maxClients: number;
  enable: boolean;
  usage: ResellerUsage;
}

const GB = 1024 * 1024 * 1024;

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
  if (bytes < GB) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  return `${(bytes / GB).toFixed(2)} GB`;
}

function statusTag(enable: boolean) {
  return (
    <Tag color={enable ? 'green' : 'grey'}>{enable ? 'Enabled' : 'Disabled'}</Tag>
  );
}

export default function ResellersPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  setMessageInstance(messageApi);

  const [resellers, setResellers] = useState<Reseller[]>([]);
  const [allInbounds, setAllInbounds] = useState<InboundOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [saveLoading, setSaveLoading] = useState(false);
  const [modal, setModal] = useState<{
    open: boolean;
    mode: 'create' | 'edit';
    reseller?: Reseller;
  }>({ open: false, mode: 'create' });
  const [assignOpen, setAssignOpen] = useState(false);
  const [assignResellerId, setAssignResellerId] = useState<number | null>(null);
  const [assignedInboundIds, setAssignedInboundIds] = useState<number[]>([]);

  // Client assignment to a reseller
  const [clientAssignOpen, setClientAssignOpen] = useState(false);
  const [clientAssignResellerId, setClientAssignResellerId] = useState<number | null>(null);
  const [allClients, setAllClients] = useState<ClientItem[]>([]);
  const [selectedClientEmails, setSelectedClientEmails] = useState<string[]>([]);
  const [clientsLoading, setClientsLoading] = useState(false);

  const fetchList = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.get<Reseller[]>(
        '/panel/api/resellers/list',
        undefined,
        { silent: true },
      );
      if (msg?.success && Array.isArray(msg.obj)) {
        setResellers(msg.obj);
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } catch (err: any) {
      messageApi.error(err?.msg || t('somethingWentWrong'));
    } finally {
      setLoading(false);
    }
  }, [messageApi, t]);

  const fetchInbounds = useCallback(async () => {
    const msg = await HttpUtil.get<InboundOption[]>(
      '/panel/api/inbounds/options',
      undefined,
      { silent: true },
    );
    if (msg?.success && Array.isArray(msg.obj)) {
      setAllInbounds(msg.obj);
    }
  }, []);

  const fetchClients = useCallback(async () => {
    setClientsLoading(true);
    try {
      const msg = await HttpUtil.get<any[]>(
        '/panel/api/clients/list',
        undefined,
        { silent: true },
      );
      if (msg?.success && Array.isArray(msg.obj)) {
        setAllClients(
          msg.obj.map((c: any) => ({
            id: c.id,
            email: c.email,
            userId: c.userId ?? 0,
            enable: c.enable ?? true,
            totalGB: c.totalGB ?? 0,
            expiryTime: c.expiryTime ?? 0,
          })),
        );
      }
    } catch (err: any) {
      messageApi.error(err?.msg || t('somethingWentWrong'));
    } finally {
      setClientsLoading(false);
    }
  }, [messageApi, t]);

  useEffect(() => {
    fetchList();
    fetchInbounds();
  }, [fetchList, fetchInbounds]);

  // Load clients when the client-assign modal opens so we can show current ownership.
  useEffect(() => {
    if (clientAssignOpen) {
      fetchClients();
    }
  }, [clientAssignOpen, fetchClients]);

  const cols = useMemo(
    () => [
      {
        title: t('pages.resellers.username'),
        dataIndex: 'username',
        key: 'username',
      },
      {
        title: t('pages.resellers.status'),
        dataIndex: 'enable',
        key: 'enable',
        render: (enable: boolean) => statusTag(!!enable),
      },
      {
        title: t('pages.resellers.clientCount'),
        dataIndex: 'usage',
        key: 'clientCount',
        render: (u: ResellerUsage) => u.clientCount,
      },
      {
        title: t('pages.resellers.trafficUsage'),
        dataIndex: 'usage',
        key: 'traffic',
        render: (u: ResellerUsage, record: Reseller) => {
          const usedGB = u.usedGB / GB;
          const quota = record.totalGB;
          const remaining = quota > 0 ? Math.max(0, quota - usedGB) : 0;
          return (
            <div>
              <Text>{t('pages.resellers.used')}: {formatBytes(u.usedGB)}</Text>
              {quota > 0 ? (
                <Text type="secondary">
                  {t('pages.resellers.remaining')}: {remaining.toFixed(2)} GB /
                  {quota} GB
                </Text>
              ) : (
                <Text type="secondary">{t('pages.resellers.unlimited')}</Text>
              )}
            </div>
          );
        },
      },
      {
        title: t('pages.resellers.actions'),
        dataIndex: 'id',
        key: 'actions',
        render: (id: number) => (
          <>
            <Button
              size="small"
              onClick={() => openEdit(id)}
              style={{ marginRight: 6 }}
            >
              {t('edit')}
            </Button>
            <Button
              size="small"
              icon={<UserOutlined />}
              onClick={() => openClientAssign(id)}
              style={{ marginRight: 6 }}
              title={t('pages.resellers.assignClients')}
            />
            <Popconfirm
              title={t('pages.resellers.deleteConfirm')}
              onConfirm={() => handleDelete(id)}
              okText={t('sure')}
              cancelText={t('cancel')}
            >
              <Button size="small" danger>{t('delete')}</Button>
            </Popconfirm>
          </>
        ),
      },
    ],
    [t],
  );

  const openCreate = () => {
    setModal({ open: true, mode: 'create' });
  };

  const openEdit = (id: number) => {
    const record = resellers.find((r) => r.id === id);
    setModal({ open: true, mode: 'edit', reseller: record });
  };

  const handleDelete = async (id: number) => {
    try {
      const msg = await HttpUtil.post(
        `/panel/api/resellers/del/${id}`,
        undefined,
        { silent: true },
      );
      if (msg?.success) {
        messageApi.success(t('pages.resellers.deleteSuccess'));
        fetchList();
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } catch (err: any) {
      messageApi.error(err?.msg || t('somethingWentWrong'));
    }
  };

  const handleFormSubmit = async (values: any) => {
    setSaveLoading(true);
    try {
      if (modal.mode === 'create') {
        const msg = await HttpUtil.post(
          '/panel/api/resellers/add',
          {
            username: values.username,
            password: values.password,
            totalGB: values.totalGB ?? 0,
            maxClients: values.maxClients ?? 0,
          },
          { silent: true },
        );
        if (msg?.success) {
          messageApi.success(t('pages.resellers.createSuccess'));
          setModal({ open: false, mode: 'create' });
          fetchList();
        } else {
          messageApi.error(msg?.msg || t('somethingWentWrong'));
        }
      } else {
        const record = modal.reseller;
        if (!record) return;
        const msg = await HttpUtil.post(
          `/panel/api/resellers/update/${record.id}`,
          {
            username: values.username,
            password: values.password || undefined,
            totalGB: values.totalGB ?? 0,
            maxClients: values.maxClients ?? 0,
            enable: values.enable,
          },
          { silent: true },
        );
        if (msg?.success) {
          messageApi.success(t('pages.resellers.updateSuccess'));
          setModal({ open: false, mode: 'edit' });
          fetchList();
        } else {
          messageApi.error(msg?.msg || t('somethingWentWrong'));
        }
      }
    } catch (err: any) {
      messageApi.error(err?.msg || t('somethingWentWrong'));
    } finally {
      setSaveLoading(false);
    }
  };

  const openAssign = (id: number) => {
    setAssignResellerId(id);
    setAssignedInboundIds([]);
    setAssignOpen(true);
  };

  useEffect(() => {
    if (assignOpen && assignResellerId != null) {
      HttpUtil.get<number[]>(
        `/panel/api/resellers/${assignResellerId}/inbounds`,
        undefined,
        { silent: true },
      ).then((msg) => {
        if (msg?.success && Array.isArray(msg.obj)) {
          setAssignedInboundIds(msg.obj);
        }
      });
    }
  }, [assignOpen, assignResellerId]);

  const saveAssign = async () => {
    if (assignResellerId == null) return;
    setSaveLoading(true);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/resellers/${assignResellerId}/inbounds`,
        { inboundIds: assignedInboundIds },
        { silent: true },
      );
      if (msg?.success) {
        messageApi.success(t('pages.resellers.updateSuccess'));
        setAssignOpen(false);
        fetchList();
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } catch (err: any) {
      messageApi.error(err?.msg || t('somethingWentWrong'));
    } finally {
      setSaveLoading(false);
    }
  };

  const openClientAssign = (id: number) => {
    setClientAssignResellerId(id);
    setSelectedClientEmails([]);
    setClientAssignOpen(true);
  };

  const saveClientAssign = async () => {
    if (clientAssignResellerId == null) return;
    setSaveLoading(true);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/resellers/${clientAssignResellerId}/clients/assign`,
        { clientEmails: selectedClientEmails },
        { silent: true },
      );
      if (msg?.success) {
        messageApi.success(t('pages.resellers.clientsAssigned'));
        setClientAssignOpen(false);
        setSelectedClientEmails([]);
        fetchList();
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } catch (err: any) {
      messageApi.error(err?.msg || t('somethingWentWrong'));
    } finally {
      setSaveLoading(false);
    }
  };

  const formRef = useRef<Form.Instance>(null);

  const pageClass = useMemo(() => {
    const classes = ['resellers-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <Card>
              <Card.Actions style={{ marginBottom: 16 }}>
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={openCreate}
                  disabled={saveLoading}
                >
                  {t('pages.resellers.createReseller')}
                </Button>
                <Button
                  icon={<SyncOutlined />}
                  onClick={fetchList}
                  loading={loading}
                  style={{ marginLeft: 8 }}
                />
              </Card.Actions>

              <Spin spinning={loading}>
                {resellers.length === 0 ? (
                  <div style={{ textAlign: 'center', padding: 40 }}>
                    <Text type="secondary">{t('pages.resellers.noResellers')}</Text>
                  </div>
                ) : (
                  <Table
                    rowKey="id"
                    dataSource={resellers}
                    columns={cols}
                    size="middle"
                    scroll={{ x: 700 }}
                  />
                )}
              </Spin>
            </Card>

            <Modal
              open={modal.open}
              title={
                modal.mode === 'create'
                  ? t('pages.resellers.createReseller')
                  : t('pages.resellers.editReseller')
              }
              okText={t('save')}
              cancelText={t('cancel')}
              confirmLoading={saveLoading}
              onOk={async () => {
                const fields = await formRef.current?.validateFields();
                if (fields) await handleFormSubmit(fields);
              }}
              onCancel(() => setModal({ open: false, mode: 'create' }))
              width={520}
            >
              <ResellerForm
                formRef={formRef}
                mode={modal.mode}
                record={modal.reseller}
              />
            </Modal>

            <Modal
              open={assignOpen}
              title={t('pages.resellers.assignedInbounds')}
              okText={t('save')}
              cancelText={t('cancel')}
              confirmLoading={saveLoading}
              onOk={saveAssign}
              onCancel(() => setAssignOpen(false))
              width={600}
            >
              <AssignInboundsForm
                inbounds={allInbounds}
                selected={assignedInboundIds}
                onChange={setAssignedInboundIds}
              />
            </Modal>

            {/* Client assignment modal: assign existing clients to a reseller */}
            <Modal
              open={clientAssignOpen}
              title={t('pages.resellers.assignClientsTitle')}
              okText={t('save')}
              cancelText={t('cancel')}
              confirmLoading={saveLoading}
              onOk={saveClientAssign}
              onCancel(() => setClientAssignOpen(false))
              width={700}
            >
              <div style={{ padding: 16 }}>
                {clientsLoading ? (
                  <Spin />
                ) : (
                  <div style={{ maxHeight: 400, overflowY: 'auto', border: '1px solid var(--border-color)', borderRadius: 6, padding: 8, background: 'var(--item-bg)' }}>
                    {allClients.length === 0 ? (
                      <div style={{ textAlign: 'center', padding: 24, color: 'var(--text-color-secondary)' }}>
                        {t('pages.resellers.noClients')}
                      </div>
                    ) : (
                      allClients.map((client) => {
                        const alreadyOwner = client.userId === clientAssignResellerId;
                        const selected = selectedClientEmails.includes(client.email);
                        const ownerLabel =
                          client.userId === 0
                            ? t('pages.resellers.ownerAdmin')
                            : `#${client.userId}`;
                        return (
                          <div
                            key={client.email}
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              padding: 8,
                              borderBottom: '1px solid var(--border-color)',
                              background: selected ? 'var(--primary-color)' : 'transparent',
                              color: selected ? '#fff' : 'inherit',
                              cursor: 'pointer',
                            }}
                            onClick={() => {
                              if (selected) {
                                setSelectedClientEmails(
                                  selectedClientEmails.filter((e) => e !== client.email),
                                );
                              } else {
                                setSelectedClientEmails([...selectedClientEmails, client.email]);
                              }
                            }}
                          >
                                                        <span style={{ marginRight: 10, minWidth: 30 }}>
                              {selected ? '✓' : '○'}
                            </span>
                                                        <span style={{ flex: 1, fontSize: 13 }}>{client.email}</span>
                                                        <span
                                                          style={{
                                                            fontSize: 11,
                                                            padding: '2px 6px',
                                                            borderRadius: 10,
                                                            background: alreadyOwner
                                                              ? 'var(--success-color)'
                                                              : 'var(--border-color)',
                                                            color: alreadyOwner ? '#fff' : 'var(--text-color-secondary)',
                                                            marginRight: 8,
                                                          }}
                                                        >
                                                          {ownerLabel}
                                                        </span>
                                                    </div>
                                                  );
                                                })
                                              )}
                                          </div>
                                        )}
                                        <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                          <span style={{ fontSize: 12, color: 'var(--text-color-secondary)' }}>
                                            {selectedClientEmails.length} {t('pages.resellers.clientsSelected')}
                                          </span>
                                          <span style={{ fontSize: 12, color: 'var(--text-color-secondary)' }}>
                                            {t('pages.resellers.ownerLabel')}:
                                            {' '}
                                            <strong style={{ color: 'var(--primary-color)' }}>{`#${clientAssignResellerId}`}</strong>
                                          </span>
                                        </div>
                                      </div>
                                    </Modal>
                                  </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}

interface ResellerFormProps {
  formRef: React.RefObject<Form.Instance>;
  mode: 'create' | 'edit';
  record?: Reseller;
}

function ResellerForm({
  formRef,
  mode,
  record,
}: ResellerFormProps) {
  const { t } = useTranslation();
  return (
    <Form
      ref={formRef}
      layout="vertical"
      preserve={false}
    >
      <Form.Item
        label={t('pages.resellers.username')}
        name="username"
        rules={[{ required: true, message: t('pages.resellers.username') + ' is required' }]}
      >
        <Input defaultValue={mode === 'edit' ? record?.username : ''} />
      </Form.Item>
      <Form.Item
        label={t('pages.resellers.password')}
        name="password"
        rules={
          mode === 'create'
            ? [{ required: true, message: t('pages.resellers.password') + ' is required' }]
            : []
        }
      >
        {/* Editing the hash is not supported; leave blank to keep the current password. */}
        <Input.Password defaultValue={mode === 'edit' ? '' : ''} />
      </Form.Item>
      <Form.Item
        label={t('pages.resellers.totalTrafficGB')}
        name="totalGB"
        extra={t('pages.resellers.totalTrafficGBDesc')}
        rules={[{ type: 'number', min: 0 }]}
      >
        <InputNumber defaultValue={mode === 'edit' ? (record?.totalGB || 0) : 0} min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.resellers.maxClients')}
        name="maxClients"
        extra={t('pages.resellers.maxClientsDesc')}
        rules={[{ type: 'number', min: 0 }]}
      >
        <InputNumber defaultValue={mode === 'edit' ? (record?.maxClients || 0) : 0} min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.resellers.enable')}
        name="enable"
        valuePropName="checked"
      >
        <Switch defaultChecked={mode === 'edit' ? record?.enable ?? true : true} />
      </Form.Item>
    </Form>
  );
}

function AssignInboundsForm({
  inbounds,
  selected,
  onChange,
}: {
  inbounds: InboundOption[];
  selected: number[];
  onChange: (ids: number[]) => void;
}) {
  return (
    <Select
      mode="multiple"
      style={{ width: '100%' }}
      placeholder="Select inbounds"
      defaultValue={selected}
      onChange={onChange}
    >
      {inbounds.map((ib) => (
        <Option key={ib.id} value={ib.id}>
          {ib.remark} ({ib.tag}) - Port {ib.port}
        </Option>
      ))}
    </Select>
  );
}
