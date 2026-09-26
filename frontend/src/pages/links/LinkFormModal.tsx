import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Controller, FormProvider, useWatch } from 'react-hook-form';
import { Form, Input, InputNumber, Modal, Select, Switch, message } from 'antd';
import dayjs from 'dayjs';

import { DateTimePicker } from '@/components/form';
import { FormField, useZodForm, rhfZodValidate } from '@/components/form/rhf';
import { LinkSaveSchema, type LinkRecord, type LinkSaveValues } from '@/schemas/api/link';

interface LinkFormModalProps {
  open: boolean;
  link: LinkRecord | null;
  save: (payload: LinkSaveValues) => Promise<{ success?: boolean; msg?: string } | undefined>;
  onOpenChange: (open: boolean) => void;
}

function defaultsFor(link: LinkRecord | null): LinkSaveValues {
  return {
    id: link?.id,
    kind: link?.kind ?? 'link',
    value: link?.value ?? '',
    remark: link?.remark ?? '',
    namePrefix: link?.namePrefix ?? '',
    enable: link?.enable ?? true,
    expiryTime: link?.expiryTime ?? 0,
    userAgent: link?.userAgent ?? '',
    cacheTtl: link?.cacheTtl ?? 0,
  };
}

export default function LinkFormModal({ open, link, save, onOpenChange }: LinkFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const methods = useZodForm(LinkSaveSchema, { defaultValues: defaultsFor(link) });
  const kind = useWatch({ control: methods.control, name: 'kind' }) ?? 'link';

  useEffect(() => {
    if (open) methods.reset(defaultsFor(link));
  }, [open, link, methods]);

  const onFinish = async (values: LinkSaveValues) => {
    if (loading) return;
    setLoading(true);
    try {
      const res = await save(values);
      if (res?.success) {
        messageApi.success(t('pages.links.saved'));
        onOpenChange(false);
      } else if (res?.msg) {
        messageApi.error(res.msg);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      open={open}
      title={t(link ? 'pages.links.edit' : 'pages.links.add')}
      onOk={methods.handleSubmit(onFinish)}
      onCancel={() => onOpenChange(false)}
      confirmLoading={loading}
      okText={t('save')}
      cancelText={t('cancel')}
      destroyOnHidden
      width={640}
    >
      {messageContextHolder}
      <FormProvider {...methods}>
        <Form
          colon={false}
          labelCol={{ sm: { span: 7 } }}
          wrapperCol={{ sm: { span: 15 } }}
          labelWrap
        >
          <FormField name="kind" label={t('pages.links.kind')}>
            <Select
              options={[
                { value: 'link', label: t('pages.links.kindLink') },
                { value: 'subscription', label: t('pages.links.kindSubscription') },
              ]}
            />
          </FormField>
          <FormField
            name="value"
            label={t('pages.links.value')}
            rules={{ validate: rhfZodValidate(LinkSaveSchema.shape.value) }}
          >
            <Input.TextArea
              rows={2}
              autoSize={{ minRows: 2, maxRows: 4 }}
              placeholder={
                kind === 'subscription'
                  ? 'https://provider.example/sub?token=…'
                  : 'vless:// · vmess:// · trojan:// · ss:// · hysteria2://'
              }
            />
          </FormField>
          <FormField name="remark" label={t('remark')}>
            <Input maxLength={256} />
          </FormField>
          <FormField name="namePrefix" label={t('pages.links.namePrefix')}>
            <Input maxLength={64} />
          </FormField>
          <FormField name="enable" label={t('enable')} valueProp="checked">
            <Switch size="small" />
          </FormField>
          <Controller
            control={methods.control}
            name="expiryTime"
            render={({ field: expiryField }) => (
              <Form.Item label={t('subscription.expiry')}>
                <DateTimePicker
                  value={Number(expiryField.value) > 0 ? dayjs(Number(expiryField.value)) : null}
                  onChange={(v) => expiryField.onChange(v ? v.valueOf() : 0)}
                  allowClear
                />
              </Form.Item>
            )}
          />
          {kind === 'subscription' && (
            <>
              <FormField
                name="userAgent"
                label={t('pages.links.userAgent')}
                extra={t('pages.links.fetchFieldHint')}
              >
                <Input maxLength={256} />
              </FormField>
              <FormField
                name="cacheTtl"
                label={t('pages.links.cacheTtl')}
                extra={t('pages.links.fetchFieldHint')}
              >
                <InputNumber min={0} max={86400} />
              </FormField>
            </>
          )}
        </Form>
      </FormProvider>
    </Modal>
  );
}
