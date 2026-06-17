import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, ConfigProvider, Layout, Space, Typography, message } from 'antd';
import { CopyOutlined } from '@ant-design/icons';

import { ClipboardManager } from '@/utils';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import './DonatePage.css';

const WALLETS = [
  {
    key: 'ton',
    labelKey: 'pages.donate.ton',
    address: 'UQAa5FpNlK8Gp7tO8luJXHD-Sf0pPjJbNHGo8hdkyuUBhWEa',
  },
  {
    key: 'tron',
    labelKey: 'pages.donate.tron',
    address: 'TLqtTfYSzPLFm8mtFDkSnXvzucxx7DS5VL',
  },
  {
    key: 'erc20Bep20',
    labelKey: 'pages.donate.erc20Bep20',
    address: '0x2fe632d70f4612b87670f8a28b4587ea2641452d',
  },
] as const;

export default function DonatePage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();

  const copyAddress = useCallback(async (address: string) => {
    const ok = await ClipboardManager.copyText(address);
    if (ok) {
      message.success(t('copied'));
    }
  }, [t]);

  const pageClass = ['donate-page', isDark && 'is-dark', isUltra && 'is-ultra'].filter(Boolean).join(' ');

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <div className="page-header">
              <Typography.Title level={3}>{t('pages.donate.title')}</Typography.Title>
              <Typography.Paragraph type="secondary">{t('pages.donate.subtitle')}</Typography.Paragraph>
            </div>

            <Space direction="vertical" size="middle" className="donate-wallets">
              {WALLETS.map(({ key, labelKey, address }) => (
                <Card key={key} className="donate-wallet-card">
                  <div className="donate-wallet-row">
                    <div className="donate-wallet-info">
                      <Typography.Text strong>{t(labelKey)}</Typography.Text>
                      <Typography.Text code className="donate-wallet-address">{address}</Typography.Text>
                    </div>
                    <Button
                      icon={<CopyOutlined />}
                      onClick={() => copyAddress(address)}
                      aria-label={t('copy')}
                    >
                      {t('copy')}
                    </Button>
                  </div>
                </Card>
              ))}
            </Space>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
