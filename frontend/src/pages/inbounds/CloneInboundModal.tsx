import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Select, Typography, message } from 'antd';

import { HttpUtil } from '@/utils';
import { buildClonePayload, pickClonePort } from '@/lib/xray/inbound-clone';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { DBInbound } from '@/models/dbinbound';

// 0 is the "local panel" sentinel (inbounds without a nodeId) — the same
// convention as the clients page node filter (#4997).
const LOCAL_PANEL = 0;

interface CloneInboundModalProps {
  open: boolean;
  dbInbound: DBInbound | null;
  nodes: NodeRecord[];
  portsInUse: Map<number, Set<number>>;
  onClose: () => void;
  onCloned: () => void | Promise<void>;
}

export default function CloneInboundModal({
  open,
  dbInbound,
  nodes,
  portsInUse,
  onClose,
  onCloned,
}: CloneInboundModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [targets, setTargets] = useState<number[]>([LOCAL_PANEL]);
  const [submitting, setSubmitting] = useState(false);

  const targetOptions = useMemo(() => [
    { value: LOCAL_PANEL, label: t('pages.inbounds.localPanel'), disabled: false },
    ...(nodes || []).filter((n) => n.enable).map((n) => ({
      value: n.id,
      label: `${n.name}${n.status === 'offline' ? ' (offline)' : ''}`,
      disabled: n.status !== 'online',
    })),
  ], [nodes, t]);

  // Reset the selection on every open: pre-select the source inbound's own
  // node when it is a selectable target, otherwise the local panel (the only
  // destination the clone action had before this picker existed).
  useEffect(() => {
    if (!open || !dbInbound) return;
    const src = dbInbound.nodeId ?? LOCAL_PANEL;
    const srcOption = targetOptions.find((o) => o.value === src);
    setTargets([srcOption && !srcOption.disabled ? src : LOCAL_PANEL]);
  }, [open, dbInbound, targetOptions]);

  async function submit() {
    if (!dbInbound || targets.length === 0) return;
    setSubmitting(true);
    try {
      // Sequential posts keep per-target results in selection order; every
      // target gets its own fresh port because ports are only node-scoped.
      const results: { ok: boolean; reason: string }[] = [];
      for (const target of targets) {
        const msg = await HttpUtil.post(
          '/panel/api/inbounds/add',
          buildClonePayload(dbInbound, pickClonePort(portsInUse.get(target)), target === LOCAL_PANEL ? null : target),
          { silent: true },
        );
        results.push({ ok: !!msg?.success, reason: msg?.success ? '' : (msg?.msg || '') });
      }
      const okCount = results.filter((r) => r.ok).length;
      const failed = results.length - okCount;
      if (failed === 0) {
        messageApi.success(okCount === 1
          ? t('pages.inbounds.toasts.inboundCreateSuccess')
          : t('pages.inbounds.toasts.clonedMany', { count: okCount }));
      } else {
        const firstError = results.find((r) => !r.ok)?.reason ?? '';
        const base = t('pages.inbounds.toasts.clonedMixed', { ok: okCount, failed });
        messageApi.warning(firstError ? `${base} — ${firstError}` : base);
      }
      if (okCount > 0) await onCloned();
      onClose();
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.inbounds.cloneConfirmTitle', { remark: dbInbound?.remark ?? '' })}
        okText={t('pages.inbounds.clone')}
        cancelText={t('cancel')}
        okButtonProps={{ disabled: targets.length === 0, loading: submitting }}
        onCancel={onClose}
        onOk={submit}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {t('pages.inbounds.cloneConfirmContent')}
        </Typography.Paragraph>
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          value={targets}
          onChange={setTargets}
          options={targetOptions}
          placeholder={t('pages.inbounds.deployTo')}
          showSearch={{ optionFilterProp: 'label' }}
          autoFocus
        />
      </Modal>
    </>
  );
}
