import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  ConfigProvider,
  Divider,
  Layout,
  Popover,
  QRCode,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import { CopyOutlined, LogoutOutlined, QrcodeOutlined } from '@ant-design/icons';

import { ClipboardManager, HttpUtil, SizeFormatter } from '@/utils';
import { LinkTags, parseLinkParts } from '@/lib/xray/link-label';
import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import { setMessageInstance } from '@/utils/messageBus';
import { useTheme } from '@/hooks/useTheme';
import '@/pages/sub/SubPage.css';
import './CabinetPage.css';

const QR_SIZE = 220;
const basePath = window.X_UI_BASE_PATH || '';

interface CabinetData {
  subId: string;
  subUrl: string;
  links: string[] | null;
  up: number;
  down: number;
  total: number;
  expiryTime: number;
  enable: boolean;
}

export default function CabinetPage() {
  const { t } = useTranslation();
  const { antdThemeConfig, isDark } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [data, setData] = useState<CabinetData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.get<CabinetData>('/cabinet/data', undefined, { silent: true });
      if (cancelled) return;
      if (msg.success && msg.obj) setData(msg.obj);
      setLoading(false);
    })();
    return () => { cancelled = true; };
  }, []);

  const links = useMemo(() => data?.links ?? [], [data]);

  const copy = useCallback(async (text: string) => {
    const ok = await ClipboardManager.copyText(text);
    if (ok) messageApi.success(t('copied'));
  }, [messageApi, t]);

  const copyAll = useCallback(() => copy(links.join('\n')), [copy, links]);

  const onLogout = useCallback(async () => {
    await HttpUtil.post('/logout');
    window.location.href = basePath;
  }, []);

  const totalLabel = data && data.total > 0 ? SizeFormatter.sizeFormat(data.total) : t('subscription.unlimited');
  const usedLabel = data ? SizeFormatter.sizeFormat(data.up + data.down) : '';
  const expiryLabel = data && data.expiryTime > 0
    ? new Date(data.expiryTime).toLocaleString()
    : t('subscription.noExpiry');

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={`cabinet-app${isDark ? ' is-dark' : ''}`}>
        <Layout.Content className="cabinet-content">
          <div className="cabinet-wrapper">
            <div className="cabinet-header">
              <Typography.Title level={3} style={{ margin: 0 }}>{t('pages.cabinet.title')}</Typography.Title>
              <Button icon={<LogoutOutlined />} onClick={onLogout}>{t('logout')}</Button>
            </div>

            {loading ? (
              <div className="cabinet-loading"><Spin size="large" /></div>
            ) : !data || links.length === 0 ? (
              <Card><Typography.Text type="secondary">{t('pages.cabinet.empty')}</Typography.Text></Card>
            ) : (
              <Card>
                <Space wrap size="large">
                  <span>
                    <Typography.Text strong>{t('subscription.status')}: </Typography.Text>
                    <Tag color={data.enable ? 'green' : 'red'}>
                      {data.enable ? t('subscription.active') : t('subscription.inactive')}
                    </Tag>
                  </span>
                  <span>
                    <Typography.Text type="secondary">{t('subscription.totalQuota')}: </Typography.Text>
                    <Typography.Text>{usedLabel} / {totalLabel}</Typography.Text>
                  </span>
                  <span>
                    <Typography.Text type="secondary">{t('subscription.expiry')}: </Typography.Text>
                    <Typography.Text>{expiryLabel}</Typography.Text>
                  </span>
                </Space>

                {data.subUrl && (
                  <>
                    <Divider>{t('subscription.title')}</Divider>
                    <div className="links-section">
                      <div className="sub-link-row">
                        <Tag color="green" className="sub-link-tag">SUB</Tag>
                        <a
                          href={data.subUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="sub-link-title sub-link-anchor"
                          title={data.subUrl}
                        >
                          {data.subId}
                        </a>
                        <div className="sub-link-actions">
                          <Button size="small" icon={<CopyOutlined />} onClick={() => copy(data.subUrl)} aria-label={t('copy')} title={t('copy')} />
                          <Popover
                            trigger="click"
                            placement="left"
                            destroyOnHidden
                            content={
                              <div className="sub-link-qr-popover">
                                <Tag color="green" className="qr-tag">{t('pages.settings.subSettings')}</Tag>
                                <QRCode value={data.subUrl} size={QR_SIZE} type="svg" bordered={false} color="#000000" bgColor="#ffffff" />
                              </div>
                            }
                          >
                            <Button size="small" icon={<QrcodeOutlined />} aria-label="QR" title="QR" />
                          </Popover>
                        </div>
                      </div>
                    </div>
                  </>
                )}

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
                    const rowTitle = parts?.remark || `Link ${idx + 1}`;
                    const canQr = !isPostQuantumLink(link);
                    return (
                      <Fragment key={link}>
                        <div className="sub-link-row">
                          {parts ? <LinkTags parts={parts} /> : <Tag className="sub-link-tag">LINK</Tag>}
                          <span className="sub-link-title" title={rowTitle}>{rowTitle}</span>
                          <div className="sub-link-actions">
                            <Button size="small" icon={<CopyOutlined />} onClick={() => copy(link)} aria-label={t('copy')} title={t('copy')} />
                            {canQr && (
                              <Popover
                                trigger="click"
                                placement="left"
                                destroyOnHidden
                                content={
                                  <div className="sub-link-qr-popover">
                                    <Tag className="qr-tag">{rowTitle}</Tag>
                                    <QRCode value={link} size={QR_SIZE} type="svg" bordered={false} color="#000000" bgColor="#ffffff" />
                                  </div>
                                }
                              >
                                <Button size="small" icon={<QrcodeOutlined />} aria-label="QR" title="QR" />
                              </Popover>
                            )}
                          </div>
                        </div>
                      </Fragment>
                    );
                  })}
                </div>
              </Card>
            )}
          </div>
        </Layout.Content>
      </Layout>
    </ConfigProvider>
  );
}
