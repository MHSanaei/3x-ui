import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  ConfigProvider,
  Form,
  Input,
  Layout,
  Menu,
  Popover,
  Progress,
  Space,
  message,
} from 'antd';
import {
  LockOutlined,
  MailOutlined,
  MoonFilled,
  MoonOutlined,
  PhoneOutlined,
  SunOutlined,
  TranslationOutlined,
  UserOutlined,
} from '@ant-design/icons';

import { HttpUtil, LanguageManager } from '@/utils';
import { antdRule } from '@/utils/zodForm';
import { setMessageInstance } from '@/utils/messageBus';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import {
  EmailSchema,
  FullNameSchema,
  PasswordSchema,
  PhoneSchema,
  UsernameSchema,
  passwordScore,
  type RegisterFormValues,
} from '@/schemas/register';
import '../login/LoginPage.css';
import './RegisterPage.css';

const basePath = window.X_UI_BASE_PATH || '';
const REDIRECT_DELAY_MS = 1200;

const STRENGTH_COLORS = ['#ff4d4f', '#ff4d4f', '#faad14', '#3b82f6', '#389e0a'];

export default function RegisterPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<RegisterFormValues>();

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const [submitting, setSubmitting] = useState(false);
  const [lang, setLang] = useState<string>(() => LanguageManager.getLanguage());

  const passwordValue = Form.useWatch('password', form) || '';
  const score = useMemo(() => passwordScore(passwordValue), [passwordValue]);
  const strengthLabel = useMemo(() => {
    const labels = [
      'pages.register.strengthWeak',
      'pages.register.strengthWeak',
      'pages.register.strengthFair',
      'pages.register.strengthGood',
      'pages.register.strengthStrong',
    ];
    return t(labels[score]);
  }, [score, t]);

  // When registration is disabled the server never serves this page, but the
  // Vite dev server serves it statically — guard here too so the dev flow and
  // any cached page redirect to login.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.post('/getRegistrationEnable', undefined, { silent: true });
      if (cancelled) return;
      if (!(msg.success && msg.obj)) {
        window.location.replace(basePath || '/');
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const onSubmit = useCallback(async (values: RegisterFormValues) => {
    setSubmitting(true);
    try {
      const payload = {
        fullName: values.fullName.trim(),
        phone: values.phone.trim(),
        email: values.email.trim(),
        username: values.username.trim(),
        password: values.password,
        confirmPassword: values.confirmPassword,
      };
      const msg = await HttpUtil.post('/register', payload);
      if (msg.success) {
        window.setTimeout(() => {
          window.location.href = basePath || '/';
        }, REDIRECT_DELAY_MS);
      } else {
        setSubmitting(false);
      }
    } catch {
      setSubmitting(false);
    }
  }, []);

  const onLangChange = useCallback((next: string) => {
    setLang(next);
    LanguageManager.setLanguage(next);
  }, []);

  const cycleTheme = useCallback(() => {
    pauseAnimationsUntilLeave('register-theme-cycle');
    if (!isDark) {
      toggleTheme();
      if (isUltra) toggleUltra();
    } else if (!isUltra) {
      toggleUltra();
    } else {
      toggleUltra();
      toggleTheme();
    }
  }, [isDark, isUltra, toggleTheme, toggleUltra]);

  const pageClass = useMemo(() => {
    const classes = ['login-app'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const langMenuItems = useMemo(
    () => (LanguageManager.supportedLanguages as { value: string; name: string; icon: string }[]).map((l) => ({
      key: l.value,
      label: (
        <Space size={8}>
          <span aria-hidden="true">{l.icon}</span>
          <span>{l.name}</span>
        </Space>
      ),
    })),
    [],
  );

  const themeIcon = !isDark ? <SunOutlined /> : !isUltra ? <MoonOutlined /> : <MoonFilled />;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <Layout.Content className="login-content">
          <div className="login-toolbar">
            <Button
              id="register-theme-cycle"
              shape="circle"
              size="large"
              className="toolbar-btn"
              aria-label={t('menu.theme')}
              title={t('menu.theme')}
              icon={themeIcon}
              onClick={cycleTheme}
            />
            <Popover
              rootClassName={isDark ? 'dark' : 'light'}
              placement="bottomRight"
              trigger="click"
              styles={{ content: { padding: 4 } }}
              content={
                <Menu
                  mode="vertical"
                  selectable
                  selectedKeys={[lang]}
                  items={langMenuItems}
                  onClick={({ key }) => onLangChange(key)}
                  style={{ border: 'none', minWidth: 160 }}
                />
              }
            >
              <Button
                shape="circle"
                size="large"
                className="toolbar-btn"
                aria-label={t('pages.settings.language')}
                icon={<TranslationOutlined />}
              />
            </Popover>
          </div>

          <div className="login-wrapper">
            <div className="login-card register-card">
              <div className="brand">
                <span className="brand-name">3X-UI</span>
                <span className="brand-accent" aria-hidden="true" />
              </div>
              <h2 className="welcome register-welcome">
                <b>{t('pages.register.title')}</b>
              </h2>
              <p className="register-subtitle">{t('pages.register.subtitle')}</p>

              <Form
                form={form}
                layout="vertical"
                className="login-form"
                onFinish={onSubmit}
                requiredMark={false}
                initialValues={{
                  fullName: '',
                  phone: '',
                  email: '',
                  username: '',
                  password: '',
                  confirmPassword: '',
                }}
              >
                <Form.Item
                  label={t('fullName')}
                  name="fullName"
                  rules={[antdRule(FullNameSchema, t)]}
                >
                  <Input
                    prefix={<UserOutlined />}
                    size="large"
                    autoComplete="name"
                    placeholder={t('pages.register.placeholders.fullName')}
                    autoFocus
                  />
                </Form.Item>

                <Form.Item
                  label={t('phoneNumber')}
                  name="phone"
                  rules={[antdRule(PhoneSchema, t)]}
                >
                  <Input
                    prefix={<PhoneOutlined />}
                    size="large"
                    autoComplete="tel"
                    inputMode="tel"
                    placeholder={t('pages.register.placeholders.phone')}
                  />
                </Form.Item>

                <Form.Item
                  label={t('email')}
                  name="email"
                  rules={[antdRule(EmailSchema, t)]}
                >
                  <Input
                    prefix={<MailOutlined />}
                    size="large"
                    autoComplete="email"
                    inputMode="email"
                    placeholder={t('pages.register.placeholders.email')}
                  />
                </Form.Item>

                <Form.Item
                  label={t('username')}
                  name="username"
                  rules={[antdRule(UsernameSchema, t)]}
                >
                  <Input
                    prefix={<UserOutlined />}
                    size="large"
                    autoComplete="username"
                    placeholder={t('pages.register.placeholders.username')}
                  />
                </Form.Item>

                <Form.Item
                  label={t('password')}
                  name="password"
                  rules={[antdRule(PasswordSchema, t)]}
                >
                  <Input.Password
                    prefix={<LockOutlined />}
                    size="large"
                    autoComplete="new-password"
                    placeholder={t('pages.register.placeholders.password')}
                  />
                </Form.Item>

                {passwordValue ? (
                  <div className="password-strength" aria-live="polite">
                    <Progress
                      percent={(score / 4) * 100}
                      showInfo={false}
                      size="small"
                      strokeColor={STRENGTH_COLORS[score]}
                    />
                    <span className="password-strength-label">
                      {t('pages.register.passwordStrength')}: {strengthLabel}
                    </span>
                  </div>
                ) : null}

                <Form.Item
                  label={t('confirmPassword')}
                  name="confirmPassword"
                  dependencies={['password']}
                  rules={[
                    { required: true, message: t('pages.register.errors.confirmPassword') },
                    ({ getFieldValue }) => ({
                      validator(_rule, value) {
                        if (!value || getFieldValue('password') === value) {
                          return Promise.resolve();
                        }
                        return Promise.reject(new Error(t('pages.register.errors.confirmPassword')));
                      },
                    }),
                  ]}
                >
                  <Input.Password
                    prefix={<LockOutlined />}
                    size="large"
                    autoComplete="new-password"
                    placeholder={t('pages.register.placeholders.confirmPassword')}
                  />
                </Form.Item>

                <Form.Item className="submit-row">
                  <Button
                    type="primary"
                    htmlType="submit"
                    loading={submitting}
                    size="large"
                    block
                  >
                    {t('pages.register.submit')}
                  </Button>
                </Form.Item>
              </Form>

              <div className="register-footer">
                <span>{t('pages.register.haveAccount')}</span>{' '}
                <a href={basePath || '/'}>{t('pages.register.backToLogin')}</a>
              </div>
            </div>
          </div>
        </Layout.Content>
      </Layout>
    </ConfigProvider>
  );
}
