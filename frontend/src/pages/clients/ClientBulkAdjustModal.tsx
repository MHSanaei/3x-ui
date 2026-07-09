import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Form, InputNumber, Modal, Select, message } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { ClientBulkAdjustFormSchema, type ClientBulkAdjustFormValues } from '@/schemas/client';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives/flow';
import { FormField } from '@/components/form/rhf';

const GB = 1024 * 1024 * 1024;

const FLOW_CLEAR = 'none';

const EMPTY: ClientBulkAdjustFormValues = { addDays: 0, addGB: 0, flow: '' };

interface ClientBulkAdjustModalProps {
  open: boolean;
  count: number;
  onOpenChange: (open: boolean) => void;
  onSubmit: (addDays: number, addBytes: number, flow: string) => Promise<{ adjusted: number; skipped?: { email: string; reason: string }[] } | null>;
}

export default function ClientBulkAdjustModal({ open, count, onOpenChange, onSubmit }: ClientBulkAdjustModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [submitting, setSubmitting] = useState(false);
  const methods = useForm<ClientBulkAdjustFormValues>({ defaultValues: EMPTY });

  useEffect(() => {
    if (open) methods.reset(EMPTY);
  }, [open, methods]);

  async function handleOk() {
    const values = methods.getValues();
    const validated = ClientBulkAdjustFormSchema.safeParse({
      addDays: Math.trunc(Number(values.addDays) || 0),
      addGB: Number(values.addGB) || 0,
      flow: values.flow,
    });
    if (!validated.success) {
      messageApi.warning(t(validated.error.issues[0]?.message ?? 'somethingWentWrong'));
      return;
    }
    const { addDays: days, addGB: gb, flow: flowValue } = validated.data;
    setSubmitting(true);
    try {
      const bytes = Math.trunc(gb * GB);
      const result = await onSubmit(days, bytes, flowValue);
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
          title={t('pages.clients.bulkAdjustHint')}
        />
        <FormProvider {...methods}>
          <Form layout="vertical">
            <FormField name="addDays" label={t('pages.clients.addDays')}>
              <InputNumber style={{ width: '100%' }} step={1} precision={0} />
            </FormField>
            <FormField name="addGB" label={t('pages.clients.addTrafficGB')}>
              <InputNumber style={{ width: '100%' }} step={1} />
            </FormField>
            <FormField name="flow" label={t('pages.clients.bulkFlow')}>
              <Select
                style={{ width: '100%' }}
                options={[
                  { value: '', label: t('pages.clients.bulkFlowNoChange') },
                  { value: FLOW_CLEAR, label: t('pages.clients.bulkFlowDisable') },
                  ...Object.values(TLS_FLOW_CONTROL).map((k) => ({ value: k, label: k })),
                ]}
              />
            </FormField>
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
