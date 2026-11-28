import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router';
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
  message,
} from 'antd';

import { HttpUtil, PromiseUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { AllSettingSchema } from '@/schemas/setting';
import { PanelRestartSchema } from '@/generated/zod';
import AppSidebar from '@/layouts/AppSidebar';
import GeneralTab from './GeneralTab';
import SecurityTab from './SecurityTab';
import TelegramTab from './TelegramTab';
import EmailTab from './EmailTab';
import DiscordTab from './DiscordTab';
import WechatTab from './WechatTab';
import SubscriptionGeneralTab from './SubscriptionGeneralTab';
import SubscriptionFormatsTab from './SubscriptionFormatsTab';
import SubscriptionBalancersTab from './SubscriptionBalancersTab';
import { panelUrlAfterRestart } from './restart-url';
import './SettingsPage.css';

interface ApiMsg {
  success?: boolean;
  obj?: unknown;
}

const tabSlugs = [
  'general',
  'security',
  'telegram',
  'email',
  'discord',
  'wechat',
  'subscription',
  'subscription-formats',
  'subscription-balancers',
];

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
    savePayload,
  } = useAllSettings();

  const [alertVisible, setAlertVisible] = useState(true);
  const location = useLocation();
  const slug = location.hash.replace(/^#/, '');
  const activeSlug = tabSlugs.includes(slug) ? slug : 'general';

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
          const msg = (await HttpUtil.post('/panel/api/setting/restartPanel')) as ApiMsg;
          if (!msg?.success) return;
          const restart = PanelRestartSchema.safeParse(msg.obj);
          const target = panelUrlAfterRestart(window.location.href, restart.data, allSetting);
          await PromiseUtil.waitForPanel(target);
          window.location.replace(target);
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
        try {
          subPath = new URL(allSetting.subURI).pathname;
        } catch {
          /* noop */
        }
      }
      if (subPath === '/sub/') {
        out.push(t('pages.settings.warnDefaultSubPath'));
      }
    }
    if (allSetting.subJsonEnable) {
      let p = allSetting.subJsonPath;
      if (allSetting.subJsonURI) {
        try {
          p = new URL(allSetting.subJsonURI).pathname;
        } catch {
          /* noop */
        }
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

  const categoryBody = useMemo(() => {
    switch (activeSlug) {
      case 'security':
        return (
          <SecurityTab
            allSetting={allSetting}
            updateSetting={updateSetting}
            saveSetting={savePayload}
          />
        );
      case 'telegram':
        return <TelegramTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'email':
        return <EmailTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'discord':
        return <DiscordTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'wechat':
        return (
          <WechatTab
            allSetting={allSetting}
            updateSetting={updateSetting}
            hasPendingChanges={!saveDisabled}
          />
        );
      case 'subscription':
        return <SubscriptionGeneralTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'subscription-formats':
        return <SubscriptionFormatsTab allSetting={allSetting} updateSetting={updateSetting} />;
      case 'subscription-balancers':
        return <SubscriptionBalancersTab allSetting={allSetting} updateSetting={updateSetting} />;
      default:
        return <GeneralTab allSetting={allSetting} updateSetting={updateSetting} />;
    }
  }, [activeSlug, allSetting, updateSetting, savePayload, saveDisabled]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin
              aria-live="off"
              spinning={spinning || !fetched}
              delay={200}
              description={t('loading')}
              size="large"
            >
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
                      description={
                        <>
                          <b>{t('pages.settings.panelExposed')}</b>
                          <ul>
                            {confAlerts.map((msg, i) => (
                              <li key={i}>{msg}</li>
                            ))}
                          </ul>
                        </>
                      }
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
                              <Button
                                type="primary"
                                danger
                                disabled={!saveDisabled}
                                onClick={restartPanel}
                              >
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
                      <Card hoverable>{categoryBody}</Card>
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
