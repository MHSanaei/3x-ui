import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  InputNumber,
  Layout,
  Result,
  Row,
  Space,
  Statistic,
  Table,
  Tag,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { WalletOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useMe, ME_QUERY_KEY } from '@/hooks/useMe';
import { useCurrency } from '@/hooks/useCurrency';
import { HttpUtil, IntlUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;
const QUICK_AMOUNTS = [50000, 100000, 200000, 500000];

interface Payment {
  id: number;
  amount: number;
  status: string;
  refId: string;
  gateway: string;
  createdAt: number;
}

async function fetchPayments(): Promise<Payment[]> {
  const msg = await HttpUtil.get('/panel/api/billing/payments', undefined, { silent: true });
  if (!msg?.success) return [];
  return (msg.obj as Payment[]) ?? [];
}

export default function BillingPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { me } = useMe();
  const { format: formatMoney, formatNumber, unit, clientCostPerGB } = useCurrency();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();

  const [amount, setAmount] = useState<number>(QUICK_AMOUNTS[1]);

  const paymentsQuery = useQuery({ queryKey: ['billing', 'payments'], queryFn: fetchPayments });

  const paidStats = useMemo(() => {
    const rows = paymentsQuery.data ?? [];
    let totalPaid = 0;
    let count = 0;
    for (const p of rows) {
      if (p.status === 'paid') {
        totalPaid += p.amount || 0;
        count += 1;
      }
    }
    return { totalPaid, count };
  }, [paymentsQuery.data]);

  // Handle the redirect back from ZarinPal (?status=ok|cancelled|failed).
  useEffect(() => {
    const status = searchParams.get('status');
    if (!status) return;
    const refId = searchParams.get('refId');
    if (status === 'ok') {
      messageApi.success(refId ? t('pages.billing.toasts.paidWithRef', { refId }) : t('pages.billing.toasts.paid'));
      queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['billing', 'payments'] });
    } else if (status === 'cancelled') {
      messageApi.warning(t('pages.billing.toasts.cancelled'));
    } else {
      messageApi.error(t('pages.billing.toasts.failed'));
    }
    setSearchParams({}, { replace: true });
  }, [searchParams, setSearchParams, messageApi, t, queryClient]);

  const payMut = useMutation({
    mutationFn: (amt: number) =>
      HttpUtil.post('/panel/api/billing/zarinpal/request', { amount: amt }, JSON_HEADERS),
    onSuccess: (msg) => {
      const url = (msg?.obj as { url?: string } | null)?.url;
      if (msg?.success && url) {
        window.location.href = url; // hand off to the ZarinPal gateway
      }
    },
  });

  function startPayment() {
    if (!amount || amount <= 0) {
      messageApi.error(t('pages.billing.toasts.invalidAmount'));
      return;
    }
    payMut.mutate(amount);
  }

  const columns: TableColumnsType<Payment> = [
    {
      title: t('pages.billing.amount'),
      dataIndex: 'amount',
      key: 'amount',
      render: (a: number) => <strong>{formatMoney(a)}</strong>,
    },
    {
      title: t('pages.users.txType'),
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => {
        const color = s === 'paid' ? 'green' : s === 'failed' ? 'volcano' : 'gold';
        return <Tag color={color}>{t(`pages.billing.status_${s}`, { defaultValue: s })}</Tag>;
      },
    },
    { title: t('pages.billing.refId'), dataIndex: 'refId', key: 'refId', responsive: ['md'] },
    {
      title: t('pages.users.txDate'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (ts: number) => IntlUtil.formatDate(ts),
    },
  ];

  const pageClass = useMemo(() => {
    const classes = ['billing-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const disabled = !!me && !me.zarinpalEnable;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Row gutter={[16, 16]} justify="center">
              <Col xs={24} md={18} lg={14} xxl={12}>
                <Card size="small" hoverable className="summary-card" style={{ marginBottom: 16 }}>
                  <Row gutter={[16, 12]}>
                    <Col xs={24} sm={8}>
                      <Statistic
                        title={t('pages.billing.currentBalance')}
                        value={me?.balance ?? 0}
                        prefix={<WalletOutlined />}
                        suffix={unit}
                        groupSeparator=","
                      />
                    </Col>
                    <Col xs={12} sm={8}>
                      <Statistic
                        title={t('pages.billing.totalPaid')}
                        value={paidStats.totalPaid}
                        suffix={unit}
                        groupSeparator=","
                      />
                    </Col>
                    <Col xs={12} sm={8}>
                      <Statistic title={t('pages.billing.paymentsCount')} value={String(paidStats.count)} />
                    </Col>
                  </Row>
                  {clientCostPerGB > 0 && (
                    <div style={{ marginTop: 12, opacity: 0.75 }}>
                      {t('pages.billing.perGbInfo', { price: formatMoney(clientCostPerGB) })}
                    </div>
                  )}
                </Card>

                {disabled ? (
                  <Card size="small" hoverable>
                    <Result status="info" title={t('pages.billing.disabledTitle')} subTitle={t('pages.billing.disabledDesc')} />
                  </Card>
                ) : (
                  <Card size="small" hoverable title={t('pages.billing.topUpTitle')}>
                    <Alert type="info" showIcon style={{ marginBottom: 16 }} message={t('pages.billing.topUpHint')} />
                    <Space wrap style={{ marginBottom: 12 }}>
                      {QUICK_AMOUNTS.map((a) => (
                        <Button key={a} onClick={() => setAmount(a)} type={amount === a ? 'primary' : 'default'}>
                          {formatNumber(a)}
                        </Button>
                      ))}
                    </Space>
                    <Row gutter={8} align="middle">
                      <Col flex="auto">
                        <InputNumber
                          min={1000}
                          step={10000}
                          value={amount}
                          onChange={(v) => setAmount(Number(v) || 0)}
                          style={{ width: '100%' }}
                          formatter={(v) => `${v}`.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}
                          parser={(v) => Number((v || '').replace(/[^\d]/g, ''))}
                          addonAfter={unit}
                        />
                      </Col>
                      <Col>
                        <Button type="primary" loading={payMut.isPending} onClick={startPayment}>
                          {t('pages.billing.payWithZarinpal')}
                        </Button>
                      </Col>
                    </Row>
                  </Card>
                )}

                <Card size="small" hoverable title={t('pages.billing.history')} style={{ marginTop: 16 }}>
                  <Table<Payment>
                    dataSource={paymentsQuery.data ?? []}
                    columns={columns}
                    rowKey="id"
                    size="small"
                    loading={paymentsQuery.isFetching}
                    pagination={{ pageSize: 10, hideOnSinglePage: true }}
                  />
                </Card>
              </Col>
            </Row>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
