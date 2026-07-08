import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AutoComplete, Form, Modal, message } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';

type GroupFormValues = { group: string };

const EMPTY: GroupFormValues = { group: '' };

interface BulkAddToGroupModalProps {
  open: boolean;
  count: number;
  groups: string[];
  onOpenChange: (open: boolean) => void;
  onSubmit: (group: string) => Promise<{ affected?: number } | null>;
}

export default function BulkAddToGroupModal({
  open,
  count,
  groups,
  onOpenChange,
  onSubmit,
}: BulkAddToGroupModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [submitting, setSubmitting] = useState(false);
  const methods = useForm<GroupFormValues>({ defaultValues: EMPTY });
  const group = useWatch({ control: methods.control, name: 'group' });

  useEffect(() => {
    if (open) methods.reset(EMPTY);
  }, [open, methods]);

  async function submit() {
    const next = (methods.getValues().group ?? '').trim();
    if (!next) return;
    setSubmitting(true);
    try {
      const result = await onSubmit(next);
      if (result) {
        const affected = result.affected ?? 0;
        messageApi.success(t('pages.clients.addToGroupSuccessToast', { count: affected, group: next }));
        onOpenChange(false);
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.addToGroupTitle', { count })}
        okText={t('add')}
        cancelText={t('cancel')}
        confirmLoading={submitting}
        okButtonProps={{ disabled: !(group ?? '').trim() }}
        onCancel={() => onOpenChange(false)}
        onOk={submit}
        destroyOnHidden
      >
        <FormProvider {...methods}>
          <Form layout="vertical">
            <FormField
              name="group"
              label={t('pages.clients.group')}
              tooltip={t('pages.clients.addToGroupTooltip')}
              transform={{ output: (v) => v ?? '' }}
            >
              <AutoComplete
                placeholder={t('pages.clients.groupName')}
                options={groups.map((g) => ({ value: g }))}
                allowClear
                autoFocus
              />
            </FormField>
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
