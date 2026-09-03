import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Tooltip,
} from 'antd';
import {
  DeleteOutlined,
  DeploymentUnitOutlined,
  EditOutlined,
  PlusOutlined,
  RadarChartOutlined,
} from '@ant-design/icons';

import { useSubBalancersQuery } from '@/api/queries/useSubBalancersQuery';
import { useSubBalancerMutations } from '@/api/queries/useSubBalancerMutations';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { formatInboundLabel } from '@/lib/inbounds/label';
import type { AllSetting } from '@/models/setting';
import { onNumber } from '@/utils/onNumber';
import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import type { SubBalancer, SubBalancerFormValues } from '@/schemas/subBalancer';
import { PingConfigSchema, type PingConfigObject } from '@/schemas/observatory';
import { DEFAULT_BURST_OBSERVATORY } from '@/pages/xray/balancers/balancer-helpers';
import SubBalancerFormModal from './SubBalancerFormModal';
import { catTabLabel } from './catTabLabel';
import './SubscriptionFormatsTab.css';

const STRATEGY_COLORS: Record<string, string> = {
  leastLoad: 'geekblue',
  leastPing: 'green',
  random: 'orange',
  roundRobin: 'purple',
};

// Single source for the burst-observatory ping defaults: the Zod schema and
// DEFAULT_BURST_OBSERVATORY are kept in sync, so the tab just parses through it.
const DEFAULT_PING_CONFIG = PingConfigSchema.parse({ ...DEFAULT_BURST_OBSERVATORY.pingConfig });

function parsePingConfig(raw: string): PingConfigObject {
  try {
    return PingConfigSchema.parse(raw ? JSON.parse(raw) : {});
  } catch {
    return DEFAULT_PING_CONFIG;
  }
}

interface SubscriptionBalancersTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export default function SubscriptionBalancersTab({
  allSetting,
  updateSetting,
}: SubscriptionBalancersTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
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
    const msg = editing ? await update(editing.id, values) : await create(values);
    if (msg?.success) setModalOpen(false);
  }

  async function toggleEnabled(balancer: SubBalancer) {
    await update(balancer.id, {
      remark: balancer.remark,
      strategy: balancer.strategy,
      inboundIds: balancer.inboundIds,
      memberWeights: balancer.memberWeights ?? undefined,
      sortOrder: balancer.sortOrder,
      enabled: !balancer.enabled,
    });
  }

  const observatoryEnabled = allSetting.subJsonObservatory !== '';
  const observatoryObj = useMemo(
    () => parsePingConfig(allSetting.subJsonObservatory),
    [allSetting.subJsonObservatory],
  );

  function setObservatoryEnabled(v: boolean) {
    updateSetting({ subJsonObservatory: v ? JSON.stringify(DEFAULT_PING_CONFIG) : '' });
  }

  function setObservatoryField<K extends keyof PingConfigObject>(
    key: K,
    value: PingConfigObject[K],
  ) {
    const next = { ...observatoryObj, [key]: value };
    updateSetting({ subJsonObservatory: JSON.stringify(next) });
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

  const balancersTab = (
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
    </div>
  );

  const observatoryTab = (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={t('pages.settings.subBalancers.observatory.note')}
      />
      <SettingListItem
        paddings="small"
        title={t('pages.settings.subBalancers.observatory.title')}
        description={t('pages.settings.subBalancers.observatory.desc')}
      >
        <Switch checked={observatoryEnabled} onChange={setObservatoryEnabled} />
      </SettingListItem>
      {observatoryEnabled && (
        <div className="format-settings">
          <SettingListItem
            paddings="small"
            title={t('pages.settings.subBalancers.observatory.destination')}
            description={t('pages.settings.subBalancers.observatory.destinationDesc')}
          >
            <Input
              value={observatoryObj.destination}
              placeholder="https://www.google.com/generate_204"
              onChange={(e) => setObservatoryField('destination', e.target.value)}
            />
          </SettingListItem>
          <SettingListItem
            paddings="small"
            title={t('pages.settings.subBalancers.observatory.connectivity')}
            description={t('pages.settings.subBalancers.observatory.connectivityDesc')}
          >
            <Input
              value={observatoryObj.connectivity}
              placeholder="http://connectivitycheck.platform.hicloud.com/generate_204"
              onChange={(e) => setObservatoryField('connectivity', e.target.value)}
            />
          </SettingListItem>
          <SettingListItem
            paddings="small"
            title={t('pages.settings.subBalancers.observatory.interval')}
            description={t('pages.settings.subBalancers.observatory.intervalDesc')}
          >
            <Input
              value={observatoryObj.interval}
              placeholder="1m"
              onChange={(e) => setObservatoryField('interval', e.target.value)}
            />
          </SettingListItem>
          <SettingListItem
            paddings="small"
            title={t('pages.settings.subBalancers.observatory.timeout')}
            description={t('pages.settings.subBalancers.observatory.timeoutDesc')}
          >
            <Input
              value={observatoryObj.timeout}
              placeholder="5s"
              onChange={(e) => setObservatoryField('timeout', e.target.value)}
            />
          </SettingListItem>
          <SettingListItem
            paddings="small"
            title={t('pages.settings.subBalancers.observatory.sampling')}
            description={t('pages.settings.subBalancers.observatory.samplingDesc')}
          >
            <InputNumber
              value={observatoryObj.sampling}
              min={1}
              style={{ width: '100%' }}
              onChange={onNumber((v) => setObservatoryField('sampling', v))}
            />
          </SettingListItem>
          <SettingListItem
            paddings="small"
            title={t('pages.settings.subBalancers.observatory.httpMethod')}
            description={t('pages.settings.subBalancers.observatory.httpMethodDesc')}
          >
            <Select
              value={observatoryObj.httpMethod}
              style={{ width: '100%' }}
              onChange={(v) => setObservatoryField('httpMethod', v)}
              options={['HEAD', 'GET'].map((m) => ({ value: m, label: m }))}
            />
          </SettingListItem>
        </div>
      )}
    </>
  );

  return (
    <>
      <Tabs
        defaultActiveKey="balancers"
        items={[
          {
            key: 'balancers',
            label: catTabLabel(
              <DeploymentUnitOutlined />,
              t('pages.settings.subBalancers.tabBalancers'),
              isMobile,
            ),
            children: balancersTab,
          },
          {
            key: 'observatory',
            label: catTabLabel(
              <RadarChartOutlined />,
              t('pages.settings.subBalancers.tabObservatory'),
              isMobile,
            ),
            children: observatoryTab,
          },
        ]}
      />
      <SubBalancerFormModal
        open={modalOpen}
        balancer={editing}
        onClose={() => setModalOpen(false)}
        onConfirm={onConfirm}
      />
    </>
  );
}
