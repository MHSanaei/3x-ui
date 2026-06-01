import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  FloatButton,
  Layout,
  Modal,
  Row,
  Space,
  Spin,
  Tabs,
  Tooltip,
  message,
} from 'antd';
import {
  CloudServerOutlined,
  CodeOutlined,
  MessageOutlined,
  SafetyOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import { HttpUtil, PromiseUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { AllSettingSchema } from '@/schemas/setting';
import AppSidebar from '@/layouts/AppSidebar';
import GeneralTab from './GeneralTab';
import SecurityTab from './SecurityTab';
import TelegramTab from './TelegramTab';
import SubscriptionGeneralTab from './SubscriptionGeneralTab';
import SubscriptionFormatsTab from './SubscriptionFormatsTab';
import './SettingsPage.css';

interface ApiMsg {
  success?: boolean;
}

const tabSlugs = ['general', 'security', 'telegram', 'subscription', 'subscription-formats'];

function slugToKey(slug: string): string {
  const i = tabSlugs.indexOf(slug);
  return i >= 0 ? String(i + 1) : '1';
}

function keyToSlug(key: string): string {
  return tabSlugs[Number(key) - 1] || tabSlugs[0];
}

function isIp(h: string): boolean {
  if (typeof h !== 'string') return false;
  const v4 = h.split('.');
  if (v4.length === 4 && v4.every((p) => /^\d{1,3}$/.test(p) && Number(p) <= 255)) return true;
  if (!h.includes(':') || h.includes(':::')) return false;
  const parts = h.split('::');
  if (parts.length > 2) return false;
  const split = (s: string) => (s ? s.split(':').filter(Boolean) : []);
  const head = split(parts[0]);
  const tail = split(parts[1]);
  const valid = (seg: string) => /^[0-9a-fA-F]{1,4}$/.test(seg);
  if (![...head, ...tail].every(valid)) return false;
  const groups = head.length + tail.length;
  return parts.length === 2 ? groups < 8 : groups === 8;
}

function scrollTarget() {
  return document.getElementById('content-layout') as HTMLElement;
}

export default function SettingsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const {
    allSetting,
    updateSetting,
    fetched,
    spinning,
    setSpinning,
    saveDisabled,
    saveAll,
  } = useAllSettings();

  const [entryHost, setEntryHost] = useState('');
  const [entryPort, setEntryPort] = useState('');
  const [entryIsIP, setEntryIsIP] = useState(false);

  useEffect(() => {
     
    const host = window.location.hostname;
    setEntryHost(host);
    setEntryPort(window.location.port);
    setEntryIsIP(isIp(host));
     
  }, []);

  const [alertVisible, setAlertVisible] = useState(true);
  const [activeTabKey, setActiveTabKey] = useState<string>(() => slugToKey(window.location.hash.slice(1)));

  useEffect(() => {
    const onHashChange = () => setActiveTabKey(slugToKey(window.location.hash.slice(1)));
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  function onTabChange(key: string) {
    setActiveTabKey(key);
    const slug = keyToSlug(key);
    if (window.location.hash !== `#${slug}`) {
      history.replaceState(null, '', `#${slug}`);
    }
  }

  function rebuildUrlAfterRestart(): string {
    const { webDomain, webPort, webBasePath, webCertFile, webKeyFile } = allSetting;
    const newProtocol = (webCertFile || webKeyFile) ? 'https:' : 'http:';

    let base = webBasePath ? webBasePath.replace(/^\//, '') : '';
    if (base && !base.endsWith('/')) base += '/';

    if (!entryIsIP) {
      const url = new URL(window.location.href);
      url.pathname = `/${base}panel/settings`;
      url.protocol = newProtocol;
      return url.toString();
    }

    let finalHost = entryHost;
    let finalPort = entryPort || '';
    if (webDomain && isIp(webDomain)) finalHost = webDomain;
    if (webPort && Number(webPort) !== Number(entryPort)) finalPort = String(webPort);

    const url = new URL(`${newProtocol}//${finalHost}`);
    if (finalPort) url.port = finalPort;
    url.pathname = `/${base}panel/settings`;
    return url.toString();
  }

  async function onSave() {
    const result = AllSettingSchema.safeParse(allSetting);
    if (!result.success) {
      const issue = result.error.issues[0];
      const fieldPath = issue?.path.join('.') ?? 'value';
      const msgKey = issue?.message ?? 'somethingWentWrong';
      messageApi.error(`${fieldPath}: ${t(msgKey, { defaultValue: msgKey })}`);
      return;
    }
    await saveAll();
  }

  function restartPanel() {
    modal.confirm({
      title: t('pages.settings.restartPanel'),
      content: t('pages.settings.restartPanelDesc'),
      okText: t('pages.settings.restartPanel'),
      okButtonProps: { danger: true },
      cancelText: t('cancel'),
      onOk: async () => {
        setSpinning(true);
        try {
          const msg = await HttpUtil.post('/panel/setting/restartPanel') as ApiMsg;
          if (!msg?.success) return;
          await PromiseUtil.sleep(5000);
          window.location.replace(rebuildUrlAfterRestart());
        } finally {
          setSpinning(false);
        }
      },
    });
  }

  const confAlerts = useMemo<string[]>(() => {
    const out: string[] = [];
    if (window.location.protocol !== 'https:') {
      out.push(t('pages.settings.warnHttp'));
    }
    if (allSetting.webPort === 2053) {
      out.push(t('pages.settings.warnDefaultPort'));
    }
    const segs = window.location.pathname.split('/').length < 4;
    if (segs && allSetting.webBasePath === '/') {
      out.push(t('pages.settings.warnDefaultBasePath'));
    }
    if (allSetting.subEnable) {
      let subPath = allSetting.subPath;
      if (allSetting.subURI) {
        try { subPath = new URL(allSetting.subURI).pathname; } catch { /* noop */ }
      }
      if (subPath === '/sub/') {
        out.push(t('pages.settings.warnDefaultSubPath'));
      }
    }
    if (allSetting.subJsonEnable) {
      let p = allSetting.subJsonPath;
      if (allSetting.subJsonURI) {
        try { p = new URL(allSetting.subJsonURI).pathname; } catch { /* noop */ }
      }
      if (p === '/json/') {
        out.push(t('pages.settings.warnDefaultJsonPath'));
      }
    }
    return out;
  }, [allSetting, t]);

  const pageClass = useMemo(() => {
    const classes = ['settings-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const tabItems = useMemo(() => {
    const items: { key: string; label: React.ReactNode; children: React.ReactNode }[] = [
      {
        key: '1',
        label: (
          <Tooltip title={isMobile ? t('pages.settings.panelSettings') : null}>
            <span><SettingOutlined />{!isMobile && <> {t('pages.settings.panelSettings')}</>}</span>
          </Tooltip>
        ),
        children: <GeneralTab allSetting={allSetting} updateSetting={updateSetting} />,
      },
      {
        key: '2',
        label: (
          <Tooltip title={isMobile ? t('pages.settings.securitySettings') : null}>
            <span><SafetyOutlined />{!isMobile && <> {t('pages.settings.securitySettings')}</>}</span>
          </Tooltip>
        ),
        children: <SecurityTab allSetting={allSetting} updateSetting={updateSetting} />,
      },
      {
        key: '3',
        label: (
          <Tooltip title={isMobile ? t('pages.settings.TGBotSettings') : null}>
            <span><MessageOutlined />{!isMobile && <> {t('pages.settings.TGBotSettings')}</>}</span>
          </Tooltip>
        ),
        children: <TelegramTab allSetting={allSetting} updateSetting={updateSetting} />,
      },
      {
        key: '4',
        label: (
          <Tooltip title={isMobile ? t('pages.settings.subSettings') : null}>
            <span><CloudServerOutlined />{!isMobile && <> {t('pages.settings.subSettings')}</>}</span>
          </Tooltip>
        ),
        children: <SubscriptionGeneralTab allSetting={allSetting} updateSetting={updateSetting} />,
      },
    ];
    if (allSetting.subJsonEnable || allSetting.subClashEnable) {
      items.push({
        key: '5',
        label: (
          <Tooltip title={isMobile ? `${t('pages.settings.subSettings')} (Formats)` : null}>
            <span><CodeOutlined />{!isMobile && <> {t('pages.settings.subSettings')} (Formats)</>}</span>
          </Tooltip>
        ),
        children: <SubscriptionFormatsTab allSetting={allSetting} updateSetting={updateSetting} />,
      });
    }
    return items;
  }, [allSetting, updateSetting, isMobile, t]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={spinning || !fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : (
                <>
                  {confAlerts.length > 0 && alertVisible && (
                    <Alert
                      type="error"
                      showIcon
                      closable={{ onClose: () => setAlertVisible(false) }}
                      className="conf-alert"
                      title={t('pages.settings.securityWarnings')}
                      description={(
                        <>
                          <b>{t('pages.settings.panelExposed')}</b>
                          <ul>
                            {confAlerts.map((msg, i) => <li key={i}>{msg}</li>)}
                          </ul>
                        </>
                      )}
                    />
                  )}

                  <Row gutter={[isMobile ? 8 : 16, isMobile ? 0 : 12]}>
                    <Col span={24}>
                      <Card hoverable>
                        <Row className="header-row">
                          <Col xs={24} sm={10} className="header-actions">
                            <Space>
                              <Button type="primary" disabled={saveDisabled} onClick={onSave}>
                                {t('pages.settings.save')}
                              </Button>
                              <Button type="primary" danger disabled={!saveDisabled} onClick={restartPanel}>
                                {t('pages.settings.restartPanel')}
                              </Button>
                            </Space>
                          </Col>
                          <Col xs={24} sm={14} className="header-info">
                            <FloatButton.BackTop target={scrollTarget} visibilityHeight={200} />
                            <Alert type="warning" showIcon title={t('pages.settings.infoDesc')} />
                          </Col>
                        </Row>
                      </Card>
                    </Col>

                    <Col span={24}>
                      <Card hoverable>
                        <Tabs
                          activeKey={activeTabKey}
                          onChange={onTabChange}
                          className={isMobile ? 'icons-only' : ''}
                          items={tabItems}
                        />
                      </Card>
                    </Col>
                  </Row>
                </>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
