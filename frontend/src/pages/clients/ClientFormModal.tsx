import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  AutoComplete,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  message,
} from 'antd';
import { EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';

import { HttpUtil, RandomUtil } from '@/utils';
import { DateTimePicker } from '@/components/form';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { ClientFormSchema, ClientCreateFormSchema } from '@/schemas/client';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);
const VMESS_SECURITY_OPTIONS = ['auto', 'aes-128-gcm', 'chacha20-poly1305', 'none', 'zero'] as const;

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria',
]);

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

type Mode = 'add' | 'edit';

interface SaveMetaEdit {
  isEdit: true;
  email: string;
  attach: number[];
  detach: number[];
}

interface SaveMetaCreate {
  isEdit: false;
}

interface SaveCreatePayload {
  client: Record<string, unknown>;
  inboundIds: number[];
}

interface ClientFormModalProps {
  open: boolean;
  mode: Mode;
  client: ClientRecord | null;
  inbounds: InboundOption[];
  attachedIds?: number[];
  ipLimitEnable?: boolean;
  tgBotEnable?: boolean;
  groups?: string[];
  save: (
    payload: Record<string, unknown> | SaveCreatePayload,
    meta: SaveMetaEdit | SaveMetaCreate,
  ) => Promise<ApiMsg | null>;
  onOpenChange: (open: boolean) => void;
}

interface FormState {
  email: string;
  subId: string;
  uuid: string;
  password: string;
  auth: string;
  flow: string;
  security: string;
  reverseTag: string;
  totalGB: number;
  expiryDate: Dayjs | null;
  delayedStart: boolean;
  delayedDays: number;
  reset: number;
  limitIp: number;
  tgId: number;
  group: string;
  comment: string;
  enable: boolean;
  inboundIds: number[];
}

function emptyForm(): FormState {
  return {
    email: '',
    subId: '',
    uuid: '',
    password: '',
    auth: '',
    flow: '',
    security: 'auto',
    reverseTag: '',
    totalGB: 0,
    expiryDate: null,
    delayedStart: false,
    delayedDays: 0,
    reset: 0,
    limitIp: 0,
    tgId: 0,
    group: '',
    comment: '',
    enable: true,
    inboundIds: [],
  };
}

function bytesToGB(bytes: number): number {
  if (!bytes || bytes <= 0) return 0;
  return Math.round((bytes / (1024 * 1024 * 1024)) * 100) / 100;
}

function gbToBytes(gb: number): number {
  if (!gb || gb <= 0) return 0;
  return Math.round(gb * 1024 * 1024 * 1024);
}

