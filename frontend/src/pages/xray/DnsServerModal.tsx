import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Input, InputNumber, Modal, Select, Space, Switch } from 'antd';
import { PlusOutlined, MinusOutlined } from '@ant-design/icons';
import InputAddon from '@/components/InputAddon';

export type DnsServerValue =
  | string
  | {
      address: string;
      port?: number;
      domains?: string[];
      expectedIPs?: string[];
      expectIPs?: string[];
      unexpectedIPs?: string[];
      queryStrategy?: string;
      skipFallback?: boolean;
      disableCache?: boolean;
      finalQuery?: boolean;
      tag?: string;
      clientIP?: string;
      serveStale?: boolean;
      serveExpiredTTL?: number;
      timeoutMs?: number;
      [key: string]: unknown;
    };

interface DnsServerModalProps {
  open: boolean;
  server: DnsServerValue | null;
  isEdit: boolean;
  onClose: () => void;
  onConfirm: (value: DnsServerValue) => void;
}

const STRATEGIES = ['UseSystem', 'UseIP', 'UseIPv4', 'UseIPv6'];

interface DnsForm {
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
}

function defaultServer(): DnsForm {
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

export default function DnsServerModal({
  open,
  server,
  isEdit,
  onClose,
  onConfirm,
}: DnsServerModalProps) {
  const { t } = useTranslation();
  const [form, setForm] = useState<DnsForm>(defaultServer());

  useEffect(() => {
    if (!open) return;
    if (server == null) {
      setForm(defaultServer());
      return;
    }
    if (typeof server === 'string') {
      setForm({ ...defaultServer(), address: server });
      return;
    }
    setForm({
      ...defaultServer(),
      ...server,
      domains: [...(server.domains || [])],
      expectedIPs: [...(server.expectedIPs || server.expectIPs || [])],
      unexpectedIPs: [...(server.unexpectedIPs || [])],
    });
  }, [open, server]);

  const update = <K extends keyof DnsForm>(key: K, value: DnsForm[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  function updateList(key: 'domains' | 'expectedIPs' | 'unexpectedIPs', mutator: (next: string[]) => void) {
    setForm((prev) => {
      const next = [...prev[key]];
      mutator(next);
      return { ...prev, [key]: next };
    });
  }

  function submit() {
    const isPlain =
      form.domains.length === 0 &&
      form.expectedIPs.length === 0 &&
      form.unexpectedIPs.length === 0 &&
      form.port === 53 &&
      form.queryStrategy === 'UseIP' &&
      form.skipFallback === false &&
      form.disableCache === false &&
      form.finalQuery === false &&
      !form.tag &&
      !form.clientIP &&
      form.serveStale === false &&
      form.serveExpiredTTL === 0 &&
      form.timeoutMs === 4000;
    if (isPlain) {
      onConfirm(form.address);
      return;
    }
    const out: Record<string, unknown> = {
      address: form.address,
      port: form.port,
      domains: form.domains.filter(Boolean),
      expectedIPs: form.expectedIPs.filter(Boolean),
      unexpectedIPs: form.unexpectedIPs.filter(Boolean),
      queryStrategy: form.queryStrategy,
      skipFallback: form.skipFallback,
      disableCache: form.disableCache,
      finalQuery: form.finalQuery,
      serveStale: form.serveStale,
      serveExpiredTTL: form.serveExpiredTTL,
      timeoutMs: form.timeoutMs,
    };
    if (form.tag) out.tag = form.tag;
    if (form.clientIP) out.clientIP = form.clientIP;
    onConfirm(out as DnsServerValue);
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
      <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
        <Form.Item label={t('pages.inbounds.address')}>
          <Input value={form.address} onChange={(e) => update('address', e.target.value)} />
        </Form.Item>
        <Form.Item label={t('pages.inbounds.port')}>
          <InputNumber value={form.port} min={1} max={65535} onChange={(v) => update('port', Number(v) || 53)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.tag')}>
          <Input value={form.tag} onChange={(e) => update('tag', e.target.value)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.clientIp')}>
          <Input value={form.clientIP} onChange={(e) => update('clientIP', e.target.value)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.strategy')}>
          <Select
            value={form.queryStrategy}
            onChange={(v) => update('queryStrategy', v)}
            style={{ width: '100%' }}
            options={STRATEGIES.map((s) => ({ value: s, label: s }))}
          />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.timeoutMs')}>
          <InputNumber value={form.timeoutMs} min={0} step={500} onChange={(v) => update('timeoutMs', Number(v) || 0)} />
        </Form.Item>

        <Divider style={{ margin: '5px 0' }} />

        <Form.Item label={t('pages.xray.dns.domains')}>
          <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => updateList('domains', (d) => d.push(''))} />
          {form.domains.map((value, idx) => (
            <Space.Compact key={`d${idx}`} block style={{ marginTop: 4 }}>
              <Input
                value={value}
                onChange={(e) => updateList('domains', (d) => { d[idx] = e.target.value; })}
              />
              <InputAddon onClick={() => updateList('domains', (d) => d.splice(idx, 1))}>
                <MinusOutlined />
              </InputAddon>
            </Space.Compact>
          ))}
        </Form.Item>

        <Form.Item label={t('pages.xray.dns.expectIPs')}>
          <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => updateList('expectedIPs', (d) => d.push(''))} />
          {form.expectedIPs.map((value, idx) => (
            <Space.Compact key={`e${idx}`} block style={{ marginTop: 4 }}>
              <Input
                value={value}
                onChange={(e) => updateList('expectedIPs', (d) => { d[idx] = e.target.value; })}
              />
              <InputAddon onClick={() => updateList('expectedIPs', (d) => d.splice(idx, 1))}>
                <MinusOutlined />
              </InputAddon>
            </Space.Compact>
          ))}
        </Form.Item>

        <Form.Item label={t('pages.xray.dns.unexpectIPs')}>
          <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => updateList('unexpectedIPs', (d) => d.push(''))} />
          {form.unexpectedIPs.map((value, idx) => (
            <Space.Compact key={`u${idx}`} block style={{ marginTop: 4 }}>
              <Input
                value={value}
                onChange={(e) => updateList('unexpectedIPs', (d) => { d[idx] = e.target.value; })}
              />
              <InputAddon onClick={() => updateList('unexpectedIPs', (d) => d.splice(idx, 1))}>
                <MinusOutlined />
              </InputAddon>
            </Space.Compact>
          ))}
        </Form.Item>

        <Divider style={{ margin: '5px 0' }} />

        <Form.Item label={t('pages.xray.dns.skipFallback')}>
          <Switch checked={form.skipFallback} onChange={(v) => update('skipFallback', v)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.finalQuery')}>
          <Switch checked={form.finalQuery} onChange={(v) => update('finalQuery', v)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.disableCache')}>
          <Switch checked={form.disableCache} onChange={(v) => update('disableCache', v)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.serveStale')}>
          <Switch checked={form.serveStale} onChange={(v) => update('serveStale', v)} />
        </Form.Item>
        <Form.Item label={t('pages.xray.dns.serveExpiredTTL')}>
          <InputNumber
            value={form.serveExpiredTTL}
            min={0}
            step={60}
            onChange={(v) => update('serveExpiredTTL', Number(v) || 0)}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
