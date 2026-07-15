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
import { FormProvider, useForm, useWatch, useFieldArray } from 'react-hook-form';

import { HttpUtil, RandomUtil, Wireguard } from '@/utils';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { generateMtprotoSecret } from '@/lib/xray/inbound-defaults';
import { normalizeClientIps, type ClientIpInfo } from '@/lib/clients/ip-log';
import { DateTimePicker, SelectAllClearButtons } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { TLS_FLOW_CONTROL } from '@/schemas/primitives';
import type { ClientRecord, InboundOption, ExternalLink, ExternalLinkInput } from '@/hooks/useClients';
import { useFail2banStatusQuery, getLimitIpNotice } from '@/api/queries/useFail2banStatusQuery';
import { ClientFormSchema, ClientCreateFormSchema, type ClientFormValues } from '@/schemas/client';

const FLOW_OPTIONS = Object.values(TLS_FLOW_CONTROL);
const VMESS_SECURITY_OPTIONS = ['auto', 'aes-128-gcm', 'chacha20-poly1305'] as const;

const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks', 'vless', 'vmess', 'trojan', 'hysteria', 'wireguard', 'mtproto',
]);

const CLIENT_FORM_MODAL_Z_INDEX = 1000;
const CLIENT_IP_LOG_MODAL_Z_INDEX = CLIENT_FORM_MODAL_Z_INDEX + 1;

