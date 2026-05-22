import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Modal, Space, Table, Tag, Tooltip } from 'antd';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  ExportOutlined,
  ClusterOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  HolderOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import RuleFormModal from './RuleFormModal';
import type { RoutingRule } from './RuleFormModal';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import './RoutingTab.css';

interface RoutingTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  inboundTags: string[];
  clientReverseTags: string[];
  isMobile: boolean;
}

interface RuleRow {
  key: number;
  domain?: string;
  ip?: string;
  port?: string;
  sourcePort?: string;
  vlessRoute?: string;
  network?: string;
  sourceIP?: string;
  user?: string;
  inboundTag?: string;
  protocol?: string;
  attrs?: string;
  outboundTag?: string;
  balancerTag?: string;
}

function arrJoin(v: unknown): string | undefined {
  if (v == null) return undefined;
  if (Array.isArray(v)) return v.join(',');
  return String(v);
}

function csv(value?: string): string[] {
  if (!value) return [];
  return String(value).split(',').map((s) => s.trim()).filter(Boolean);
}

function chipPreview(value?: string): string {
  const parts = csv(value);
  if (parts.length === 0) return '';
  if (parts.length === 1) return parts[0];
  return `${parts[0]} +${parts.length - 1}`;
}

export default function RoutingTab({
  templateSettings,
  setTemplateSettings,
  inboundTags,
  clientReverseTags,
  isMobile,
}: RoutingTabProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [ruleModalOpen, setRuleModalOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<RoutingRule | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [dropTargetIndex, setDropTargetIndex] = useState<number | null>(null);
  const dragRef = useRef<{ from: number | null; to: number | null; startY: number; moved: boolean }>({
    from: null,
    to: null,
    startY: 0,
    moved: false,
  });

  const rules = useMemo(
    () => (templateSettings?.routing?.rules || []) as RoutingRule[],
    [templateSettings?.routing?.rules],
  );

  const rows: RuleRow[] = useMemo(
    () =>
      rules.map((rule, idx) => {
        const r: RuleRow = { key: idx };
        r.domain = arrJoin(rule.domain);
        r.ip = arrJoin(rule.ip);
        r.port = rule.port;
        r.sourcePort = rule.sourcePort;
        r.vlessRoute = rule.vlessRoute;
        r.network = rule.network;
        r.sourceIP = arrJoin(rule.sourceIP);
        r.user = arrJoin(rule.user);
        r.inboundTag = arrJoin(rule.inboundTag);
        r.protocol = arrJoin(rule.protocol);
        if (rule.attrs && typeof rule.attrs === 'object' && !Array.isArray(rule.attrs)) {
          r.attrs = JSON.stringify(rule.attrs, null, 2);
        }
        r.outboundTag = rule.outboundTag;
        r.balancerTag = rule.balancerTag;
        return r;
      }),
    [rules],
  );

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

  const inboundTagOptions = useMemo(() => {
    const seen = new Set<string>();
    const out: string[] = [];
    const push = (tag?: string) => {
      if (!tag || seen.has(tag)) return;
      seen.add(tag);
      out.push(tag);
    };
    for (const ib of (templateSettings?.inbounds as Array<{ tag?: string }>) || []) push(ib?.tag);
    for (const tag of inboundTags || []) push(tag);
    for (const ob of templateSettings?.outbounds || []) {
      const obx = ob as { reverse?: { tag?: string }; settings?: { reverse?: { tag?: string }; inboundTag?: string } };
      push(obx?.reverse?.tag || obx?.settings?.reverse?.tag || obx?.settings?.inboundTag);
    }
    push((templateSettings?.dns as { tag?: string } | undefined)?.tag);
    for (const s of (templateSettings?.dns as { servers?: Array<{ tag?: string }> } | undefined)?.servers || []) {
      if (typeof s === 'object' && s?.tag) push(s.tag);
    }
    return out;
  }, [templateSettings, inboundTags]);

  const outboundTagOptions = useMemo(() => {
    const out = new Set<string>(['']);
    for (const ob of templateSettings?.outbounds || []) {
      if (ob?.tag) out.add(ob.tag);
    }
    for (const tag of clientReverseTags || []) {
      if (tag) out.add(tag);
    }
    return [...out];
  }, [templateSettings?.outbounds, clientReverseTags]);

  const balancerTagOptions = useMemo(() => {
    const out: string[] = [''];
    for (const b of (templateSettings?.routing?.balancers as Array<{ tag?: string }>) || []) {
      if (b?.tag) out.push(b.tag);
    }
    return out;
  }, [templateSettings?.routing?.balancers]);

  function openAdd() {
    setEditingRule(null);
    setEditingIndex(null);
    setRuleModalOpen(true);
  }
  function openEdit(idx: number) {
    setEditingRule(rules[idx]);
    setEditingIndex(idx);
    setRuleModalOpen(true);
  }
  function onRuleConfirm(rule: Record<string, unknown>) {
    if (JSON.stringify(rule).length <= 3) {
      setRuleModalOpen(false);
      return;
    }
    mutate((tt) => {
      if (!tt.routing) tt.routing = { rules: [] };
      if (!Array.isArray(tt.routing.rules)) tt.routing.rules = [];
      if (editingIndex == null) tt.routing.rules.push(rule);
      else tt.routing.rules[editingIndex] = rule;
    });
    setRuleModalOpen(false);
  }

  function confirmDelete(idx: number) {
    modal.confirm({
      title: `${t('delete')} ${t('pages.xray.Routings')} #${idx + 1}?`,
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: () => mutate((tt) => {
        tt.routing?.rules?.splice(idx, 1);
      }),
    });
  }

  function moveUp(idx: number) {
    if (idx <= 0) return;
    mutate((tt) => {
      const list = tt.routing?.rules;
      if (!list) return;
      [list[idx - 1], list[idx]] = [list[idx], list[idx - 1]];
    });
  }
  function moveDown(idx: number) {
    mutate((tt) => {
      const list = tt.routing?.rules;
      if (!list || idx >= list.length - 1) return;
      [list[idx + 1], list[idx]] = [list[idx], list[idx + 1]];
    });
  }

  function onHandlePointerDown(idx: number, ev: React.PointerEvent) {
    if (ev.button != null && ev.button !== 0) return;
    ev.preventDefault();
    try {
      (ev.currentTarget as Element).setPointerCapture(ev.pointerId);
    } catch { /* ignore */ }
    dragRef.current = { from: idx, to: idx, startY: ev.clientY, moved: false };
    setDraggedIndex(idx);
    setDropTargetIndex(idx);

    const onMove = (e: PointerEvent) => {
      const state = dragRef.current;
      if (state.from == null) return;
      if (!state.moved && Math.abs(e.clientY - state.startY) < 5) return;
      state.moved = true;
      const el = document.elementFromPoint(e.clientX, e.clientY);
      if (!el) return;
      const target = el.closest('[data-row-key]');
      if (!target) return;
      const newIdx = Number(target.getAttribute('data-row-key'));
      if (Number.isFinite(newIdx) && newIdx !== state.to) {
        state.to = newIdx;
        setDropTargetIndex(newIdx);
      }
    };

    const onUp = () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
      document.removeEventListener('pointercancel', onUp);
      const { from, to, moved } = dragRef.current;
      dragRef.current = { from: null, to: null, startY: 0, moved: false };
      setDraggedIndex(null);
      setDropTargetIndex(null);
      if (!moved || from == null || to == null || from === to) return;
      mutate((tt) => {
        const list = tt.routing?.rules;
        if (!list) return;
        const [movedItem] = list.splice(from, 1);
        list.splice(to, 0, movedItem);
      });
    };

    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
    document.addEventListener('pointercancel', onUp);
  }

  function ruleCriteriaChips(rule: RuleRow) {
    const chips: { label: string; value?: string }[] = [];
    if (rule.domain) chips.push({ label: 'Domain', value: rule.domain });
    if (rule.ip) chips.push({ label: 'IP', value: rule.ip });
    if (rule.port) chips.push({ label: 'Port', value: rule.port });
    if (rule.sourceIP) chips.push({ label: 'Src IP', value: rule.sourceIP });
    if (rule.sourcePort) chips.push({ label: 'Src Port', value: rule.sourcePort });
    if (rule.network) chips.push({ label: 'L4', value: rule.network });
    if (rule.protocol) chips.push({ label: 'Protocol', value: rule.protocol });
    if (rule.user) chips.push({ label: 'User', value: rule.user });
    if (rule.vlessRoute) chips.push({ label: 'VLESS', value: rule.vlessRoute });
    return chips;
  }

  const desktopColumns: ColumnsType<RuleRow> = useMemo(
    () => [
      {
        title: '#',
        align: 'center',
        width: 100,
        key: 'action',
        render: (_v, _r, index) => (
          <div className="action-cell">
            <HolderOutlined
              className="drag-handle"
              title="Drag to reorder"
              onPointerDown={(ev: React.PointerEvent) => onHandlePointerDown(index, ev)}
            />
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
                      ? [{ key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEdit(index) }]
                      : []),
                    { key: 'up', label: <ArrowUpOutlined />, disabled: index === 0, onClick: () => moveUp(index) },
                    {
                      key: 'down',
                      label: <ArrowDownOutlined />,
                      disabled: index === rows.length - 1,
                      onClick: () => moveDown(index),
                    },
                    { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                  ],
                }}
              >
                <Button shape="circle" size="small" icon={<MoreOutlined />} />
              </Dropdown>
            </div>
          </div>
        ),
      },
      {
        title: 'Source',
        align: 'left',
        width: 180,
        key: 'source',
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.sourceIP && <CriterionRow label="IP" value={record.sourceIP} title={`Source IP: ${record.sourceIP}`} />}
            {record.sourcePort && <CriterionRow label="Port" value={record.sourcePort} title={`Source port: ${record.sourcePort}`} />}
            {record.vlessRoute && <CriterionRow label="VLESS" value={record.vlessRoute} title={`VLESS route: ${record.vlessRoute}`} />}
            {!record.sourceIP && !record.sourcePort && !record.vlessRoute && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.inbounds.network'),
        align: 'left',
        width: 180,
        key: 'network',
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.network && <CriterionRow label="L4" value={record.network} title={`L4: ${record.network}`} />}
            {record.protocol && <CriterionRow label="Protocol" value={record.protocol} title={`Protocol: ${record.protocol}`} />}
            {record.attrs && <CriterionRow label="Attrs" value={record.attrs} title={`Attrs: ${record.attrs}`} />}
            {!record.network && !record.protocol && !record.attrs && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: 'Destination',
        align: 'left',
        key: 'destination',
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.ip && <CriterionRow label="IP" value={record.ip} title={`Destination IP: ${record.ip}`} />}
            {record.domain && <CriterionRow label="Domain" value={record.domain} title={`Domain: ${record.domain}`} />}
            {record.port && <CriterionRow label="Port" value={record.port} title={`Destination port: ${record.port}`} />}
            {!record.ip && !record.domain && !record.port && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.xray.Inbounds'),
        align: 'left',
        width: 180,
        key: 'inbound',
        render: (_v, record) => (
          <div className="criterion-flow">
            {record.inboundTag && <CriterionRow label="Tag" value={record.inboundTag} title={`Inbound tag: ${record.inboundTag}`} />}
            {record.user && <CriterionRow label="User" value={record.user} title={`User: ${record.user}`} />}
            {!record.inboundTag && !record.user && <span className="criterion-empty">—</span>}
          </div>
        ),
      },
      {
        title: t('pages.xray.Outbounds'),
        align: 'left',
        width: 170,
        key: 'outbound',
        render: (_v, record) =>
          record.outboundTag ? (
            <div className="target-row">
              <ExportOutlined className="target-icon" />
              <Tag color="green">{record.outboundTag}</Tag>
            </div>
          ) : (
            <span className="criterion-empty">—</span>
          ),
      },
      {
        title: t('pages.xray.Balancers'),
        align: 'left',
        width: 150,
        key: 'balancer',
        render: (_v, record) =>
          record.balancerTag ? (
            <div className="target-row">
              <ClusterOutlined className="target-icon" />
              <Tag color="purple">{record.balancerTag}</Tag>
            </div>
          ) : (
            <span className="criterion-empty">—</span>
          ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, isMobile, rows.length],
  );

  return (
    <>
      {modalContextHolder}
      <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
          {t('pages.xray.Routings')}
        </Button>

        {isMobile ? (
          <div className="rule-list">
            {rows.length === 0 ? (
              <div className="rule-empty">—</div>
            ) : (
              rows.map((rule, index) => (
                <div
                  key={rule.key}
                  className={`rule-card ${draggedIndex === index ? 'row-dragging' : ''} ${
                    dropTargetIndex === index && draggedIndex != null && index < draggedIndex ? 'drop-before' : ''
                  } ${dropTargetIndex === index && draggedIndex != null && index > draggedIndex ? 'drop-after' : ''}`}
                  data-row-key={index}
                >
                  <div className="rule-card-head">
                    <HolderOutlined
                      className="drag-handle"
                      onPointerDown={(ev) => onHandlePointerDown(index, ev)}
                    />
                    <span className="rule-number">#{index + 1}</span>
                    <Dropdown
                      trigger={['click']}
                      menu={{
                        items: [
                          { key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEdit(index) },
                          { key: 'up', label: <ArrowUpOutlined />, disabled: index === 0, onClick: () => moveUp(index) },
                          { key: 'down', label: <ArrowDownOutlined />, disabled: index === rows.length - 1, onClick: () => moveDown(index) },
                          { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => confirmDelete(index) },
                        ],
                      }}
                    >
                      <Button shape="circle" size="small" icon={<MoreOutlined />} />
                    </Dropdown>
                  </div>

                  <div className="rule-flow">
                    <div className="flow-side">
                      <span className="flow-label">{t('pages.xray.Inbounds')}</span>
                      {rule.inboundTag ? (
                        <Tag color="blue" className="flow-tag">{chipPreview(rule.inboundTag)}</Tag>
                      ) : (
                        <span className="criterion-empty">any</span>
                      )}
                    </div>
                    <span className="flow-arrow">→</span>
                    <div className="flow-side flow-side-target">
                      <span className="flow-label">
                        {rule.balancerTag ? t('pages.xray.balancer') || 'Balancer' : t('pages.xray.Outbounds')}
                      </span>
                      {rule.outboundTag ? (
                        <Tag color="green" className="flow-tag">
                          <ExportOutlined /> {rule.outboundTag}
                        </Tag>
                      ) : rule.balancerTag ? (
                        <Tag color="purple" className="flow-tag">
                          <ClusterOutlined /> {rule.balancerTag}
                        </Tag>
                      ) : (
                        <span className="criterion-empty">—</span>
                      )}
                    </div>
                  </div>

                  {ruleCriteriaChips(rule).length > 0 && (
                    <div className="rule-criteria">
                      {ruleCriteriaChips(rule).map((chip) => (
                        <Tooltip key={chip.label} title={`${chip.label}: ${chip.value}`}>
                          <span className="criterion-chip">
                            <span className="criterion-chip-label">{chip.label}</span>
                            <span className="criterion-chip-value">{chipPreview(chip.value)}</span>
                          </span>
                        </Tooltip>
                      ))}
                    </div>
                  )}
                </div>
              ))
            )}
          </div>
        ) : (
          <Table
            columns={desktopColumns}
            dataSource={rows}
            rowKey={(r) => r.key}
            pagination={false}
            scroll={{ x: 1150 }}
            size="small"
            className="routing-table"
            onRow={(_record, index) => {
              const classes: string[] = [];
              const i = index ?? -1;
              if (draggedIndex === i) classes.push('row-dragging');
              if (dropTargetIndex === i && draggedIndex !== i && draggedIndex != null) {
                classes.push(i > draggedIndex ? 'drop-after' : 'drop-before');
              }
              return { className: classes.join(' '), 'data-row-key': i } as React.HTMLAttributes<HTMLElement>;
            }}
          />
        )}

        <RuleFormModal
          open={ruleModalOpen}
          rule={editingRule}
          inboundTags={inboundTagOptions}
          outboundTags={outboundTagOptions}
          balancerTags={balancerTagOptions}
          onClose={() => setRuleModalOpen(false)}
          onConfirm={onRuleConfirm}
        />
      </Space>
    </>
  );
}

function CriterionRow({ label, value, title }: { label: string; value?: string; title: string }) {
  const parts = csv(value);
  if (parts.length === 0) return null;
  return (
    <Tooltip title={title}>
      <span className="criterion-row">
        <span className="criterion-label">{label}</span>
        <span className="criterion-value">{parts[0]}</span>
        {parts.length > 1 && <span className="criterion-more">+{parts.length - 1}</span>}
      </span>
    </Tooltip>
  );
}
