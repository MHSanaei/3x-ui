import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, InputNumber, Modal, Select, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';

import { InputAddon } from '@/components/ui';
import {
  DnsQueryStrategySchema,
  DnsServerObjectInnerSchema,
  DnsServerObjectSchema,
  type DnsServerObject,
} from '@/schemas/dns';
import { antdRule } from '@/utils/zodForm';

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
  const [form] = Form.useForm<DnsServerForm>();

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue(valuesFromServer(server));
  }, [open, server, form]);

  async function submit() {
    const values = await form.validateFields();
    onConfirm(valuesToWire(values));
  }

  const title = isEdit ? t('pages.xray.dns.edit') : t('pages.xray.dns.add');

  return (
    <Modal
      open={open}
      title={title}
      okText={t('confirm')}
      cancelText={t('close')}
      mask={{ closable: false }}
      onOk={submit}
      onCancel={onClose}
    >
      <Form
        form={form}
        colon={false}
        labelCol={{ md: { span: 8 } }}
        wrapperCol={{ md: { span: 14 } }}
        initialValues={defaultFormValues()}
      >
        <Form.Item
          label={t('pages.inbounds.address')}
          name="address"
          rules={[antdRule(shape.address, t)]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          label={t('pages.inbounds.port')}
          name="port"
          rules={[antdRule(shape.port, t)]}
        >
          <InputNumber min={1} max={65535} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.tag')} name="tag">
          <Input />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.clientIp')} name="clientIP">
          <Input />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.strategy')} name="queryStrategy">
          <Select
            style={{ width: '100%' }}
            options={STRATEGIES.map((s) => ({ value: s, label: s }))}
          />
        </Form.Item>
        <Form.Item
          label={t('pages.xray.dns.timeoutMs')}
          name="timeoutMs"
          rules={[antdRule(shape.timeoutMs, t)]}
        >
          <InputNumber min={0} step={500} />
        </Form.Item>

        <Divider style={{ margin: '5px 0' }} />

        <Form.List name="domains">
          {(fields, { add, remove }) => (
            <Form.Item label={t('pages.xray.dns.domains')}>
              <Button size="small" type="primary" icon={<PlusOutlined />} aria-label={t('add')} onClick={() => add('')} />
              {fields.map((field) => (
                <Space.Compact key={field.key} block style={{ marginTop: 4 }}>
                  <Form.Item name={field.name} noStyle>
                    <Input />
                  </Form.Item>
                  <InputAddon ariaLabel={t('remove')} onClick={() => remove(field.name)}>
                    <MinusOutlined />
                  </InputAddon>
                </Space.Compact>
              ))}
            </Form.Item>
          )}
        </Form.List>

        <Form.List name="expectedIPs">
          {(fields, { add, remove }) => (
            <Form.Item label={t('pages.xray.dns.expectIPs')}>
              <Button size="small" type="primary" icon={<PlusOutlined />} aria-label={t('add')} onClick={() => add('')} />
              {fields.map((field) => (
                <Space.Compact key={field.key} block style={{ marginTop: 4 }}>
                  <Form.Item name={field.name} noStyle>
                    <Input />
                  </Form.Item>
                  <InputAddon ariaLabel={t('remove')} onClick={() => remove(field.name)}>
                    <MinusOutlined />
                  </InputAddon>
                </Space.Compact>
              ))}
            </Form.Item>
          )}
        </Form.List>

        <Form.List name="unexpectedIPs">
          {(fields, { add, remove }) => (
            <Form.Item label={t('pages.xray.dns.unexpectIPs')}>
              <Button size="small" type="primary" icon={<PlusOutlined />} aria-label={t('add')} onClick={() => add('')} />
              {fields.map((field) => (
                <Space.Compact key={field.key} block style={{ marginTop: 4 }}>
                  <Form.Item name={field.name} noStyle>
                    <Input />
                  </Form.Item>
                  <InputAddon ariaLabel={t('remove')} onClick={() => remove(field.name)}>
                    <MinusOutlined />
                  </InputAddon>
                </Space.Compact>
              ))}
            </Form.Item>
          )}
        </Form.List>

        <Divider style={{ margin: '5px 0' }} />

        <Form.Item label={t('pages.xray.dns.skipFallback')} name="skipFallback" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.finalQuery')} name="finalQuery" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.disableCache')} name="disableCache" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.serveStale')} name="serveStale" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.serveExpiredTTL')} name="serveExpiredTTL">
          <InputNumber min={0} step={60} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
