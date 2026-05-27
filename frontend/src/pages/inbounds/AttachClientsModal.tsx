import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Modal, Select, Typography, message } from 'antd';

import { HttpUtil } from '@/utils';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';
import { isInboundMultiUser } from './InboundList';

interface AttachClientsModalProps {
  open: boolean;
  source: DBInbound | null;
  dbInbounds: DBInbound[];
  onClose: () => void;
  onAttached?: () => void;
}

interface BulkAttachResult {
  attached?: string[];
  skipped?: string[];
  errors?: string[];
}

function readClientEmails(settings: unknown): string[] {
  const parsed = coerceInboundJsonField(settings) as { clients?: Array<{ email?: string }> };
  const clients = Array.isArray(parsed?.clients) ? parsed.clients : [];
  return clients.map((c) => (c?.email || '').trim()).filter(Boolean);
}

export default function AttachClientsModal({
  open,
  source,
  dbInbounds,
  onClose,
  onAttached,
}: AttachClientsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [targetIds, setTargetIds] = useState<number[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) setTargetIds([]);
  }, [open]);

  const emails = useMemo(() => (source ? readClientEmails(source.settings) : []), [source]);

  const targetOptions = useMemo(() => {
    if (!source) return [];
    return (dbInbounds || [])
      .filter((ib) => ib.id !== source.id && isInboundMultiUser(ib))
      .map((ib) => ({ value: ib.id, label: `${ib.remark} (${ib.protocol}@${ib.port})` }));
  }, [dbInbounds, source]);

  async function submit() {
    if (!source || targetIds.length === 0 || emails.length === 0) return;
    setSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/clients/bulkAttach', { emails, inboundIds: targetIds }, { headers: { 'Content-Type': 'application/json' } });
      if (!msg?.success) {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
        return;
      }
      const result = (msg.obj || {}) as BulkAttachResult;
      const attached = result.attached?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      const errors = result.errors?.length ?? 0;
      if (errors > 0) {
        messageApi.warning(t('pages.inbounds.attachClientsResultMixed', { attached, skipped, errors }));
      } else {
        messageApi.success(t('pages.inbounds.attachClientsResult', { attached, skipped }));
      }
      onAttached?.();
      onClose();
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      onOk={submit}
      okButtonProps={{ disabled: targetIds.length === 0 || emails.length === 0, loading: saving }}
      okText={t('pages.inbounds.attachClients')}
      cancelText={t('cancel')}
      title={t('pages.inbounds.attachClientsTitle', { remark: source?.remark ?? '' })}
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.inbounds.attachClientsDesc', { count: emails.length })}
      </Typography.Paragraph>
      {targetOptions.length === 0 ? (
        <Alert type="info" showIcon message={t('pages.inbounds.attachClientsNoTargets')} />
      ) : (
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          value={targetIds}
          onChange={setTargetIds}
          options={targetOptions}
          placeholder={t('pages.inbounds.attachClientsTargets')}
          optionFilterProp="label"
        />
      )}
    </Modal>
  );
}
