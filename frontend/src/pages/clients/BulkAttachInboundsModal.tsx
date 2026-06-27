import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Modal, Select, Typography, message } from 'antd';

import { SelectAllClearButtons } from '@/components/form';
import type { InboundOption } from '@/hooks/useClients';
import { formatInboundLabel } from '@/lib/inbounds/label';
import type { BulkAttachResult } from '@/schemas/client';

const MULTI_USER_PROTOCOLS = new Set(['vmess', 'vless', 'trojan', 'hysteria', 'shadowsocks']);

interface BulkAttachInboundsModalProps {
  open: boolean;
  count: number;
  inbounds: InboundOption[];
  onOpenChange: (open: boolean) => void;
  onSubmit: (inboundIds: number[]) => Promise<BulkAttachResult | null>;
}

export default function BulkAttachInboundsModal({
  open,
  count,
  inbounds,
  onOpenChange,
  onSubmit,
}: BulkAttachInboundsModalProps) {
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
      const attached = result.attached?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      const errors = result.errors?.length ?? 0;
      if (errors > 0) {
        messageApi.warning(
          t('pages.inbounds.attachClientsResultMixed', { attached, skipped, errors }),
        );
      } else {
        messageApi.success(t('pages.inbounds.attachClientsResult', { attached, skipped }));
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
        title={t('pages.clients.attachToInboundsTitle', { count })}
        okText={t('pages.inbounds.attachClients')}
        cancelText={t('cancel')}
        okButtonProps={{ disabled: targetIds.length === 0, loading: submitting }}
        onCancel={() => onOpenChange(false)}
        onOk={submit}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {t('pages.clients.attachToInboundsDesc', { count })}
        </Typography.Paragraph>
        {targetOptions.length === 0 ? (
          <Alert type="info" showIcon title={t('pages.clients.attachToInboundsNoTargets')} />
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
              placeholder={t('pages.clients.attachToInboundsTargets')}
              optionFilterProp="label"
              autoFocus
            />
          </>
        )}
      </Modal>
    </>
  );
}
