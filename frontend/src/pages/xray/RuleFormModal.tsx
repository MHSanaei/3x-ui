import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Modal, Select, Space, Tooltip } from 'antd';
import { PlusOutlined, MinusOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import InputAddon from '@/components/InputAddon';
import { RuleFormSchema, type RuleFormValues } from '@/schemas/xray';

export interface RoutingRule {
  type?: string;
  domain?: string | string[];
  ip?: string | string[];
  port?: string;
  sourcePort?: string;
  vlessRoute?: string;
  network?: string;
  sourceIP?: string | string[];
  user?: string | string[];
  inboundTag?: string[];
  protocol?: string[];
  attrs?: Record<string, string>;
  outboundTag?: string;
  balancerTag?: string;
  [key: string]: unknown;
}

interface RuleFormModalProps {
  open: boolean;
  rule: RoutingRule | null;
  inboundTags: string[];
  outboundTags: string[];
  balancerTags: string[];
  onClose: () => void;
  onConfirm: (rule: Record<string, unknown>) => void;
}

type FormState = RuleFormValues;

const initialForm = (): FormState => ({
  domain: '',
  ip: '',
  port: '',
  sourcePort: '',
  vlessRoute: '',
  network: '',
  sourceIP: '',
  user: '',
  inboundTag: [],
  protocol: [],
  attrs: [],
  outboundTag: '',
  balancerTag: '',
});

const NETWORKS = ['', 'TCP', 'UDP', 'TCP,UDP'];
const PROTOCOLS = ['http', 'tls', 'bittorrent', 'quic'];

function csv(value: string): string[] {
  if (!value) return [];
  return value.split(',').map((s) => s.trim()).filter(Boolean);
}

export default function RuleFormModal({
  open,
  rule,
  inboundTags,
  outboundTags,
  balancerTags,
  onClose,
  onConfirm,
}: RuleFormModalProps) {
  const { t } = useTranslation();
  const [form, setForm] = useState<FormState>(initialForm);
  const isEdit = rule != null;

  useEffect(() => {
    if (!open) return;
    if (rule) {
      setForm({
        domain: Array.isArray(rule.domain) ? rule.domain.join(',') : rule.domain || '',
        ip: Array.isArray(rule.ip) ? rule.ip.join(',') : rule.ip || '',
        port: rule.port || '',
        sourcePort: rule.sourcePort || '',
        vlessRoute: rule.vlessRoute || '',
        network: rule.network || '',
        sourceIP: Array.isArray(rule.sourceIP) ? rule.sourceIP.join(',') : rule.sourceIP || '',
        user: Array.isArray(rule.user) ? rule.user.join(',') : rule.user || '',
        inboundTag: rule.inboundTag || [],
        protocol: rule.protocol || [],
        attrs: rule.attrs ? Object.entries(rule.attrs) : [],
        outboundTag: rule.outboundTag || '',
        balancerTag: rule.balancerTag || '',
      });
    } else {
      setForm(initialForm());
    }
  }, [open, rule]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  function submit() {
    const validated = RuleFormSchema.safeParse(form);
    if (!validated.success) return;
    const v = validated.data;
    const built: Record<string, unknown> = {
      type: 'field',
      domain: csv(v.domain),
      ip: csv(v.ip),
      port: v.port,
      sourcePort: v.sourcePort,
      vlessRoute: v.vlessRoute,
      network: v.network,
      sourceIP: csv(v.sourceIP),
      user: csv(v.user),
      inboundTag: v.inboundTag,
      protocol: v.protocol,
      attrs: Object.fromEntries(v.attrs.filter(([k]) => k)),
      outboundTag: v.outboundTag === '' ? undefined : v.outboundTag,
      balancerTag: v.balancerTag === '' ? undefined : v.balancerTag,
    };
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(built)) {
      if (v == null) continue;
      if (Array.isArray(v) && v.length === 0) continue;
      if (typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length === 0) continue;
      if (v === '') continue;
      out[k] = v;
    }
    onConfirm(out);
  }

  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Routings')}`
    : `+ ${t('pages.xray.Routings')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText={t('close')}
      mask={{ closable: false }}
      width={640}
      onOk={submit}
      onCancel={onClose}
    >
      <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              Source IPs <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.sourceIP} onChange={(e) => update('sourceIP', e.target.value)} placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              Source port <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.sourcePort} onChange={(e) => update('sourcePort', e.target.value)} placeholder="53,443,1000-2000" />
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              VLESS route <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.vlessRoute} onChange={(e) => update('vlessRoute', e.target.value)} placeholder="53,443,1000-2000" />
        </Form.Item>

        <Form.Item label="Network">
          <Select
            value={form.network}
            onChange={(v) => update('network', v)}
            options={NETWORKS.map((n) => ({ value: n, label: n || '(any)' }))}
          />
        </Form.Item>

        <Form.Item label="Protocol">
          <Select
            mode="multiple"
            value={form.protocol}
            onChange={(v) => update('protocol', v)}
            options={PROTOCOLS.map((p) => ({ value: p, label: p }))}
          />
        </Form.Item>

        <Form.Item label="Attributes">
          <Button size="small" icon={<PlusOutlined />} onClick={() => update('attrs', [...form.attrs, ['', '']])} />
        </Form.Item>
        <Form.Item wrapperCol={{ span: 24 }}>
          {form.attrs.map((attr, idx) => (
            <Space.Compact key={idx} block className="mb-8">
              <InputAddon>{`${idx + 1}`}</InputAddon>
              <Input
                value={attr[0]}
                placeholder="Name"
                onChange={(e) => {
                  const next = form.attrs.map((a, i) => (i === idx ? ([e.target.value, a[1]] as [string, string]) : a));
                  update('attrs', next);
                }}
              />
              <Input
                value={attr[1]}
                placeholder="Value"
                onChange={(e) => {
                  const next = form.attrs.map((a, i) => (i === idx ? ([a[0], e.target.value] as [string, string]) : a));
                  update('attrs', next);
                }}
              />
              <Button
                icon={<MinusOutlined />}
                onClick={() => update('attrs', form.attrs.filter((_, i) => i !== idx))}
              />
            </Space.Compact>
          ))}
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              IP <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.ip} onChange={(e) => update('ip', e.target.value)} placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              Domain <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.domain} onChange={(e) => update('domain', e.target.value)} placeholder="google.com, geosite:cn" />
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              User <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.user} onChange={(e) => update('user', e.target.value)} placeholder="email address" />
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Comma-separated list">
              Port <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Input value={form.port} onChange={(e) => update('port', e.target.value)} placeholder="53,443,1000-2000" />
        </Form.Item>

        <Form.Item label="Inbound tags">
          <Select
            mode="multiple"
            value={form.inboundTag}
            onChange={(v) => update('inboundTag', v)}
            options={inboundTags.map((tag) => ({ value: tag, label: tag }))}
          />
        </Form.Item>

        <Form.Item label="Outbound tag">
          <Select
            value={form.outboundTag}
            onChange={(v) => update('outboundTag', v)}
            options={outboundTags.map((tag) => ({ value: tag, label: tag || '(none)' }))}
          />
        </Form.Item>

        <Form.Item
          label={
            <Tooltip title="Routes traffic through one of the configured load balancers">
              Balancer tag <QuestionCircleOutlined />
            </Tooltip>
          }
        >
          <Select
            value={form.balancerTag}
            onChange={(v) => update('balancerTag', v)}
            options={balancerTags.map((tag) => ({ value: tag, label: tag || '(none)' }))}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
