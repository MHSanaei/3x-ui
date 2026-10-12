import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Col, ConfigProvider, Layout, Row, Spin, Tag, Typography } from 'antd';
import {
  CrownOutlined,
  DashboardOutlined,
  LoginOutlined,
  MenuUnfoldOutlined,
  PlusOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { useSponsorsQuery } from '@/api/queries/useSponsorsQuery';
import SponsorCard from '@/components/sponsor/SponsorCard';
import { IntlUtil } from '@/utils';
import { useDatepicker } from '@/hooks/useDatepicker';
import { placementStatus, sponsorsForSlot, type SponsorSlot } from '@/lib/sponsors';
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

function YourBrandCard({ contact, large }: { contact?: string; large?: boolean }) {
  const { t } = useTranslation();
  return (
    <div className={`sponsors-yourbrand${large ? ' is-large' : ''}`}>
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

  const { datepicker } = useDatepicker();
  const placements: { slot: SponsorSlot; icon: ReactNode; title: string; desc: string }[] = [
    {
      slot: 'dashboard',
      icon: <DashboardOutlined />,
      title: t('pages.sponsors.placementDashboard'),
      desc: t('pages.sponsors.placementDashboardDesc'),
    },
    {
      slot: 'sidebar',
      icon: <MenuUnfoldOutlined />,
      title: t('pages.sponsors.placementSidebar'),
      desc: t('pages.sponsors.placementSidebarDesc'),
    },
    {
      slot: 'login',
      icon: <LoginOutlined />,
      title: t('pages.sponsors.placementLogin'),
      desc: t('pages.sponsors.placementLoginDesc'),
    },
    {
      slot: 'page',
      icon: <CrownOutlined />,
      title: t('pages.sponsors.placementPage'),
      desc: t('pages.sponsors.placementPageDesc'),
    },
  ];

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
                  fetched && <YourBrandCard contact={data.contact} large />
                ) : (
                  <Row gutter={[16, 16]}>
                    {sponsors.map((sponsor) => (
                      <Col key={sponsor.id} xs={24} sm={12} xl={8}>
                        <SponsorCard sponsor={sponsor} variant="card" />
                      </Col>
                    ))}
                    {data.contact && (
                      <Col xs={24} sm={12} xl={8}>
                        <YourBrandCard contact={data.contact} />
                      </Col>
                    )}
                  </Row>
                )}
              </Spin>

              <Typography.Title level={5} className="sponsors-section-title">
                {t('pages.sponsors.placements')}
              </Typography.Title>
              <Row gutter={[16, 16]}>
                {placements.map((p) => {
                  const status = placementStatus(data.sponsors, p.slot);
                  return (
                    <Col key={p.slot} xs={24} sm={12} xl={6}>
                      <div className="sponsors-placement">
                        <span className="sponsors-placement-icon">{p.icon}</span>
                        <div className="sponsors-placement-body">
                          <div className="sponsors-placement-title">{p.title}</div>
                          <div className="sponsors-placement-desc">{p.desc}</div>
                          <div className="sponsors-placement-status">
                            {status.takenUntil ? (
                              <Tag color="orange">
                                {t('pages.sponsors.takenUntil', {
                                  date: IntlUtil.formatDate(status.takenUntil, datepicker),
                                })}
                              </Tag>
                            ) : (
                              <Tag color="green">{t('pages.sponsors.available')}</Tag>
                            )}
                            {status.count > 0 && !status.takenUntil && (
                              <Tag>
                                {t('pages.sponsors.activeCount', {
                                  count:
                                    status.capacity && status.capacity > 1
                                      ? `${status.count}/${status.capacity}`
                                      : status.count,
                                })}
                              </Tag>
                            )}
                          </div>
                        </div>
                      </div>
                    </Col>
                  );
                })}
              </Row>
            </div>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
