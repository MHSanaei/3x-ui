import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Col, ConfigProvider, Layout, Row, Spin, Typography } from 'antd';
import { CrownOutlined, PlusOutlined } from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { useSponsorsQuery } from '@/api/queries/useSponsorsQuery';
import SponsorCard from '@/components/sponsor/SponsorCard';
import { sponsorsForSlot } from '@/lib/sponsors';
import './SponsorsPage.css';

function BecomeButton({ contact, block }: { contact?: string; block?: boolean }) {
  const { t } = useTranslation();
  if (!contact) return null;
  return (
    <Button
      type="primary"
      icon={<CrownOutlined />}
      href={contact}
      target="_blank"
      rel="noopener noreferrer"
      block={block}
    >
      {t('pages.sponsors.become')}
    </Button>
  );
}

function YourBrandCard({ contact }: { contact?: string }) {
  const { t } = useTranslation();
  return (
    <div className="sponsors-yourbrand">
      <span className="sponsors-yourbrand-icon">
        <PlusOutlined />
      </span>
      <div className="sponsors-yourbrand-title">{t('pages.sponsors.yourBrand')}</div>
      <div className="sponsors-yourbrand-text">{t('pages.sponsors.yourBrandText')}</div>
      <BecomeButton contact={contact} />
    </div>
  );
}

export default function SponsorsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { data, fetched } = useSponsorsQuery();
  const sponsors = sponsorsForSlot(data.sponsors, 'page');

  const pageClass = useMemo(() => {
    const classes = ['sponsors-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <div className="sponsors-inner">
              <div className="sponsors-header">
                <div>
                  <Typography.Title level={3} className="sponsors-title">
                    <CrownOutlined className="sponsors-title-icon" /> {t('pages.sponsors.title')}
                  </Typography.Title>
                  <Typography.Text type="secondary">{t('pages.sponsors.intro')}</Typography.Text>
                </div>
                <BecomeButton contact={data.contact} />
              </div>

              <Spin spinning={!fetched} delay={200}>
                {sponsors.length === 0 ? (
                  fetched && <YourBrandCard contact={data.contact} />
                ) : (
                  <Row gutter={[16, 16]}>
                    {sponsors.map((sponsor) => (
                      <Col key={sponsor.id} xs={24} lg={12}>
                        <SponsorCard sponsor={sponsor} variant="card" />
                      </Col>
                    ))}
                  </Row>
                )}
              </Spin>
            </div>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
