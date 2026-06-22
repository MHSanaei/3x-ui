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
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { DeleteOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, RetweetOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { HttpUtil, RandomUtil } from '@/utils';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { normalizeClientIps, type ClientIpInfo } from '@/lib/clients/ip-log';
import { DateTimePicker, SelectAllClearButtons } from '@/components/form';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives';
import type { ClientRecord, InboundOption, ExternalLink, ExternalLinkInput } from '@/hooks/useClients';
import { useFail2banStatusQuery, getLimitIpNotice } from '@/api/queries/useFail2banStatusQuery';
import { ClientFormSchema, ClientCreateFormSchema } from '@/schemas/client';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);
const VMESS_SECURITY_OPTIONS = ['auto', 'aes-128-gcm', 'chacha20-poly1305', 'none', 'zero'] as const;

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria',
]);

const CLIENT_FORM_MODAL_Z_INDEX = 1000;
const CLIENT_IP_LOG_MODAL_Z_INDEX = CLIENT_FORM_MODAL_Z_INDEX + 1;

// One editable row in the Links tab. `key` is a stable client-side id for React.
interface ExternalLinkRow {
  key: number;
  kind: 'link' | 'subscription';
  value: string;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  msg?: string;
  obj?: T;
}

type Mode = 'add' | 'edit';

interface SaveMetaEdit {
  isEdit: true;
  email: string;
  attach: number[];
  detach: number[];
  externalLinks: ExternalLinkInput[];
}

interface SaveMetaCreate {
  isEdit: false;
  email: string;
  externalLinks: ExternalLinkInput[];
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
  attachedExternalLinks?: ExternalLink[];
  attachedIds?: number[];
  tgBotEnable?: boolean;
  groups?: string[];
  save: (
    payload: Record<string, unknown> | SaveCreatePayload,
    meta: SaveMetaEdit | SaveMetaCreate,
  ) => Promise<ApiMsg | null>;
  resetTraffic?: (client: ClientRecord) => Promise<ApiMsg | null>;
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
  externalLinks: ExternalLinkRow[];
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
    externalLinks: [],
  };
}

