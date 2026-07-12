import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, Modal, Select, Switch, message } from 'antd';

import type { ManagedLinkFormValues, ManagedLinkRecord } from '@/schemas/api/link';
import { useMediaQuery } from '@/hooks/useMediaQuery';

interface LinkFormModalProps {
  open: boolean;
  mode: 'add' | 'edit';
  link: ManagedLinkRecord | null;
  save: (payload: ManagedLinkFormValues) => Promise<{ success?: boolean; msg?: string } | undefined>;
  onOpenChange: (open: boolean) => void;
}

function defaultsFor(link: ManagedLinkRecord | null): ManagedLinkFormValues & { enable: boolean } {
  return {
    kind: link?.kind ?? 'link',
    value: link?.value ?? '',
    remark: link?.remark ?? '',
    isDisabled: link?.isDisabled ?? false,
    enable: link ? !link.isDisabled : true,
  };
}

export default function LinkFormModal({ open, mode, link, save, onOpenChange }: LinkFormModalProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [form] = Form.useForm<ManagedLinkFormValues & { enable: boolean }>();
  const kind = Form.useWatch('kind', form) ?? 'link';

  useEffect(() => {
    if (open) form.setFieldsValue(defaultsFor(link));
  }, [open, link, form]);

  const onOk = async () => {
    let values: ManagedLinkFormValues & { enable: boolean };
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const { enable, ...rest } = values;
    const res = await save({ ...rest, isDisabled: !enable });
    if (res?.success) {
      message.success(t(mode === 'add' ? 'pages.links.toasts.add' : 'pages.links.toasts.update'));
      onOpenChange(false);
    } else if (res?.msg) {
      message.error(res.msg);
    }
  };

  return (
    <Modal
      open={open}
      title={t(mode === 'add' ? 'pages.links.addLink' : 'pages.links.editLink')}
      onOk={onOk}
      onCancel={() => onOpenChange(false)}
      okText={t('save')}
      cancelText={t('cancel')}
      destroyOnHidden
      width={isMobile ? '95vw' : 640}
    >
      <Form
        form={form}
        colon={false}
        labelCol={{ sm: { span: 7 } }}
        wrapperCol={{ sm: { span: 15 } }}
        labelWrap
        preserve={false}
        initialValues={defaultsFor(link)}
      >
        <Form.Item name="kind" label={t('pages.links.fields.kind')} rules={[{ required: true }]}>
          <Select
            options={[
              { value: 'link', label: t('pages.links.kind.link') },
              { value: 'subscription', label: t('pages.links.kind.subscription') },
            ]}
          />
        </Form.Item>
        <Form.Item name="remark" label={t('pages.links.fields.remark')}>
          <Input maxLength={256} />
        </Form.Item>
        <Form.Item name="value" label={t('pages.links.fields.value')} rules={[{ required: true }]}>
          <Input.TextArea
            autoSize={{ minRows: 3, maxRows: 7 }}
            placeholder={kind === 'subscription' ? 'https://provider.example/sub/...' : 'vless://...'}
          />
        </Form.Item>
        <Form.Item name="enable" label={t('pages.links.fields.enable')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </Modal>
  );
}
