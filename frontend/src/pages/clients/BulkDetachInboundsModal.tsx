import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Modal, Select, Typography, message } from 'antd';

import { SelectAllClearButtons } from '@/components/form';
import type { InboundOption } from '@/hooks/useClients';
import { formatInboundLabel } from '@/lib/inbounds/label';
import type { BulkDetachResult } from '@/schemas/client';

const MULTI_USER_PROTOCOLS = new Set(['vmess', 'vless', 'trojan', 'hysteria', 'shadowsocks']);

interface BulkDetachInboundsModalProps {
  open: boolean;
  count: number;
  inbounds: InboundOption[];
  onOpenChange: (open: boolean) => void;
  onSubmit: (inboundIds: number[]) => Promise<BulkDetachResult | null>;
}

export default function BulkDetachInboundsModal({
  open,
  count,
  inbounds,
  onOpenChange,
  onSubmit,
}: BulkDetachInboundsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [targetIds, setTargetIds] = useState<number[]>([]);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) setTargetIds([]);
  }, [open]);

  const targetOptions = useMemo(() => {
    return (inbounds || [])
      .filter((ib) => MULTI_USER_PROTOCOLS.has((ib.protocol || '').toLowerCase()))
      .map((ib) => ({
        value: ib.id,
        label: formatInboundLabel(ib.tag, ib.remark),
      }));
  }, [inbounds]);

  async function submit() {
    if (targetIds.length === 0 || count === 0) return;
    setSubmitting(true);
    try {
      const result = await onSubmit(targetIds);
      if (!result) return;
      const detached = result.detached?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      const errors = result.errors?.length ?? 0;
      if (errors > 0) {
        messageApi.warning(
          t('pages.clients.detachFromInboundsResultMixed', { detached, skipped, errors }),
        );
      } else {
        messageApi.success(t('pages.clients.detachFromInboundsResult', { detached, skipped }));
      }
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.detachFromInboundsTitle', { count })}
        okText={t('pages.clients.detach')}
        cancelText={t('cancel')}
        okButtonProps={{ danger: true, disabled: targetIds.length === 0, loading: submitting }}
        onCancel={() => onOpenChange(false)}
        onOk={submit}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {t('pages.clients.detachFromInboundsDesc', { count })}
        </Typography.Paragraph>
        {targetOptions.length === 0 ? (
          <Alert type="info" showIcon title={t('pages.clients.detachFromInboundsNoTargets')} />
        ) : (
          <>
            <SelectAllClearButtons
              options={targetOptions}
              value={targetIds}
              onChange={setTargetIds}
            />
            <Select
              mode="multiple"
              style={{ width: '100%' }}
              value={targetIds}
              onChange={setTargetIds}
              options={targetOptions}
              placeholder={t('pages.clients.detachFromInboundsTargets')}
              optionFilterProp="label"
              autoFocus
            />
          </>
        )}
      </Modal>
    </>
  );
}
