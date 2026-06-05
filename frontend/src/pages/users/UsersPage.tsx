import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Input,
  InputNumber,
  Layout,
  Modal,
  Result,
  Row,
  Segmented,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  CrownOutlined,
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
  PlusOutlined,
  SearchOutlined,
  TeamOutlined,
  UserOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useMe, ME_QUERY_KEY } from '@/hooks/useMe';
import { useCurrency } from '@/hooks/useCurrency';
import { HttpUtil, IntlUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

interface PanelUser {
  id: number;
  username: string;
  role: string;
  fullName: string;
  phone: string;
  email: string;
  balance: number;
}

interface Transaction {
  id: number;
  userId: number;
  amount: number;
  type: string;
  description: string;
  balanceBefore: number;
  balanceAfter: number;
  createdAt: number;
}

interface UserFormValues {
  username: string;
  password?: string;
  fullName?: string;
  phone?: string;
  email?: string;
  role: string;
  balance?: number;
}

type BalanceOp = 'add' | 'deduct' | 'set';

async function fetchUsers(): Promise<PanelUser[]> {
  const msg = await HttpUtil.get('/panel/api/admin/users', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load users');
  return (msg.obj as PanelUser[]) ?? [];
}

async function fetchTransactions(userId: number): Promise<Transaction[]> {
  const msg = await HttpUtil.get(`/panel/api/admin/transactions?userId=${userId}`, undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load transactions');
  return (msg.obj as Transaction[]) ?? [];
}

export default function UsersPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const { me } = useMe();
  const { format: formatMoney, formatNumber, unit } = useCurrency();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);
  const queryClient = useQueryClient();

  const usersQuery = useQuery({ queryKey: ['admin', 'users'], queryFn: fetchUsers });
  const users = useMemo(() => usersQuery.data ?? [], [usersQuery.data]);

  const [search, setSearch] = useState('');
  const filteredUsers = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return users;
    return users.filter((u) =>
      [u.username, u.email, u.fullName, u.phone, u.role]
        .some((v) => (v || '').toLowerCase().includes(needle)),
    );
  }, [users, search]);
  const fetched = usersQuery.data !== undefined || usersQuery.isError;
  const fetchError = usersQuery.error ? (usersQuery.error as Error).message : '';

  const stats = useMemo(() => {
    let admins = 0;
    let totalBalance = 0;
    for (const u of users) {
      if (u.role === 'admin') admins += 1;
      totalBalance += u.balance || 0;
    }
    return { total: users.length, admins, resellers: users.length - admins, totalBalance };
  }, [users]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY });
  };

  // ---- create / edit user ----
  const [userForm] = Form.useForm<UserFormValues>();
  const [userModalOpen, setUserModalOpen] = useState(false);
  const [editing, setEditing] = useState<PanelUser | null>(null);

  const saveUserMut = useMutation({
    mutationFn: (values: UserFormValues) => {
      const url = editing ? `/panel/api/admin/users/${editing.id}` : '/panel/api/admin/users';
      return HttpUtil.post(url, values, JSON_HEADERS);
    },
    onSuccess: (msg) => {
      if (msg?.success) {
        invalidate();
        setUserModalOpen(false);
        messageApi.success(editing ? t('pages.users.toasts.userUpdated') : t('pages.users.toasts.userCreated'));
      }
    },
  });

  function openCreate() {
    setEditing(null);
    userForm.resetFields();
    userForm.setFieldsValue({ role: 'user', balance: 0 });
    setUserModalOpen(true);
  }

  function openEdit(row: PanelUser) {
    setEditing(row);
    userForm.resetFields();
    userForm.setFieldsValue({
      username: row.username,
      fullName: row.fullName,
      phone: row.phone,
      email: row.email,
      role: row.role,
    });
    setUserModalOpen(true);
  }

  async function submitUser() {
    const values = await userForm.validateFields();
    await saveUserMut.mutateAsync(values);
  }

  function onDelete(row: PanelUser) {
    modal.confirm({
      title: t('pages.users.deleteConfirmTitle', { name: row.username }),
      content: t('pages.users.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await HttpUtil.post(`/panel/api/admin/users/${row.id}/del`);
        if (msg?.success) invalidate();
      },
    });
  }

  // ---- balance adjustment ----
  const [balanceForm] = Form.useForm<{ op: BalanceOp; amount: number; description: string }>();
  const [balanceTarget, setBalanceTarget] = useState<PanelUser | null>(null);

  const balanceMut = useMutation({
    mutationFn: (body: { op: BalanceOp; amount: number; description: string }) =>
      HttpUtil.post(`/panel/api/admin/users/${balanceTarget!.id}/balance`, body, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) {
        invalidate();
        setBalanceTarget(null);
        messageApi.success(t('pages.users.toasts.balanceUpdated'));
      }
    },
  });

  function openBalance(row: PanelUser) {
    setBalanceTarget(row);
    balanceForm.resetFields();
    balanceForm.setFieldsValue({ op: 'add', amount: 0, description: '' });
  }

  async function submitBalance() {
    const values = await balanceForm.validateFields();
    await balanceMut.mutateAsync(values);
  }

  // ---- transaction history ----
  const [historyTarget, setHistoryTarget] = useState<PanelUser | null>(null);
  const txQuery = useQuery({
    queryKey: ['admin', 'transactions', historyTarget?.id],
    queryFn: () => fetchTransactions(historyTarget!.id),
    enabled: !!historyTarget,
  });

  const columns: TableColumnsType<PanelUser> = [
    {
      title: t('pages.users.actions'),
      key: 'actions',
      width: 170,
      render: (_v, row) => (
        <Space size={2}>
          <Tooltip title={t('pages.users.editUser')}>
            <Button size="small" type="text" icon={<EditOutlined />} onClick={() => openEdit(row)} />
          </Tooltip>
          <Tooltip title={t('pages.users.manageBalance')}>
            <Button size="small" type="text" icon={<WalletOutlined />} onClick={() => openBalance(row)} />
          </Tooltip>
          <Tooltip title={t('pages.users.history')}>
            <Button size="small" type="text" icon={<HistoryOutlined />} onClick={() => setHistoryTarget(row)} />
          </Tooltip>
          <Tooltip title={t('delete')}>
            <Button
              size="small"
              type="text"
              danger
              icon={<DeleteOutlined />}
              disabled={!!me && me.id === row.id}
              onClick={() => onDelete(row)}
            />
          </Tooltip>
        </Space>
      ),
    },
    { title: t('username'), dataIndex: 'username', key: 'username' },
    {
      title: t('pages.users.role'),
      dataIndex: 'role',
      key: 'role',
      render: (role: string) => (
        <Tag color={role === 'admin' ? 'gold' : 'blue'}>{t(`pages.users.role_${role === 'admin' ? 'admin' : 'user'}`)}</Tag>
      ),
    },
    { title: t('email'), dataIndex: 'email', key: 'email', responsive: ['md'] },
    {
      title: t('balance'),
      dataIndex: 'balance',
      key: 'balance',
      render: (b: number) => <strong>{formatMoney(b)}</strong>,
    },
  ];

  const txColumns: TableColumnsType<Transaction> = [
    {
      title: t('pages.users.txType'),
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => (
        <Tag color={type === 'credit' ? 'green' : 'volcano'}>
          {t(`pages.users.tx_${type === 'credit' ? 'credit' : 'debit'}`)}
        </Tag>
      ),
    },
    { title: t('pages.users.txAmount'), dataIndex: 'amount', key: 'amount', render: (a: number) => formatNumber(a) },
    { title: t('pages.users.txBefore'), dataIndex: 'balanceBefore', key: 'balanceBefore', responsive: ['sm'], render: (a: number) => formatNumber(a) },
    { title: t('pages.users.txAfter'), dataIndex: 'balanceAfter', key: 'balanceAfter', render: (a: number) => formatNumber(a) },
    { title: t('pages.users.txDescription'), dataIndex: 'description', key: 'description', responsive: ['md'] },
    {
      title: t('pages.users.txDate'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (ts: number) => IntlUtil.formatDate(ts),
    },
  ];

  const pageClass = useMemo(() => {
    const classes = ['users-page'];
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
                  extra={<Button type="primary" onClick={() => usersQuery.refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={8} md={6}>
                          <Statistic title={t('pages.users.totalUsers')} value={String(stats.total)} prefix={<TeamOutlined />} />
                        </Col>
                        <Col xs={12} sm={8} md={6}>
                          <Statistic title={t('pages.users.admins')} value={String(stats.admins)} prefix={<CrownOutlined />} />
                        </Col>
                        <Col xs={12} sm={8} md={6}>
                          <Statistic title={t('pages.users.resellers')} value={String(stats.resellers)} prefix={<UserOutlined />} />
                        </Col>
                        <Col xs={12} sm={8} md={6}>
                          <Statistic
                            title={t('pages.users.totalBalance')}
                            value={formatNumber(stats.totalBalance)}
                            prefix={<WalletOutlined />}
                            suffix={unit}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card
                      size="small"
                      hoverable
                      title={
                        <div className="card-toolbar" style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                            {!isMobile && t('pages.users.addUser')}
                          </Button>
                          <Input
                            allowClear
                            prefix={<SearchOutlined />}
                            placeholder={t('pages.users.searchPlaceholder')}
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            style={{ maxWidth: 280 }}
                          />
                        </div>
                      }
                    >
                      <Table<PanelUser>
                        dataSource={filteredUsers}
                        columns={columns}
                        rowKey="id"
                        size="small"
                        pagination={false}
                        loading={usersQuery.isFetching}
                        locale={{
                          emptyText: (
                            <div className="card-empty">
                              <TeamOutlined style={{ fontSize: 32, marginBottom: 8 }} />
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

        {/* Create / edit user */}
        <Modal
          open={userModalOpen}
          title={editing ? t('pages.users.editUser') : t('pages.users.addUser')}
          okText={t('save')}
          cancelText={t('cancel')}
          confirmLoading={saveUserMut.isPending}
          onCancel={() => setUserModalOpen(false)}
          onOk={submitUser}
          destroyOnHidden
        >
          <Form form={userForm} layout="vertical">
            <Form.Item
              label={t('username')}
              name="username"
              rules={[{ required: true, pattern: /^[A-Za-z0-9_]{3,32}$/, message: t('pages.register.errors.username') }]}
            >
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item
              label={t('password')}
              name="password"
              rules={editing
                ? []
                : [{ required: true, min: 8, message: t('pages.register.errors.password') }]}
              extra={editing ? t('pages.users.passwordEditHint') : undefined}
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
            <Form.Item label={t('fullName')} name="fullName">
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item label={t('phoneNumber')} name="phone">
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item label={t('email')} name="email">
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item label={t('pages.users.role')} name="role" rules={[{ required: true }]}>
              <Select
                options={[
                  { value: 'user', label: t('pages.users.role_user') },
                  { value: 'admin', label: t('pages.users.role_admin') },
                ]}
              />
            </Form.Item>
            {!editing && (
              <Form.Item label={t('pages.users.initialBalance')} name="balance">
                <InputNumber min={0} style={{ width: '100%' }} addonAfter={unit} />
              </Form.Item>
            )}
          </Form>
        </Modal>

        {/* Balance adjustment */}
        <Modal
          open={!!balanceTarget}
          title={balanceTarget ? t('pages.users.balanceTitle', { name: balanceTarget.username }) : ''}
          okText={t('confirm')}
          cancelText={t('cancel')}
          confirmLoading={balanceMut.isPending}
          onCancel={() => setBalanceTarget(null)}
          onOk={submitBalance}
          destroyOnHidden
        >
          {balanceTarget && (
            <p style={{ marginTop: 0 }}>
              {t('balance')}: <strong>{formatMoney(balanceTarget.balance)}</strong>
            </p>
          )}
          <Form form={balanceForm} layout="vertical">
            <Form.Item name="op" label={t('pages.users.operation')} rules={[{ required: true }]}>
              <Segmented
                options={[
                  { value: 'add', label: t('pages.users.opAdd') },
                  { value: 'deduct', label: t('pages.users.opDeduct') },
                  { value: 'set', label: t('pages.users.opSet') },
                ]}
              />
            </Form.Item>
            <Form.Item
              name="amount"
              label={t('pages.users.amount')}
              rules={[{ required: true, type: 'number', min: 0, message: t('pages.users.toasts.invalidAmount') }]}
            >
              <InputNumber min={0} style={{ width: '100%' }} addonAfter={unit} />
            </Form.Item>
            <Form.Item name="description" label={t('pages.users.txDescription')}>
              <Input maxLength={200} />
            </Form.Item>
          </Form>
        </Modal>

        {/* Transaction history */}
        <Modal
          open={!!historyTarget}
          title={historyTarget ? t('pages.users.historyTitle', { name: historyTarget.username }) : ''}
          footer={null}
          width={760}
          onCancel={() => setHistoryTarget(null)}
          destroyOnHidden
        >
          <Table<Transaction>
            dataSource={txQuery.data ?? []}
            columns={txColumns}
            rowKey="id"
            size="small"
            loading={txQuery.isFetching}
            pagination={{ pageSize: 10, hideOnSinglePage: true }}
          />
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
