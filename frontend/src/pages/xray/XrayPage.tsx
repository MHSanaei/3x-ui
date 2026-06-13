import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  FloatButton,
  Layout,
  message,
  Radio,
  Result,
  Row,
  Space,
  Spin,
} from 'antd';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useXraySetting } from '@/hooks/useXraySetting';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import AppSidebar from '@/layouts/AppSidebar';
import { JsonEditor } from '@/components/form';
import { setMessageInstance } from '@/utils/messageBus';

import { BasicsTab } from './basics';
import { propagateOutboundTagRename } from './basics/helpers';
import { RoutingTab } from './routing';
import { OutboundsTab } from './outbounds';
import { BalancersTab } from './balancers';
import { DnsTab } from './dns';
import { WarpModal, NordModal } from './overrides';
import './XrayPage.css';

const SECTION_SLUGS = ['basic', 'routing', 'outbound', 'balancer', 'dns', 'advanced'];

type AdvKey = 'xraySetting' | 'inboundSettings' | 'outboundSettings' | 'routingRuleSettings';

export default function XrayPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);
  const xs = useXraySetting();
  const {
    fetched,
    spinning,
    saveDisabled,
    fetchError,
    xraySetting,
    setXraySetting,
    templateSettings,
    setTemplateSettings,
    outboundTestUrl,
    setOutboundTestUrl,
    inboundTags,
    clientReverseTags,
    subscriptionOutbounds,
    subscriptionOutboundTags,
    outboundsTraffic,
    outboundTestStates,
    subscriptionTestStates,
    testingAll,
    fetchAll,
    resetOutboundsTraffic,
    testOutbound,
    testSubscriptionOutbound,
    testAllOutbounds,
    saveAll,
    resetToDefault,
  } = xs;

  const [warpOpen, setWarpOpen] = useState(false);
  const [nordOpen, setNordOpen] = useState(false);
  const [advSettings, setAdvSettings] = useState<AdvKey>('xraySetting');
  const location = useLocation();
  const navigate = useNavigate();
  const sectionSlug = location.hash.replace(/^#/, '');
  const activeSection = SECTION_SLUGS.includes(sectionSlug) ? sectionSlug : 'basic';

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  async function onTestOutbound(idx: number, mode: string) {
    const outbound = templateSettings?.outbounds?.[idx];
    if (outbound) await testOutbound(idx, outbound, mode);
  }

  async function onTestSubscription(outbound: Record<string, unknown>, mode: string) {
    const tag = typeof outbound?.tag === 'string' ? outbound.tag : '';
    if (tag) await testSubscriptionOutbound(tag, outbound, mode);
  }

  function onAddOutbound(outbound: Record<string, unknown>) {
    mutate((tt) => {
      if (!Array.isArray(tt.outbounds)) tt.outbounds = [];
      tt.outbounds.push(outbound as never);
    });
  }
  function onResetOutbound(payload: { index: number; outbound: Record<string, unknown>; oldTag?: string; newTag?: string }) {
    mutate((tt) => {
      if (!tt.outbounds || payload.index < 0) return;
      tt.outbounds[payload.index] = payload.outbound as never;
      if (payload.oldTag && payload.newTag) {
        propagateOutboundTagRename(tt, payload.oldTag, payload.newTag);
      }
    });
  }
  function onRemoveOutboundByTag(tag: string) {
    mutate((tt) => {
      if (!tt.outbounds) return;
      const idx = tt.outbounds.findIndex((o) => o?.tag === tag);
      if (idx >= 0) tt.outbounds.splice(idx, 1);
    });
  }
  function onRemoveOutboundByIndex(index: number) {
    mutate((tt) => {
      if (tt.outbounds && index >= 0) tt.outbounds.splice(index, 1);
    });
  }
  function onRemoveRoutingRules(payload: { prefix: string }) {
    mutate((tt) => {
      const rules = tt.routing?.rules;
      if (!Array.isArray(rules)) return;
      tt.routing!.rules = rules.filter((r) => !r?.outboundTag?.startsWith?.(payload.prefix));
    });
  }

  const advancedText = useMemo(() => {
    if (advSettings === 'xraySetting') return xraySetting;
    const tpl = templateSettings;
    if (!tpl) return '';
    try {
      switch (advSettings) {
        case 'inboundSettings': return JSON.stringify(tpl.inbounds || [], null, 2);
        case 'outboundSettings': return JSON.stringify(tpl.outbounds || [], null, 2);
        case 'routingRuleSettings': return JSON.stringify(tpl.routing?.rules || [], null, 2);
        default: return '';
      }
    } catch {
      return '';
    }
  }, [advSettings, xraySetting, templateSettings]);

  function onAdvancedTextChange(next: string) {
    if (advSettings === 'xraySetting') {
      setXraySetting(next);
      return;
    }
    let parsed;
    try {
      parsed = JSON.parse(next);
    } catch {
      return;
    }
    mutate((tt) => {
      switch (advSettings) {
        case 'inboundSettings':
          tt.inbounds = parsed;
          break;
        case 'outboundSettings':
          tt.outbounds = parsed;
          break;
        case 'routingRuleSettings':
          if (!tt.routing) tt.routing = {};
          tt.routing.rules = parsed;
          break;
      }
    });
  }

  function onSaveAll() {
    try {
      JSON.parse(xraySetting);
    } catch (e) {
      messageApi.error(`Advanced JSON: ${(e as Error).message}`);
      navigate('/xray#advanced');
      return;
    }
    saveAll();
  }

  const scrollTarget = () => document.getElementById('content-layout') || window;

  const pageClass = `xray-page ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`.trim();

  const sectionBody = (() => {
    switch (activeSection) {
      case 'routing':
        return (
          <RoutingTab
            templateSettings={templateSettings}
            setTemplateSettings={setTemplateSettings}
            inboundTags={inboundTags}
            clientReverseTags={clientReverseTags}
            subscriptionOutboundTags={subscriptionOutboundTags}
            isMobile={isMobile}
          />
        );
      case 'outbound':
        return (
          <OutboundsTab
            templateSettings={templateSettings}
            setTemplateSettings={setTemplateSettings}
            outboundsTraffic={outboundsTraffic}
            outboundTestStates={outboundTestStates}
            subscriptionTestStates={subscriptionTestStates}
            testingAll={testingAll}
            inboundTags={inboundTags}
            subscriptionOutbounds={subscriptionOutbounds}
            isMobile={isMobile}
            onResetTraffic={resetOutboundsTraffic}
            onTest={onTestOutbound}
            onTestSubscription={onTestSubscription}
            onTestAll={testAllOutbounds}
            onShowWarp={() => setWarpOpen(true)}
            onShowNord={() => setNordOpen(true)}
            onRefreshXrayData={fetchAll}
          />
        );
      case 'balancer':
        return (
          <BalancersTab
            templateSettings={templateSettings}
            setTemplateSettings={setTemplateSettings}
            clientReverseTags={clientReverseTags}
            subscriptionOutboundTags={subscriptionOutboundTags}
            isMobile={isMobile}
          />
        );
      case 'dns':
        return (
          <DnsTab
            templateSettings={templateSettings}
            setTemplateSettings={setTemplateSettings}
          />
        );
      case 'advanced':
        return (
          <>
            <div className="advanced-meta">
              <h4>{t('pages.xray.Template')}</h4>
              <p>{t('pages.xray.TemplateDesc')}</p>
            </div>
            <Radio.Group
              value={advSettings}
              buttonStyle="solid"
              size={isMobile ? 'small' : 'middle'}
              style={{ margin: '12px 0' }}
              onChange={(e) => setAdvSettings(e.target.value)}
            >
              <Radio.Button value="xraySetting">{t('pages.xray.completeTemplate')}</Radio.Button>
              <Radio.Button value="inboundSettings">{t('pages.xray.Inbounds')}</Radio.Button>
              <Radio.Button value="outboundSettings">{t('pages.xray.Outbounds')}</Radio.Button>
              <Radio.Button value="routingRuleSettings">{t('pages.xray.Routings')}</Radio.Button>
            </Radio.Group>
            <JsonEditor
              value={advancedText}
              onChange={onAdvancedTextChange}
              minHeight="420px"
              maxHeight="720px"
            />
          </>
        );
      default:
        return (
          <BasicsTab
            templateSettings={templateSettings}
            setTemplateSettings={setTemplateSettings}
            outboundTestUrl={outboundTestUrl}
            onChangeOutboundTestUrl={setOutboundTestUrl}
            onResetDefault={resetToDefault}
          />
        );
    }
  })();

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={spinning || !fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" onClick={fetchAll}>{t('check')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 0 : 12]}>
                  <Col span={24}>
                    <Card hoverable>
                      <Row className="header-row">
                        <Col xs={24} sm={14} className="header-actions">
                          <Space>
                            <Button type="primary" disabled={saveDisabled} onClick={onSaveAll}>
                              {t('pages.xray.save')}
                            </Button>
                          </Space>
                        </Col>
                        <Col xs={24} sm={10} className="header-info">
                          <FloatButton.BackTop target={scrollTarget} visibilityHeight={200} />
                          <Alert type="warning" showIcon title={t('pages.settings.infoDesc')} />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card hoverable>
                      {sectionBody}
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <WarpModal
          open={warpOpen}
          templateSettings={templateSettings}
          onClose={() => setWarpOpen(false)}
          onAddOutbound={onAddOutbound}
          onResetOutbound={onResetOutbound}
          onRemoveOutbound={onRemoveOutboundByTag}
        />
        <NordModal
          open={nordOpen}
          templateSettings={templateSettings}
          onClose={() => setNordOpen(false)}
          onAddOutbound={onAddOutbound}
          onResetOutbound={onResetOutbound}
          onRemoveOutbound={onRemoveOutboundByIndex}
          onRemoveRoutingRules={onRemoveRoutingRules}
        />
      </Layout>
    </ConfigProvider>
  );
}
