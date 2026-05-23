import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Form, InputNumber, Modal, message } from 'antd';

const GB = 1024 * 1024 * 1024;

interface ClientBulkAdjustModalProps {
  open: boolean;
  count: number;
  onOpenChange: (open: boolean) => void;
  onSubmit: (addDays: number, addBytes: number) => Promise<{ adjusted: number; skipped?: { email: string; reason: string }[] } | null>;
}

export default function ClientBulkAdjustModal({ open, count, onOpenChange, onSubmit }: ClientBulkAdjustModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [addDays, setAddDays] = useState<number>(0);
  const [addGB, setAddGB] = useState<number>(0);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setAddDays(0);
      setAddGB(0);
    }
  }, [open]);

  async function handleOk() {
    const days = Math.trunc(Number(addDays) || 0);
    const gb = Number(addGB) || 0;
    if (days === 0 && gb === 0) {
      messageApi.warning(t('pages.clients.bulkAdjustNothing'));
      return;
    }
    setSubmitting(true);
    try {
      const bytes = Math.trunc(gb * GB);
      const result = await onSubmit(days, bytes);
      if (!result) return;
      const ok = result.adjusted ?? 0;
      const skipped = result.skipped?.length ?? 0;
      if (skipped === 0) {
        messageApi.success(t('pages.clients.toasts.bulkAdjusted', { count: ok }));
      } else {
        const firstReason = result.skipped?.[0]?.reason ?? '';
        messageApi.warning(firstReason
          ? `${t('pages.clients.toasts.bulkAdjustedMixed', { ok, skipped })} — ${firstReason}`
          : t('pages.clients.toasts.bulkAdjustedMixed', { ok, skipped }));
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
        title={t('pages.clients.bulkAdjustTitle', { count })}
        okText={t('apply')}
        cancelText={t('cancel')}
        confirmLoading={submitting}
        onOk={handleOk}
        onCancel={() => onOpenChange(false)}
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('pages.clients.bulkAdjustHint')}
        />
        <Form layout="vertical">
          <Form.Item label={t('pages.clients.addDays')}>
            <InputNumber
              value={addDays}
              onChange={(v) => setAddDays(Number(v) || 0)}
              style={{ width: '100%' }}
              step={1}
              precision={0}
            />
          </Form.Item>
          <Form.Item label={t('pages.clients.addTrafficGB')}>
            <InputNumber
              value={addGB}
              onChange={(v) => setAddGB(Number(v) || 0)}
              style={{ width: '100%' }}
              step={1}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
