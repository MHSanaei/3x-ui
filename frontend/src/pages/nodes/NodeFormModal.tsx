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
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { Msg } from '@/utils';
import { NodeFormSchema, type NodeFormValues, type ProbeResult } from '@/schemas/node';
import { antdRule } from '@/utils/zodForm';
import './NodeFormModal.css';

type Mode = 'add' | 'edit';

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  testConnection: (payload: Partial<NodeRecord>) => Promise<Msg<ProbeResult>>;
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
  };
}

export default function NodeFormModal({
  open,
  mode,
  node,
  testConnection,
  save,
  onOpenChange,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<NodeFormValues>();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<ProbeResult | null>(null);

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next: NodeFormValues = mode === 'edit' && node
      ? {
        ...base,
        ...(node as unknown as Partial<NodeFormValues>),
        id: node.id,
        scheme: (node.scheme as 'http' | 'https') || base.scheme,
      }
      : base;
    form.resetFields();
    form.setFieldsValue(next);
    setTestResult(null);
  }, [open, mode, node, form]);

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
    };
  }

  async function onTest() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
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

  async function onFinish(values: NodeFormValues) {
    const result = NodeFormSchema.safeParse(values);
    if (!result.success) {
      messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
      return;
    }
    setSubmitting(true);
    try {
      const msg = await save(buildPayload(result.data));
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
        onOk={() => form.submit()}
        onCancel={close}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={defaultValues()}
          onFinish={onFinish}
        >
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.name')}
                name="name"
                rules={[antdRule(NodeFormSchema.shape.name, t)]}
              >
                <Input placeholder={t('pages.nodes.namePlaceholder')} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.nodes.remark')} name="remark">
                <Input />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={6}>
              <Form.Item label={t('pages.nodes.scheme')} name="scheme">
                <Select
                  options={[
                    { value: 'https', label: 'https' },
                    { value: 'http', label: 'http' },
                  ]}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.address')}
                name="address"
                rules={[antdRule(NodeFormSchema.shape.address, t)]}
              >
                <Input placeholder={t('pages.nodes.addressPlaceholder')} />
              </Form.Item>
            </Col>
            <Col xs={24} md={6}>
              <Form.Item
                label={t('pages.nodes.port')}
                name="port"
                rules={[antdRule(NodeFormSchema.shape.port, t)]}
              >
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label={t('pages.nodes.basePath')} name="basePath">
                <Input placeholder="/" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                label={t('pages.nodes.enable')}
                name="enable"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            label={t('pages.nodes.allowPrivateAddress')}
            name="allowPrivateAddress"
            valuePropName="checked"
            extra={t('pages.nodes.allowPrivateAddressHint')}
          >
            <Switch />
          </Form.Item>

          <Form.Item
            label={t('pages.nodes.apiToken')}
            name="apiToken"
            rules={[antdRule(NodeFormSchema.shape.apiToken, t)]}
            extra={t('pages.nodes.apiTokenHint')}
          >
            <Input.Password placeholder={t('pages.nodes.apiTokenPlaceholder')} />
          </Form.Item>

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
      </Modal>
    </>
  );
}
