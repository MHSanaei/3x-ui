import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AutoComplete, Button, Form, Input, InputNumber, Modal, Select, Space, Switch, Tooltip, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { RandomUtil, SizeFormatter } from '@/utils';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives';
import { DateTimePicker, SelectAllClearButtons } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { useClients, type InboundOption } from '@/hooks/useClients';
import { useFail2banStatusQuery, getLimitIpNotice } from '@/api/queries/useFail2banStatusQuery';
import { ClientBulkAddFormSchema, type ClientBulkAddFormValues } from '@/schemas/client';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria', 'wireguard',
]);

const EMPTY: ClientBulkAddFormValues = {
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

interface ClientBulkAddModalProps {
  open: boolean;
  inbounds: InboundOption[];
  groups?: string[];
  onOpenChange: (open: boolean) => void;
  onSaved?: () => void;
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

  const methods = useForm<ClientBulkAddFormValues>({ defaultValues: EMPTY });
  const inboundIds = useWatch({ control: methods.control, name: 'inboundIds' });
  const emailMethod = useWatch({ control: methods.control, name: 'emailMethod' });
  const firstNum = useWatch({ control: methods.control, name: 'firstNum' });
  const flow = useWatch({ control: methods.control, name: 'flow' });
  const expiryTime = useWatch({ control: methods.control, name: 'expiryTime' });
  const subId = useWatch({ control: methods.control, name: 'subId' });
  const limitIp = useWatch({ control: methods.control, name: 'limitIp' });
  const [delayedStart, setDelayedStart] = useState(false);
  const [saving, setSaving] = useState(false);
  const fail2ban = useFail2banStatusQuery();
  const limitIpDisabled = !fail2ban.usable;
  const limitIpNotice = getLimitIpNotice(fail2ban, t);

  useEffect(() => {
    if (!open) return;

    methods.reset(EMPTY);
    setDelayedStart(false);

  }, [open, methods]);

  const flowCapableIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row?.tlsFlowCapable) ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const showFlow = useMemo(
    () => (inboundIds || []).some((id) => flowCapableIds.has(id)),
    [inboundIds, flowCapableIds],
  );

  const ss2022Method = useMemo(() => {
    for (const id of inboundIds || []) {
      const ib = (inbounds || []).find((row) => row.id === id);
      const method = ib?.ssMethod;
      if (method && method.substring(0, 4) === '2022') return method;
    }
    return '';
  }, [inboundIds, inbounds]);

  useEffect(() => {
    if (!showFlow && flow) {

      methods.setValue('flow', '');
    }
  }, [showFlow, flow, methods]);

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
    () => (expiryTime > 0 ? dayjs(expiryTime) : null),
    [expiryTime],
  );

  const delayedExpireDays = expiryTime < 0 ? expiryTime / -86400000 : 0;

  function buildEmails(values: ClientBulkAddFormValues): string[] {
    const method = values.emailMethod;
    const out: string[] = [];
    let start: number;
    let end: number;
    if (method > 1) {
      start = values.firstNum;
      end = values.lastNum + 1;
    } else {
      start = 0;
      end = values.quantity;
    }
    const prefix = method > 0 && values.emailPrefix.length > 0 ? values.emailPrefix : '';
    const useNum = method > 1;
    const postfix = method > 2 && values.emailPostfix.length > 0 ? values.emailPostfix : '';
    for (let i = start; i < end; i++) {
      let email = '';
      if (method !== 4) email = RandomUtil.randomLowerAndNum(10);
      email += useNum ? prefix + String(i) + postfix : prefix + postfix;
      out.push(email);
    }
    return out;
  }

  async function submit() {
    const current = methods.getValues();
    const validated = ClientBulkAddFormSchema.safeParse(current);
    if (!validated.success) {
      messageApi.error(t(validated.error.issues[0]?.message ?? 'somethingWentWrong'));
      return;
    }
    const emails = buildEmails(current);
    if (emails.length === 0) return;

    setSaving(true);
    try {
      const payloads = emails.map((email) => ({
        client: {
          email,
          subId: current.subId || RandomUtil.randomLowerAndNum(16),
          id: RandomUtil.randomUUID(),
          password: ss2022Method
            ? RandomUtil.randomShadowsocksPassword(ss2022Method)
            : RandomUtil.randomLowerAndNum(16),
          auth: RandomUtil.randomLowerAndNum(16),
          flow: showFlow ? (current.flow || '') : '',
          totalGB: Math.round((current.totalGB || 0) * SizeFormatter.ONE_GB),
          expiryTime: current.expiryTime,
          reset: Number(current.reset) || 0,
          limitIp: Number(current.limitIp) || 0,
          group: current.group,
          comment: current.comment,
          enable: true,
        },
        inboundIds: current.inboundIds,
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
        <FormProvider {...methods}>
          <Form colon={false} labelCol={{ sm: { span: 8 } }} wrapperCol={{ sm: { span: 14 } }}>
            <Form.Item label={t('pages.clients.attachedInbounds')} required>
              <SelectAllClearButtons
                options={inboundOptions}
                value={inboundIds}
                onChange={(v) => methods.setValue('inboundIds', v)}
              />
              <Select
                mode="multiple"
                value={inboundIds}
                onChange={(v) => methods.setValue('inboundIds', v)}
                options={inboundOptions}
                placeholder={t('pages.clients.selectInbound')}
                showSearch={{
                  filterOption: (input, option) => ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase()),
                }}
              />
            </Form.Item>

            <FormField name="emailMethod" label={t('pages.clients.method')}>
              <Select
                options={[
                  { value: 0, label: 'Random' },
                  { value: 1, label: 'Random + Prefix' },
                  { value: 2, label: 'Random + Prefix + Num' },
                  { value: 3, label: 'Random + Prefix + Num + Postfix' },
                  { value: 4, label: 'Prefix + Num + Postfix' },
                ]}
              />
            </FormField>

            {emailMethod > 1 && (
              <>
                <FormField name="firstNum" label={t('pages.clients.first')} transform={{ output: (v) => Number(v) || 1 }}>
                  <InputNumber min={1} />
                </FormField>
                <FormField name="lastNum" label={t('pages.clients.last')} transform={{ output: (v) => Number(v) || 1 }}>
                  <InputNumber min={firstNum} />
                </FormField>
              </>
            )}
            {emailMethod > 0 && (
              <FormField name="emailPrefix" label={t('pages.clients.prefix')}>
                <Input />
              </FormField>
            )}
            {emailMethod > 2 && (
              <FormField name="emailPostfix" label={t('pages.clients.postfix')}>
                <Input />
              </FormField>
            )}
            {emailMethod < 2 && (
              <FormField name="quantity" label={t('pages.clients.clientCount')} transform={{ output: (v) => Number(v) || 1 }}>
                <InputNumber min={1} max={1000} />
              </FormField>
            )}

            <Form.Item label={t('pages.clients.subId')}>
              <Space.Compact style={{ display: 'flex' }}>
                <Input
                  value={subId}
                  onChange={(e) => methods.setValue('subId', e.target.value)}
                  style={{ flex: 1 }}
                />
                <Button
                  aria-label={t('regenerate')}
                  icon={<ReloadOutlined />}
                  onClick={() => methods.setValue('subId', RandomUtil.randomLowerAndNum(16))}
                />
              </Space.Compact>
            </Form.Item>

            <FormField
              name="group"
              label={t('pages.clients.group')}
              tooltip={t('pages.clients.groupDesc')}
              transform={{ output: (v) => v ?? '' }}
            >
              <AutoComplete
                placeholder={t('pages.clients.groupPlaceholder')}
                options={groups.map((g) => ({ value: g }))}
                allowClear
              />
            </FormField>

            <FormField name="comment" label={t('comment')}>
              <Input />
            </FormField>

            {showFlow && (
              <FormField name="flow" label={t('pages.clients.flow')}>
                <Select
                  style={{ width: 220 }}
                  options={[
                    { value: '', label: t('none') },
                    ...FLOW_OPTIONS.map((k) => ({ value: k, label: k })),
                  ]}
                />
              </FormField>
            )}

            <Form.Item label={t('pages.clients.limitIp')}>
              <Tooltip title={limitIpNotice || undefined}>
                <span style={{ display: 'inline-flex' }}>
                  <InputNumber value={limitIp} min={0} disabled={limitIpDisabled}
                    style={limitIpDisabled ? { pointerEvents: 'none' } : undefined}
                    onChange={(v) => methods.setValue('limitIp', Number(v) || 0)} />
                </span>
              </Tooltip>
            </Form.Item>

            <FormField name="totalGB" label={t('pages.clients.totalGB')} transform={{ output: (v) => Number(v) || 0 }}>
              <InputNumber min={0} step={1} />
            </FormField>

            <Form.Item label={t('pages.clients.delayedStart')}>
              <Switch
                checked={delayedStart}
                onClick={() => { setDelayedStart(!delayedStart); methods.setValue('expiryTime', 0); }}
              />
            </Form.Item>

            {delayedStart ? (
              <Form.Item label={t('pages.clients.expireDays')}>
                <InputNumber
                  value={delayedExpireDays}
                  min={0}
                  onChange={(v) => methods.setValue('expiryTime', -86400000 * (Number(v) || 0))}
                />
              </Form.Item>
            ) : (
              <Form.Item label={t('pages.inbounds.expireDate')}>
                <DateTimePicker
                  value={expiryDate}
                  onChange={(next) => methods.setValue('expiryTime', next ? next.valueOf() : 0)}
                />
              </Form.Item>
            )}

            <FormField
              name="reset"
              label={t('pages.clients.renew')}
              tooltip={t('pages.clients.renewDesc')}
              transform={{ output: (v) => Number(v) || 0 }}
            >
              <InputNumber min={0} />
            </FormField>
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
