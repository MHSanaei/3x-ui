import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Modal, Select, Switch, message } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

import { HttpUtil, RandomUtil, SizeFormatter } from '@/utils';
import { TLS_FLOW_CONTROL } from '@/models/inbound';
import DateTimePicker from '@/components/DateTimePicker';
import type { InboundOption } from '@/hooks/useClients';
import './ClientBulkAddModal.css';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria', 'hysteria2',
]);

interface ApiMsg {
  success?: boolean;
  msg?: string;
}

interface ClientBulkAddModalProps {
  open: boolean;
  inbounds: InboundOption[];
  ipLimitEnable?: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved?: () => void;
}

interface FormState {
  emailMethod: number;
  firstNum: number;
  lastNum: number;
  emailPrefix: string;
  emailPostfix: string;
  quantity: number;
  subId: string;
  comment: string;
  flow: string;
  limitIp: number;
  totalGB: number;
  expiryTime: number;
  inboundIds: number[];
}

function emptyForm(): FormState {
  return {
    emailMethod: 0,
    firstNum: 1,
    lastNum: 1,
    emailPrefix: '',
    emailPostfix: '',
    quantity: 1,
    subId: '',
    comment: '',
    flow: '',
    limitIp: 0,
    totalGB: 0,
    expiryTime: 0,
    inboundIds: [],
  };
}

