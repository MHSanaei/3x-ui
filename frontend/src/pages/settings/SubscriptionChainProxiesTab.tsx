import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Popconfirm, Space, Switch, Table, Tabs, Tag, Tooltip } from 'antd';
import {
  ApartmentOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  SwapOutlined,
} from '@ant-design/icons';

import { useChainProxiesQuery, useChainSelectorsQuery } from '@/api/queries/useChainProxyQueries';
import { useChainProxyMutations } from '@/api/queries/useChainProxyMutations';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import type {
  ChainProxy,
  ChainProxyFormValues,
  ChainSelector,
  ChainSelectorFormValues,
} from '@/schemas/chainProxy';
import { catTabLabel } from './catTabLabel';
import ChainProxyFormModal from './ChainProxyFormModal';
import ChainSelectorFormModal from './ChainSelectorFormModal';

const STRATEGY_COLORS: Record<string, string> = {
  leastLoad: 'geekblue',
  leastPing: 'green',
  random: 'orange',
  roundRobin: 'purple',
};

function chainProxyLabel(proxy: ChainProxy): string {
  return `${proxy.type}://${proxy.server}:${proxy.port}`;
}

// Chain proxies sit between the subscriber's client and this panel's inbound;
// each inbound names one hop or one selector, never both.
export default function SubscriptionChainProxiesTab() {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const { proxies, loading, fetched, fetchError, refetch } = useChainProxiesQuery();
  const {
    selectors,
    loading: selectorsLoading,
    fetched: selectorsFetched,
  } = useChainSelectorsQuery();
  const { createProxy, updateProxy, removeProxy, createSelector, updateSelector, removeSelector } =
    useChainProxyMutations();

  const [proxyModalOpen, setProxyModalOpen] = useState(false);
  const [editingProxy, setEditingProxy] = useState<ChainProxy | null>(null);
  const [selectorModalOpen, setSelectorModalOpen] = useState(false);
  const [editingSelector, setEditingSelector] = useState<ChainSelector | null>(null);

  const proxyLabels = useMemo(() => {
    const map = new Map<number, string>();
    for (const proxy of proxies) map.set(proxy.id, proxy.remark);
    return map;
  }, [proxies]);

  async function onProxyConfirm(values: ChainProxyFormValues) {
    const msg = editingProxy
      ? await updateProxy(editingProxy.id, values)
      : await createProxy(values);
    if (msg?.success) {
      setProxyModalOpen(false);
      refetch();
    }
  }

  async function onSelectorConfirm(values: ChainSelectorFormValues) {
    const msg = editingSelector
      ? await updateSelector(editingSelector.id, values)
      : await createSelector(values);
    if (msg?.success) setSelectorModalOpen(false);
  }

  const proxyColumns = [
    {
      title: t('pages.settings.chainProxies.remark'),
      dataIndex: 'remark',
      key: 'remark',
    },
    {
      title: t('pages.settings.chainProxies.type'),
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'http' ? 'gold' : 'blue'}>
          {t(
            type === 'http'
              ? 'pages.settings.chainProxies.typeHttp'
              : 'pages.settings.chainProxies.typeSocks',
          )}
        </Tag>
      ),
    },
    {
      title: t('pages.settings.chainProxies.endpoint'),
      key: 'endpoint',
      render: (_: unknown, r: ChainProxy) => (
        <Tooltip title={chainProxyLabel(r)}>
          <span>{`${r.server}:${r.port}`}</span>
        </Tooltip>
      ),
    },
    {
      title: t('pages.settings.chainProxies.auth'),
      key: 'auth',
      width: 120,
      render: (_: unknown, r: ChainProxy) => {
        const hasAuth = r.username !== '' || r.password !== '';
        return (
          <Tag color={hasAuth ? 'green' : 'default'}>
            {t(
              hasAuth
                ? 'pages.settings.chainProxies.authPassword'
                : 'pages.settings.chainProxies.authNone',
            )}
          </Tag>
        );
      },
    },
    {
      title: t('pages.settings.chainProxies.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      align: 'center' as const,
      render: (_: unknown, r: ChainProxy) => (
        <Switch
          size="small"
          aria-label={t('pages.settings.chainProxies.enabled')}
          checked={r.enabled}
          onChange={() =>
            updateProxy(r.id, {
              remark: r.remark,
              type: r.type,
              server: r.server,
              port: r.port,
              username: r.username,
              password: r.password,
              tls: r.tls,
              enabled: !r.enabled,
            })
          }
        />
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 96,
      render: (_: unknown, r: ChainProxy) => (
        <Space>
          <Button
            aria-label={t('edit')}
            size="small"
            icon={<EditOutlined aria-hidden />}
            title={t('edit')}
            onClick={() => {
              setEditingProxy(r);
              setProxyModalOpen(true);
            }}
          />
          <Popconfirm
            title={t('pages.settings.chainProxies.deleteConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={() => removeProxy(r.id)}
          >
            <Button
              aria-label={t('delete')}
              size="small"
              danger
              icon={<DeleteOutlined aria-hidden />}
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const selectorColumns = [
    {
      title: t('pages.settings.chainSelectors.remark'),
      dataIndex: 'remark',
      key: 'remark',
    },
    {
      title: t('pages.settings.chainSelectors.strategy'),
      dataIndex: 'strategy',
      key: 'strategy',
      width: 130,
      render: (strategy: string) => (
        <Tag color={STRATEGY_COLORS[strategy] ?? 'default'}>{strategy}</Tag>
      ),
    },
    {
      title: t('pages.settings.chainSelectors.proxies'),
      key: 'proxies',
      render: (_: unknown, r: ChainSelector) => {
        const labels = r.proxyIds.map((id) => proxyLabels.get(id) ?? `#${id}`);
        return (
          <Tooltip title={labels.join(', ')}>
            <span>{t('pages.settings.chainSelectors.proxiesCount', { count: labels.length })}</span>
          </Tooltip>
        );
      },
    },
    {
      title: t('pages.settings.chainSelectors.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      align: 'center' as const,
      render: (_: unknown, r: ChainSelector) => (
        <Switch
          size="small"
          aria-label={t('pages.settings.chainSelectors.enabled')}
          checked={r.enabled}
          onChange={() =>
            updateSelector(r.id, {
              remark: r.remark,
              strategy: r.strategy,
              proxyIds: r.proxyIds,
              enabled: !r.enabled,
            })
          }
        />
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 96,
      render: (_: unknown, r: ChainSelector) => (
        <Space>
          <Button
            aria-label={t('edit')}
            size="small"
            icon={<EditOutlined aria-hidden />}
            title={t('edit')}
            onClick={() => {
              setEditingSelector(r);
              setSelectorModalOpen(true);
            }}
          />
          <Popconfirm
            title={t('pages.settings.chainSelectors.deleteConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={() => removeSelector(r.id)}
          >
            <Button
              aria-label={t('delete')}
              size="small"
              danger
              icon={<DeleteOutlined aria-hidden />}
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const proxiesTab = (
    <div>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={t('pages.settings.chainProxies.desc')}
      />
      {fetchError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          title={fetchError}
          action={
            <Button size="small" onClick={() => refetch()}>
              {t('refresh')}
            </Button>
          }
        />
      )}
      <div style={{ marginBottom: 12 }}>
        <Button
          type="primary"
          icon={<PlusOutlined aria-hidden />}
          onClick={() => {
            setEditingProxy(null);
            setProxyModalOpen(true);
          }}
        >
          {t('pages.settings.chainProxies.add')}
        </Button>
      </div>
      <Table
        size="small"
        dataSource={proxies}
        rowKey={(r) => r.id}
        pagination={false}
        loading={loading && !fetched}
        scroll={{ x: true }}
        locale={{ emptyText: t('pages.settings.chainProxies.empty') }}
        columns={proxyColumns}
      />
    </div>
  );

  const selectorsTab = (
    <div>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={t('pages.settings.chainSelectors.desc')}
      />
      <div style={{ marginBottom: 12 }}>
        <Button
          type="primary"
          icon={<PlusOutlined aria-hidden />}
          disabled={proxies.length === 0}
          onClick={() => {
            setEditingSelector(null);
            setSelectorModalOpen(true);
          }}
        >
          {t('pages.settings.chainSelectors.add')}
        </Button>
      </div>
      <Table
        size="small"
        dataSource={selectors}
        rowKey={(r) => r.id}
        pagination={false}
        loading={selectorsLoading && !selectorsFetched}
        scroll={{ x: true }}
        locale={{ emptyText: t('pages.settings.chainSelectors.empty') }}
        columns={selectorColumns}
      />
    </div>
  );

  return (
    <>
      <Tabs
        defaultActiveKey="proxies"
        items={[
          {
            key: 'proxies',
            label: catTabLabel(
              <SwapOutlined aria-hidden />,
              t('pages.settings.chainProxies.tabProxies'),
              isMobile,
            ),
            children: proxiesTab,
          },
          {
            key: 'selectors',
            label: catTabLabel(
              <ApartmentOutlined aria-hidden />,
              t('pages.settings.chainSelectors.tabSelectors'),
              isMobile,
            ),
            children: selectorsTab,
          },
        ]}
      />
      <ChainProxyFormModal
        open={proxyModalOpen}
        proxy={editingProxy}
        onClose={() => setProxyModalOpen(false)}
        onConfirm={onProxyConfirm}
      />
      <ChainSelectorFormModal
        open={selectorModalOpen}
        selector={editingSelector}
        proxies={proxies}
        onClose={() => setSelectorModalOpen(false)}
        onConfirm={onSelectorConfirm}
      />
    </>
  );
}
