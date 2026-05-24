import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Dropdown, Empty, Modal, Radio, Space, Table, Tag } from 'antd';
import { PlusOutlined, MoreOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import BalancerFormModal from './BalancerFormModal';
import type { BalancerFormValue } from './BalancerFormModal';
import JsonEditor from '@/components/JsonEditor';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';

interface BalancersTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  clientReverseTags: string[];
  isMobile: boolean;
}

interface BalancerRecord {
  tag: string;
  selector?: string[];
  fallbackTag?: string;
  strategy?: { type?: string };
}

interface BalancerRow {
  key: number;
  tag: string;
  strategy: string;
  selector: string[];
  fallbackTag: string;
}

const STRATEGY_LABELS: Record<string, string> = {
  random: 'Random',
  roundRobin: 'Round robin',
  leastLoad: 'Least load',
  leastPing: 'Least ping',
};

const DEFAULT_OBSERVATORY = Object.freeze({
  subjectSelector: [] as string[],
  probeURL: 'https://www.google.com/generate_204',
  probeInterval: '1m',
  enableConcurrency: true,
});

const DEFAULT_BURST_OBSERVATORY = Object.freeze({
  subjectSelector: [] as string[],
  pingConfig: {
    destination: 'https://www.google.com/generate_204',
    interval: '1m',
    connectivity: 'http://connectivitycheck.platform.hicloud.com/generate_204',
    timeout: '5s',
    sampling: 2,
  },
});

function collectSelectors(list: BalancerRecord[]): string[] {
  const out = new Set<string>();
  list.forEach((b) => (b.selector || []).forEach((s) => s && out.add(s)));
  return [...out];
}

function syncObservatories(t: XraySettingsValue) {
  const balancers = (t.routing?.balancers || []) as BalancerRecord[];

  const leastPings = balancers.filter((b) => b.strategy?.type === 'leastPing');
  if (leastPings.length > 0) {
    if (!t.observatory) t.observatory = JSON.parse(JSON.stringify(DEFAULT_OBSERVATORY));
    (t.observatory as { subjectSelector: string[] }).subjectSelector = collectSelectors(leastPings);
  } else {
    delete t.observatory;
  }

  const burstFeeders = balancers.filter((b) => {
    const type = b.strategy?.type || 'random';
    return type === 'leastLoad' || type === 'random' || type === 'roundRobin';
  });
  if (burstFeeders.length > 0) {
    if (!t.burstObservatory) t.burstObservatory = JSON.parse(JSON.stringify(DEFAULT_BURST_OBSERVATORY));
    (t.burstObservatory as { subjectSelector: string[] }).subjectSelector = collectSelectors(burstFeeders);
  } else {
    delete t.burstObservatory;
  }
}

