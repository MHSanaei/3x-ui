import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AutoComplete, Button, Form, Input, InputNumber, Modal, Select, Space, Switch, Tooltip, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

import { RandomUtil, SizeFormatter } from '@/utils';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives';
import { DateTimePicker, SelectAllClearButtons } from '@/components/form';
import { useClients, type InboundOption } from '@/hooks/useClients';
import { useFail2banStatusQuery, getLimitIpNotice } from '@/api/queries/useFail2banStatusQuery';
import { ClientBulkAddFormSchema, type ClientBulkAddFormValues } from '@/schemas/client';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria', 'wireguard',
]);

interface ClientBulkAddModalProps {
  open: boolean;
  inbounds: InboundOption[];
  groups?: string[];
  onOpenChange: (open: boolean) => void;
  onSaved?: () => void;
}

type FormState = ClientBulkAddFormValues;

function emptyForm(): FormState {
  return {
    emailMethod: 0,
    firstNum: 1,
    lastNum: 1,
    emailPrefix: '',
    emailPostfix: '',
    quantity: 1,
    subId: '',
    group: '',
    comment: '',
    flow: '',
    limitIp: 0,
    totalGB: 0,
    expiryTime: 0,
    reset: 0,
    inboundIds: [],
  };
}

export default function ClientBulkAddModal({
  open,
  inbounds,
  groups = [],
  onOpenChange,
  onSaved,
}: ClientBulkAddModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const { bulkCreate } = useClients();

  const [form, setForm] = useState<FormState>(emptyForm);
  const [delayedStart, setDelayedStart] = useState(false);
  const [saving, setSaving] = useState(false);
  const fail2ban = useFail2banStatusQuery();
  const limitIpDisabled = !fail2ban.usable;
  const limitIpNotice = getLimitIpNotice(fail2ban, t);

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

  const ss2022Method = useMemo(() => {
    for (const id of form.inboundIds || []) {
      const ib = (inbounds || []).find((row) => row.id === id);
      const method = ib?.ssMethod;
      if (method && method.substring(0, 4) === '2022') return method;
    }
    return '';
  }, [form.inboundIds, inbounds]);

  useEffect(() => {
    if (!showFlow && form.flow) {

      update('flow', '');
    }
  }, [showFlow, form.flow]);

  const inboundOptions = useMemo(
    () => (inbounds || [])
      .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol || ''))
      .map((ib) => ({
        label: formatInboundLabel(ib.tag, ib.remark),
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
      if (method !== 4) email = RandomUtil.randomLowerAndNum(10);
      email += useNum ? prefix + String(i) + postfix : prefix + postfix;
      out.push(email);
    }
    return out;
  }

  async function submit() {
    const validated = ClientBulkAddFormSchema.safeParse(form);
    if (!validated.success) {
      messageApi.error(t(validated.error.issues[0]?.message ?? 'somethingWentWrong'));
      return;
    }
    const emails = buildEmails();
    if (emails.length === 0) return;

    setSaving(true);
    try {
      const payloads = emails.map((email) => ({
        client: {
          email,
          subId: form.subId || RandomUtil.randomLowerAndNum(16),
          id: RandomUtil.randomUUID(),
          password: ss2022Method
            ? RandomUtil.randomShadowsocksPassword(ss2022Method)
            : RandomUtil.randomLowerAndNum(16),
          auth: RandomUtil.randomLowerAndNum(16),
          flow: showFlow ? (form.flow || '') : '',
          totalGB: Math.round((form.totalGB || 0) * SizeFormatter.ONE_GB),
          expiryTime: form.expiryTime,
          reset: Number(form.reset) || 0,
          limitIp: Number(form.limitIp) || 0,
          group: form.group,
          comment: form.comment,
          enable: true,
        },
        inboundIds: form.inboundIds,
      }));
      const msg = await bulkCreate(payloads);
      const ok = msg?.obj?.created ?? 0;
      const skipped = msg?.obj?.skipped ?? [];
      const failed = skipped.length;
      const firstError = skipped[0]?.reason ?? msg?.msg ?? '';
      if (failed === 0 && msg?.success) {
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
            <SelectAllClearButtons
              options={inboundOptions}
              value={form.inboundIds}
              onChange={(v) => update('inboundIds', v)}
            />
            <Select
              mode="multiple"
              value={form.inboundIds}
              onChange={(v) => update('inboundIds', v)}
              options={inboundOptions}
              placeholder={t('pages.clients.selectInbound')}
              showSearch={{
                filterOption: (input, option) => ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase()),
              }}
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
              <InputNumber value={form.quantity} min={1} max={1000} onChange={(v) => update('quantity', Number(v) || 1)} />
            </Form.Item>
          )}

          <Form.Item label={t('pages.clients.subId')}>
            <Space.Compact style={{ display: 'flex' }}>
              <Input
                value={form.subId}
                onChange={(e) => update('subId', e.target.value)}
                style={{ flex: 1 }}
              />
              <Button
                icon={<ReloadOutlined />}
                onClick={() => update('subId', RandomUtil.randomLowerAndNum(16))}
              />
            </Space.Compact>
          </Form.Item>

          <Form.Item label={t('pages.clients.group')} tooltip={t('pages.clients.groupDesc')}>
            <AutoComplete
              value={form.group}
              placeholder={t('pages.clients.groupPlaceholder')}
              options={groups.map((g) => ({ value: g }))}
              onChange={(v) => update('group', v ?? '')}
              allowClear
            />
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

          <Form.Item label={t('pages.clients.limitIp')}>
            <Tooltip title={limitIpNotice || undefined}>
              <span style={{ display: 'inline-flex' }}>
                <InputNumber value={form.limitIp} min={0} disabled={limitIpDisabled}
                  style={limitIpDisabled ? { pointerEvents: 'none' } : undefined}
                  onChange={(v) => update('limitIp', Number(v) || 0)} />
              </span>
            </Tooltip>
          </Form.Item>

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

          <Form.Item
            label={t('pages.clients.renew')}
            tooltip={t('pages.clients.renewDesc')}
          >
            <InputNumber
              value={form.reset}
              min={0}
              onChange={(v) => update('reset', Number(v) || 0)}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
