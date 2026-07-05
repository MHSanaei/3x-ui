import type { ReactNode } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Input,
  Layout,
  Row,
  Space,
  Spin,
  Switch,
  Tabs,
  message,
} from 'antd';
import {
  KeyOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  RobotOutlined,
  SafetyOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useTgBot } from '@/hooks/useTgBot';
import AppSidebar from '@/layouts/AppSidebar';
import './TgBotPage.css';

// Известные поля .env, для которых делаем удобные формы вместо "сырого" редактора.
// Ключи должны совпадать 1:1 с тем, что реально лежит в .env бота.
const KNOWN_FIELDS: { key: string; labelKey: string; icon: ReactNode; secret?: boolean }[] = [
  { key: 'BOT_TOKEN', labelKey: 'pages.tgbot.botToken', icon: <KeyOutlined />, secret: true },
  { key: 'ADMIN_IDS', labelKey: 'pages.tgbot.adminIds', icon: <SafetyOutlined /> },
];

// Маленький живой индикатор рядом с переключателем "Live" —
// просто пульсирующая точка, чистый CSS, без лишней библиотеки.
function LiveDot() {
  return <span className="tgbot-live-dot" />;
}

export default function TgBotPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const {
    running,
    statusLoading,
    actionLoading,
    start,
    stop,
    restart,
    envData,
    envLoading,
    saveEnvValues,
    getEnvRaw,
    saveEnvRaw,
    dependencies,
    depsLoading,
    installed,
    installing,
    installLog,
    installBot,
    logs,
    logsLoading,
    refreshLogs,
    liveLines,
    streaming,
    startLogStream,
    stopLogStream,
  } = useTgBot();

  const [form] = Form.useForm();
  const [savingFields, setSavingFields] = useState(false);
  const [rawContent, setRawContent] = useState('');
  const [rawLoaded, setRawLoaded] = useState(false);
  const [savingRaw, setSavingRaw] = useState(false);
  const [logsLoaded, setLogsLoaded] = useState(false);
  const logsBoxRef = useRef<HTMLDivElement | null>(null);

  const pageClass = useMemo(() => {
    const classes = ['tgbot-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const initialValues = useMemo(() => {
    const out: Record<string, string> = {};
    for (const f of KNOWN_FIELDS) out[f.key] = envData.values[f.key] ?? '';
    return out;
  }, [envData]);

  // Автоскролл живых логов к последней строке.
  useEffect(() => {
    if (logsBoxRef.current) {
      logsBoxRef.current.scrollTop = logsBoxRef.current.scrollHeight;
    }
  }, [liveLines]);

  // Останавливаем стрим, если пользователь ушёл со страницы
  // (доп. защита сверх unmount-очистки внутри самого хука).
  useEffect(() => () => stopLogStream(), [stopLogStream]);

  async function onSaveFields() {
    const values = await form.validateFields();
    setSavingFields(true);
    try {
      const res = await saveEnvValues(values);
      if (res?.success) {
        message.success(t('pages.tgbot.envSaved'));
      } else {
        message.error(res?.msg || t('somethingWentWrong'));
      }
    } finally {
      setSavingFields(false);
    }
  }

  async function onOpenRawTab() {
    if (rawLoaded) return;
    const content = await getEnvRaw();
    setRawContent(content);
    setRawLoaded(true);
  }

  async function onSaveRaw() {
    setSavingRaw(true);
    try {
      const res = await saveEnvRaw(rawContent);
      if (res?.success) {
        message.success(t('pages.tgbot.envSaved'));
      } else {
        message.error(res?.msg || t('somethingWentWrong'));
      }
    } finally {
      setSavingRaw(false);
    }
  }

  async function onAction(action: 'start' | 'stop' | 'restart') {
    const fn = action === 'start' ? start : action === 'stop' ? stop : restart;
    const res = await fn();
    if (res?.success) {
      message.success(t(`pages.tgbot.${action}Success`));
    } else {
      message.error(res?.msg || t('somethingWentWrong'));
    }
  }

  async function onInstall() {
    const res = await installBot();
    if (res?.success) {
      message.success(t('pages.tgbot.installSuccess'));
    } else {
      message.error(t('pages.tgbot.installFailed'));
    }
  }

  function onSettingsTabChange(key: string) {
    if (key === 'raw') onOpenRawTab();
    if (key === 'logs' && !logsLoaded) {
      refreshLogs();
      setLogsLoaded(true);
    }
    // Ушли со вкладки логов — гасим живой стрим, чтобы не висело
    // открытое SSE-соединение впустую.
    if (key !== 'logs' && streaming) {
      stopLogStream();
    }
  }

  function onToggleLive(checked: boolean) {
    if (checked) startLogStream();
    else stopLogStream();
  }

  const depsSatisfied = dependencies.length > 0 && dependencies.every((d) => d.available);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <Row gutter={[16, 16]}>
              {/* --- Статус и управление службой --- */}
              <Col span={24}>
                <Card size="small" hoverable className="summary-card">
                  <Row align="middle" gutter={16}>
                    <Col flex="none">
                      <RobotOutlined style={{ fontSize: 28 }} />
                    </Col>
                    <Col flex="auto">
                      <Space direction="vertical" size={0}>
                        <span className="tgbot-title">{t('pages.tgbot.title')}</span>
                        {statusLoading ? (
                          <Spin size="small" />
                        ) : installed === false ? (
                          <Badge status="default" text={t('pages.tgbot.notInstalled')} />
                        ) : (
                          <Badge
                            status={running ? 'success' : 'error'}
                            text={running ? t('pages.tgbot.running') : t('pages.tgbot.stopped')}
                          />
                        )}
                      </Space>
                    </Col>
                    <Col flex="none">
                      <Space>
                        <Button
                          icon={<PlayCircleOutlined />}
                          type="primary"
                          disabled={!!running || installed === false}
                          loading={actionLoading === 'start'}
                          onClick={() => onAction('start')}
                        >
                          {t('pages.tgbot.start')}
                        </Button>
                        <Button
                          icon={<PauseCircleOutlined />}
                          danger
                          disabled={!running}
                          loading={actionLoading === 'stop'}
                          onClick={() => onAction('stop')}
                        >
                          {t('pages.tgbot.stop')}
                        </Button>
                        <Button
                          icon={<ReloadOutlined />}
                          disabled={installed === false}
                          loading={actionLoading === 'restart'}
                          onClick={() => onAction('restart')}
                        >
                          {t('pages.tgbot.restart')}
                        </Button>
                      </Space>
                    </Col>
                  </Row>
                </Card>
              </Col>

              {/* --- Установка (показывается только если бот не установлен) --- */}
              {installed === false && (
                <Col span={24}>
                  <Card size="small" hoverable title={t('pages.tgbot.installTitle')}>
                    <Space direction="vertical" style={{ width: '100%' }}>
                      <div className="tgbot-deps-list">
                        {depsLoading ? (
                          <Spin size="small" />
                        ) : (
                          dependencies.map((d) => (
                            <div key={d.name} className="tgbot-dep-row">
                              <Badge status={d.available ? 'success' : 'error'} />
                              <span className="dep-name">{d.name}</span>
                              {d.detail && <span className="dep-detail">{d.detail}</span>}
                            </div>
                          ))
                        )}
                      </div>

                      <span className="tgbot-source-note">
                        {t('pages.tgbot.installSource')}: github.com/KimaruBs/3x-ui
                      </span>

                      <Button
                        type="primary"
                        loading={installing}
                        disabled={!depsSatisfied}
                        onClick={onInstall}
                      >
                        {t('pages.tgbot.installButton')}
                      </Button>

                      {installLog && (
                        <Input.TextArea
                          value={installLog}
                          readOnly
                          autoSize={{ minRows: 6, maxRows: 16 }}
                          className="tgbot-raw-editor"
                        />
                      )}
                    </Space>
                  </Card>
                </Col>
              )}

              {/* --- Настройки бота: базовые поля / сырой .env / логи --- */}
              <Col span={24}>
                <Card size="small" hoverable title={t('pages.tgbot.envSettings')}>
                  <Tabs
                    defaultActiveKey="fields"
                    onChange={onSettingsTabChange}
                    items={[
                      {
                        key: 'fields',
                        label: t('pages.tgbot.basicSettings'),
                        children: (
                          <Spin spinning={envLoading}>
                            <Form
                              form={form}
                              layout="vertical"
                              initialValues={initialValues}
                              key={JSON.stringify(initialValues)}
                            >
                              {KNOWN_FIELDS.map((f) => (
                                <Form.Item key={f.key} name={f.key} label={t(f.labelKey)}>
                                  <Input
                                    prefix={f.icon}
                                    type={f.secret ? 'password' : 'text'}
                                    placeholder={t(f.labelKey)}
                                  />
                                </Form.Item>
                              ))}
                              <Button type="primary" loading={savingFields} onClick={onSaveFields}>
                                {t('save')}
                              </Button>
                            </Form>
                          </Spin>
                        ),
                      },
                      {
                        key: 'raw',
                        label: t('pages.tgbot.rawEnv'),
                        children: (
                          <Space direction="vertical" style={{ width: '100%' }}>
                            <Input.TextArea
                              value={rawContent}
                              onChange={(e) => setRawContent(e.target.value)}
                              autoSize={{ minRows: 12, maxRows: 24 }}
                              spellCheck={false}
                              className="tgbot-raw-editor"
                            />
                            <Button type="primary" loading={savingRaw} onClick={onSaveRaw}>
                              {t('save')}
                            </Button>
                          </Space>
                        ),
                      },
                      {
                        key: 'logs',
                        label: t('pages.tgbot.logs'),
                        children: (
                          <Space direction="vertical" style={{ width: '100%' }}>
                            <div className="tgbot-logs-toolbar">
                              <Space>
                                <Switch
                                  checked={streaming}
                                  onChange={onToggleLive}
                                  disabled={installed === false}
                                />
                                <span>{t('pages.tgbot.liveLogs')}</span>
                                {streaming && <LiveDot />}
                              </Space>
                              {!streaming && (
                                <Button onClick={refreshLogs} loading={logsLoading}>
                                  {t('refresh')}
                                </Button>
                              )}
                            </div>

                            {streaming ? (
                              <div ref={logsBoxRef} className="tgbot-logs-live">
                                {liveLines.length === 0 ? (
                                  <div className="tgbot-logs-waiting">{t('pages.tgbot.waitingForLogs')}</div>
                                ) : (
                                  liveLines.map((line, idx) => (
                                    <div key={idx} className="tgbot-log-line">{line}</div>
                                  ))
                                )}
                              </div>
                            ) : (
                              <Input.TextArea
                                value={logs}
                                readOnly
                                autoSize={{ minRows: 12, maxRows: 24 }}
                                className="tgbot-raw-editor"
                              />
                            )}
                          </Space>
                        ),
                      },
                    ]}
                  />
                </Card>
              </Col>
            </Row>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
