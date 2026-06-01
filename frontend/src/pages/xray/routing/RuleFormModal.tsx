import {type ChangeEvent, useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Button, Col, Form, Input, Modal, Row, Select, Space, Typography} from 'antd';
import {PlusOutlined, MinusOutlined} from '@ant-design/icons';
import {InputAddon} from '@/components/ui';
import {RuleFormSchema, type RuleFormValues} from '@/schemas/xray';
import {LabelWithOnePerLineTooltip, LabelWithTooltip} from "@/components/ui/TooltipsHelper";

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

const CommaSeparatedTextArea = ({value, onChange, placeholder}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) => {
  const displayValue = value ? value.split(',').join('\n') : '';
  
  const handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    const commaSeparated = e.target.value
      .split(/\r?\n/)
      .join(',');
    onChange(commaSeparated);
  };
  
  return (
    <Input.TextArea
      autoSize={{minRows: 2, maxRows: 10}}
      value={displayValue}
      onChange={handleChange}
      placeholder={placeholder}
    />
  );
};

export default function RuleFormModal({
  open,
  rule,
  inboundTags,
  outboundTags,
  balancerTags,
  onClose,
  onConfirm,
}: RuleFormModalProps) {
  const {t} = useTranslation();
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
    setForm((prev) => ({...prev, [key]: value}));
  
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
  
  const rowLayout = {gutter: 16};
  const colLayout = {xs: 24, md: 8};
  
  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText={t('close')}
      mask={{closable: false}}
      width={1400}
      onOk={submit}
      onCancel={onClose}
      style={{top: 20}}
      styles={{
        body: {
          maxHeight: 'calc(100vh - 160px)',
          overflowY: 'auto',
          padding: '8px',
        },
      }}
    >
      <Form layout="vertical" colon={false}>
        <Row {...rowLayout}>
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="pages.xray.ruleForm.sourceIps"/>}>
              <CommaSeparatedTextArea
                value={form.sourceIP}
                onChange={(v) => update('sourceIP', v)}
                placeholder={"0.0.0.0/8\nfc00::/7\ngeoip:ir"}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="pages.xray.ruleForm.sourcePort"/>}>
              <CommaSeparatedTextArea
                value={form.sourcePort}
                onChange={(v) => update('sourcePort', v)}
                placeholder={"53\n443\n1000-2000"}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="pages.xray.ruleForm.vlessRoute"/>}>
              <CommaSeparatedTextArea
                value={form.vlessRoute}
                onChange={(v) => update('vlessRoute', v)}
                placeholder={"53\n443\n1000-2000"}
              />
            </Form.Item>
          </Col>
        </Row>
        
        <Row {...rowLayout}>
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="pages.xray.ruleForm.user"/>}>
              <CommaSeparatedTextArea
                value={form.user}
                onChange={(v) => update('user', v)}
                placeholder="email address"
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={t('pages.inbounds.network')}>
              <Select
                value={form.network}
                onChange={(v) => update('network', v)}
                options={NETWORKS.map((n) => ({value: n, label: n || '(any)'}))}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={t('pages.inbounds.protocol')}>
              <Select
                mode="multiple"
                value={form.protocol}
                onChange={(v) => update('protocol', v)}
                options={PROTOCOLS.map((p) => ({value: p, label: p}))}
              />
            </Form.Item>
          </Col>
        </Row>
        
        <Row {...rowLayout}>
          <Col {...colLayout}>
            <Form.Item label={t('pages.xray.ruleForm.inboundTags')}>
              <Select
                mode="multiple"
                value={form.inboundTag}
                onChange={(v) => update('inboundTag', v)}
                options={inboundTags.map((tag) => ({value: tag, label: tag}))}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={t('pages.xray.ruleForm.outboundTag')}>
              <Select
                value={form.outboundTag}
                onChange={(v) => update('outboundTag', v)}
                options={outboundTags.map((tag) => ({value: tag, label: tag || '(none)'}))}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item
              label={
                <LabelWithTooltip
                  labelKey="pages.xray.ruleForm.balancerTag"
                  tooltipKey="pages.xray.ruleForm.balancerTagTooltip"
                />
              }
            >
              <Select
                value={form.balancerTag}
                onChange={(v) => update('balancerTag', v)}
                options={balancerTags.map((tag) => ({value: tag, label: tag || '(none)'}))}
              />
            </Form.Item>
          </Col>
        </Row>
        
        <Row {...rowLayout}>
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="IP"/>}>
              <CommaSeparatedTextArea
                value={form.ip}
                onChange={(v) => update('ip', v)}
                placeholder={`0.0.0.0/8\nfc00::/7\ngeoip:ir`}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="pages.inbounds.port"/>}>
              <CommaSeparatedTextArea
                value={form.port}
                onChange={(v) => update('port', v)}
                placeholder={`53\n443\n1000-2000`}
              />
            </Form.Item>
          </Col>
          
          <Col {...colLayout}>
            <Form.Item label={<LabelWithOnePerLineTooltip labelKey="domainName"/>}>
              <CommaSeparatedTextArea
                value={form.domain}
                onChange={(v) => update('domain', v)}
                placeholder={`google.com\ngeosite:cn`}
              />
            </Form.Item>
          </Col>
        </Row>
        
        <Form.Item>
          <Space orientation="horizontal">
            <Typography.Text>
              {t('pages.xray.ruleForm.attributes')}
            </Typography.Text>
            
            <Button
              size="small"
              icon={<PlusOutlined/>}
              onClick={() => update('attrs', [...form.attrs, ['', '']])}
            />
          </Space>
        </Form.Item>
        
        {form.attrs.length > 0 && (
          <Form.Item>
            {form.attrs.map((attr, idx) => (
              <Space.Compact key={idx} block className="mb-8">
                <InputAddon>{`${idx + 1}`}</InputAddon>
                <Input
                  value={attr[0]}
                  placeholder={t('pages.nodes.name')}
                  onChange={(e) => {
                    const next = form.attrs.map((a, i) => (i === idx ? ([e.target.value, a[1]] as [string, string]) : a));
                    update('attrs', next);
                  }}
                />
                <Input
                  value={attr[1]}
                  placeholder={t('pages.xray.ruleForm.value')}
                  onChange={(e) => {
                    const next = form.attrs.map((a, i) => (i === idx ? ([a[0], e.target.value] as [string, string]) : a));
                    update('attrs', next);
                  }}
                />
                <Button
                  icon={<MinusOutlined/>}
                  onClick={() => update('attrs', form.attrs.filter((_, i) => i !== idx))}
                />
              </Space.Compact>
            ))}
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
}