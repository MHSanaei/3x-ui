import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, InputNumber, Modal, Select, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { InputAddon } from '@/components/ui';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import {
  DnsQueryStrategySchema,
  DnsServerObjectInnerSchema,
  DnsServerObjectSchema,
  type DnsServerObject,
} from '@/schemas/dns';

export type DnsServerValue =
  | string
  | (DnsServerObject & {
      expectIPs?: string[];
      [key: string]: unknown;
    });

interface DnsServerModalProps {
  open: boolean;
  server: DnsServerValue | null;
  isEdit: boolean;
  onClose: () => void;
  onConfirm: (value: DnsServerValue) => void;
}

const STRATEGIES = DnsQueryStrategySchema.options;

type DnsServerForm = {
  address: string;
  port: number;
  domains: string[];
  expectedIPs: string[];
  unexpectedIPs: string[];
  queryStrategy: string;
  skipFallback: boolean;
  disableCache: boolean;
  finalQuery: boolean;
  tag: string;
  clientIP: string;
  serveStale: boolean;
  serveExpiredTTL: number;
  timeoutMs: number;
};

function defaultFormValues(): DnsServerForm {
  return {
    address: 'localhost',
    port: 53,
    domains: [],
    expectedIPs: [],
    unexpectedIPs: [],
    queryStrategy: 'UseIP',
    skipFallback: false,
    disableCache: false,
    finalQuery: false,
    tag: '',
    clientIP: '',
    serveStale: false,
    serveExpiredTTL: 0,
    timeoutMs: 4000,
  };
}

function valuesFromServer(server: DnsServerValue | null): DnsServerForm {
  if (server == null) return defaultFormValues();
  if (typeof server === 'string') return { ...defaultFormValues(), address: server };
  const parsed = DnsServerObjectSchema.safeParse(server);
  const data = parsed.success ? parsed.data : null;
  return {
    ...defaultFormValues(),
    ...(data ?? {}),
    address: (data?.address ?? server.address) || 'localhost',
    domains: data?.domains ?? server.domains ?? [],
    expectedIPs: data?.expectedIPs ?? server.expectedIPs ?? server.expectIPs ?? [],
    unexpectedIPs: data?.unexpectedIPs ?? server.unexpectedIPs ?? [],
    queryStrategy: data?.queryStrategy ?? server.queryStrategy ?? 'UseIP',
    skipFallback: data?.skipFallback ?? server.skipFallback ?? false,
    disableCache: data?.disableCache ?? server.disableCache ?? false,
    finalQuery: data?.finalQuery ?? server.finalQuery ?? false,
    tag: data?.tag ?? server.tag ?? '',
    clientIP: data?.clientIP ?? server.clientIP ?? '',
    serveStale: data?.serveStale ?? server.serveStale ?? false,
    serveExpiredTTL: data?.serveExpiredTTL ?? server.serveExpiredTTL ?? 0,
    timeoutMs: data?.timeoutMs ?? server.timeoutMs ?? 4000,
  };
}

function valuesToWire(values: DnsServerForm): DnsServerValue {
  const isPlain
    = values.domains.length === 0
    && values.expectedIPs.length === 0
    && values.unexpectedIPs.length === 0
    && values.port === 53
    && values.queryStrategy === 'UseIP'
    && values.skipFallback === false
    && values.disableCache === false
    && values.finalQuery === false
    && !values.tag
    && !values.clientIP
    && values.serveStale === false
    && values.serveExpiredTTL === 0
    && values.timeoutMs === 4000;
  if (isPlain) return values.address;

  const out: Record<string, unknown> = {
    address: values.address,
    port: values.port,
    domains: values.domains.filter(Boolean),
    expectedIPs: values.expectedIPs.filter(Boolean),
    unexpectedIPs: values.unexpectedIPs.filter(Boolean),
    queryStrategy: values.queryStrategy,
    skipFallback: values.skipFallback,
    disableCache: values.disableCache,
    finalQuery: values.finalQuery,
    serveStale: values.serveStale,
    serveExpiredTTL: values.serveExpiredTTL,
    timeoutMs: values.timeoutMs,
  };
  if (values.tag) out.tag = values.tag;
  if (values.clientIP) out.clientIP = values.clientIP;
  return out as DnsServerValue;
}

const shape = DnsServerObjectInnerSchema.shape;

