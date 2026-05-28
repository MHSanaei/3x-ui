import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AutoComplete, Form, Modal, message } from 'antd';

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
  const [value, setValue] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) setValue('');
  }, [open]);

  async function submit() {
    const next = value.trim();
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
        okButtonProps={{ disabled: !value.trim() }}
        onCancel={() => onOpenChange(false)}
        onOk={submit}
        destroyOnHidden
      >
        <Form layout="vertical">
          <Form.Item
            label={t('pages.clients.group')}
            tooltip={t('pages.clients.addToGroupTooltip')}
          >
            <AutoComplete
              value={value}
              placeholder={t('pages.clients.addToGroupPlaceholder')}
              options={groups.map((g) => ({ value: g }))}
              onChange={(v) => setValue(v ?? '')}
              filterOption={(input, option) =>
                String(option?.value ?? '').toLowerCase().includes((input || '').toLowerCase())
              }
              allowClear
              style={{ width: '100%' }}
              autoFocus
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
