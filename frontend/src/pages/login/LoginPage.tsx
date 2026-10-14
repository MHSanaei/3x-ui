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
  Space,
  Spin,
  message,
} from 'antd';
import {
  KeyOutlined,
  LockOutlined,
  MoonFilled,
  MoonOutlined,
  SendOutlined,
  SunOutlined,
  TranslationOutlined,
  UserOutlined,
} from '@ant-design/icons';

import { FormProvider, useForm } from 'react-hook-form';
import { ClipboardManager, HttpUtil, LanguageManager } from '@/utils';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { setMessageInstance } from '@/utils/messageBus';
import SponsorSlot from '@/components/sponsor/SponsorSlot';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import { LoginFormSchema, TwoFactorCodeSchema, type LoginFormValues } from '@/schemas/login';
import './LoginPage.css';

const HEADLINE_INTERVAL_MS = 2000;
const TELEGRAM_POLL_INTERVAL_MS = 2000;

interface TelegramStatus {
  available: boolean;
  linked: boolean;
  botUsername?: string;
}

interface TelegramChallenge {
  code: string;
  expiresAt: number;
}

type LoginForm = LoginFormValues;

const basePath = window.X_UI_BASE_PATH || '';

export default function LoginPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const [fetched, setFetched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [twoFactorEnable, setTwoFactorEnable] = useState(false);
  const [headlineIndex, setHeadlineIndex] = useState(0);
  const [telegramAvailable, setTelegramAvailable] = useState(false);
  const [telegramBotUsername, setTelegramBotUsername] = useState('');
  const [telegramStarting, setTelegramStarting] = useState(false);
  const [telegramChallenge, setTelegramChallenge] = useState<TelegramChallenge | null>(null);
  const methods = useForm<LoginForm>({
    defaultValues: { username: '', password: '', twoFactorCode: '' },
  });
  const [lang, setLang] = useState<string>(() => LanguageManager.getLanguage());

  const headlineWords = useMemo(() => [t('pages.login.hello'), t('pages.login.title')], [t]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      setHeadlineIndex((i) => (i + 1) % headlineWords.length);
    }, HEADLINE_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [headlineWords.length]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [msg, telegram] = await Promise.all([
        HttpUtil.post('/getTwoFactorEnable'),
        HttpUtil.get<TelegramStatus>('/telegram-auth/status', undefined, { silent: true }),
      ]);
      if (cancelled) return;
      if (msg.success) setTwoFactorEnable(!!msg.obj);
      if (telegram.success && telegram.obj?.available && telegram.obj.linked) {
        setTelegramAvailable(true);
        setTelegramBotUsername(telegram.obj.botUsername || '');
      }
      setFetched(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!telegramChallenge) return;
    let active = true;
    let polling = false;
    const poll = async () => {
      if (polling || !active) return;
      if (Date.now() >= telegramChallenge.expiresAt) {
        setTelegramChallenge(null);
        messageApi.warning(t('pages.login.telegramExpired'));
        return;
      }
      polling = true;
      try {
        const result = await HttpUtil.post<{ pending: boolean }>(
          '/telegram-auth/complete',
          { code: telegramChallenge.code },
          { silent: true },
        );
        if (!active) return;
        if (result.success && result.obj?.pending === false) {
          window.location.href = basePath + 'panel/';
        } else if (!result.success) {
          setTelegramChallenge(null);
          messageApi.error(result.msg || t('pages.login.telegramFailed'));
        }
      } finally {
        polling = false;
      }
    };
    const timer = window.setInterval(() => void poll(), TELEGRAM_POLL_INTERVAL_MS);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [telegramChallenge, messageApi, t]);

  const startTelegramLogin = useCallback(async () => {
    setTelegramStarting(true);
    try {
      const result = await HttpUtil.post<TelegramChallenge>('/telegram-auth/start');
      if (result.success && result.obj?.code) setTelegramChallenge(result.obj);
    } finally {
      setTelegramStarting(false);
    }
  }, []);

  const copyTelegramCommand = useCallback(async () => {
    if (!telegramChallenge) return;
    const copied = await ClipboardManager.copyText(`/login ${telegramChallenge.code}`);
    if (copied) messageApi.success(t('copied'));
    else messageApi.error(t('copyFail'));
  }, [telegramChallenge, messageApi, t]);

  const telegramBotLink =
    telegramChallenge && /^[a-zA-Z0-9_]{5,32}$/.test(telegramBotUsername)
      ? `https://t.me/${telegramBotUsername}?start=login_${telegramChallenge.code}`
      : null;

  const onSubmit = useCallback(async (values: LoginForm) => {
    setSubmitting(true);
    try {
      const msg = await HttpUtil.post('/login', values);
      if (msg.success) window.location.href = basePath + 'panel/';
    } finally {
      setSubmitting(false);
    }
  }, []);

  const onLangChange = useCallback((next: string) => {
    setLang(next);
    LanguageManager.setLanguage(next);
  }, []);

  const cycleTheme = useCallback(() => {
    pauseAnimationsUntilLeave('login-theme-cycle');
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
    () =>
      (LanguageManager.supportedLanguages as { value: string; name: string; icon: string }[]).map(
        (l) => ({
          key: l.value,
          label: (
            <Space size={8}>
              <span aria-hidden="true">{l.icon}</span>
              <span>{l.name}</span>
            </Space>
          ),
        }),
      ),
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
              id="login-theme-cycle"
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
            {!fetched ? (
              <div className="login-loading">
                <Spin size="large" />
              </div>
            ) : (
              <div className="login-card">
                <div className="brand">
                  <span className="brand-name">3X-UI</span>
                  <span className="brand-accent" aria-hidden="true" />
                </div>
                <h2 className="welcome">
                  <b key={headlineIndex}>{headlineWords[headlineIndex]}</b>
                </h2>

                <FormProvider {...methods}>
                  <Form
                    layout="vertical"
                    className="login-form"
                    onFinish={methods.handleSubmit(onSubmit)}
                  >
                    <FormField
                      name="username"
                      label={t('username')}
                      rules={{ validate: rhfZodValidate(LoginFormSchema.shape.username) }}
                    >
                      <Input
                        prefix={<UserOutlined />}
                        autoComplete="username"
                        size="large"
                        placeholder={t('username')}
                        autoFocus
                      />
                    </FormField>

                    <FormField
                      name="password"
                      label={t('password')}
                      rules={{ validate: rhfZodValidate(LoginFormSchema.shape.password) }}
                    >
                      <Input.Password
                        prefix={<LockOutlined />}
                        autoComplete="current-password"
                        size="large"
                        placeholder={t('password')}
                      />
                    </FormField>

                    {twoFactorEnable && (
                      <FormField
                        name="twoFactorCode"
                        label={t('twoFactorCode')}
                        rules={{ validate: rhfZodValidate(TwoFactorCodeSchema) }}
                      >
                        <Input
                          prefix={<KeyOutlined />}
                          autoComplete="one-time-code"
                          size="large"
                          placeholder={t('twoFactorCode')}
                        />
                      </FormField>
                    )}

                    <Form.Item className="submit-row">
                      <Button
                        type="primary"
                        htmlType="submit"
                        loading={submitting}
                        size="large"
                        block
                      >
                        {t('login')}
                      </Button>
                    </Form.Item>
                  </Form>
                </FormProvider>
                {telegramAvailable && (
                  <div className="telegram-login">
                    {telegramChallenge ? (
                      <>
                        <p>{t('pages.login.telegramInstruction')}</p>
                        <code className="telegram-login-command">
                          /login {telegramChallenge.code}
                        </code>
                        <Space direction="vertical" style={{ width: '100%' }}>
                          {telegramBotLink && (
                            <Button
                              block
                              type="primary"
                              href={telegramBotLink}
                              target="_blank"
                              rel="noopener noreferrer"
                            >
                              {t('pages.login.telegramOpenBot')}
                            </Button>
                          )}
                          <Button block onClick={copyTelegramCommand}>
                            {t('copy')}
                          </Button>
                        </Space>
                        <p className="telegram-login-waiting">{t('pages.login.telegramWaiting')}</p>
                        <Button block onClick={() => setTelegramChallenge(null)}>
                          {t('cancel')}
                        </Button>
                      </>
                    ) : (
                      <Button
                        block
                        size="large"
                        icon={<SendOutlined />}
                        loading={telegramStarting}
                        onClick={startTelegramLogin}
                      >
                        {t('pages.login.telegramButton')}
                      </Button>
                    )}
                  </div>
                )}
                <SponsorSlot slot="login" variant="compact" className="login-sponsor" />
              </div>
            )}
          </div>
        </Layout.Content>
      </Layout>
    </ConfigProvider>
  );
}
