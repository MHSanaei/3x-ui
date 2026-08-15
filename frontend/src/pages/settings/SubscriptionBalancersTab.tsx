import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Popconfirm, Space, Switch, Table, Tag, Tooltip } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
 
import { useSubBalancersQuery } from '@/api/queries/useSubBalancersQuery';
import { useSubBalancerMutations } from '@/api/queries/useSubBalancerMutations';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { formatInboundLabel } from '@/lib/inbounds/label';
import type { SubBalancer, SubBalancerFormValues } from '@/schemas/subBalancer';
import SubBalancerFormModal from './SubBalancerFormModal';
 
const STRATEGY_COLORS: Record<string, string> = {
  leastLoad: 'geekblue',
  leastPing: 'green',
  random: 'orange',
  roundRobin: 'purple',
};
 
export default function SubscriptionBalancersTab() {
  const { t } = useTranslation();
  const { balancers, loading, fetched, fetchError, refetch } = useSubBalancersQuery();
  const { create, update, remove } = useSubBalancerMutations();
  const { data: inboundOptionsRaw } = useInboundOptions();
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<SubBalancer | null>(null);
 
  const inboundLabels = useMemo(() => {
    const map = new Map<number, string>();
    for (const ib of inboundOptionsRaw ?? []) {
      map.set(ib.id, formatInboundLabel(ib.tag, ib.remark));
    }
    return map;
  }, [inboundOptionsRaw]);
 
  async function onConfirm(values: SubBalancerFormValues) {
    const msg = editing
      ? await update(editing.id, values)
      : await create(values);
    if (msg?.success) setModalOpen(false);
  }
 
  async function toggleEnabled(balancer: SubBalancer) {
    await update(balancer.id, {
      remark: balancer.remark,
      strategy: balancer.strategy,
      inboundIds: balancer.inboundIds,
      sortOrder: balancer.sortOrder,
      enabled: !balancer.enabled,
    });
  }
 
  const columns = [
    {
      title: t('pages.settings.subBalancers.sortOrder'),
      dataIndex: 'sortOrder',
      key: 'sortOrder',
      width: 80,
      align: 'center' as const,
    },
    {
      title: t('pages.settings.subBalancers.remark'),
      dataIndex: 'remark',
      key: 'remark',
    },
    {
      title: t('pages.settings.subBalancers.strategy'),
      dataIndex: 'strategy',
      key: 'strategy',
      width: 120,
      render: (strategy: string) => (
        <Tag color={STRATEGY_COLORS[strategy] ?? 'default'}>{strategy}</Tag>
      ),
    },
    {
      title: t('pages.settings.subBalancers.inbounds'),
      key: 'inbounds',
      render: (_: unknown, r: SubBalancer) => {
        const labels = r.inboundIds.map((id) => inboundLabels.get(id) ?? `#${id}`);
        return (
          <Tooltip title={labels.join(', ')}>
            <span>{t('pages.settings.subBalancers.inboundsCount', { count: labels.length })}</span>
          </Tooltip>
        );
      },
    },
    {
      title: t('pages.settings.subBalancers.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      align: 'center' as const,
      render: (_: unknown, r: SubBalancer) => (
        <Switch size="small" checked={r.enabled} onChange={() => toggleEnabled(r)} />
      ),
    },
    {
      title: '',
      key: 'actions',
      width: 96,
      render: (_: unknown, r: SubBalancer) => (
        <Space>
          <Button
            aria-label={t('edit')}
            size="small"
            icon={<EditOutlined />}
            title={t('edit')}
            onClick={() => {
              setEditing(r);
              setModalOpen(true);
            }}
          />
          <Popconfirm
            title={t('pages.settings.subBalancers.deleteConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={() => remove(r.id)}
          >
            <Button aria-label={t('delete')} size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];
 
  return (
    <div>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={t('pages.settings.subBalancers.desc')}
      />
      {fetchError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          title={fetchError}
          action={<Button size="small" onClick={() => refetch()}>{t('refresh')}</Button>}
        />
      )}
      <div style={{ marginBottom: 12 }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setEditing(null);
            setModalOpen(true);
          }}
        >
          {t('pages.settings.subBalancers.add')}
        </Button>
      </div>
      <Table
        size="small"
        dataSource={balancers}
        rowKey={(r) => r.id}
        pagination={false}
        loading={loading && !fetched}
        scroll={{ x: true }}
        locale={{ emptyText: t('pages.settings.subBalancers.empty') }}
        columns={columns}
      />
      <SubBalancerFormModal
        open={modalOpen}
        balancer={editing}
        onClose={() => setModalOpen(false)}
        onConfirm={onConfirm}
      />
    </div>
  );
}