let externalLinkRowSeq = 0;
function toExternalLinkRows(links: ExternalLink[] | undefined): ExternalLinkRow[] {
  return (links || []).map((l) => ({
    key: (externalLinkRowSeq += 1),
    kind: l.kind === 'subscription' ? 'subscription' : 'link',
    value: l.value || '',
  }));
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
  attachedExternalLinks = [],
  attachedIds = [],
  tgBotEnable = false,
  groups = [],
  save,
  resetTraffic,
  onOpenChange,
}: ClientFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const isEdit = mode === 'edit';

  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [clientIps, setClientIps] = useState<ClientIpInfo[]>([]);
  const [ipsLoading, setIpsLoading] = useState(false);
  const [ipsClearing, setIpsClearing] = useState(false);
  const [ipsModalOpen, setIpsModalOpen] = useState(false);
  const fail2ban = useFail2banStatusQuery();
  const limitIpDisabled = !fail2ban.usable;
  const limitIpNotice = getLimitIpNotice(fail2ban, t);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function addExternalLinkRow(kind: 'link' | 'subscription') {
    setForm((prev) => ({
      ...prev,
      externalLinks: [...prev.externalLinks, { key: (externalLinkRowSeq += 1), kind, value: '' }],
    }));
  }

  function updateExternalLinkRow(key: number, value: string) {
    setForm((prev) => ({
      ...prev,
      externalLinks: prev.externalLinks.map((r) => (r.key === key ? { ...r, value } : r)),
    }));
  }

  function removeExternalLinkRow(key: number) {
    setForm((prev) => ({
      ...prev,
      externalLinks: prev.externalLinks.filter((r) => r.key !== key),
    }));
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
        externalLinks: toExternalLinkRows(attachedExternalLinks),
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

  const ss2022Method = useMemo(() => {
    for (const id of form.inboundIds || []) {
      const ib = (inbounds || []).find((row) => row.id === id);
      const method = ib?.ssMethod;
      if (method && method.substring(0, 4) === '2022') return method;
    }
    return '';
  }, [form.inboundIds, inbounds]);

  function regeneratePassword() {
    update('password', ss2022Method
      ? RandomUtil.randomShadowsocksPassword(ss2022Method)
      : RandomUtil.randomLowerAndNum(16));
  }

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

  useEffect(() => {
    if (!ss2022Method) return;
    setForm((prev) => (
      RandomUtil.isShadowsocks2022Password(prev.password, ss2022Method)
        ? prev
        : { ...prev, password: RandomUtil.randomShadowsocksPassword(ss2022Method) }
    ));
  }, [ss2022Method]);

  const inboundOptions = useMemo(
    () => (inbounds || [])
      .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol || ''))
      .map((ib) => ({
        label: formatInboundLabel(ib.tag, ib.remark),
        value: ib.id,
        title: formatInboundLabel(ib.tag, ib.remark),
      })),
    [inbounds],
  );

  const linkRows = useMemo(() => form.externalLinks.filter((r) => r.kind === 'link'), [form.externalLinks]);
  const subscriptionRows = useMemo(() => form.externalLinks.filter((r) => r.kind === 'subscription'), [form.externalLinks]);

  async function loadIps() {
    if (!isEdit || !client?.email) return;
    setIpsLoading(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/clients/ips/${encodeURIComponent(client.email)}`) as ApiMsg<unknown[]>;
      if (!msg?.success) { setClientIps([]); return; }
      setClientIps(normalizeClientIps(msg.obj));
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

  async function onResetTraffic() {
    if (!isEdit || !client?.email || !resetTraffic) return;
    setResetting(true);
    try {
      const msg = await resetTraffic(client);
      if (msg?.success) {
        messageApi.success(t('pages.clients.toasts.trafficReset'));
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } finally {
      setResetting(false);
    }
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

    const externalLinks: ExternalLinkInput[] = form.externalLinks
      .map((r) => ({ kind: r.kind, value: r.value.trim(), remark: '' }))
      .filter((r) => r.value !== '');

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
          externalLinks,
        });
      } else {
        msg = await save(
          { client: clientPayload, inboundIds: form.inboundIds },
          { isEdit: false, email: clientPayload.email as string, externalLinks },
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
        width={720}
        zIndex={CLIENT_FORM_MODAL_Z_INDEX}
        style={{ top: 20 }}
        styles={{ body: { maxHeight: 'calc(100vh - 160px)', overflowY: 'auto', overflowX: 'hidden' } }}
        onCancel={close}
        footer={
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {isEdit && resetTraffic && (
              <Popconfirm
                title={t('pages.inbounds.resetTraffic')}
                description={t('pages.inbounds.resetTrafficContent')}
                okText={t('reset')}
                cancelText={t('cancel')}
                zIndex={CLIENT_IP_LOG_MODAL_Z_INDEX}
                onConfirm={onResetTraffic}
              >
                <Button color="danger" variant="filled" icon={<RetweetOutlined />} loading={resetting}>
                  {t('pages.inbounds.resetTraffic')}
                </Button>
              </Popconfirm>
            )}
            <div style={{ marginInlineStart: 'auto', display: 'flex', gap: 8 }}>
              <Button onClick={close}>{t('cancel')}</Button>
              <Button type="primary" loading={submitting} onClick={onSubmit}>
                {isEdit ? t('save') : t('create')}
              </Button>
            </div>
          </div>
        }
      >
        <Form layout="vertical">
          <Tabs
            defaultActiveKey="basic"
            items={[
              {
                key: 'basic',
                label: t('pages.clients.tabBasics'),
                children: (
                  <>
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
                            {!isEdit && (
                              <Button icon={<ReloadOutlined />} onClick={() => update('email', RandomUtil.randomLowerAndNum(12))} />
                            )}
                          </Space.Compact>
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={6}>
                        <Form.Item label={t('pages.clients.totalGB')} tooltip={t('pages.clients.totalGBDesc')}>
                          <InputNumber value={form.totalGB} min={0} step={1} style={{ width: '100%' }}
                            onChange={(v) => update('totalGB', Number(v) || 0)} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={6}>
                        <Form.Item label={t('pages.clients.limitIp')} tooltip={t('pages.clients.limitIpDesc')}>
                          <Tooltip title={limitIpNotice || undefined}>
                            <span style={{ display: 'flex', width: '100%' }}>
                              <Space.Compact style={{ display: 'flex', flex: 1 }}>
                                <InputNumber value={form.limitIp} min={0} disabled={limitIpDisabled}
                                  style={{ flex: 1, ...(limitIpDisabled ? { pointerEvents: 'none' } : null) }}
                                  onChange={(v) => update('limitIp', Number(v) || 0)} />
                                {isEdit && (
                                  <Tooltip title={t('pages.clients.ipLog')}>
                                    <Button icon={<EyeOutlined />} loading={ipsLoading} onClick={openIpsModal}>
                                      {clientIps.length > 0 ? clientIps.length : ''}
                                    </Button>
                                  </Tooltip>
                                )}
                              </Space.Compact>
                            </span>
                          </Tooltip>
                        </Form.Item>
                      </Col>
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
                      <Col xs={12} md={6}>
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
                      <Col xs={12} md={6}>
                        <Form.Item
                          label={t('pages.clients.renewDays')}
                          tooltip={t('pages.clients.renewDesc')}
                        >
                          <InputNumber value={form.reset} min={0} style={{ width: '100%' }}
                            onChange={(v) => update('reset', Number(v) || 0)} />
                        </Form.Item>
                      </Col>
                    </Row>

                    <Row gutter={16}>
                      <Col xs={24} md={12}>
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
                            allowClear
                          />
                        </Form.Item>
                      </Col>
                    </Row>

                    {(tgBotEnable || showReverseTag) && (
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
                        {showReverseTag && (
                          <Col xs={24} md={12}>
                            <Form.Item label={t('pages.clients.reverseTag')}>
                              <Input value={form.reverseTag} placeholder={t('pages.clients.reverseTagPlaceholder')}
                                onChange={(e) => update('reverseTag', e.target.value)} />
                            </Form.Item>
                          </Col>
                        )}
                      </Row>
                    )}

                    <Form.Item label={t('pages.clients.attachedInbounds')} required={!isEdit}>
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
                  </>
                ),
              },
              {
                key: 'config',
                label: t('pages.clients.tabCredentials'),
                children: (
                  <>
                    <Form.Item label={t('pages.clients.uuid')}>
                      <Space.Compact style={{ display: 'flex' }}>
                        <Input value={form.uuid} style={{ flex: 1 }} onChange={(e) => update('uuid', e.target.value)} />
                        <Button icon={<ReloadOutlined />} onClick={() => update('uuid', RandomUtil.randomUUID())} />
                      </Space.Compact>
                    </Form.Item>

                    <Form.Item label={t('pages.clients.password')}>
                      <Space.Compact style={{ display: 'flex' }}>
                        <Input value={form.password} style={{ flex: 1 }} onChange={(e) => update('password', e.target.value)} />
                        <Button icon={<ReloadOutlined />} onClick={regeneratePassword} />
                      </Space.Compact>
                    </Form.Item>

                    <Form.Item label={t('pages.clients.subId')}>
                      <Space.Compact style={{ display: 'flex' }}>
                        <Input value={form.subId} style={{ flex: 1 }} onChange={(e) => update('subId', e.target.value)} />
                        <Button icon={<ReloadOutlined />} onClick={() => update('subId', RandomUtil.randomLowerAndNum(16))} />
                      </Space.Compact>
                    </Form.Item>

                    <Form.Item label={t('pages.clients.hysteriaAuth')}>
                      <Space.Compact style={{ display: 'flex' }}>
                        <Input value={form.auth} style={{ flex: 1 }} onChange={(e) => update('auth', e.target.value)} />
                        <Button icon={<ReloadOutlined />} onClick={() => update('auth', RandomUtil.randomLowerAndNum(16))} />
                      </Space.Compact>
                    </Form.Item>

                    {showFlow && (
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
                    )}
                    {showSecurity && (
                      <Form.Item label={t('pages.clients.vmessSecurity')}>
                        <Select
                          value={form.security}
                          onChange={(v) => update('security', v)}
                          options={VMESS_SECURITY_OPTIONS.map((k) => ({ value: k, label: k }))}
                        />
                      </Form.Item>
                    )}
                  </>
                ),
              },
              {
                key: 'links',
                label: t('pages.clients.tabLinks'),
                children: (
                  <>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 4 }}>
                      {t('pages.clients.linksHint')}
                    </Typography.Paragraph>

                    <Button type="primary" icon={<PlusOutlined />} onClick={() => addExternalLinkRow('link')}>
                      {t('pages.clients.addExternalLink')}
                    </Button>
                    <div style={{ marginTop: 12, marginBottom: 24 }}>
                      {linkRows.length === 0 ? (
                        <Typography.Text type="secondary">{t('pages.clients.noExternalLinks')}</Typography.Text>
                      ) : linkRows.map((row) => (
                        <div key={row.key} style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                          <Input
                            value={row.value}
                            onChange={(e) => updateExternalLinkRow(row.key, e.target.value)}
                            placeholder="vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://"
                          />
                          <Tooltip title={t('delete')}>
                            <Button danger icon={<DeleteOutlined />} onClick={() => removeExternalLinkRow(row.key)} />
                          </Tooltip>
                        </div>
                      ))}
                    </div>

                    <Button type="primary" icon={<PlusOutlined />} onClick={() => addExternalLinkRow('subscription')}>
                      {t('pages.clients.addExternalSubscription')}
                    </Button>
                    <div style={{ marginTop: 12 }}>
                      {subscriptionRows.length === 0 ? (
                        <Typography.Text type="secondary">{t('pages.clients.noExternalSubscriptions')}</Typography.Text>
                      ) : subscriptionRows.map((row) => (
                        <div key={row.key} style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                          <Input
                            value={row.value}
                            onChange={(e) => updateExternalLinkRow(row.key, e.target.value)}
                            placeholder="https://provider.example/sub/…"
                          />
                          <Tooltip title={t('delete')}>
                            <Button danger icon={<DeleteOutlined />} onClick={() => removeExternalLinkRow(row.key)} />
                          </Tooltip>
                        </div>
                      ))}
                    </div>
                  </>
                ),
              },
            ]}
          />
        </Form>
      </Modal>

      <Modal
        open={ipsModalOpen}
        title={`${t('pages.clients.ipLog')}${client?.email ? ` — ${client.email}` : ''}`}
        width={440}
        zIndex={CLIENT_IP_LOG_MODAL_Z_INDEX}
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
            {clientIps.map((entry, idx) => (
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
                {entry.ip}{entry.time ? ` (${entry.time})` : ''}
                {entry.node ? (
                  <span style={{ marginInlineStart: 6, opacity: 0.85, fontWeight: 600 }}>@ {entry.node}</span>
                ) : null}
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