interface ExternalLinkRow {
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

type Values = ClientFormValues & {
  expiryDate: number;
  externalLinks: ExternalLinkRow[];
  wgPrivateKey: string;
  wgPublicKey: string;
  wgPreSharedKey: string;
  wgAllowedIPs: string;
  secret: string;
  adTag: string;
};

const EMPTY: Values = {
  email: '',
  subId: '',
  uuid: '',
  password: '',
  auth: '',
  flow: '',
  security: 'auto',
  reverseTag: '',
  totalGB: 0,
  expiryDate: 0,
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
  wgPrivateKey: '',
  wgPublicKey: '',
  wgPreSharedKey: '',
  wgAllowedIPs: '',
  secret: '',
  adTag: '',
};

function toExternalLinkRows(links: ExternalLink[] | undefined): ExternalLinkRow[] {
  return (links || []).map((l) => ({
    kind: l.kind === 'subscription' ? 'subscription' : 'link',
    value: l.value || '',
  }));
}

function bytesToGB(bytes: number): number {
  if (!bytes || bytes <= 0) return 0;
  return Math.round((bytes / (1024 * 1024 * 1024)) * 100) / 100;
}

export function gbToBytes(gb: number): number {
  if (!gb || gb <= 0) return 0;
  return Math.round(gb * 1024 * 1024 * 1024);
}

export function resolveTotalBytes(originalBytes: number | null | undefined, displayedGB: number): number {
  if (originalBytes != null && displayedGB === bytesToGB(originalBytes)) {
    return originalBytes;
  }
  return gbToBytes(displayedGB);
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

  const methods = useForm<Values>({ defaultValues: EMPTY });
  const inboundIds = useWatch({ control: methods.control, name: 'inboundIds' });
  const delayedStart = useWatch({ control: methods.control, name: 'delayedStart' });
  const expiryDate = useWatch({ control: methods.control, name: 'expiryDate' });
  const enable = useWatch({ control: methods.control, name: 'enable' });
  const flow = useWatch({ control: methods.control, name: 'flow' });
  const reverseTag = useWatch({ control: methods.control, name: 'reverseTag' });
  const secret = useWatch({ control: methods.control, name: 'secret' });
  const email = useWatch({ control: methods.control, name: 'email' });
  const uuid = useWatch({ control: methods.control, name: 'uuid' });
  const password = useWatch({ control: methods.control, name: 'password' });
  const subId = useWatch({ control: methods.control, name: 'subId' });
  const auth = useWatch({ control: methods.control, name: 'auth' });
  const wgPrivateKey = useWatch({ control: methods.control, name: 'wgPrivateKey' });
  const limitIp = useWatch({ control: methods.control, name: 'limitIp' });
  const {
    fields: externalLinkFields,
    append: appendExternalLink,
    remove: removeExternalLink,
  } = useFieldArray({ control: methods.control, name: 'externalLinks' });

  const [submitting, setSubmitting] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [clientIps, setClientIps] = useState<ClientIpInfo[]>([]);
  const [ipsLoading, setIpsLoading] = useState(false);
  const [ipsClearing, setIpsClearing] = useState(false);
  const [ipsModalOpen, setIpsModalOpen] = useState(false);
  const fail2ban = useFail2banStatusQuery();
  const limitIpDisabled = !fail2ban.usable;
  const limitIpNotice = getLimitIpNotice(fail2ban, t);

  function addExternalLinkRow(kind: 'link' | 'subscription') {
    appendExternalLink({ kind, value: '' });
  }

  useEffect(() => {
    if (!open) return;
    setIpsModalOpen(false);

    if (isEdit && client) {
      const et = Number(client.expiryTime) || 0;
      const seed: Values = {
        ...EMPTY,
        email: client.email || '',
        subId: client.subId || '',
        uuid: client.uuid || '',
        password: client.password || '',
        auth: client.auth || '',
        flow: client.flow || '',
        security: !client.security || client.security === 'none' || client.security === 'zero'
          ? 'auto'
          : client.security,
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
        wgPrivateKey: client.privateKey || '',
        wgPublicKey: client.publicKey || '',
        wgPreSharedKey: client.preSharedKey || '',
        wgAllowedIPs: client.allowedIPs || '',
        secret: client.secret || '',
        adTag: client.adTag || '',
      };
      if (et < 0) {
        seed.delayedStart = true;
        seed.delayedDays = Math.round(et / -86400000);
        seed.expiryDate = 0;
      } else {
        seed.delayedStart = false;
        seed.delayedDays = 0;
        seed.expiryDate = et > 0 ? et : 0;
      }
      methods.reset(seed);
      void loadIps();
    } else {
      const wgKeypair = Wireguard.generateKeypair();
      methods.reset({
        ...EMPTY,
        email: RandomUtil.randomLowerAndNum(10),
        uuid: RandomUtil.randomUUID(),
        subId: RandomUtil.randomLowerAndNum(16),
        password: RandomUtil.randomLowerAndNum(16),
        auth: RandomUtil.randomLowerAndNum(16),
        wgPrivateKey: wgKeypair.privateKey,
        wgPublicKey: wgKeypair.publicKey,
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

  const wireguardIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row && row.protocol === 'wireguard') ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const mtprotoIds = useMemo(() => {
    const ids = new Set<number>();
    for (const row of inbounds || []) {
      if (row && row.protocol === 'mtproto') ids.add(row.id);
    }
    return ids;
  }, [inbounds]);

  const mtprotoDomain = useMemo(() => {
    for (const id of inboundIds || []) {
      const ib = (inbounds || []).find((row) => row.id === id);
      if (ib?.protocol === 'mtproto' && ib.mtprotoDomain) return ib.mtprotoDomain;
    }
    return 'www.cloudflare.com';
  }, [inboundIds, inbounds]);

  const ss2022Method = useMemo(() => {
    for (const id of inboundIds || []) {
      const ib = (inbounds || []).find((row) => row.id === id);
      const method = ib?.ssMethod;
      if (method && method.substring(0, 4) === '2022') return method;
    }
    return '';
  }, [inboundIds, inbounds]);

  function regeneratePassword() {
    methods.setValue('password', ss2022Method
      ? RandomUtil.randomShadowsocksPassword(ss2022Method)
      : RandomUtil.randomLowerAndNum(16));
  }

  const showFlow = useMemo(
    () => (inboundIds || []).some((id) => flowCapableIds.has(id)),
    [inboundIds, flowCapableIds],
  );

  const showReverseTag = useMemo(
    () => (inboundIds || []).some((id) => vlessLikeIds.has(id)),
    [inboundIds, vlessLikeIds],
  );

  const showSecurity = useMemo(
    () => (inboundIds || []).some((id) => vmessIds.has(id)),
    [inboundIds, vmessIds],
  );

  const showWireguard = useMemo(
    () => (inboundIds || []).some((id) => wireguardIds.has(id)),
    [inboundIds, wireguardIds],
  );

  const showMtproto = useMemo(
    () => (inboundIds || []).some((id) => mtprotoIds.has(id)),
    [inboundIds, mtprotoIds],
  );

  function regenerateWireguardKeys() {
    const kp = Wireguard.generateKeypair();
    methods.setValue('wgPrivateKey', kp.privateKey);
    methods.setValue('wgPublicKey', kp.publicKey);
  }

  function regenerateMtprotoSecret() {
    methods.setValue('secret', generateMtprotoSecret(mtprotoDomain));
  }

  useEffect(() => {
    if (!showFlow && flow) {
      methods.setValue('flow', '');
    }
  }, [showFlow, flow, methods]);

  useEffect(() => {
    if (!showReverseTag && reverseTag) {
      methods.setValue('reverseTag', '');
    }
  }, [showReverseTag, reverseTag, methods]);

  useEffect(() => {
    if (!ss2022Method) return;
    const current = methods.getValues('password');
    if (!RandomUtil.isShadowsocks2022Password(current, ss2022Method)) {
      methods.setValue('password', RandomUtil.randomShadowsocksPassword(ss2022Method));
    }
  }, [ss2022Method, methods]);

  useEffect(() => {
    if (showMtproto && !secret) {
      methods.setValue('secret', generateMtprotoSecret(mtprotoDomain));
    }
  }, [showMtproto, secret, mtprotoDomain, methods]);

  const inboundOptions = useMemo(
    () => (inbounds || [])
      .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol || ''))
      .filter((ib) => ib.enable || (inboundIds || []).includes(ib.id))
      .map((ib) => ({
        label: formatInboundLabel(ib.tag, ib.remark),
        value: ib.id,
        title: formatInboundLabel(ib.tag, ib.remark),
      })),
    [inbounds, inboundIds],
  );

  const expiryDayjs = useMemo<Dayjs | null>(
    () => (expiryDate > 0 ? dayjs(expiryDate) : null),
    [expiryDate],
  );

  const linkRows = externalLinkFields
    .map((field, index) => ({ field, index }))
    .filter((row) => row.field.kind === 'link');
  const subscriptionRows = externalLinkFields
    .map((field, index) => ({ field, index }))
    .filter((row) => row.field.kind === 'subscription');

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
    const values = methods.getValues();
    const schema = isEdit ? ClientFormSchema : ClientCreateFormSchema;
    const validated = schema.safeParse({
      email: values.email,
      subId: values.subId,
      uuid: values.uuid,
      password: values.password,
      auth: values.auth,
      flow: values.flow,
      security: values.security,
      reverseTag: values.reverseTag,
      totalGB: values.totalGB,
      delayedStart: values.delayedStart,
      delayedDays: values.delayedDays,
      reset: values.reset,
      limitIp: values.limitIp,
      tgId: values.tgId,
      group: values.group,
      comment: values.comment,
      enable: values.enable,
      inboundIds: values.inboundIds,
    });
    if (!validated.success) {
      const issue = validated.error.issues[0];
      messageApi.error(t(issue?.message ?? 'somethingWentWrong'));
      return;
    }
    const expiryTime = values.delayedStart
      ? -86400000 * (Number(values.delayedDays) || 0)
      : (values.expiryDate || 0);
    const totalBytes = resolveTotalBytes(client ? (client.totalGB ?? 0) : null, values.totalGB);
    const clientPayload: Record<string, unknown> = {
      email: values.email.trim(),
      subId: values.subId,
      id: values.uuid,
      password: values.password,
      auth: values.auth,
      flow: showFlow ? (values.flow || '') : '',
      security: showSecurity ? (values.security || 'auto') : 'auto',
      totalGB: totalBytes,
      expiryTime,
      reset: Number(values.reset) || 0,
      limitIp: Number(values.limitIp) || 0,
      tgId: Number(values.tgId) || 0,
      group: values.group,
      comment: values.comment,
      enable: !!values.enable,
    };
    const reverseTagValue = showReverseTag ? (values.reverseTag || '').trim() : '';
    if (reverseTagValue) {
      clientPayload.reverse = { tag: reverseTagValue };
    }

    if (showWireguard) {
      clientPayload.privateKey = values.wgPrivateKey;
      clientPayload.publicKey = values.wgPublicKey;
      if (values.wgPreSharedKey) {
        clientPayload.preSharedKey = values.wgPreSharedKey;
      }
      const allowedIPs = values.wgAllowedIPs
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s !== '');
      if (allowedIPs.length > 0) {
        clientPayload.allowedIPs = allowedIPs;
      }
    }

    if (showMtproto) {
      const adTag = values.adTag.trim();
      if (adTag !== '' && !/^[0-9a-fA-F]{32}$/.test(adTag)) {
        messageApi.error(t('pages.inbounds.form.mtgAdTagInvalid'));
        return;
      }
      clientPayload.secret = values.secret;
      clientPayload.adTag = adTag;
    }

    const externalLinks: ExternalLinkInput[] = values.externalLinks
      .map((r) => ({ kind: r.kind, value: r.value.trim(), remark: '' }))
      .filter((r) => r.value !== '');

    setSubmitting(true);
    try {
      let msg;
      if (isEdit && client) {
        const original = new Set(attachedIds || []);
        const next = new Set(values.inboundIds || []);
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
          { client: clientPayload, inboundIds: values.inboundIds },
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
        <FormProvider {...methods}>
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
                                value={email}
                                placeholder={t('pages.clients.email')}
                                style={{ flex: 1 }}
                                onChange={(e) => methods.setValue('email', e.target.value)}
                              />
                              {!isEdit && (
                                <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={() => methods.setValue('email', RandomUtil.randomLowerAndNum(12))} />
                              )}
                            </Space.Compact>
                          </Form.Item>
                        </Col>
                        <Col xs={24} md={6}>
                          <FormField
                            name="totalGB"
                            label={t('pages.clients.totalGB')}
                            tooltip={t('pages.clients.totalGBDesc')}
                            transform={{ output: (v) => Number(v) || 0 }}
                          >
                            <InputNumber min={0} step={1} style={{ width: '100%' }} />
                          </FormField>
                        </Col>
                        <Col xs={24} md={6}>
                          <Form.Item label={t('pages.clients.limitIp')} tooltip={t('pages.clients.limitIpDesc')}>
                            <Tooltip title={limitIpNotice || undefined}>
                              <span style={{ display: 'flex', width: '100%' }}>
                                <Space.Compact style={{ display: 'flex', flex: 1 }}>
                                  <InputNumber value={limitIp} min={0} disabled={limitIpDisabled}
                                    style={{ flex: 1, ...(limitIpDisabled ? { pointerEvents: 'none' } : null) }}
                                    onChange={(v) => methods.setValue('limitIp', Number(v) || 0)} />
                                  {isEdit && (
                                    <Tooltip title={t('pages.clients.ipLog')}>
                                      <Button aria-label={t('pages.clients.ipLog')} icon={<EyeOutlined />} loading={ipsLoading} onClick={openIpsModal}>
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
                          {delayedStart ? (
                            <FormField
                              name="delayedDays"
                              label={t('pages.clients.expireDays')}
                              transform={{ output: (v) => Number(v) || 0 }}
                            >
                              <InputNumber min={0} style={{ width: '100%' }} />
                            </FormField>
                          ) : (
                            <Form.Item label={t('pages.clients.expiryTime')}>
                              <DateTimePicker
                                value={expiryDayjs}
                                onChange={(d) => methods.setValue('expiryDate', d ? d.valueOf() : 0)}
                              />
                            </Form.Item>
                          )}
                        </Col>
                        <Col xs={12} md={6}>
                          <Form.Item label={t('pages.clients.delayedStart')}>
                            <Switch
                              checked={delayedStart}
                              onChange={(v) => {
                                methods.setValue('delayedStart', v);
                                if (v) methods.setValue('expiryDate', 0);
                                else methods.setValue('delayedDays', 0);
                              }}
                            />
                          </Form.Item>
                        </Col>
                        <Col xs={12} md={6}>
                          <FormField
                            name="reset"
                            label={t('pages.clients.renewDays')}
                            tooltip={t('pages.clients.renewDesc')}
                            transform={{ output: (v) => Number(v) || 0 }}
                          >
                            <InputNumber min={0} style={{ width: '100%' }} />
                          </FormField>
                        </Col>
                      </Row>

                      <Row gutter={16}>
                        <Col xs={24} md={12}>
                          <FormField name="comment" label={t('pages.clients.comment')}>
                            <Input />
                          </FormField>
                        </Col>
                        <Col xs={24} md={12}>
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
                        </Col>
                      </Row>

                      {(tgBotEnable || showReverseTag) && (
                        <Row gutter={16}>
                          {tgBotEnable && (
                            <Col xs={24} md={12}>
                              <FormField
                                name="tgId"
                                label={t('pages.clients.telegramId')}
                                transform={{ output: (v) => Number(v) || 0 }}
                              >
                                <InputNumber min={0} controls={false}
                                  placeholder={t('pages.clients.telegramIdPlaceholder')} style={{ width: '100%' }} />
                              </FormField>
                            </Col>
                          )}
                          {showReverseTag && (
                            <Col xs={24} md={12}>
                              <FormField name="reverseTag" label={t('pages.clients.reverseTag')}>
                                <Input placeholder={t('pages.clients.reverseTagPlaceholder')} />
                              </FormField>
                            </Col>
                          )}
                        </Row>
                      )}

                      <Form.Item label={t('pages.clients.attachedInbounds')} required={!isEdit}>
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
                          maxTagCount="responsive"
                          placement="topLeft"
                          listHeight={220}
                          showSearch={{
                            filterOption: (input, option) => ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase()),
                          }}
                        />
                      </Form.Item>

                      <Form.Item>
                        <Switch aria-label={t('enable')} checked={enable} onChange={(v) => methods.setValue('enable', v)} />
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
                          <Input value={uuid} style={{ flex: 1 }} onChange={(e) => methods.setValue('uuid', e.target.value)} />
                          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={() => methods.setValue('uuid', RandomUtil.randomUUID())} />
                        </Space.Compact>
                      </Form.Item>

                      <Form.Item label={t('pages.clients.password')} tooltip={t('pages.clients.passwordDesc')}>
                        <Space.Compact style={{ display: 'flex' }}>
                          <Input value={password} style={{ flex: 1 }} onChange={(e) => methods.setValue('password', e.target.value)} />
                          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regeneratePassword} />
                        </Space.Compact>
                      </Form.Item>

                      <Form.Item label={t('pages.clients.subId')}>
                        <Space.Compact style={{ display: 'flex' }}>
                          <Input value={subId} style={{ flex: 1 }} onChange={(e) => methods.setValue('subId', e.target.value)} />
                          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={() => methods.setValue('subId', RandomUtil.randomLowerAndNum(16))} />
                        </Space.Compact>
                      </Form.Item>

                      <Form.Item label={t('pages.clients.hysteriaAuth')} tooltip={t('pages.clients.hysteriaAuthDesc')}>
                        <Space.Compact style={{ display: 'flex' }}>
                          <Input value={auth} style={{ flex: 1 }} onChange={(e) => methods.setValue('auth', e.target.value)} />
                          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={() => methods.setValue('auth', RandomUtil.randomLowerAndNum(16))} />
                        </Space.Compact>
                      </Form.Item>

                      {showFlow && (
                        <FormField name="flow" label={t('pages.clients.flow')}>
                          <Select
                            options={[
                              { value: '', label: t('none') },
                              ...FLOW_OPTIONS.map((k) => ({ value: k, label: k })),
                            ]}
                          />
                        </FormField>
                      )}
                      {showSecurity && (
                        <FormField name="security" label={t('pages.clients.vmessSecurity')}>
                          <Select
                            options={VMESS_SECURITY_OPTIONS.map((k) => ({ value: k, label: k }))}
                          />
                        </FormField>
                      )}
                      {showWireguard && (
                        <>
                          <Form.Item label={t('pages.clients.wireguardPrivateKey')}>
                            <Space.Compact style={{ display: 'flex' }}>
                              <Input
                                value={wgPrivateKey}
                                style={{ flex: 1 }}
                                onChange={(e) => {
                                  const priv = e.target.value;
                                  methods.setValue('wgPrivateKey', priv);
                                  methods.setValue('wgPublicKey', priv ? Wireguard.generateKeypair(priv).publicKey : '');
                                }}
                              />
                              <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenerateWireguardKeys} />
                            </Space.Compact>
                          </Form.Item>
                          <FormField name="wgPublicKey" label={t('pages.clients.wireguardPublicKey')}>
                            <Input disabled />
                          </FormField>
                          <FormField name="wgPreSharedKey" label={t('pages.clients.wireguardPreSharedKey')}>
                            <Input />
                          </FormField>
                          <FormField
                            name="wgAllowedIPs"
                            label={t('pages.clients.wireguardAllowedIPs')}
                            extra={t('pages.clients.wireguardAllowedIPsHint')}
                          >
                            <Input placeholder="10.0.0.2/32" />
                          </FormField>
                        </>
                      )}
                      {showMtproto && (
                        <>
                          <Form.Item label={t('pages.clients.mtprotoSecret')} extra={t('pages.clients.mtprotoSecretHint')}>
                            <Space.Compact style={{ display: 'flex' }}>
                              <Input value={secret} style={{ flex: 1 }} onChange={(e) => methods.setValue('secret', e.target.value)} />
                              <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenerateMtprotoSecret} />
                            </Space.Compact>
                          </Form.Item>
                          <FormField
                            name="adTag"
                            label={t('pages.clients.mtprotoAdTag')}
                            extra={t('pages.clients.mtprotoAdTagHint')}
                          >
                            <Input
                              allowClear
                              placeholder="0123456789abcdef0123456789abcdef"
                            />
                          </FormField>
                        </>
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
                        ) : linkRows.map(({ field, index }) => (
                          <div key={field.id} style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                            <FormField name={`externalLinks.${index}.value`} noStyle>
                              <Input
                                style={{ flex: 1 }}
                                aria-label="vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://"
                                placeholder="vless:// · vmess:// · trojan:// · ss:// · hysteria2:// · wireguard://"
                              />
                            </FormField>
                            <Tooltip title={t('delete')}>
                              <Button aria-label={t('delete')} danger icon={<DeleteOutlined />} onClick={() => removeExternalLink(index)} />
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
                        ) : subscriptionRows.map(({ field, index }) => (
                          <div key={field.id} style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                            <FormField name={`externalLinks.${index}.value`} noStyle>
                              <Input
                                style={{ flex: 1 }}
                                aria-label="https://provider.example/sub/…"
                                placeholder="https://provider.example/sub/…"
                              />
                            </FormField>
                            <Tooltip title={t('delete')}>
                              <Button aria-label={t('delete')} danger icon={<DeleteOutlined />} onClick={() => removeExternalLink(index)} />
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
        </FormProvider>
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
