import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Empty, Modal, Select, Space, Table, Tabs, Tag, Tooltip, message } from 'antd';
import { PlusOutlined, MoreOutlined, EditOutlined, DeleteOutlined, SyncOutlined, DeploymentUnitOutlined, RadarChartOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import BalancerFormModal from './BalancerFormModal';
import type { BalancerFormValue } from './BalancerFormModal';
import { syncObservatories } from './balancer-helpers';
import {
  isBalancerLoopbackTag,
  loopbackTagFor,
  resolveLoopbackFallback,
  ensureBalancerLoopback,
  removeBalancerLoopback,
  removeBalancerLoopbackIfOrphaned,
  propagateBalancerTagRename,
} from './balancer-loopback';
import { planBalancerDeletion, applyBalancerDeletion } from '../reference-cleanup';
import DeletionImpactList from '../DeletionImpactList';
import ObservatorySettingsTab from './ObservatorySettingsTab';
import { catTabLabel } from '@/pages/settings/catTabLabel';
import { HttpUtil } from '@/utils';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import type {
  BalancerObject,
  BalancerStrategySettings,
  BalancerStrategyType,
} from '@/schemas/routing';

// Live state of one balancer inside the running core, as reported by the
// panel's /xray/balancerStatus endpoint (RoutingService.GetBalancerInfo).
interface BalancerLiveStatus {
  tag: string;
  running: boolean;
  override: string;
  selected: string[];
}

interface BalancersTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  clientReverseTags: string[];
  subscriptionOutboundTags?: string[];
  isMobile: boolean;
}

type BalancerRecord = BalancerObject;

interface BalancerRow {
  key: number;
  tag: string;
  strategy: BalancerStrategyType;
  selector: string[];
  fallbackTag: string;
  displayFallbackTag: string;
  settings?: BalancerStrategySettings;
}

const STRATEGY_LABELS: Record<string, string> = {
  random: 'Random',
  roundRobin: 'Round robin',
  leastLoad: 'Least load',
  leastPing: 'Least ping',
};

