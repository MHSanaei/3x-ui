import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  Result,
  Row,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  CrownOutlined,
  DollarOutlined,
  FallOutlined,
  ReloadOutlined,
  RiseOutlined,
  TeamOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useCurrency } from '@/hooks/useCurrency';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';
import Sparkline from '@/components/viz/Sparkline';

interface PeriodStat {
  amount: number;
  count: number;
}

interface DailyPoint {
  date: string;
  revenue: number;
  spend: number;
}

interface ResellerStat {
  userId: number;
  username: string;
  spend: number;
  clients: number;
}

interface IncomeReport {
  revenue: Record<string, PeriodStat>;
  spend: Record<string, PeriodStat>;
  newClients: Record<string, number>;
  daily: DailyPoint[];
  topResellers: ResellerStat[] | null;
  pendingCount: number;
  totalUsers: number;
  totalClients: number;
  outstanding: number;
}

// Period slugs in the order the backend reports them. Each maps to a localized
// label and feeds the breakdown table rows.
const PERIOD_KEYS = ['today', 'yesterday', 'last7', 'thisMonth', 'lastMonth', 'thisYear', 'allTime'] as const;
type PeriodKey = (typeof PERIOD_KEYS)[number];

interface PeriodRow {
  key: PeriodKey;
  revenue: number;
  payments: number;
  spend: number;
  newClients: number;
}

async function fetchReport(): Promise<IncomeReport> {
  const msg = await HttpUtil.get('/panel/api/admin/reports/income', undefined, { silent: true });
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to load report');
  return msg.obj as IncomeReport;
}

