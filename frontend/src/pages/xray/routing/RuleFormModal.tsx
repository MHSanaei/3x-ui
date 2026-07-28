import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Modal, Select, Space, Switch, Tooltip } from 'antd';
import { PlusOutlined, MinusOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { InputAddon } from '@/components/ui';
import { FormField } from '@/components/form/rhf';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { RuleFormSchema, type RuleFormValues } from '@/schemas/xray';
import { buildRemarkByTag, formatInboundTag, isApiRule } from './helpers';

export interface RoutingRule {
  enabled?: boolean;
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

const initialForm = (): RuleFormValues => ({
  enabled: true,
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

const NETWORKS = ['', 'tcp', 'udp', 'tcp,udp'];
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
  const methods = useForm<RuleFormValues>({ defaultValues: initialForm() });
  const isEdit = rule != null;

  const { data: inboundOptions } = useInboundOptions();
  const remarkByTag = useMemo(() => buildRemarkByTag(inboundOptions || []), [inboundOptions]);

  useEffect(() => {
    if (!open) return;
    if (rule) {
      methods.reset({
        enabled: rule.enabled !== false,
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
      methods.reset(initialForm());
    }
  }, [open, rule, methods]);

  const attrs = useWatch({ control: methods.control, name: 'attrs' }) ?? [];

  function submit() {
    const validated = RuleFormSchema.safeParse(methods.getValues());
    if (!validated.success) return;
    const v = validated.data;
    const built: Record<string, unknown> = {
      type: 'field',
      enabled: v.enabled,
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
    const managedKeys = new Set(Object.keys(built));
    const out: Record<string, unknown> = {};
    if (rule) {
      for (const [key, value] of Object.entries(rule)) {
        if (!managedKeys.has(key) && value !== undefined) out[key] = value;
      }
    }
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
      <FormProvider {...methods}>
        <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
          <FormField name="enabled" label={t('enable')} valueProp="checked">
            <Switch disabled={isApiRule(rule ?? {})} />
          </FormField>

          <FormField
            name="sourceIP"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.sourceIps')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
          </FormField>

          <FormField
            name="sourcePort"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.sourcePort')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="53,443,1000-2000" />
          </FormField>

          <FormField
            name="vlessRoute"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.vlessRoute')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="53,443,1000-2000" />
          </FormField>

          <FormField name="network" label={t('pages.inbounds.network')}>
            <Select options={NETWORKS.map((n) => ({ value: n, label: n || '(any)' }))} />
          </FormField>

          <FormField name="protocol" label={t('pages.inbounds.protocol')}>
            <Select mode="multiple" options={PROTOCOLS.map((p) => ({ value: p, label: p }))} />
          </FormField>

          <Form.Item label={t('pages.xray.ruleForm.attributes')}>
            <Button
              size="small"
              aria-label={t('add')}
              icon={<PlusOutlined />}
              onClick={() => methods.setValue('attrs', [...attrs, ['', ''] as [string, string]])}
            />
          </Form.Item>
          <Form.Item wrapperCol={{ span: 24 }}>
            {attrs.map((attr, idx) => (
              <Space.Compact key={idx} block className="mb-8">
                <InputAddon>{`${idx + 1}`}</InputAddon>
                <Input
                  value={attr[0]}
                  aria-label={t('pages.nodes.name')}
                  placeholder={t('pages.nodes.name')}
                  onChange={(e) => {
                    const next = attrs.map((a, i) => (i === idx ? ([e.target.value, a[1]] as [string, string]) : a));
                    methods.setValue('attrs', next);
                  }}
                />
                <Input
                  value={attr[1]}
                  aria-label={t('pages.xray.ruleForm.value')}
                  placeholder={t('pages.xray.ruleForm.value')}
                  onChange={(e) => {
                    const next = attrs.map((a, i) => (i === idx ? ([a[0], e.target.value] as [string, string]) : a));
                    methods.setValue('attrs', next);
                  }}
                />
                <Button
                  aria-label={t('remove')}
                  icon={<MinusOutlined />}
                  onClick={() => methods.setValue('attrs', attrs.filter((_, i) => i !== idx))}
                />
              </Space.Compact>
            ))}
          </Form.Item>

          <FormField
            name="ip"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                IP <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="0.0.0.0/8, fc00::/7, geoip:ir" />
          </FormField>

          <FormField
            name="domain"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('domainName')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="google.com, geosite:cn" />
          </FormField>

          <FormField
            name="user"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.xray.ruleForm.user')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="email address" />
          </FormField>

          <FormField
            name="port"
            label={
              <Tooltip title={t('pages.xray.rules.useComma')}>
                {t('pages.inbounds.port')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Input placeholder="53,443,1000-2000" />
          </FormField>

          <FormField name="inboundTag" label={t('pages.xray.ruleForm.inboundTags')}>
            <Select
              mode="multiple"
              options={inboundTags.map((tag) => ({ value: tag, label: formatInboundTag(tag, remarkByTag) }))}
            />
          </FormField>

          <FormField name="outboundTag" label={t('pages.xray.ruleForm.outboundTag')}>
            <Select options={outboundTags.map((tag) => ({ value: tag, label: tag || '(none)' }))} />
          </FormField>

          <FormField
            name="balancerTag"
            label={
              <Tooltip title={t('pages.xray.ruleForm.balancerTagTooltip')}>
                {t('pages.xray.ruleForm.balancerTag')} <QuestionCircleOutlined aria-hidden="true" />
              </Tooltip>
            }
          >
            <Select options={balancerTags.map((tag) => ({ value: tag, label: tag || '(none)' }))} />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