export default function BalancersTab({
  templateSettings,
  setTemplateSettings,
  clientReverseTags,
  subscriptionOutboundTags,
  isMobile,
}: BalancersTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingBalancer, setEditingBalancer] = useState<BalancerFormValue | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);

  const balancerObjects = useMemo(
    () => (templateSettings?.routing?.balancers || []) as BalancerObject[],
    [templateSettings?.routing?.balancers],
  );

  const rows: BalancerRow[] = useMemo(() => {
    const list = balancerObjects;
    return list.map((b, idx) => ({
      key: idx,
      tag: b.tag || '',
      strategy: (b.strategy?.type ?? 'random') as BalancerStrategyType,
      selector: b.selector || [],
      fallbackTag: b.fallbackTag || '',
      displayFallbackTag: resolveLoopbackFallback(templateSettings!, b.fallbackTag || ''),
      settings: b.strategy?.settings,
    }));
  }, [balancerObjects, templateSettings]);

  const outboundTags = useMemo(() => {
    const tags = new Set<string>();
    for (const o of templateSettings?.outbounds || []) {
      if (o?.tag && !isBalancerLoopbackTag(o.tag)) tags.add(o.tag);
    }
    for (const tag of clientReverseTags || []) {
      if (tag) tags.add(tag);
    }
    for (const tag of subscriptionOutboundTags || []) {
      if (tag) tags.add(tag);
    }
    return [...tags];
  }, [templateSettings?.outbounds, clientReverseTags, subscriptionOutboundTags]);

  const otherTags = useMemo(() => {
    if (editingIndex == null) return rows.map((b) => b.tag).filter(Boolean);
    return rows.filter((b) => b.key !== editingIndex).map((b) => b.tag).filter(Boolean);
  }, [rows, editingIndex]);

  const balancerTags = useMemo(() => {
    return otherTags.filter((tg) => !isBalancerLoopbackTag(tg));
  }, [otherTags]);

  const overrideOptions: Array<{ value: string; label: React.ReactNode }> = useMemo(() => {
    return outboundTags.map((tag) => ({ value: tag, label: tag }));
  }, [outboundTags]);

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  const [liveStatus, setLiveStatus] = useState<Record<string, BalancerLiveStatus>>({});
  const [liveLoading, setLiveLoading] = useState(false);
  const liveTags = useMemo(
    () => rows.map((r) => r.tag).filter(Boolean).join(','),
    [rows],
  );

  const refreshLive = useCallback(async () => {
    if (!liveTags) {
      setLiveStatus({});
      return;
    }
    setLiveLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/balancerStatus', { tags: liveTags }, { silent: true });
      if (msg?.success && msg.obj && typeof msg.obj === 'object') {
        setLiveStatus(msg.obj as Record<string, BalancerLiveStatus>);
      }
    } finally {
      setLiveLoading(false);
    }
  }, [liveTags]);

  useEffect(() => {
    refreshLive();
  }, [refreshLive]);

  async function setOverride(tag: string, target: string) {
    const msg = await HttpUtil.post('/panel/api/xray/balancerOverride', { tag, target });
    if (msg?.success) await refreshLive();
  }

  function openAdd() {
    setEditingBalancer(null);
    setEditingIndex(null);
    setModalOpen(true);
  }
  function openEdit(idx: number) {
    const row = rows[idx];
    const resolved: BalancerFormValue = {
      ...row,
      fallbackTag: resolveLoopbackFallback(templateSettings!, row.fallbackTag),
    };
    setEditingBalancer(resolved);
    setEditingIndex(idx);
    setModalOpen(true);
  }

  function onConfirm(form: BalancerFormValue) {
    mutate((tt) => {
      if (!tt.routing) tt.routing = { rules: [], balancers: [] };
      if (!Array.isArray(tt.routing.balancers)) tt.routing.balancers = [];
      const list = tt.routing.balancers as BalancerRecord[];

      const wire: BalancerRecord = {
        tag: form.tag,
        selector: [...form.selector],
        fallbackTag: '',
      };
      if (form.strategy && form.strategy !== 'random') {
        wire.strategy = { type: form.strategy };
        if (form.strategy === 'leastLoad' && form.settings) {
          wire.strategy.settings = form.settings;
        }
      }

      const isFallbackABalancer = form.fallbackTag && balancerTags.includes(form.fallbackTag);

      if (isFallbackABalancer) {
        wire.fallbackTag = loopbackTagFor(form.fallbackTag);
      } else {
        wire.fallbackTag = form.fallbackTag || '';
      }

      if (editingIndex == null) {
        list.push(wire);
        if (isFallbackABalancer) {
          ensureBalancerLoopback(tt, form.fallbackTag);
        }
      } else {
        const oldTag = list[editingIndex]?.tag;
        const oldFallback = list[editingIndex]?.fallbackTag || '';
        list[editingIndex] = wire;

        if (oldTag && oldTag !== wire.tag) {
          const rules = tt.routing.rules || [];
          for (const rule of rules) {
            if (rule?.balancerTag === oldTag) rule.balancerTag = wire.tag;
          }
          propagateBalancerTagRename(tt, oldTag, wire.tag);
        }

        const oldTarget = isBalancerLoopbackTag(oldFallback)
          ? (oldFallback.slice(4))
          : null;

        if (oldTarget && oldTarget !== form.fallbackTag) {
          removeBalancerLoopbackIfOrphaned(tt, oldTarget);
        }
        if (isFallbackABalancer) {
          ensureBalancerLoopback(tt, form.fallbackTag);
        }
      }
      syncObservatories(tt);
    });
    setModalOpen(false);
  }

  function confirmDelete(idx: number) {
    const deletedTag = rows[idx]?.tag;
    const lbTag = loopbackTagFor(deletedTag);
    const dependents = (templateSettings?.routing?.balancers || [])
      .filter((b) => b.tag !== deletedTag && b.fallbackTag === lbTag)
      .map((b) => b.tag);
    if (dependents.length > 0) {
      messageApi.error(t('pages.xray.balancer.balancerDeleteInUse', { names: dependents.join(', ') }));
      return;
    }
    const impact = templateSettings
      ? planBalancerDeletion(templateSettings, idx)
      : { rules: [], balancers: [], observatory: false, burst: false };
    modal.confirm({
      title: `${t('delete')} ${t('pages.xray.Balancers')} #${idx + 1}?`,
      content: <DeletionImpactList impact={impact} />,
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: () => mutate((tt) => {
        const tag = tt.routing?.balancers?.[idx]?.tag ?? '';
        removeBalancerLoopback(tt, tag);
        applyBalancerDeletion(tt, idx);
      }),
    });
  }

  const columns: ColumnsType<BalancerRow> = [
    {
      title: '#',
      key: 'action',
      align: 'center',
      width: 100,
      render: (_v, _record, index) => (
        <div className="action-cell">
          <span className="row-index">{index + 1}</span>
          <div className={!isMobile ? 'action-buttons' : ''}>
            {!isMobile && (
              <Button aria-label={t('edit')} shape="circle" size="small" icon={<EditOutlined />} onClick={() => openEdit(index)} />
            )}
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  ...(isMobile
                    ? [
                        {
                          key: 'edit',
                          label: (
                            <>
                              <EditOutlined /> {t('edit')}
                            </>
                          ),
                          onClick: () => openEdit(index),
                        },
                      ]
                    : []),
                  {
                    key: 'del',
                    danger: true,
                    label: (
                      <>
                        <DeleteOutlined /> {t('delete')}
                      </>
                    ),
                    onClick: () => confirmDelete(index),
                  },
                ],
              }}
            >
              <Button aria-label={t('more')} shape="circle" size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </div>
        </div>
      ),
    },
    { title: 'Tag', dataIndex: 'tag', key: 'tag', align: 'center', width: 160 },
    {
      title: 'Strategy',
      key: 'strategy',
      align: 'center',
      width: 140,
      render: (_v, record) => (
        <Tag color={record.strategy === 'random' ? 'purple' : 'green'}>
          {STRATEGY_LABELS[record.strategy] || record.strategy}
        </Tag>
      ),
    },
    {
      title: 'Selector',
      key: 'selector',
      align: 'center',
      render: (_v, record) =>
        (record.selector || []).map((sel) => (
          <Tag key={sel} className="info-large-tag" style={{ margin: 0, marginRight: 4 }}>
            {sel}
          </Tag>
        )),
    },
    { title: 'Fallback', dataIndex: 'displayFallbackTag', key: 'displayFallbackTag', align: 'center', width: 160 },
    {
      title: t('pages.xray.balancerLive'),
      key: 'live',
      align: 'center',
      width: 170,
      render: (_v, record) => {
        const live = liveStatus[record.tag];
        if (!live?.running) {
          return (
            <Tooltip title={t('pages.xray.balancerNotRunning')}>
              <Tag>—</Tag>
            </Tooltip>
          );
        }
        const resolve = (tag: string) => isBalancerLoopbackTag(tag) ? resolveLoopbackFallback(templateSettings!, tag) : tag;
        const picked = live.override ? resolve(live.override) : live.selected?.[0] ? resolve(live.selected[0]) : record.displayFallbackTag;
        const tooltipText = live.override
          ? resolve(live.override)
          : (live.selected || []).map(resolve).join(', ');
        return (
          <Tooltip title={tooltipText || undefined}>
            <Tag color={live.override ? 'orange' : 'blue'}>{picked || '—'}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: t('pages.xray.balancerOverride'),
      key: 'overrideTarget',
      align: 'center',
      width: 200,
      render: (_v, record) => {
        const live = liveStatus[record.tag];
        const resolvedFB = record.displayFallbackTag;
        let options = overrideOptions;
        if (resolvedFB && !outboundTags.includes(resolvedFB)) {
          options = [...overrideOptions, {
            value: resolvedFB,
            label: (
              <span>
                <Tag color="blue" style={{ marginRight: 4 }}>{t('pages.xray.rules.balancer')}</Tag>
                {resolvedFB}
              </span>
            ),
          }];
        }
        const rawOverride = live?.override || undefined;
        const resolvedOverride = rawOverride && isBalancerLoopbackTag(rawOverride)
          ? resolveLoopbackFallback(templateSettings!, rawOverride)
          : rawOverride;
        return (
          <Select
            size="small"
            style={{ width: 170 }}
            placeholder={t('pages.xray.balancerOverridePh')}
            allowClear
            disabled={!live?.running}
            value={resolvedOverride}
            options={options}
            onChange={(v) => setOverride(record.tag, (v as string | undefined) || '')}
          />
        );
      },
    },
  ];

  const balancerSettingsTab = (
    <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
      {rows.length === 0 ? (
        <Empty description={t('emptyBalancersDesc')}>
          <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
            {t('pages.xray.Balancers')}
          </Button>
        </Empty>
      ) : (
        <>
          <Space>
            <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
              {t('pages.xray.Balancers')}
            </Button>
            <Tooltip title={t('pages.xray.balancerLiveRefresh')}>
              <Button aria-label={t('pages.xray.balancerLiveRefresh')} icon={<SyncOutlined spin={liveLoading} />} onClick={refreshLive} />
            </Tooltip>
          </Space>

          <Table
            columns={columns}
            dataSource={rows}
            rowKey={(r) => r.key}
            pagination={false}
            size="small"
            scroll={{ x: 700 }}
          />
        </>
      )}
    </Space>
  );

  return (
    <>
      {modalContextHolder}
      {messageContextHolder}
      <Tabs
        items={[
          {
            key: 'balancers',
            label: catTabLabel(<DeploymentUnitOutlined />, t('pages.xray.tabBalancerSettings'), isMobile),
            children: balancerSettingsTab,
          },
          {
            key: 'observatory',
            label: catTabLabel(<RadarChartOutlined />, t('pages.xray.tabObservatory'), isMobile),
            children: (
              <ObservatorySettingsTab
                templateSettings={templateSettings}
                mutate={mutate}
              />
            ),
          },
        ]}
      />

      <BalancerFormModal
        key={modalOpen ? `${editingIndex ?? 'new'}-${editingBalancer?.tag ?? ''}` : 'closed'}
        open={modalOpen}
        balancer={editingBalancer}
        outboundTags={outboundTags}
        balancerTags={balancerTags}
        balancers={balancerObjects}
        templateSettings={templateSettings}
        otherTags={otherTags}
        onClose={() => setModalOpen(false)}
        onConfirm={onConfirm}
      />
    </>
  );
}
