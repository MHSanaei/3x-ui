import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Switch,
  message,
} from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { RemoteInboundOption } from '@/api/queries/useNodeMutations';
import type { Msg } from '@/utils';
import { NodeFormSchema, type NodeFormValues, type ProbeResult } from '@/schemas/node';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { useOutboundTagGroups } from '@/api/queries/useOutboundTags';
import './NodeFormModal.css';

type Mode = 'add' | 'edit';

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  testConnection: (payload: Partial<NodeRecord>) => Promise<Msg<ProbeResult>>;
  fetchFingerprint: (payload: Partial<NodeRecord>) => Promise<Msg<string>>;
  fetchInbounds: (payload: Partial<NodeRecord>) => Promise<Msg<RemoteInboundOption[]>>;
  save: (payload: Partial<NodeRecord>) => Promise<Msg<unknown>>;
  onOpenChange: (open: boolean) => void;
}

function defaultValues(): NodeFormValues {
  return {
    id: 0,
    name: '',
    remark: '',
    scheme: 'https',
    address: '',
    port: 2053,
    basePath: '/',
    apiToken: '',
    enable: true,
    allowPrivateAddress: false,
    tlsVerifyMode: 'verify',
    pinnedCertSha256: '',
    inboundSyncMode: 'all',
    inboundTags: [],
    outboundTag: '',
  };
}

