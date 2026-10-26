import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Form, Input, InputNumber, Modal, Select, Tabs, message } from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import {
  ClientBulkAdjustFormSchema,
  type ClientBulkAdjustFormValues,
  type BulkAdjustResult,
} from '@/schemas/client';
import {
  ExternalLinksAppendSchema,
  type ExternalLinksFormValues,
  type ExternalLinkInput,
} from '@/schemas/external-link';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives/flow';
import { FormField } from '@/components/form/rhf';
import ClientExternalLinksFields from './ClientExternalLinksFields';
import './ClientBulkAdjustModal.css';

const GB = 1024 * 1024 * 1024;

const FLOW_CLEAR = 'none';

const EMPTY: ClientBulkAdjustFormValues & ExternalLinksFormValues = {
  addDays: 0,
  addGB: 0,
  flow: '',
  limitHwid: null,
  adTag: '',
  externalLinks: [],
};

interface ClientBulkAdjustModalProps {
  open: boolean;
  count: number;
  onOpenChange: (open: boolean) => void;
  onSubmit: (
    addDays: number,
    addBytes: number,
    flow: string,
    limitHwid?: number | null,
    adTag?: string,
    externalLinks?: ExternalLinkInput[],
  ) => Promise<BulkAdjustResult | null>;
}

export default function ClientBulkAdjustModal({
  open,
  count,
  onOpenChange,
  onSubmit,
}: ClientBulkAdjustModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [submitting, setSubmitting] = useState(false);
  const [mode, setMode] = useState('adjust');
  const methods = useForm<ClientBulkAdjustFormValues & ExternalLinksFormValues>({
    defaultValues: EMPTY,
  });

  useEffect(() => {
    if (open) {
      methods.reset(EMPTY);
    }
  }, [open, methods]);

  async function handleOk() {
    const values = methods.getValues();
    const validated =
      mode === 'links'
        ? ExternalLinksAppendSchema.safeParse(values)
        : ClientBulkAdjustFormSchema.safeParse({
            addDays: Math.trunc(Number(values.addDays) || 0),
            addGB: Number(values.addGB) || 0,
            flow: values.flow,
            limitHwid:
              values.limitHwid !== null &&
              values.limitHwid !== undefined &&
              (values.limitHwid as unknown) !== ''
                ? Math.trunc(Number(values.limitHwid))
                : null,
            adTag: values.adTag?.trim() ?? '',
          });
    if (!validated.success) {
      messageApi.warning(t(validated.error.issues[0]?.message ?? 'somethingWentWrong'));
      return;
    }
    const data = validated.data;
    const days = 'addDays' in data ? data.addDays : 0;
    const gb = 'addGB' in data ? data.addGB : 0;
    const flowValue = 'flow' in data ? data.flow : '';
    const limitHwid = 'limitHwid' in data ? data.limitHwid : null;
    const adTag = 'adTag' in data ? data.adTag : '';
    const externalLinks =
      'externalLinks' in data ? data.externalLinks.filter((row) => row.value !== '') : undefined;
    setSubmitting(true);
    try {
      const bytes = Math.trunc(gb * GB);
      const result = await onSubmit(days, bytes, flowValue, limitHwid, adTag, externalLinks);
      if (!result) return;
      const ok = result.adjusted ?? 0;
      const skipped = result.skipped?.length ?? 0;
      if (result.externalLinks) {
        const summary = t('pages.clients.toasts.bulkLinksAppended', {
          clients: ok,
          ...result.externalLinks,
        });
        if (result.externalLinks.errors > 0) messageApi.warning(summary);
        else messageApi.success(summary);
      } else if (skipped === 0) {
        messageApi.success(t('pages.clients.toasts.bulkAdjusted', { count: ok }));
      } else {
        const firstReason = result.skipped?.[0]?.reason ?? '';
        messageApi.warning(
          firstReason
            ? `${t('pages.clients.toasts.bulkAdjustedMixed', { ok, skipped })} — ${firstReason}`
            : t('pages.clients.toasts.bulkAdjustedMixed', { ok, skipped }),
        );
      }
      onOpenChange(false);
    } catch {
      messageApi.error(t('somethingWentWrong'));
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
        okText={t('confirm')}
        cancelText={t('cancel')}
        confirmLoading={submitting}
        onOk={handleOk}
        onCancel={() => onOpenChange(false)}
        destroyOnHidden
        afterClose={() => setMode('adjust')}
        width={mode === 'links' ? 800 : 520}
      >
        <FormProvider {...methods}>
          <Form layout="vertical">
            <Tabs
              className="client-bulk-adjust-tabs"
              activeKey={mode}
              onChange={setMode}
              items={[
                {
                  key: 'adjust',
                  label: t('pages.clients.bulkAdjustSettings'),
                  children: (
                    <>
                      <Alert
                        type="info"
                        showIcon
                        style={{ marginBottom: 16 }}
                        title={t('pages.clients.bulkAdjustHint')}
                      />
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
                      <FormField
                        name="limitHwid"
                        label={t('pages.clients.limitHwid')}
                        tooltip={t('pages.clients.limitHwidDesc')}
                      >
                        <InputNumber
                          style={{ width: '100%' }}
                          min={0}
                          step={1}
                          precision={0}
                          placeholder={t('pages.clients.bulkFlowNoChange')}
                        />
                      </FormField>
                      <FormField
                        name="adTag"
                        label={t('pages.clients.mtprotoAdTag')}
                        extra={t('pages.clients.bulkAdTagHint')}
                      >
                        <Input placeholder={t('pages.clients.bulkFlowNoChange')} allowClear />
                      </FormField>
                    </>
                  ),
                },
                {
                  key: 'links',
                  label: t('pages.clients.bulkAppendLinks'),
                  children: (
                    <>
                      <Alert
                        type="info"
                        showIcon
                        style={{ marginBottom: 16 }}
                        title={t('pages.clients.bulkAppendLinksHint')}
                      />
                      <ClientExternalLinksFields showFetchStatus={false} />
                    </>
                  ),
                },
              ]}
            />
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