export default function BalancersTab({
  templateSettings,
  setTemplateSettings,
  clientReverseTags,
  isMobile,
}: BalancersTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingBalancer, setEditingBalancer] = useState<BalancerFormValue | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);

  const rows: BalancerRow[] = useMemo(() => {
    const list = (templateSettings?.routing?.balancers || []) as BalancerRecord[];
    return list.map((b, idx) => ({
      key: idx,
      tag: b.tag || '',
      strategy: b.strategy?.type || 'random',
      selector: b.selector || [],
      fallbackTag: b.fallbackTag || '',
    }));
  }, [templateSettings?.routing?.balancers]);

  const outboundTags = useMemo(() => {
    const tags = new Set<string>();
    for (const o of templateSettings?.outbounds || []) {
      if (o?.tag) tags.add(o.tag);
    }
    for (const tag of clientReverseTags || []) {
      if (tag) tags.add(tag);
    }
    return [...tags];
  }, [templateSettings?.outbounds, clientReverseTags]);

  const otherTags = useMemo(() => {
    if (editingIndex == null) return rows.map((b) => b.tag).filter(Boolean);
    return rows.filter((b) => b.key !== editingIndex).map((b) => b.tag).filter(Boolean);
  }, [rows, editingIndex]);

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

  function openAdd() {
    setEditingBalancer(null);
    setEditingIndex(null);
    setModalOpen(true);
  }
  function openEdit(idx: number) {
    setEditingBalancer(rows[idx]);
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
        fallbackTag: form.fallbackTag || '',
      };
      if (form.strategy && form.strategy !== 'random') {
        wire.strategy = { type: form.strategy };
      }
      if (editingIndex == null) {
        list.push(wire);
      } else {
        const oldTag = list[editingIndex]?.tag;
        list[editingIndex] = wire;
        if (oldTag && oldTag !== wire.tag) {
          const rules = tt.routing.rules || [];
          for (const rule of rules) {
            if (rule?.balancerTag === oldTag) rule.balancerTag = wire.tag;
          }
        }
      }
      syncObservatories(tt);
    });
    setModalOpen(false);
  }

  function confirmDelete(idx: number) {
    modal.confirm({
      title: `${t('delete')} ${t('pages.xray.Balancers')} #${idx + 1}?`,
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: () => mutate((tt) => {
        if (tt.routing?.balancers) {
          tt.routing.balancers.splice(idx, 1);
          syncObservatories(tt);
        }
      }),
    });
  }

  const columns: ColumnsType<BalancerRow> = useMemo(
    () => [
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
                <Button shape="circle" size="small" icon={<EditOutlined />} onClick={() => openEdit(index)} />
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
                <Button shape="circle" size="small" icon={<MoreOutlined />} />
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
            <Tag key={sel} className="info-large-tag">
              {sel}
            </Tag>
          )),
      },
      { title: 'Fallback', dataIndex: 'fallbackTag', key: 'fallbackTag', align: 'center', width: 160 },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, isMobile],
  );

  const hasObservatory = !!templateSettings?.observatory;
  const hasBurstObservatory = !!templateSettings?.burstObservatory;
  const showObsEditor = hasObservatory || hasBurstObservatory;

  const [obsView, setObsView] = useState<'observatory' | 'burstObservatory'>('observatory');

  useEffect(() => {
    if (obsView === 'observatory' && !hasObservatory && hasBurstObservatory) {
      setObsView('burstObservatory');
    } else if (obsView === 'burstObservatory' && !hasBurstObservatory && hasObservatory) {
      setObsView('observatory');
    }
  }, [obsView, hasObservatory, hasBurstObservatory]);

  const obsText = useMemo(() => {
    const src = obsView === 'observatory' ? templateSettings?.observatory : templateSettings?.burstObservatory;
    return src ? JSON.stringify(src, null, 2) : '';
  }, [obsView, templateSettings?.observatory, templateSettings?.burstObservatory]);

  function onObsTextChange(next: string) {
    let parsed;
    try {
      parsed = JSON.parse(next);
    } catch {
      return;
    }
    mutate((tt) => {
      if (obsView === 'observatory') tt.observatory = parsed;
      else tt.burstObservatory = parsed;
    });
  }

  return (
    <>
      {modalContextHolder}
      <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
        {rows.length === 0 ? (
          <Empty description={t('emptyBalancersDesc')}>
            <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
              {t('pages.xray.Balancers')}
            </Button>
          </Empty>
        ) : (
          <>
            <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
              {t('pages.xray.Balancers')}
            </Button>

            <Table
              columns={columns}
              dataSource={rows}
              rowKey={(r) => r.key}
              pagination={false}
              size="small"
              scroll={{ x: 400 }}
            />

            {showObsEditor && (
              <>
                <Divider style={{ margin: '8px 0' }} />
                <Radio.Group
                  value={obsView}
                  onChange={(e) => setObsView(e.target.value)}
                  optionType="button"
                  buttonStyle="solid"
                  size="small"
                >
                  {hasObservatory && <Radio.Button value="observatory">Observatory</Radio.Button>}
                  {hasBurstObservatory && <Radio.Button value="burstObservatory">Burst Observatory</Radio.Button>}
                </Radio.Group>
                <JsonEditor
                  value={obsText}
                  onChange={onObsTextChange}
                  minHeight="220px"
                  maxHeight="480px"
                />
              </>
            )}
          </>
        )}
      </Space>

      <BalancerFormModal
        open={modalOpen}
        balancer={editingBalancer}
        outboundTags={outboundTags}
        otherTags={otherTags}
        onClose={() => setModalOpen(false)}
        onConfirm={onConfirm}
      />
    </>
  );
}
