import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Descriptions,
  Divider,
  Dropdown,
  Layout,
  Menu,
  message,
  Popover,
  QRCode,
  Row,
  Space,
  Tag,
  Tooltip,
} from 'antd';
import {
  AndroidOutlined,
  AppleOutlined,
  CopyOutlined,
  DownOutlined,
  MoonFilled,
  MoonOutlined,
  QrcodeOutlined,
  SunOutlined,
  TranslationOutlined,
} from '@ant-design/icons';

import { ClipboardManager, IntlUtil, LanguageManager } from '@/utils';
import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import { LinkTags, parseLinkParts } from '@/lib/xray/link-label';
import { setMessageInstance } from '@/utils/messageBus';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import SubUsageSummary from './SubUsageSummary';
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
const linkEmails: string[] = Array.isArray(subData.emails) ? subData.emails : [];
const subEmail = [...new Set(linkEmails.filter(Boolean))].join(', ');
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

  const copyAll = useCallback(async () => {
    if (links.length === 0) return;
    const allLinks = links.join('\n');
    const ok = await ClipboardManager.copyText(allLinks);
    if (ok) messageApi.success(t('subscription.copyAllConfigsCopied'));
  }, [t, messageApi]);

  const open = useCallback((url: string) => {
    if (!url) return;
    window.open(url, '_blank');
  }, []);

  const shadowrocketUrl = useMemo(() => {
    if (!subUrl) return '';
    const separator = subUrl.includes('?') ? '&' : '?';
    const rawUrl = subUrl + separator + 'flag=shadowrocket';
    const base64Url = btoa(rawUrl);
    const remark = encodeURIComponent(subTitle || sId || 'Subscription');
    return `shadowrocket://add/sub://${base64Url}?remark=${remark}`;
  }, []);

  const v2boxUrl = useMemo(
    () => `v2box://install-sub?url=${encodeURIComponent(subUrl)}&name=${encodeURIComponent(sId)}`,
    [],
  );
  const streisandUrl = useMemo(() => `streisand://import/${encodeURIComponent(subUrl)}`, []);
  const happUrl = useMemo(() => `happ://add/${subUrl}`, []);
  const incyUrl = useMemo(() => `incy://add/${subUrl}`, []);

  const pageClass = useMemo(() => {
    const classes = ['subscription-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const descriptionsItems = useMemo(() => {
    const items = [
      { key: 'subId', label: t('subscription.subId'), children: sId },
      ...(subEmail ? [{ key: 'email', label: t('subscription.email'), children: subEmail }] : []),
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
    { key: 'android-incy', label: 'Incy', onClick: () => open(`incy://add/${subUrl}`) },
  ], [copy, open]);

  const iosMenuItems = useMemo(() => [
    { key: 'ios-shadowrocket', label: 'Shadowrocket', onClick: () => open(shadowrocketUrl) },
    { key: 'ios-v2box', label: 'V2Box', onClick: () => open(v2boxUrl) },
    { key: 'ios-streisand', label: 'Streisand', onClick: () => open(streisandUrl) },
    { key: 'ios-v2raytun', label: 'V2RayTun', onClick: () => copy(subUrl) },
    { key: 'ios-npvtunnel', label: 'NPV Tunnel', onClick: () => copy(subUrl) },
    { key: 'ios-happ', label: 'Happ', onClick: () => open(happUrl) },
    { key: 'ios-incy', label: 'Incy', onClick: () => open(incyUrl) },
  ], [copy, open, shadowrocketUrl, v2boxUrl, streisandUrl, happUrl, incyUrl]);

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

  const cardTitle = (
    <Space>
      <span>{t('subscription.title')}</span>
      <Tag>{sId}</Tag>
    </Space>
  );

  const cardExtra = (
    <Space size={8} align="center">
      <Button
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
                <Descriptions
                  bordered
                  column={1}
                  size="small"
                  className="info-table"
                  items={descriptionsItems}
                />

                <SubUsageSummary
                  usedByte={Number(subData.usedByte || 0)
                    || (Number(subData.downloadByte || 0) + Number(subData.uploadByte || 0))}
                  totalByte={totalByte}
                  usedLabel={used}
                  totalLabel={total}
                  remainedLabel={remained}
                  expireMs={expireMs}
                  isActive={isActive}
                />

                {(subUrl || subJsonUrl || subClashUrl) && (
                  <>
                    <Divider>{t('subscription.title')}</Divider>
                    <div className="links-section">
                      {subUrl && (
                        <div className="sub-link-row">
                          <Tag color="green" className="sub-link-tag">SUB</Tag>
                          <a
                            href={subUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="sub-link-title sub-link-anchor"
                            title={subUrl}
                          >
                            {sId}
                          </a>
                          <div className="sub-link-actions">
                            <Button size="small" icon={<CopyOutlined />} onClick={() => copy(subUrl)} aria-label={t('copy')} title={t('copy')} />
                            <Popover
                              trigger="click"
                              placement="left"
                              destroyOnHidden
                              content={
                                <div className="sub-link-qr-popover">
                                  <Tag color="green" className="qr-tag">{t('pages.settings.subSettings')}</Tag>
                                  <QRCode value={subUrl} size={QR_SIZE} type="svg" bordered={false} color="#000000" bgColor="#ffffff" />
                                </div>
                              }
                            >
                              <Button size="small" icon={<QrcodeOutlined />} aria-label="QR" title="QR" />
                            </Popover>
                          </div>
                        </div>
                      )}
                      {subJsonUrl && (
                        <div className="sub-link-row">
                          <Tag color="purple" className="sub-link-tag">JSON</Tag>
                          <a
                            href={subJsonUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="sub-link-title sub-link-anchor"
                            title={subJsonUrl}
                          >
                            {sId}
                          </a>
                          <div className="sub-link-actions">
                            <Button size="small" icon={<CopyOutlined />} onClick={() => copy(subJsonUrl)} aria-label={t('copy')} title={t('copy')} />
                            <Popover
                              trigger="click"
                              placement="left"
                              destroyOnHidden
                              content={
                                <div className="sub-link-qr-popover">
                                  <Tag color="purple" className="qr-tag">{t('pages.settings.subSettings')} JSON</Tag>
                                  <QRCode value={subJsonUrl} size={QR_SIZE} type="svg" bordered={false} color="#000000" bgColor="#ffffff" />
                                </div>
                              }
                            >
                              <Button size="small" icon={<QrcodeOutlined />} aria-label="QR" title="QR" />
                            </Popover>
                          </div>
                        </div>
                      )}
                      {subClashUrl && (
                        <div className="sub-link-row">
                          <Tooltip title="Clash / Mihomo">
                            <Tag color="gold" className="sub-link-tag">CLASH</Tag>
                          </Tooltip>
                          <a
                            href={subClashUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="sub-link-title sub-link-anchor"
                            title={subClashUrl}
                          >
                            {sId}
                          </a>
                          <div className="sub-link-actions">
                            <Button size="small" icon={<CopyOutlined />} onClick={() => copy(subClashUrl)} aria-label={t('copy')} title={t('copy')} />
                            <Popover
                              trigger="click"
                              placement="left"
                              destroyOnHidden
                              content={
                                <div className="sub-link-qr-popover">
                                  <Tag color="gold" className="qr-tag">Clash / Mihomo</Tag>
                                  <QRCode value={subClashUrl} size={QR_SIZE} type="svg" bordered={false} color="#000000" bgColor="#ffffff" />
                                </div>
                              }
                            >
                              <Button size="small" icon={<QrcodeOutlined />} aria-label="QR" title="QR" />
                            </Popover>
                          </div>
                        </div>
                      )}
                    </div>
                  </>
                )}

                {links.length > 0 && (
                  <>
                    <Divider>{t('pages.inbounds.copyLink')}</Divider>
                    <div className="links-section">
                      <div className="sub-link-row">
                        <span className="sub-link-title">{t('subscription.copyAllConfigs')}</span>
                        <div className="sub-link-actions">
                          <Button
                            size="small"
                            icon={<CopyOutlined />}
                            onClick={copyAll}
                            aria-label={t('subscription.copyAllConfigs')}
                            title={t('subscription.copyAllConfigs')}
                          />
                        </div>
                      </div>
                      {links.map((link, idx) => {
                        const parts = parseLinkParts(link);
                        const fallback = `Link ${idx + 1}`;
                        const rowTitle = parts?.remark || fallback;
                        const qrLabel = parts?.remark || rowTitle;
                        const canQr = !isPostQuantumLink(link);
                        return (
                          <div key={link} className="sub-link-row">
                            {parts
                              ? <LinkTags parts={parts} />
                              : <Tag className="sub-link-tag">LINK</Tag>}
                            <span className="sub-link-title" title={rowTitle}>
                              {rowTitle}
                            </span>
                            <div className="sub-link-actions">
                              <Button
                                size="small"
                                icon={<CopyOutlined />}
                                onClick={() => copy(link)}
                                aria-label={t('copy')}
                                title={t('copy')}
                              />
                              {canQr && (
                                <Popover
                                  trigger="click"
                                  placement="left"
                                  destroyOnHidden
                                  content={
                                    <div className="sub-link-qr-popover">
                                      <Tag className="qr-tag">{qrLabel}</Tag>
                                      <QRCode
                                        value={link}
                                        size={220}
                                        type="svg"
                                        bordered={false}
                                        color="#000000"
                                        bgColor="#ffffff"
                                      />
                                    </div>
                                  }
                                >
                                  <Button
                                    size="small"
                                    icon={<QrcodeOutlined />}
                                    aria-label="QR"
                                    title="QR"
                                  />
                                </Popover>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </>
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
