import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Input,
  Layout,
  Row,
  Statistic,
  Tag,
  message,
} from 'antd';
import { LockOutlined, MailOutlined, UserOutlined, WalletOutlined } from '@ant-design/icons';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useMe, ME_QUERY_KEY } from '@/hooks/useMe';
import { useCurrency } from '@/hooks/useCurrency';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;
const basePath = window.X_UI_BASE_PATH || '/';

interface ProfileFormValues {
  username: string;
  email?: string;
  currentPassword: string;
  newPassword?: string;
  confirmPassword?: string;
}

export default function ProfilePage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { me } = useMe();
  const { unit } = useCurrency();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ProfileFormValues>();

  useEffect(() => {
    if (me) {
      form.setFieldsValue({ username: me.username, email: me.email });
    }
  }, [me, form]);

  const saveMut = useMutation({
    mutationFn: (values: ProfileFormValues) =>
      HttpUtil.post('/panel/api/profile', {
        currentPassword: values.currentPassword,
        username: values.username,
        email: values.email ?? '',
        newPassword: values.newPassword ?? '',
      }, JSON_HEADERS),
    onSuccess: (msg) => {
      if (!msg?.success) return;
      const passwordChanged = !!(msg.obj as { passwordChanged?: boolean } | null)?.passwordChanged;
      if (passwordChanged) {
        messageApi.success(t('pages.profile.toasts.passwordChanged'));
        window.setTimeout(() => { window.location.href = basePath; }, 1200);
      } else {
        messageApi.success(t('pages.profile.toasts.saved'));
        form.setFieldsValue({ currentPassword: '', newPassword: '', confirmPassword: '' });
        queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY });
      }
    },
  });

  async function onSubmit() {
    const values = await form.validateFields();
    await saveMut.mutateAsync(values);
  }

  const pageClass = useMemo(() => {
    const classes = ['profile-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Row gutter={[16, 16]} justify="center">
              <Col xs={24} md={16} lg={12} xxl={10}>
                <Card size="small" hoverable className="summary-card" style={{ marginBottom: 16 }}>
                  <Row align="middle" gutter={16}>
                    <Col flex="auto">
                      <Statistic
                        title={t('balance')}
                        value={me?.balance ?? 0}
                        prefix={<WalletOutlined />}
                        suffix={unit}
                        groupSeparator=","
                      />
                    </Col>
                    <Col>
                      <Tag color={me?.isAdmin ? 'gold' : 'blue'}>
                        {t(`pages.users.role_${me?.isAdmin ? 'admin' : 'user'}`)}
                      </Tag>
                    </Col>
                  </Row>
                </Card>

                <Card size="small" hoverable title={t('pages.profile.accountTitle')}>
                  <Form form={form} layout="vertical" onFinish={onSubmit}>
                    <Form.Item
                      label={t('username')}
                      name="username"
                      rules={[{ required: true, pattern: /^[A-Za-z0-9_]{3,32}$/, message: t('pages.register.errors.username') }]}
                    >
                      <Input prefix={<UserOutlined />} autoComplete="username" />
                    </Form.Item>
                    <Form.Item label={t('email')} name="email">
                      <Input prefix={<MailOutlined />} autoComplete="email" placeholder={t('pages.register.placeholders.email')} />
                    </Form.Item>

                    <div style={{ fontWeight: 600, margin: '4px 0 14px' }}>{t('pages.profile.changePassword')}</div>
                    <Form.Item
                      label={t('pages.profile.newPassword')}
                      name="newPassword"
                      rules={[{ min: 8, message: t('pages.register.errors.password') }]}
                      extra={t('pages.profile.newPasswordHint')}
                    >
                      <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
                    </Form.Item>
                    <Form.Item
                      label={t('confirmPassword')}
                      name="confirmPassword"
                      dependencies={['newPassword']}
                      rules={[
                        ({ getFieldValue }) => ({
                          validator(_rule, value) {
                            if (!getFieldValue('newPassword') || getFieldValue('newPassword') === value) {
                              return Promise.resolve();
                            }
                            return Promise.reject(new Error(t('pages.register.errors.confirmPassword')));
                          },
                        }),
                      ]}
                    >
                      <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
                    </Form.Item>

                    <Form.Item
                      label={t('pages.profile.currentPassword')}
                      name="currentPassword"
                      rules={[{ required: true, message: t('pages.profile.currentPasswordRequired') }]}
                      extra={t('pages.profile.currentPasswordHint')}
                    >
                      <Input.Password prefix={<LockOutlined />} autoComplete="current-password" />
                    </Form.Item>

                    <Form.Item style={{ marginBottom: 0 }}>
                      <Button type="primary" htmlType="submit" loading={saveMut.isPending} block>
                        {t('save')}
                      </Button>
                    </Form.Item>
                  </Form>
                </Card>
              </Col>
            </Row>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