export default function DnsServerModal({
  open,
  server,
  isEdit,
  onClose,
  onConfirm,
}: DnsServerModalProps) {
  const { t } = useTranslation();
  const methods = useForm<DnsServerForm>({ defaultValues: defaultFormValues() });
  const domains = useWatch({ control: methods.control, name: 'domains' }) ?? [];
  const expectedIPs = useWatch({ control: methods.control, name: 'expectedIPs' }) ?? [];
  const unexpectedIPs = useWatch({ control: methods.control, name: 'unexpectedIPs' }) ?? [];

  useEffect(() => {
    if (!open) return;
    methods.reset(valuesFromServer(server));
  }, [open, server, methods]);

  const title = isEdit ? t('pages.xray.dns.edit') : t('pages.xray.dns.add');

  return (
    <Modal
      open={open}
      title={title}
      okText={t('confirm')}
      cancelText={t('close')}
      mask={{ closable: false }}
      onOk={methods.handleSubmit((values) => onConfirm(valuesToWire(values)))}
      onCancel={onClose}
    >
      <FormProvider {...methods}>
        <Form
          colon={false}
          labelCol={{ md: { span: 8 } }}
          wrapperCol={{ md: { span: 14 } }}
        >
          <FormField
            label={t('pages.inbounds.address')}
            name="address"
            rules={{ validate: rhfZodValidate(shape.address) }}
          >
            <Input />
          </FormField>
          <FormField
            label={t('pages.inbounds.port')}
            name="port"
            rules={{ validate: rhfZodValidate(shape.port) }}
          >
            <InputNumber min={1} max={65535} />
          </FormField>
          <FormField label={t('pages.xray.dns.tag')} name="tag">
            <Input />
          </FormField>
          <FormField label={t('pages.xray.dns.clientIp')} name="clientIP">
            <Input />
          </FormField>
          <FormField label={t('pages.xray.dns.strategy')} name="queryStrategy">
            <Select
              style={{ width: '100%' }}
              options={STRATEGIES.map((s) => ({ value: s, label: s }))}
            />
          </FormField>
          <FormField
            label={t('pages.xray.dns.timeoutMs')}
            name="timeoutMs"
            rules={{ validate: rhfZodValidate(shape.timeoutMs) }}
          >
            <InputNumber min={0} step={500} />
          </FormField>

          <Divider style={{ margin: '5px 0' }} />

          <Form.Item label={t('pages.xray.dns.domains')}>
            <Button size="small" type="primary" icon={<PlusOutlined />} aria-label={t('add')} onClick={() => methods.setValue('domains', [...domains, ''])} />
            {domains.map((_, i) => (
              <Space.Compact key={i} block style={{ marginTop: 4 }}>
                <FormField name={`domains.${i}`} noStyle>
                  <Input />
                </FormField>
                <InputAddon ariaLabel={t('remove')} onClick={() => methods.setValue('domains', domains.filter((__, idx) => idx !== i))}>
                  <MinusOutlined />
                </InputAddon>
              </Space.Compact>
            ))}
          </Form.Item>

          <Form.Item label={t('pages.xray.dns.expectIPs')}>
            <Button size="small" type="primary" icon={<PlusOutlined />} aria-label={t('add')} onClick={() => methods.setValue('expectedIPs', [...expectedIPs, ''])} />
            {expectedIPs.map((_, i) => (
              <Space.Compact key={i} block style={{ marginTop: 4 }}>
                <FormField name={`expectedIPs.${i}`} noStyle>
                  <Input />
                </FormField>
                <InputAddon ariaLabel={t('remove')} onClick={() => methods.setValue('expectedIPs', expectedIPs.filter((__, idx) => idx !== i))}>
                  <MinusOutlined />
                </InputAddon>
              </Space.Compact>
            ))}
          </Form.Item>

          <Form.Item label={t('pages.xray.dns.unexpectIPs')}>
            <Button size="small" type="primary" icon={<PlusOutlined />} aria-label={t('add')} onClick={() => methods.setValue('unexpectedIPs', [...unexpectedIPs, ''])} />
            {unexpectedIPs.map((_, i) => (
              <Space.Compact key={i} block style={{ marginTop: 4 }}>
                <FormField name={`unexpectedIPs.${i}`} noStyle>
                  <Input />
                </FormField>
                <InputAddon ariaLabel={t('remove')} onClick={() => methods.setValue('unexpectedIPs', unexpectedIPs.filter((__, idx) => idx !== i))}>
                  <MinusOutlined />
                </InputAddon>
              </Space.Compact>
            ))}
          </Form.Item>

          <Divider style={{ margin: '5px 0' }} />

          <FormField label={t('pages.xray.dns.skipFallback')} name="skipFallback" valueProp="checked">
            <Switch />
          </FormField>
          <FormField label={t('pages.xray.dns.finalQuery')} name="finalQuery" valueProp="checked">
            <Switch />
          </FormField>
          <FormField label={t('pages.xray.dns.disableCache')} name="disableCache" valueProp="checked">
            <Switch />
          </FormField>
          <FormField label={t('pages.xray.dns.serveStale')} name="serveStale" valueProp="checked">
            <Switch />
          </FormField>
          <FormField label={t('pages.xray.dns.serveExpiredTTL')} name="serveExpiredTTL">
            <InputNumber min={0} step={60} />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
