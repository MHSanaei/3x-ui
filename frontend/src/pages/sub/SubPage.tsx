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

const PROTOCOL_COLORS: Record<string, string> = {
  VLESS: 'blue',
  VMESS: 'geekblue',
  TROJAN: 'volcano',
  SS: 'magenta',
  HYSTERIA: 'cyan',
  HY2: 'green',
};

// Same idea as ClientInfoModal.trimEmail — strip the client email
// suffix from the remark so the row title isn't ugly twice.
function trimEmail(remark: string, email: string): string {
  if (!email) return remark;
  const e = email.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return remark
    .replace(new RegExp(`[-_.\\s|]+${e}$`), '')
    .replace(new RegExp(`^${e}[-_.\\s|]+`), '')
    .trim();
}

// Post-quantum keys blow up the encoded URL past what a single QR can
// hold. The algorithm names don't appear as plain text in the URL —
// they ride inside query params: mldsa65Verify → `pqv=<base64>`,
// ML-KEM-768 → `encryption=mlkem768x25519plus.<...>`. The literal
// substrings are also matched in case a config (e.g. wireguard) embeds
// them directly.
function isPostQuantumLink(link: string): boolean {
  if (/[?&]pqv=/.test(link)) return true;
  if (link.includes('mlkem768') || link.includes('mldsa65')) return true;
  if (link.includes('ML-KEM-768')) return true;
  return false;
}

// Decode a base64 string as UTF-8. atob() returns a binary string where
// each char holds one raw byte (Latin-1 interpretation), which mangles
// any multi-byte UTF-8 sequence in the payload — most commonly the
// emoji decorations the panel embeds in remarks (📊, ⏳).
function base64DecodeUtf8(b64: string): string {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder('utf-8').decode(bytes);
}

function parseLinkMeta(link: string, idx: number): { protocol: string; remark: string } {
  const fallback = `Link ${idx + 1}`;
  if (!link) return { protocol: 'LINK', remark: fallback };
  const schemeMatch = /^([a-z0-9]+):\/\//i.exec(link);
  const scheme = schemeMatch?.[1]?.toLowerCase() ?? '';
  const protocolMap: Record<string, string> = {
    vless: 'VLESS',
    vmess: 'VMESS',
    trojan: 'TROJAN',
    ss: 'SS',
    hysteria: 'HYSTERIA',
    hysteria2: 'HY2',
    hy2: 'HY2',
  };
  const protocol = protocolMap[scheme] ?? scheme.toUpperCase() ?? 'LINK';

  let remark = '';
  if (scheme === 'vmess') {
    try {
      const body = link.slice('vmess://'.length).split('#')[0];
      const json = JSON.parse(base64DecodeUtf8(body)) as { ps?: unknown };
      if (typeof json?.ps === 'string') remark = json.ps;
    } catch { /* fall through */ }
  }
  if (!remark) {
    const hashIdx = link.indexOf('#');
    if (hashIdx >= 0 && hashIdx + 1 < link.length) {
      const raw = link.slice(hashIdx + 1);
      try { remark = decodeURIComponent(raw); }
      catch { remark = raw; }
    }
  }
  return { protocol, remark: remark || fallback };
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
                      {links.map((link, idx) => {
                        const meta = parseLinkMeta(link, idx);
                        const rowEmail = linkEmails[idx] || '';
                        const rowTitle = trimEmail(meta.remark, rowEmail) || meta.remark;
                        const qrLabel = rowEmail ? `${rowTitle}-${rowEmail}` : meta.remark;
                        const canQr = !isPostQuantumLink(link);
                        return (
                          <div key={link} className="sub-link-row">
                            <Tag
                              color={PROTOCOL_COLORS[meta.protocol] ?? 'default'}
                              className="sub-link-tag"
                            >
                              {meta.protocol}
                            </Tag>
                            <span className="sub-link-title" title={meta.remark}>
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
                                      <Tag
                                        color={PROTOCOL_COLORS[meta.protocol] ?? 'default'}
                                        className="qr-tag"
                                      >
                                        {qrLabel}
                                      </Tag>
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
