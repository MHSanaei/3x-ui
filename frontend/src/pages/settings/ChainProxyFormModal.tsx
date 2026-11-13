import { useEffect, useId } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Modal, Select, Switch, message } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import {
  ChainProxyFormSchema,
  ChainProxyTypeSchema,
  type ChainProxy,
  type ChainProxyFormValues,
} from '@/schemas/chainProxy';

function initialState(proxy: ChainProxy | null): ChainProxyFormValues {
  return {
    remark: proxy?.remark ?? '',
    type: proxy?.type ?? 'socks',
    server: proxy?.server ?? '',
    port: proxy?.port ?? 1080,
    username: proxy?.username ?? '',
    password: proxy?.password ?? '',
    tls: proxy?.tls ?? false,
    enabled: proxy?.enabled ?? true,
  };
}

interface ChainProxyFormModalProps {
  open: boolean;
  proxy: ChainProxy | null;
  onClose: () => void;
  onConfirm: (values: ChainProxyFormValues) => void;
}

export default function ChainProxyFormModal({
  open,
  proxy,
  onClose,
  onConfirm,
}: ChainProxyFormModalProps) {
  const { t } = useTranslation();
  const uid = useId();
  const [messageApi, messageContextHolder] = message.useMessage();
  const methods = useForm<ChainProxyFormValues>({ defaultValues: initialState(proxy) });
  const isEdit = proxy != null;

  useEffect(() => {
    if (open) methods.reset(initialState(proxy));
  }, [open, proxy, methods]);

  const type = useWatch({ control: methods.control, name: 'type' });
  // xray's socks outbound has no TLS, so the HTTPS switch is http-only.
  const supportsTls = type === 'http';

  function onFinish(values: ChainProxyFormValues) {
    const candidate: ChainProxyFormValues = { ...values, tls: supportsTls && values.tls };
    const parsed = ChainProxyFormSchema.safeParse(candidate);
    if (!parsed.success) {
      messageApi.error(
        t(parsed.error.issues[0]?.message ?? 'pages.settings.chainProxies.errRemarkRequired'),
      );
      return;
    }
    onConfirm(parsed.data);
  }

  const types = ChainProxyTypeSchema.options.map((value) => ({
    value,
    label: t(
      value === 'socks'
        ? 'pages.settings.chainProxies.typeSocks'
        : 'pages.settings.chainProxies.typeHttp',
    ),
  }));

  return (
    <Modal
      open={open}
      title={
        isEdit
          ? `${t('edit')} ${t('pages.settings.chainProxies.title')}`
          : `+ ${t('pages.settings.chainProxies.add')}`
      }
      okText={isEdit ? t('pages.clients.submitEdit') : t('create')}
      cancelText={t('close')}
      mask={{ closable: false }}
      width="640px"
      onOk={methods.handleSubmit(onFinish)}
      onCancel={onClose}
    >
      {messageContextHolder}
      <FormProvider {...methods}>
        <Form layout="vertical">
          <FormField
            label={t('pages.settings.chainProxies.remark')}
            name="remark"
            required
            rules={{ validate: rhfZodValidate(ChainProxyFormSchema.shape.remark) }}
          >
            <Input placeholder={t('pages.settings.chainProxies.remarkPlaceholder')} />
          </FormField>

          <FormField label={t('pages.settings.chainProxies.type')} name="type" required>
            <Select options={types} />
          </FormField>

          <FormField
            label={t('pages.settings.chainProxies.server')}
            name="server"
            required
            rules={{ validate: rhfZodValidate(ChainProxyFormSchema.shape.server) }}
          >
            <Input placeholder="proxy.example.com" />
          </FormField>

          <FormField
            label={t('pages.settings.chainProxies.port')}
            name="port"
            required
            rules={{ validate: rhfZodValidate(ChainProxyFormSchema.shape.port) }}
          >
            <InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} />
          </FormField>

          <FormField label={t('pages.settings.chainProxies.username')} name="username">
            <Input
              id={`${uid}-username`}
              autoComplete="off"
              placeholder={t('pages.settings.chainProxies.usernamePlaceholder')}
            />
          </FormField>

          <FormField label={t('pages.settings.chainProxies.password')} name="password">
            <Input.Password id={`${uid}-password`} autoComplete="new-password" />
          </FormField>

          {supportsTls && (
            <FormField
              label={t('pages.settings.chainProxies.tls')}
              name="tls"
              valueProp="checked"
              tooltip={t('pages.settings.chainProxies.tlsHelp')}
            >
              <Switch />
            </FormField>
          )}

          <FormField
            label={t('pages.settings.chainProxies.enabled')}
            name="enabled"
            valueProp="checked"
          >
            <Switch />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