export default function ReportsPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const { format: formatMoney, formatNumber, unit } = useCurrency();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const reportQuery = useQuery({ queryKey: ['admin', 'income-report'], queryFn: fetchReport });
  const report = reportQuery.data;
  const fetched = reportQuery.data !== undefined || reportQuery.isError;
  const fetchError = reportQuery.error ? (reportQuery.error as Error).message : '';

  const periodRows = useMemo<PeriodRow[]>(() => {
    if (!report) return [];
    return PERIOD_KEYS.map((key) => ({
      key,
      revenue: report.revenue[key]?.amount ?? 0,
      payments: report.revenue[key]?.count ?? 0,
      spend: report.spend[key]?.amount ?? 0,
      newClients: report.newClients[key] ?? 0,
    }));
  }, [report]);

  const daily = useMemo(() => report?.daily ?? [], [report]);
  const revenueSeries = useMemo(() => daily.map((d) => d.revenue), [daily]);
  const spendSeries = useMemo(() => daily.map((d) => d.spend), [daily]);
  const dayLabels = useMemo(() => daily.map((d) => d.date.slice(5)), [daily]);

  const periodColumns: TableColumnsType<PeriodRow> = [
    {
      title: t('pages.reports.period'),
      dataIndex: 'key',
      key: 'key',
      render: (key: PeriodKey) => <strong>{t(`pages.reports.periods.${key}`)}</strong>,
    },
    {
      title: t('pages.reports.revenue'),
      dataIndex: 'revenue',
      key: 'revenue',
      align: 'right',
      render: (v: number) => <span style={{ color: 'var(--ant-color-success)' }}>{formatMoney(v)}</span>,
    },
    {
      title: t('pages.reports.payments'),
      dataIndex: 'payments',
      key: 'payments',
      align: 'right',
      responsive: ['sm'],
      render: (v: number) => formatNumber(v),
    },
    {
      title: t('pages.reports.spend'),
      dataIndex: 'spend',
      key: 'spend',
      align: 'right',
      render: (v: number) => formatMoney(v),
    },
    {
      title: t('pages.reports.newClients'),
      dataIndex: 'newClients',
      key: 'newClients',
      align: 'right',
      responsive: ['md'],
      render: (v: number) => formatNumber(v),
    },
  ];

  const resellerColumns: TableColumnsType<ResellerStat> = [
    {
      title: t('username'),
      dataIndex: 'username',
      key: 'username',
      render: (name: string) => (name ? <Tag color="purple">{name}</Tag> : <span style={{ opacity: 0.5 }}>—</span>),
    },
    {
      title: t('pages.reports.spend'),
      dataIndex: 'spend',
      key: 'spend',
      align: 'right',
      render: (v: number) => <strong>{formatMoney(v)}</strong>,
    },
    {
      title: t('clients'),
      dataIndex: 'clients',
      key: 'clients',
      align: 'right',
      responsive: ['sm'],
      render: (v: number) => formatNumber(v),
    },
  ];

  const pageClass = useMemo(() => {
    const classes = ['reports-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const allTimeRevenue = report?.revenue.allTime?.amount ?? 0;
  const monthRevenue = report?.revenue.thisMonth?.amount ?? 0;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" onClick={() => reportQuery.refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic
                            title={t('pages.reports.totalIncome')}
                            value={allTimeRevenue}
                            prefix={<DollarOutlined />}
                            suffix={unit}
                            groupSeparator=","
                          />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic
                            title={t('pages.reports.periods.thisMonth')}
                            value={monthRevenue}
                            prefix={<RiseOutlined />}
                            suffix={unit}
                            groupSeparator=","
                          />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic
                            title={t('pages.reports.outstanding')}
                            value={report?.outstanding ?? 0}
                            prefix={<WalletOutlined />}
                            suffix={unit}
                            groupSeparator=","
                          />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic title={t('pages.users.totalUsers')} value={String(report?.totalUsers ?? 0)} prefix={<TeamOutlined />} />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic title={t('pages.reports.totalClients')} value={String(report?.totalClients ?? 0)} prefix={<CrownOutlined />} />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic
                            title={t('pages.reports.pending')}
                            value={String(report?.pendingCount ?? 0)}
                            prefix={<FallOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card
                      size="small"
                      hoverable
                      title={t('pages.reports.dailyTitle')}
                      extra={
                        <Button
                          size="small"
                          icon={<ReloadOutlined />}
                          loading={reportQuery.isFetching}
                          onClick={() => reportQuery.refetch()}
                        >
                          {!isMobile && t('refresh')}
                        </Button>
                      }
                    >
                      {revenueSeries.some((v) => v > 0) || spendSeries.some((v) => v > 0) ? (
                        <Sparkline
                          data={revenueSeries}
                          data2={spendSeries}
                          labels={dayLabels}
                          name1={t('pages.reports.revenue')}
                          name2={t('pages.reports.spend')}
                          height={260}
                          showAxes
                          showTooltip
                          valueMax={null}
                          yFormatter={(v) => formatNumber(v)}
                          tooltipFormatter={(v) => formatMoney(v)}
                          strokeWidth={2}
                        />
                      ) : (
                        <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--ant-color-text-secondary)' }}>
                          {t('pages.reports.noActivity')}
                        </div>
                      )}
                    </Card>
                  </Col>

                  <Col xs={24} lg={14}>
                    <Card size="small" hoverable title={t('pages.reports.breakdownTitle')}>
                      <Table<PeriodRow>
                        dataSource={periodRows}
                        columns={periodColumns}
                        rowKey="key"
                        size="small"
                        pagination={false}
                      />
                    </Card>
                  </Col>

                  <Col xs={24} lg={10}>
                    <Card size="small" hoverable title={t('pages.reports.topResellersTitle')}>
                      <Table<ResellerStat>
                        dataSource={report?.topResellers ?? []}
                        columns={resellerColumns}
                        rowKey="userId"
                        size="small"
                        pagination={false}
                        locale={{
                          emptyText: (
                            <div style={{ padding: '24px 0', color: 'var(--ant-color-text-secondary)' }}>
                              <Space direction="vertical" align="center">
                                <CrownOutlined style={{ fontSize: 28, opacity: 0.5 }} />
                                <span>{t('pages.reports.noResellers')}</span>
                              </Space>
                            </div>
                          ),
                        }}
                      />
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
