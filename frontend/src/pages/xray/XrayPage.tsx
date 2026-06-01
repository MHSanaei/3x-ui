import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  FloatButton,
  Layout,
  message,
  Modal,
  Popover,
  Radio,
  Result,
  Row,
  Space,
  Spin,
  Tabs,
  Tooltip,
} from 'antd';
import {
  SettingOutlined,
  SwapOutlined,
  UploadOutlined,
  ClusterOutlined,
  DatabaseOutlined,
  CodeOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useXraySetting } from '@/hooks/useXraySetting';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import AppSidebar from '@/layouts/AppSidebar';
import { JsonEditor } from '@/components/form';
import { setMessageInstance } from '@/utils/messageBus';

import { BasicsTab } from './basics';
import { RoutingTab } from './routing';
import { OutboundsTab } from './outbounds';
import { BalancersTab } from './balancers';
import { DnsTab } from './dns';
import { WarpModal, NordModal } from './overrides';
import './XrayPage.css';

const TAB_KEYS = ['tpl-basic', 'tpl-routing', 'tpl-outbound', 'tpl-balancer', 'tpl-dns', 'tpl-advanced'];
const SLUG_BY_KEY: Record<string, string> = {
  'tpl-basic': 'basic',
  'tpl-routing': 'routing',
  'tpl-outbound': 'outbound',
  'tpl-balancer': 'balancer',
  'tpl-dns': 'dns',
  'tpl-advanced': 'advanced',
};
const KEY_BY_SLUG: Record<string, string> = Object.fromEntries(
  Object.entries(SLUG_BY_KEY).map(([k, v]) => [v, k]),
);

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
    restartResult,
    outboundsTraffic,
    outboundTestStates,
    testingAll,
    fetchAll,
    resetOutboundsTraffic,
    testOutbound,
    testAllOutbounds,
    saveAll,
    resetToDefault,
    restartXray,
  } = xs;

  const [modal, modalContextHolder] = Modal.useModal();
  const [warpOpen, setWarpOpen] = useState(false);
  const [nordOpen, setNordOpen] = useState(false);
  const [advSettings, setAdvSettings] = useState<AdvKey>('xraySetting');
  const [activeTabKey, setActiveTabKey] = useState(() => {
    const slug = window.location.hash.slice(1);
    return KEY_BY_SLUG[slug] || TAB_KEYS[0];
  });

  useEffect(() => {
    function syncTabFromHash() {
      const key = KEY_BY_SLUG[window.location.hash.slice(1)];
      if (key) setActiveTabKey(key);
    }
    window.addEventListener('hashchange', syncTabFromHash);
    return () => window.removeEventListener('hashchange', syncTabFromHash);
  }, []);

  function onTabChange(key: string) {
    setActiveTabKey(key);
    const slug = SLUG_BY_KEY[key];
    if (slug && window.location.hash !== `#${slug}`) {
      history.replaceState(null, '', `#${slug}`);
    }
  }

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

  const warpExist = !!templateSettings?.outbounds?.find((o) => o?.tag === 'warp');
  const nordExist = !!templateSettings?.outbounds?.find((o) => o?.tag?.startsWith?.('nord-'));

  async function onTestOutbound(idx: number, mode: string) {
    const outbound = templateSettings?.outbounds?.[idx];
    if (outbound) await testOutbound(idx, outbound, mode);
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
      if (payload.oldTag && payload.newTag && payload.oldTag !== payload.newTag) {
        const rules = tt.routing?.rules || [];
        for (const r of rules) {
          if (r?.outboundTag === payload.oldTag) r.outboundTag = payload.newTag;
        }
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

  function confirmRestart() {
    modal.confirm({
      title: t('pages.xray.restartConfirmTitle'),
      content: t('pages.xray.restartConfirmContent'),
      okText: t('pages.xray.restart'),
      cancelText: t('cancel'),
      onOk: () => restartXray(),
    });
  }

  function onSaveAll() {
    try {
      JSON.parse(xraySetting);
    } catch (e) {
      messageApi.error(`Advanced JSON: ${(e as Error).message}`);
      setActiveTabKey('tpl-advanced');
      return;
    }
    saveAll();
  }

  const scrollTarget = () => document.getElementById('content-layout') || window;

  const pageClass = `xray-page ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`.trim();

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
                            <Button type="primary" danger disabled={!saveDisabled} onClick={confirmRestart}>
                              {t('pages.xray.restart')}
                            </Button>
                            {restartResult && (
                              <Popover
                                placement="rightTop"
                                title={t('pages.xray.restartOutputTitle')}
                                content={<pre className="restart-result">{restartResult}</pre>}
                              >
                                <QuestionCircleOutlined className="restart-icon" />
                              </Popover>
                            )}
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
                    <Tabs
                      activeKey={activeTabKey}
                      onChange={onTabChange}
                      className={isMobile ? 'icons-only' : ''}
                      items={[
                        {
                          key: 'tpl-basic',
                          label: (
                            <Tooltip title={isMobile ? t('pages.xray.basicTemplate') : ''}>
                              <SettingOutlined />
                              {!isMobile && <span>{` ${t('pages.xray.basicTemplate')}`}</span>}
                            </Tooltip>
                          ),
                          children: (
                            <BasicsTab
                              templateSettings={templateSettings}
                              setTemplateSettings={setTemplateSettings}
                              outboundTestUrl={outboundTestUrl}
                              onChangeOutboundTestUrl={setOutboundTestUrl}
                              warpExist={warpExist}
                              nordExist={nordExist}
                              onShowWarp={() => setWarpOpen(true)}
                              onShowNord={() => setNordOpen(true)}
                              onResetDefault={resetToDefault}
                            />
                          ),
                        },
                        {
                          key: 'tpl-routing',
                          label: (
                            <Tooltip title={isMobile ? t('pages.xray.Routings') : ''}>
                              <SwapOutlined />
                              {!isMobile && <span>{` ${t('pages.xray.Routings')}`}</span>}
                            </Tooltip>
                          ),
                          children: (
                            <RoutingTab
                              templateSettings={templateSettings}
                              setTemplateSettings={setTemplateSettings}
                              inboundTags={inboundTags}
                              clientReverseTags={clientReverseTags}
                              isMobile={isMobile}
                            />
                          ),
                        },
                        {
                          key: 'tpl-outbound',
                          label: (
                            <Tooltip title={isMobile ? t('pages.xray.Outbounds') : ''}>
                              <UploadOutlined />
                              {!isMobile && <span>{` ${t('pages.xray.Outbounds')}`}</span>}
                            </Tooltip>
                          ),
                          children: (
                            <OutboundsTab
                              templateSettings={templateSettings}
                              setTemplateSettings={setTemplateSettings}
                              outboundsTraffic={outboundsTraffic}
                              outboundTestStates={outboundTestStates}
                              testingAll={testingAll}
                              inboundTags={inboundTags}
                              isMobile={isMobile}
                              onResetTraffic={resetOutboundsTraffic}
                              onTest={onTestOutbound}
                              onTestAll={testAllOutbounds}
                              onShowWarp={() => setWarpOpen(true)}
                              onShowNord={() => setNordOpen(true)}
                            />
                          ),
                        },
                        {
                          key: 'tpl-balancer',
                          label: (
                            <Tooltip title={isMobile ? t('pages.xray.Balancers') : ''}>
                              <ClusterOutlined />
                              {!isMobile && <span>{` ${t('pages.xray.Balancers')}`}</span>}
                            </Tooltip>
                          ),
                          children: (
                            <BalancersTab
                              templateSettings={templateSettings}
                              setTemplateSettings={setTemplateSettings}
                              clientReverseTags={clientReverseTags}
                              isMobile={isMobile}
                            />
                          ),
                        },
                        {
                          key: 'tpl-dns',
                          label: (
                            <Tooltip title={isMobile ? 'DNS' : ''}>
                              <DatabaseOutlined />
                              {!isMobile && <span> DNS</span>}
                            </Tooltip>
                          ),
                          children: (
                            <DnsTab
                              templateSettings={templateSettings}
                              setTemplateSettings={setTemplateSettings}
                            />
                          ),
                        },
                        {
                          key: 'tpl-advanced',
                          label: (
                            <Tooltip title={isMobile ? t('pages.xray.advancedTemplate') : ''}>
                              <CodeOutlined />
                              {!isMobile && <span>{` ${t('pages.xray.advancedTemplate')}`}</span>}
                            </Tooltip>
                          ),
                          children: (
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
                          ),
                        },
                      ]}
                    />
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
