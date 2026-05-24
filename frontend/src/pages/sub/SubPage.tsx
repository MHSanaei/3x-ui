import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Descriptions,
  Dropdown,
  Layout,
  message,
  Popover,
  QRCode,
  Row,
  Select,
  Space,
  Tag,
} from 'antd';
import {
  AndroidOutlined,
  AppleOutlined,
  CopyOutlined,
  DownOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import { ClipboardManager, IntlUtil, LanguageManager } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import './SubPage.css';

const QR_SIZE = 240;

const subData = window.__SUB_PAGE_DATA__ || {};

const sId = subData.sId || '';
const enabled = !!subData.enabled;
const download = subData.download || '0';
const upload = subData.upload || '0';
const total = subData.total || '∞';
const used = subData.used || '0';
const remained = subData.remained || '';
const totalByte = Number(subData.totalByte || 0);
const expireMs = Number(subData.expire || 0) * 1000;
const lastOnlineMs = Number(subData.lastOnline || 0);
const subUrl = subData.subUrl || '';
const subJsonUrl = subData.subJsonUrl || '';
const subClashUrl = subData.subClashUrl || '';
const subTitle = subData.subTitle || '';
const links: string[] = Array.isArray(subData.links) ? subData.links : [];
const datepicker = subData.datepicker || 'gregorian';

const isUnlimited = totalByte <= 0 && expireMs === 0;
const isActive = (() => {
  if (!enabled) return false;
  if (totalByte > 0) {
    const usedByteCalc = Number(subData.usedByte || 0)
      || (Number(subData.downloadByte || 0) + Number(subData.uploadByte || 0));
    if (usedByteCalc >= totalByte) return false;
  }
  if (expireMs > 0 && Date.now() >= expireMs) return false;
  return true;
})();

function linkName(link: string, idx: number): string {
  if (!link) return `Link ${idx + 1}`;
  const hashIdx = link.indexOf('#');
  if (hashIdx >= 0 && hashIdx + 1 < link.length) {
    try {
      return decodeURIComponent(link.slice(hashIdx + 1));
    } catch {
      return link.slice(hashIdx + 1);
    }
  }
  const proto = link.split('://')[0];
  return `${proto.toUpperCase()} ${idx + 1}`;
}

export default function SubPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const [isMobile, setIsMobile] = useState<boolean>(() => window.innerWidth < 576);
  const [lang, setLang] = useState<string>(() => LanguageManager.getLanguage());

  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth < 576);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  const onLangChange = useCallback((next: string) => {
    setLang(next);
    LanguageManager.setLanguage(next);
  }, []);

  const cycleTheme = useCallback(() => {
    pauseAnimationsUntilLeave('sub-theme-cycle');
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

  const copy = useCallback(async (value: string) => {
    if (!value) return;
    const ok = await ClipboardManager.copyText(value);
    if (ok) messageApi.success(t('copied'));
  }, [t, messageApi]);

  const open = useCallback((url: string) => {
    if (!url) return;
    window.open(url, '_blank');
  }, []);

  const shadowrocketUrl = useMemo(() => {
    if (!subUrl) return '';
    const separator = subUrl.includes('?') ? '&' : '?';
    const rawUrl = subUrl + separator + 'flag=shadowrocket';
    const base64Url = btoa(rawUrl).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const remark = encodeURIComponent(subTitle || sId || 'Subscription');
    return `shadowrocket://add/sub/${base64Url}?remark=${remark}`;
  }, []);

  const v2boxUrl = useMemo(
    () => `v2box://install-sub?url=${encodeURIComponent(subUrl)}&name=${encodeURIComponent(sId)}`,
    [],
  );
  const streisandUrl = useMemo(() => `streisand://import/${encodeURIComponent(subUrl)}`, []);
  const happUrl = useMemo(() => `happ://add/${subUrl}`, []);

  const pageClass = useMemo(() => {
    const classes = ['subscription-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const descriptionsItems = useMemo(() => {
    const items = [
      { key: 'subId', label: t('subscription.subId'), children: sId },
      {
        key: 'status',
        label: t('subscription.status'),
        children: !enabled
          ? <Tag color="red">{t('subscription.inactive')}</Tag>
          : isUnlimited
            ? <Tag color="purple">{t('subscription.unlimited')}</Tag>
            : <Tag color={isActive ? 'green' : 'red'}>
                {isActive ? t('subscription.active') : t('subscription.inactive')}
              </Tag>,
      },
      { key: 'down', label: t('subscription.downloaded'), children: download },
      { key: 'up', label: t('subscription.uploaded'), children: upload },
      { key: 'used', label: t('usage'), children: used },
      { key: 'total', label: t('subscription.totalQuota'), children: total },
    ];
    if (totalByte > 0) {
      items.push({ key: 'remained', label: t('remained'), children: remained });
    }
    items.push({
      key: 'lastOnline',
      label: t('lastOnline'),
      children: lastOnlineMs > 0 ? IntlUtil.formatDate(lastOnlineMs, datepicker) : '-',
    });
    items.push({
      key: 'expiry',
      label: t('subscription.expiry'),
      children: expireMs === 0
        ? t('subscription.noExpiry')
        : IntlUtil.formatDate(expireMs, datepicker),
    });
    return items;
  }, [t]);

  const androidMenuItems = useMemo(() => [
    {
      key: 'android-v2box',
      label: 'V2Box',
      onClick: () => open(`v2box://install-sub?url=${encodeURIComponent(subUrl)}&name=${encodeURIComponent(sId)}`),
    },
    {
      key: 'android-v2rayng',
      label: 'V2RayNG',
      onClick: () => open(`v2rayng://install-config?url=${encodeURIComponent(subUrl)}`),
    },
    { key: 'android-singbox', label: 'Sing-box', onClick: () => copy(subUrl) },
    { key: 'android-v2raytun', label: 'V2RayTun', onClick: () => copy(subUrl) },
    { key: 'android-npvtunnel', label: 'NPV Tunnel', onClick: () => copy(subUrl) },
    { key: 'android-happ', label: 'Happ', onClick: () => open(`happ://add/${subUrl}`) },
  ], [copy, open]);

  const iosMenuItems = useMemo(() => [
    { key: 'ios-shadowrocket', label: 'Shadowrocket', onClick: () => open(shadowrocketUrl) },
    { key: 'ios-v2box', label: 'V2Box', onClick: () => open(v2boxUrl) },
    { key: 'ios-streisand', label: 'Streisand', onClick: () => open(streisandUrl) },
    { key: 'ios-v2raytun', label: 'V2RayTun', onClick: () => copy(subUrl) },
    { key: 'ios-npvtunnel', label: 'NPV Tunnel', onClick: () => copy(subUrl) },
    { key: 'ios-happ', label: 'Happ', onClick: () => open(happUrl) },
  ], [copy, open, shadowrocketUrl, v2boxUrl, streisandUrl, happUrl]);

  const langOptions = useMemo(
    () => LanguageManager.supportedLanguages.map((l: { value: string; name: string; icon: string }) => ({
      value: l.value,
      label: (
        <>
          <span aria-label={l.name}>{l.icon}</span>
          &nbsp;&nbsp;<span>{l.name}</span>
        </>
      ),
    })),
    [],
  );

  const themeIcon = !isDark ? (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
    </svg>
  ) : !isUltra ? (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  ) : (
    <svg viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
      <path fill="none" d="M19 3l0.7 1.4 1.4 0.7-1.4 0.7L19 7.2l-0.7-1.4-1.4-0.7 1.4-0.7z" />
    </svg>
  );

  const cardTitle = (
    <Space>
      <span>{t('subscription.title')}</span>
      <Tag>{sId}</Tag>
    </Space>
  );

  const cardExtra = (
    <Space size={8} align="center">
      <button
        type="button"
        id="sub-theme-cycle"
        className="theme-cycle"
        aria-label={t('menu.theme')}
        title={t('menu.theme')}
        onClick={cycleTheme}
      >
        {themeIcon}
      </button>
      <Popover
        title={t('pages.settings.language')}
        placement="bottomRight"
        trigger="click"
        content={
          <Space orientation="vertical" size={10} className="settings-popover">
            <Select
              className="lang-select"
              value={lang}
              onChange={onLangChange}
              options={langOptions}
            />
          </Space>
        }
      >
        <Button shape="circle" icon={<SettingOutlined />} />
      </Popover>
    </Space>
  );

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <Layout.Content className="content">
          <Row justify="center">
            <Col xs={24} sm={22} md={18} lg={14} xl={12}>
              <Card hoverable className="subscription-card" title={cardTitle} extra={cardExtra}>
                <Row gutter={[8, 8]} justify="center" className="qr-row">
                  <Col xs={24} sm={subJsonUrl || subClashUrl ? 12 : 24} className="qr-col">
                    <div className="qr-box">
                      <Tag color="purple" className="qr-tag">{t('pages.settings.subSettings')}</Tag>
                      <QRCode
                        className="qr-code"
                        value={subUrl}
                        size={QR_SIZE}
                        type="svg"
                        bordered={false}
                        color="#000000"
                        bgColor="#ffffff"
                        title={t('copy')}
                        onClick={() => copy(subUrl)}
                      />
                    </div>
                  </Col>
                  {subJsonUrl && (
                    <Col xs={24} sm={12} className="qr-col">
                      <div className="qr-box">
                        <Tag color="purple" className="qr-tag">
                          {t('pages.settings.subSettings')} JSON
                        </Tag>
                        <QRCode
                          className="qr-code"
                          value={subJsonUrl}
                          size={QR_SIZE}
                          type="svg"
                          bordered={false}
                          color="#000000"
                          bgColor="#ffffff"
                          title={t('copy')}
                          onClick={() => copy(subJsonUrl)}
                        />
                      </div>
                    </Col>
                  )}
                  {subClashUrl && (
                    <Col xs={24} sm={12} className="qr-col">
                      <div className="qr-box">
                        <Tag color="purple" className="qr-tag">Clash / Mihomo</Tag>
                        <QRCode
                          className="qr-code"
                          value={subClashUrl}
                          size={QR_SIZE}
                          type="svg"
                          bordered={false}
                          color="#000000"
                          bgColor="#ffffff"
                          title={t('copy')}
                          onClick={() => copy(subClashUrl)}
                        />
                      </div>
                    </Col>
                  )}
                </Row>

                <Descriptions
                  bordered
                  column={1}
                  size="small"
                  className="info-table"
                  items={descriptionsItems}
                />

                {links.length > 0 && (
                  <div className="links-section">
                    {links.map((link, idx) => (
                      <div key={link} className="link-row" onClick={() => copy(link)}>
                        <Tag color="purple" className="link-tag">{linkName(link, idx)}</Tag>
                        <div className="link-box">
                          <CopyOutlined className="link-copy-icon" />
                          {link}
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                <Row gutter={[8, 8]} justify="center" className="apps-row">
                  <Col xs={24} sm={12} className="app-col">
                    <Dropdown trigger={['click']} menu={{ items: androidMenuItems }}>
                      <Button block={isMobile} size="large" type="primary">
                        <AndroidOutlined /> Android <DownOutlined />
                      </Button>
                    </Dropdown>
                  </Col>
                  <Col xs={24} sm={12} className="app-col">
                    <Dropdown trigger={['click']} menu={{ items: iosMenuItems }}>
                      <Button block={isMobile} size="large" type="primary">
                        <AppleOutlined /> iOS <DownOutlined />
                      </Button>
                    </Dropdown>
                  </Col>
                </Row>
              </Card>
            </Col>
          </Row>
        </Layout.Content>
      </Layout>
    </ConfigProvider>
  );
}