export default function NodeFormModal({
  open,
  mode,
  node,
  testConnection,
  fetchFingerprint,
  fetchInbounds,
  save,
  onOpenChange,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const methods = useForm<NodeFormValues>({ defaultValues: defaultValues() });
  const [messageApi, messageContextHolder] = message.useMessage();

  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [fetchingPin, setFetchingPin] = useState(false);
  const [fetchingInbounds, setFetchingInbounds] = useState(false);
  const [inboundOptions, setInboundOptions] = useState<RemoteInboundOption[]>([]);
  const [testResult, setTestResult] = useState<ProbeResult | null>(null);
  const scheme = useWatch({ control: methods.control, name: 'scheme' }) ?? 'https';
  const tlsVerifyMode = useWatch({ control: methods.control, name: 'tlsVerifyMode' }) ?? 'verify';
  const inboundSyncMode = useWatch({ control: methods.control, name: 'inboundSyncMode' }) ?? 'all';
  const { data: outboundGroups } = useOutboundTagGroups({ excludeBlackhole: true });

  // Outbounds and balancers share one picker (like the panel-outbound selector);
  // when balancers exist they get a labeled group so it's clear the selection
  // routes through a balancer. Empty falls back to the placeholder ("Direct
  // connection") rather than a synthetic option, so it can't read as a second
  // "direct" next to a real freedom outbound.
  const outboundOptions = useMemo<
    ({ label: string; value: string } | { label: string; options: { label: string; value: string }[] })[]
  >(() => {
    const outOpts = (outboundGroups?.outbounds ?? []).map((tag) => ({ label: tag, value: tag }));
    if (!outboundGroups?.balancers.length) return outOpts;
    return [
      { label: t('pages.xray.Outbounds'), options: outOpts },
      { label: t('pages.xray.Balancers'), options: outboundGroups.balancers.map((tag) => ({ label: tag, value: tag })) },
    ];
  }, [outboundGroups, t]);

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next: NodeFormValues = mode === 'edit' && node
      ? {
        ...base,
        ...(node as unknown as Partial<NodeFormValues>),
        id: node.id,
        scheme: (node.scheme as 'http' | 'https') || base.scheme,
        inboundSyncMode: (node.inboundSyncMode as 'all' | 'selected') || base.inboundSyncMode,
        inboundTags: node.inboundTags ?? [],
      }
      : base;
    if (next.scheme === 'http') next.tlsVerifyMode = 'skip';
    methods.reset(next);
    setInboundOptions((next.inboundTags || []).map((tag) => ({ tag })));
    setTestResult(null);
  }, [open, mode, node, methods]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  function buildPayload(values: NodeFormValues): Partial<NodeRecord> {
    return {
      id: values.id || 0,
      name: values.name.trim(),
      remark: values.remark?.trim() || '',
      scheme: values.scheme,
      address: values.address.trim(),
      port: values.port,
      basePath: values.basePath.trim() || '/',
      apiToken: values.apiToken.trim(),
      enable: values.enable,
      allowPrivateAddress: values.allowPrivateAddress,
      tlsVerifyMode: values.tlsVerifyMode,
      pinnedCertSha256: values.tlsVerifyMode === 'pin' ? values.pinnedCertSha256.trim() : '',
      inboundSyncMode: values.inboundSyncMode,
      inboundTags: values.inboundSyncMode === 'selected' ? values.inboundTags : [],
      outboundTag: values.outboundTag || '',
    };
  }

  async function onTest() {
    if (!(await methods.trigger(['address', 'port']))) return;
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload(methods.getValues());
      const msg = await testConnection(payload);
      if (msg?.success && msg.obj) {
        setTestResult(msg.obj);
      } else {
        setTestResult({ status: 'offline', error: msg?.msg || 'unknown error' });
      }
    } finally {
      setTesting(false);
    }
  }

  async function onFetchPin() {
    if (!(await methods.trigger(['address', 'port']))) return;
    setFetchingPin(true);
    try {
      const payload = buildPayload(methods.getValues());
      const msg = await fetchFingerprint(payload);
      if (msg?.success && msg.obj) {
        methods.setValue('pinnedCertSha256', msg.obj);
        messageApi.success(t('pages.nodes.pinFetched'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.pinFetchFailed'));
      }
    } finally {
      setFetchingPin(false);
    }
  }

  async function onFetchInbounds() {
    if (!(await methods.trigger(['name', 'address', 'port', 'apiToken']))) return;
    setFetchingInbounds(true);
    try {
      const msg = await fetchInbounds(buildPayload(methods.getValues()));
      if (msg?.success && Array.isArray(msg.obj)) {
        setInboundOptions(msg.obj);
        messageApi.success(t('pages.nodes.inboundsLoaded', { count: msg.obj.length }));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.inboundsLoadFailed'));
      }
    } finally {
      setFetchingInbounds(false);
    }
  }

  async function onFinish(values: NodeFormValues) {
    const result = NodeFormSchema.safeParse(values);
    if (!result.success) {
      messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
      return;
    }
    setSubmitting(true);
    try {
      const payload = buildPayload(result.data);
      const test = await testConnection(payload);
      const probe = test?.success ? test.obj : null;
      if (!probe || probe.status !== 'online') {
        setTestResult(probe ?? { status: 'offline', error: test?.msg || t('pages.nodes.connectionFailed') });
        return;
      }
      setTestResult(probe);
      const msg = await save(payload);
      if (msg?.success) {
        onOpenChange(false);
      }
    } finally {
      setSubmitting(false);
    }
  }

  function close() {
    if (!submitting) onOpenChange(false);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        confirmLoading={submitting}
        okText={t('save')}
        cancelText={t('cancel')}
        mask={{ closable: false }}
        width="640px"
        onOk={methods.handleSubmit(onFinish)}
        onCancel={close}
      >
        <FormProvider {...methods}>
          <Form layout="vertical">
            <Row gutter={16}>
              <Col xs={24} md={12}>
                <FormField
                  label={t('pages.nodes.name')}
                  name="name"
                  rules={{ validate: rhfZodValidate(NodeFormSchema.shape.name) }}
                >
                  <Input placeholder={t('pages.nodes.namePlaceholder')} />
                </FormField>
              </Col>
              <Col xs={24} md={12}>
                <FormField label={t('pages.nodes.remark')} name="remark">
                  <Input />
                </FormField>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col xs={24} md={6}>
                <FormField
                  label={t('pages.nodes.scheme')}
                  name="scheme"
                  onAfterChange={(value) => {
                    if (value === 'http') methods.setValue('tlsVerifyMode', 'skip');
                  }}
                >
                  <Select
                    options={[
                      { value: 'https', label: 'https' },
                      { value: 'http', label: 'http' },
                    ]}
                  />
                </FormField>
              </Col>
              <Col xs={24} md={12}>
                <FormField
                  label={t('pages.nodes.address')}
                  name="address"
                  rules={{ validate: rhfZodValidate(NodeFormSchema.shape.address) }}
                >
                  <Input placeholder={t('pages.nodes.addressPlaceholder')} />
                </FormField>
              </Col>
              <Col xs={24} md={6}>
                <FormField
                  label={t('pages.nodes.port')}
                  name="port"
                  rules={{ validate: rhfZodValidate(NodeFormSchema.shape.port) }}
                >
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                </FormField>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col xs={24} md={12}>
                <FormField label={t('pages.nodes.basePath')} name="basePath">
                  <Input placeholder="/" />
                </FormField>
              </Col>
              <Col xs={24} md={12}>
                <FormField
                  label={t('pages.nodes.enable')}
                  name="enable"
                  valueProp="checked"
                >
                  <Switch />
                </FormField>
              </Col>
            </Row>

            <FormField
              label={t('pages.nodes.allowPrivateAddress')}
              name="allowPrivateAddress"
              valueProp="checked"
              tooltip={t('pages.nodes.allowPrivateAddressHint')}
            >
              <Switch />
            </FormField>

            <FormField
              label={t('pages.nodes.tlsVerifyMode')}
              name="tlsVerifyMode"
              tooltip={t('pages.nodes.tlsVerifyModeHint')}
            >
              <Select
                disabled={scheme === 'http'}
                options={[
                  { value: 'verify', label: t('pages.nodes.tlsVerify') },
                  { value: 'pin', label: t('pages.nodes.tlsPin') },
                  { value: 'skip', label: t('pages.nodes.tlsSkip') },
                  { value: 'mtls', label: t('pages.nodes.tlsMtls') },
                ]}
              />
            </FormField>

            {tlsVerifyMode === 'skip' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 16 }}
                title={t('pages.nodes.tlsSkipWarning')}
              />
            )}

            {tlsVerifyMode === 'mtls' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                title={t('pages.nodes.mtlsFormHint')}
              />
            )}

            {tlsVerifyMode === 'pin' && (
              <FormField
                label={t('pages.nodes.pinnedCert')}
                name="pinnedCertSha256"
                tooltip={t('pages.nodes.pinnedCertHint')}
              >
                <Input.Search
                  placeholder={t('pages.nodes.pinnedCertPlaceholder')}
                  enterButton={t('pages.nodes.fetchPin')}
                  loading={fetchingPin}
                  onSearch={onFetchPin}
                />
              </FormField>
            )}

            <FormField
              label={t('pages.nodes.apiToken')}
              name="apiToken"
              rules={{ validate: rhfZodValidate(NodeFormSchema.shape.apiToken) }}
              tooltip={t('pages.nodes.apiTokenHint')}
            >
              <Input.Password placeholder={t('pages.nodes.apiTokenPlaceholder')} />
            </FormField>

            <FormField
              label={t('pages.nodes.outboundTag')}
              name="outboundTag"
              tooltip={t('pages.nodes.outboundTagHint')}
              transform={{ input: (v) => (v as string) || undefined }}
            >
              <Select
                allowClear
                showSearch
                placeholder={t('pages.nodes.outboundTagPlaceholder')}
                options={outboundOptions}
              />
            </FormField>

            <FormField
              label={t('pages.nodes.inboundSyncMode')}
              name="inboundSyncMode"
              tooltip={t('pages.nodes.inboundSyncModeHint')}
            >
              <Select
                options={[
                  { value: 'all', label: t('pages.nodes.allInbounds') },
                  { value: 'selected', label: t('pages.nodes.selectedInbounds') },
                ]}
              />
            </FormField>

            {inboundSyncMode === 'selected' && (
              <FormField
                label={t('pages.nodes.inboundTags')}
                name="inboundTags"
                tooltip={t('pages.nodes.inboundTagsHint')}
              >
                <Select
                  mode="multiple"
                  allowClear
                  loading={fetchingInbounds}
                  placeholder={t('pages.nodes.inboundTagsPlaceholder')}
                  popupRender={(menu) => (
                    <>
                      <Button type="text" block loading={fetchingInbounds} onClick={onFetchInbounds}>
                        {t('pages.nodes.loadInbounds')}
                      </Button>
                      {menu}
                    </>
                  )}
                  options={inboundOptions.map((inbound) => ({
                    value: inbound.tag,
                    label: `${inbound.remark || inbound.tag}${inbound.protocol ? ` (${inbound.protocol}:${inbound.port || 0})` : ''}`,
                  }))}
                />
              </FormField>
            )}

            <div className="test-row">
              <Button type="default" loading={testing} onClick={onTest}>
                {t('pages.nodes.testConnection')}
              </Button>
              {testResult && (
                <div className="test-result">
                  {testResult.status === 'online' ? (
                    <Alert
                      type="success"
                      showIcon
                      title={t('pages.nodes.connectionOk', { ms: testResult.latencyMs })}
                      description={testResult.xrayVersion ? `Xray ${testResult.xrayVersion}` : undefined}
                    />
                  ) : (
                    <Alert
                      type="error"
                      showIcon
                      title={t('pages.nodes.connectionFailed')}
                      description={testResult.error}
                    />
                  )}
                </div>
              )}
            </div>
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