export default function ClientBulkAddModal({
  open,
  inbounds,
  ipLimitEnable = false,
  onOpenChange,
  onSaved,
}: ClientBulkAddModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [form, setForm] = useState<FormState>(emptyForm);
  const [delayedStart, setDelayedStart] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
     
    setForm(emptyForm());
    setDelayedStart(false);
     
  }, [open]);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  const flowCapableIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row?.tlsFlowCapable) ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const showFlow = useMemo(
    () => (form.inboundIds || []).some((id) => flowCapableIds.has(id)),
    [form.inboundIds, flowCapableIds],
  );

  useEffect(() => {
    if (!showFlow && form.flow) {
       
      update('flow', '');
    }
  }, [showFlow, form.flow]);

  const inboundOptions = useMemo(
    () => (inbounds || [])
      .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol || ''))
      .map((ib) => ({
        label: `${ib.remark || `#${ib.id}`} · ${ib.protocol}:${ib.port}`,
        value: ib.id,
      })),
    [inbounds],
  );

  const expiryDate = useMemo<Dayjs | null>(
    () => (form.expiryTime > 0 ? dayjs(form.expiryTime) : null),
    [form.expiryTime],
  );

  const delayedExpireDays = form.expiryTime < 0 ? form.expiryTime / -86400000 : 0;

  function buildEmails(): string[] {
    const method = form.emailMethod;
    const out: string[] = [];
    let start: number;
    let end: number;
    if (method > 1) {
      start = form.firstNum;
      end = form.lastNum + 1;
    } else {
      start = 0;
      end = form.quantity;
    }
    const prefix = method > 0 && form.emailPrefix.length > 0 ? form.emailPrefix : '';
    const useNum = method > 1;
    const postfix = method > 2 && form.emailPostfix.length > 0 ? form.emailPostfix : '';
    for (let i = start; i < end; i++) {
      let email = '';
      if (method !== 4) email = RandomUtil.randomLowerAndNum(6);
      email += useNum ? prefix + String(i) + postfix : prefix + postfix;
      out.push(email);
    }
    return out;
  }

  async function submit() {
    if (!Array.isArray(form.inboundIds) || form.inboundIds.length === 0) {
      messageApi.error(t('pages.clients.selectInbound'));
      return;
    }
    const emails = buildEmails();
    if (emails.length === 0) return;

    setSaving(true);
    const silentJsonOpts = { ...JSON_HEADERS, silent: true };
    try {
      const results = await Promise.all(emails.map((email) => {
        const client = {
          email,
          subId: form.subId || RandomUtil.randomLowerAndNum(16),
          id: RandomUtil.randomUUID(),
          password: RandomUtil.randomLowerAndNum(16),
          auth: RandomUtil.randomLowerAndNum(16),
          flow: showFlow ? (form.flow || '') : '',
          totalGB: Math.round((form.totalGB || 0) * SizeFormatter.ONE_GB),
          expiryTime: form.expiryTime,
          limitIp: Number(form.limitIp) || 0,
          comment: form.comment,
          enable: true,
        };
        const payload = { client, inboundIds: form.inboundIds };
        return HttpUtil.post('/panel/api/clients/add', payload, silentJsonOpts) as Promise<ApiMsg>;
      }));
      let ok = 0;
      let failed = 0;
      let firstError = '';
      for (const msg of results) {
        if (msg?.success) ok++;
        else {
          failed++;
          if (!firstError && msg?.msg) firstError = msg.msg;
        }
      }
      if (failed === 0) {
        messageApi.success(t('pages.clients.toasts.bulkCreated', { count: ok }));
      } else {
        messageApi.warning(firstError
          ? `${t('pages.clients.toasts.bulkCreatedMixed', { ok, failed })} — ${firstError}`
          : t('pages.clients.toasts.bulkCreatedMixed', { ok, failed }));
      }
      onSaved?.();
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.bulk')}
        okText={t('create')}
      cancelText={t('close')}
      confirmLoading={saving}
      mask={{ closable: false }}
      width={640}
      onOk={submit}
      onCancel={() => onOpenChange(false)}
    >
      <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }}>
        <Form.Item label={t('pages.clients.attachedInbounds')} required>
          <Select
            mode="multiple"
            value={form.inboundIds}
            onChange={(v) => update('inboundIds', v)}
            options={inboundOptions}
            placeholder={t('pages.clients.selectInbound')}
            showSearch
            filterOption={(input, option) => ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase())}
          />
        </Form.Item>

        <Form.Item label={t('pages.clients.method')}>
          <Select
            value={form.emailMethod}
            onChange={(v) => update('emailMethod', v)}
            options={[
              { value: 0, label: 'Random' },
              { value: 1, label: 'Random + Prefix' },
              { value: 2, label: 'Random + Prefix + Num' },
              { value: 3, label: 'Random + Prefix + Num + Postfix' },
              { value: 4, label: 'Prefix + Num + Postfix' },
            ]}
          />
        </Form.Item>

        {form.emailMethod > 1 && (
          <>
            <Form.Item label={t('pages.clients.first')}>
              <InputNumber value={form.firstNum} min={1} onChange={(v) => update('firstNum', Number(v) || 1)} />
            </Form.Item>
            <Form.Item label={t('pages.clients.last')}>
              <InputNumber value={form.lastNum} min={form.firstNum} onChange={(v) => update('lastNum', Number(v) || 1)} />
            </Form.Item>
          </>
        )}
        {form.emailMethod > 0 && (
          <Form.Item label={t('pages.clients.prefix')}>
            <Input value={form.emailPrefix} onChange={(e) => update('emailPrefix', e.target.value)} />
          </Form.Item>
        )}
        {form.emailMethod > 2 && (
          <Form.Item label={t('pages.clients.postfix')}>
            <Input value={form.emailPostfix} onChange={(e) => update('emailPostfix', e.target.value)} />
          </Form.Item>
        )}
        {form.emailMethod < 2 && (
          <Form.Item label={t('pages.clients.clientCount')}>
            <InputNumber value={form.quantity} min={1} max={100} onChange={(v) => update('quantity', Number(v) || 1)} />
          </Form.Item>
        )}

        <Form.Item label={
          <>
            {t('subscription.title')}
            <SyncOutlined
              className="random-icon"
              onClick={() => update('subId', RandomUtil.randomLowerAndNum(16))}
            />
          </>
        }>
          <Input value={form.subId} onChange={(e) => update('subId', e.target.value)} />
        </Form.Item>

        <Form.Item label={t('comment')}>
          <Input value={form.comment} onChange={(e) => update('comment', e.target.value)} />
        </Form.Item>

        {showFlow && (
          <Form.Item label={t('pages.clients.flow')}>
            <Select
              value={form.flow}
              onChange={(v) => update('flow', v)}
              style={{ width: 220 }}
              options={[
                { value: '', label: t('none') },
                ...FLOW_OPTIONS.map((k) => ({ value: k, label: k })),
              ]}
            />
          </Form.Item>
        )}

        {ipLimitEnable && (
          <Form.Item label={t('pages.clients.limitIp')}>
            <InputNumber value={form.limitIp} min={0} onChange={(v) => update('limitIp', Number(v) || 0)} />
          </Form.Item>
        )}

        <Form.Item label={t('pages.clients.totalGB')}>
          <InputNumber value={form.totalGB} min={0} step={1} onChange={(v) => update('totalGB', Number(v) || 0)} />
        </Form.Item>

        <Form.Item label={t('pages.clients.delayedStart')}>
          <Switch
            checked={delayedStart}
            onClick={() => { setDelayedStart(!delayedStart); update('expiryTime', 0); }}
          />
        </Form.Item>

        {delayedStart ? (
          <Form.Item label={t('pages.clients.expireDays')}>
            <InputNumber
              value={delayedExpireDays}
              min={0}
              onChange={(v) => update('expiryTime', -86400000 * (Number(v) || 0))}
            />
          </Form.Item>
        ) : (
          <Form.Item label={t('pages.inbounds.expireDate')}>
            <DateTimePicker
              value={expiryDate}
              onChange={(next) => update('expiryTime', next ? next.valueOf() : 0)}
            />
          </Form.Item>
        )}
      </Form>
      </Modal>
    </>
  );
}