export default function ClientFormModal({
  open,
  mode,
  client,
  inbounds,
  attachedIds = [],
  ipLimitEnable = false,
  tgBotEnable = false,
  groups = [],
  save,
  onOpenChange,
}: ClientFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const isEdit = mode === 'edit';

  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [clientIps, setClientIps] = useState<string[]>([]);
  const [ipsLoading, setIpsLoading] = useState(false);
  const [ipsClearing, setIpsClearing] = useState(false);
  const [ipsModalOpen, setIpsModalOpen] = useState(false);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  useEffect(() => {
    if (!open) return;
    setIpsModalOpen(false);

    if (isEdit && client) {
      const et = Number(client.expiryTime) || 0;
      const next: FormState = {
        ...emptyForm(),
        email: client.email || '',
        subId: client.subId || '',
        uuid: client.uuid || '',
        password: client.password || '',
        auth: client.auth || '',
        flow: client.flow || '',
        security: client.security || 'auto',
        reverseTag: client.reverse?.tag || '',
        totalGB: bytesToGB(client.totalGB || 0),
        reset: Number(client.reset) || 0,
        limitIp: client.limitIp || 0,
        tgId: Number(client.tgId) || 0,
        group: client.group || '',
        comment: client.comment || '',
        enable: !!client.enable,
        inboundIds: Array.isArray(attachedIds) ? [...attachedIds] : [],
      };
      if (et < 0) {
        next.delayedStart = true;
        next.delayedDays = Math.round(et / -86400000);
        next.expiryDate = null;
      } else {
        next.delayedStart = false;
        next.delayedDays = 0;
        next.expiryDate = et > 0 ? dayjs(et) : null;
      }
      setForm(next);
      void loadIps();
    } else {
      setForm({
        ...emptyForm(),
        email: RandomUtil.randomLowerAndNum(10),
        uuid: RandomUtil.randomUUID(),
        subId: RandomUtil.randomLowerAndNum(16),
        password: RandomUtil.randomLowerAndNum(16),
        auth: RandomUtil.randomLowerAndNum(16),
      });
    }

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, isEdit]);

  const flowCapableIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row?.tlsFlowCapable) ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const vlessLikeIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row && row.protocol === 'vless') ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const vmessIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row && row.protocol === 'vmess') ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const showFlow = useMemo(
    () => (form.inboundIds || []).some((id) => flowCapableIds.has(id)),
    [form.inboundIds, flowCapableIds],
  );

  const showReverseTag = useMemo(
    () => (form.inboundIds || []).some((id) => vlessLikeIds.has(id)),
    [form.inboundIds, vlessLikeIds],
  );

  const showSecurity = useMemo(
    () => (form.inboundIds || []).some((id) => vmessIds.has(id)),
    [form.inboundIds, vmessIds],
  );

  useEffect(() => {
    if (!showFlow && form.flow) {

      update('flow', '');
    }
  }, [showFlow, form.flow]);

  useEffect(() => {
    if (!showReverseTag && form.reverseTag) {

      update('reverseTag', '');
    }
  }, [showReverseTag, form.reverseTag]);

  const inboundOptions = useMemo(
    () => (inbounds || [])
      .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol || ''))
      .map((ib) => ({
        label: ib.remark?.trim() || ib.tag || '',
        value: ib.id,
        title: ib.remark?.trim() || ib.tag || '',
      })),
    [inbounds],
  );

  async function loadIps() {
    if (!isEdit || !client?.email) return;
    setIpsLoading(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/clients/ips/${encodeURIComponent(client.email)}`) as ApiMsg<unknown[]>;
      if (!msg?.success) { setClientIps([]); return; }
      const arr = Array.isArray(msg.obj) ? msg.obj : [];
      setClientIps(arr.filter((x): x is string => typeof x === 'string' && x.length > 0));
    } finally {
      setIpsLoading(false);
    }
  }

  function openIpsModal() {
    setIpsModalOpen(true);
    if (clientIps.length === 0) void loadIps();
  }

  async function clearIps() {
    if (!isEdit || !client?.email) return;
    setIpsClearing(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/clients/clearIps/${encodeURIComponent(client.email)}`) as ApiMsg;
      if (msg?.success) setClientIps([]);
    } finally {
      setIpsClearing(false);
    }
  }

  function close() {
    onOpenChange(false);
  }

  async function onSubmit() {
    const schema = isEdit ? ClientFormSchema : ClientCreateFormSchema;
    const validated = schema.safeParse({
      email: form.email,
      subId: form.subId,
      uuid: form.uuid,
      password: form.password,
      auth: form.auth,
      flow: form.flow,
      security: form.security,
      reverseTag: form.reverseTag,
      totalGB: form.totalGB,
      delayedStart: form.delayedStart,
      delayedDays: form.delayedDays,
      reset: form.reset,
      limitIp: form.limitIp,
      tgId: form.tgId,
      group: form.group,
      comment: form.comment,
      enable: form.enable,
      inboundIds: form.inboundIds,
    });
    if (!validated.success) {
      const issue = validated.error.issues[0];
      messageApi.error(t(issue?.message ?? 'somethingWentWrong'));
      return;
    }
    const expiryTime = form.delayedStart
      ? -86400000 * (Number(form.delayedDays) || 0)
      : (form.expiryDate ? form.expiryDate.valueOf() : 0);
    const clientPayload: Record<string, unknown> = {
      email: form.email.trim(),
      subId: form.subId,
      id: form.uuid,
      password: form.password,
      auth: form.auth,
      flow: showFlow ? (form.flow || '') : '',
      security: showSecurity ? (form.security || 'auto') : 'auto',
      totalGB: gbToBytes(form.totalGB),
      expiryTime,
      reset: Number(form.reset) || 0,
      limitIp: Number(form.limitIp) || 0,
      tgId: Number(form.tgId) || 0,
      group: form.group,
      comment: form.comment,
      enable: !!form.enable,
    };
    const reverseTag = showReverseTag ? (form.reverseTag || '').trim() : '';
    if (reverseTag) {
      clientPayload.reverse = { tag: reverseTag };
    }

    setSubmitting(true);
    try {
      let msg;
      if (isEdit && client) {
        const original = new Set(attachedIds || []);
        const next = new Set(form.inboundIds || []);
        const toAttach = [...next].filter((id) => !original.has(id));
        const toDetach = [...original].filter((id) => !next.has(id));
        msg = await save(clientPayload, {
          isEdit: true,
          email: client.email,
          attach: toAttach,
          detach: toDetach,
        });
      } else {
        msg = await save(
          { client: clientPayload, inboundIds: form.inboundIds },
          { isEdit: false },
        );
      }
      if (msg?.success) close();
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={isEdit ? t('pages.clients.editClient') : t('pages.clients.addClient')}
        destroyOnHidden
        okText={isEdit ? t('save') : t('create')}
        cancelText={t('cancel')}
        okButtonProps={{ loading: submitting }}
        width={720}
        style={{ top: 20 }}
        styles={{ body: { maxHeight: 'calc(100vh - 160px)', overflowY: 'auto', overflowX: 'hidden' } }}
        onOk={onSubmit}
        onCancel={close}
      >
        <Form layout="vertical">
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.email')} required>
                <Space.Compact style={{ display: 'flex' }}>
                  <Input
                    value={form.email}
                    placeholder={t('pages.clients.email')}
                    style={{ flex: 1 }}
                    onChange={(e) => update('email', e.target.value)}
                  />
                  <Button icon={<ReloadOutlined />} onClick={() => update('email', RandomUtil.randomLowerAndNum(12))} />
                </Space.Compact>
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.subId')}>
                <Space.Compact style={{ display: 'flex' }}>
                  <Input value={form.subId} style={{ flex: 1 }} onChange={(e) => update('subId', e.target.value)} />
                  <Button icon={<ReloadOutlined />} onClick={() => update('subId', RandomUtil.randomLowerAndNum(16))} />
                </Space.Compact>
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.hysteriaAuth')}>
                <Space.Compact style={{ display: 'flex' }}>
                  <Input value={form.auth} style={{ flex: 1 }} onChange={(e) => update('auth', e.target.value)} />
                  <Button icon={<ReloadOutlined />} onClick={() => update('auth', RandomUtil.randomLowerAndNum(16))} />
                </Space.Compact>
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.password')}>
                <Space.Compact style={{ display: 'flex' }}>
                  <Input value={form.password} style={{ flex: 1 }} onChange={(e) => update('password', e.target.value)} />
                  <Button icon={<ReloadOutlined />} onClick={() => update('password', RandomUtil.randomLowerAndNum(16))} />
                </Space.Compact>
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.uuid')}>
                <Space.Compact style={{ display: 'flex' }}>
                  <Input value={form.uuid} style={{ flex: 1 }} onChange={(e) => update('uuid', e.target.value)} />
                  <Button icon={<ReloadOutlined />} onClick={() => update('uuid', RandomUtil.randomUUID())} />
                </Space.Compact>
              </Form.Item>
            </Col>
            <Col xs={24} md={ipLimitEnable ? 8 : 12}>
              <Form.Item label={t('pages.clients.totalGB')}>
                <InputNumber value={form.totalGB} min={0} step={1} style={{ width: '100%' }}
                  onChange={(v) => update('totalGB', Number(v) || 0)} />
              </Form.Item>
            </Col>
            {ipLimitEnable && (
              <Col xs={24} md={4}>
                <Form.Item label={t('pages.clients.limitIp')}>
                  <InputNumber value={form.limitIp} min={0} style={{ width: '100%' }}
                    onChange={(v) => update('limitIp', Number(v) || 0)} />
                </Form.Item>
              </Col>
            )}
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              {form.delayedStart ? (
                <Form.Item label={t('pages.clients.expireDays')}>
                  <InputNumber value={form.delayedDays} min={0} style={{ width: '100%' }}
                    onChange={(v) => update('delayedDays', Number(v) || 0)} />
                </Form.Item>
              ) : (
                <Form.Item label={t('pages.clients.expiryTime')}>
                  <DateTimePicker
                    value={form.expiryDate}
                    onChange={(d) => update('expiryDate', d || null)}
                  />
                </Form.Item>
              )}
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.delayedStart')}>
                <Switch
                  checked={form.delayedStart}
                  onChange={(v) => {
                    update('delayedStart', v);
                    if (v) update('expiryDate', null);
                    else update('delayedDays', 0);
                  }}
                />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.clients.renew')}
                tooltip={t('pages.clients.renewDesc')}
              >
                <InputNumber value={form.reset} min={0} style={{ width: '100%' }}
                  onChange={(v) => update('reset', Number(v) || 0)} />
              </Form.Item>
            </Col>
            {showReverseTag && (
              <Col xs={24} md={12}>
                <Form.Item label={t('pages.clients.reverseTag')}>
                  <Input value={form.reverseTag} placeholder={t('pages.clients.reverseTagPlaceholder')}
                    onChange={(e) => update('reverseTag', e.target.value)} />
                </Form.Item>
              </Col>
            )}
            {showFlow && (
              <Col xs={24} md={12}>
                <Form.Item label={t('pages.clients.flow')}>
                  <Select
                    value={form.flow}
                    onChange={(v) => update('flow', v)}
                    options={[
                      { value: '', label: t('none') },
                      ...FLOW_OPTIONS.map((k) => ({ value: k, label: k })),
                    ]}
                  />
                </Form.Item>
              </Col>
            )}
            {showSecurity && (
              <Col xs={24} md={12}>
                <Form.Item label={t('pages.clients.vmessSecurity')}>
                  <Select
                    value={form.security}
                    onChange={(v) => update('security', v)}
                    options={VMESS_SECURITY_OPTIONS.map((k) => ({ value: k, label: k }))}
                  />
                </Form.Item>
              </Col>
            )}
          </Row>

          <Row gutter={16}>
            {tgBotEnable && (
              <Col xs={24} md={12}>
                <Form.Item label={t('pages.clients.telegramId')}>
                  <InputNumber value={form.tgId} min={0} controls={false}
                    placeholder={t('pages.clients.telegramIdPlaceholder')} style={{ width: '100%' }}
                    onChange={(v) => update('tgId', Number(v) || 0)} />
                </Form.Item>
              </Col>
            )}
            <Col xs={24} md={tgBotEnable ? 12 : 24}>
              <Form.Item label={t('pages.clients.comment')}>
                <Input value={form.comment} onChange={(e) => update('comment', e.target.value)} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.clients.group')} tooltip={t('pages.clients.groupDesc')}>
                <AutoComplete
                  value={form.group}
                  placeholder={t('pages.clients.groupPlaceholder')}
                  options={groups.map((g) => ({ value: g }))}
                  onChange={(v) => update('group', v ?? '')}
                  filterOption={(input, option) =>
                    String(option?.value ?? '').toLowerCase().includes((input || '').toLowerCase())
                  }
                  allowClear
                  style={{ width: '100%' }}
                />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item label={t('pages.clients.attachedInbounds')} required={!isEdit}>
            <Select
              mode="multiple"
              value={form.inboundIds}
              onChange={(v) => update('inboundIds', v)}
              options={inboundOptions}
              placeholder={t('pages.clients.selectInbound')}
              maxTagCount="responsive"
              placement="topLeft"
              listHeight={220}
              showSearch={{
                filterOption: (input, option) => ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase()),
              }}
            />
          </Form.Item>

          <Form.Item>
            <Switch checked={form.enable} onChange={(v) => update('enable', v)} />
            <span style={{ marginLeft: 8 }}>{t('enable')}</span>
          </Form.Item>

          {isEdit && ipLimitEnable && (
            <Form.Item label={t('pages.clients.ipLog')}>
              <Button icon={<EyeOutlined />} loading={ipsLoading} onClick={openIpsModal}>
                {clientIps.length > 0 ? clientIps.length : ''}
              </Button>
            </Form.Item>
          )}
        </Form>
      </Modal>

      <Modal
        open={ipsModalOpen}
        title={`${t('pages.clients.ipLog')}${client?.email ? ` — ${client.email}` : ''}`}
        width={440}
        onCancel={() => setIpsModalOpen(false)}
        footer={[
          <Button key="refresh" icon={<ReloadOutlined />} loading={ipsLoading} onClick={loadIps}>
            {t('refresh')}
          </Button>,
          <Button key="clear" danger loading={ipsClearing} disabled={clientIps.length === 0} onClick={clearIps}>
            {t('pages.clients.clearAll')}
          </Button>,
          <Button key="close" type="primary" onClick={() => setIpsModalOpen(false)}>
            {t('close')}
          </Button>,
        ]}
      >
        {clientIps.length > 0 ? (
          <div style={{ maxHeight: 360, overflowY: 'auto' }}>
            {clientIps.map((ip, idx) => (
              <Tag
                key={idx}
                color="blue"
                style={{
                  display: 'block',
                  width: 'fit-content',
                  maxWidth: '100%',
                  marginBottom: 6,
                  padding: '2px 8px',
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                }}
              >
                {ip}
              </Tag>
            ))}
          </div>
        ) : (
          <Tag>{t('tgbot.noIpRecord')}</Tag>
        )}
      </Modal>
    </>
  );
}
