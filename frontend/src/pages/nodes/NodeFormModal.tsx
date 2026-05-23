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
import type { NodeRecord } from '@/hooks/useNodes';
import './NodeFormModal.css';

type Mode = 'add' | 'edit';

interface ApiMsg<T = unknown> {
  success?: boolean;
  msg?: string;
  obj?: T;
}

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  testConnection: (payload: Partial<NodeRecord>) => Promise<ApiMsg<{
    status: string;
    latencyMs?: number;
    xrayVersion?: string;
    error?: string;
  }>>;
  save: (payload: Partial<NodeRecord>) => Promise<ApiMsg>;
  onOpenChange: (open: boolean) => void;
}

interface FormState {
  id: number;
  name: string;
  remark: string;
  scheme: 'http' | 'https';
  address: string;
  port: number;
  basePath: string;
  apiToken: string;
  enable: boolean;
  allowPrivateAddress: boolean;
}

function defaultForm(): FormState {
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
  const [messageApi, messageContextHolder] = message.useMessage();

  const [form, setForm] = useState<FormState>(defaultForm);
  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    status: string;
    latencyMs?: number;
    xrayVersion?: string;
    error?: string;
  } | null>(null);

  useEffect(() => {
    if (!open) return;
    const base = defaultForm();
    const next: FormState = mode === 'edit' && node
      ? {
        ...base,
        ...(node as unknown as Partial<FormState>),
        id: node.id,
        scheme: (node.scheme as 'http' | 'https') || base.scheme,
      }
      : base;
     
    setForm(next);
    setTestResult(null);
     
  }, [open, mode, node]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  function buildPayload(): Partial<NodeRecord> {
    return {
      id: form.id || 0,
      name: form.name?.trim() || '',
      remark: form.remark?.trim() || '',
      scheme: form.scheme || 'https',
      address: form.address?.trim() || '',
      port: Number(form.port) || 0,
      basePath: form.basePath?.trim() || '/',
      apiToken: form.apiToken?.trim() || '',
      enable: !!form.enable,
      allowPrivateAddress: !!form.allowPrivateAddress,
    };
  }

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function onTest() {
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload();
      if (!payload.address || !payload.port) {
        messageApi.error(t('pages.nodes.toasts.fillRequired'));
        return;
      }
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

  async function onSave() {
    const payload = buildPayload();
    if (!payload.name || !payload.address || !payload.port) {
      messageApi.error(t('pages.nodes.toasts.fillRequired'));
      return;
    }
    setSubmitting(true);
    try {
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
      onOk={onSave}
      onCancel={close}
    >
      <Form layout="vertical">
        <Row gutter={16}>
          <Col xs={24} md={12}>
            <Form.Item label={t('pages.nodes.name')} required>
              <Input
                value={form.name}
                placeholder={t('pages.nodes.namePlaceholder')}
                onChange={(e) => update('name', e.target.value)}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item label={t('pages.nodes.remark')}>
              <Input value={form.remark} onChange={(e) => update('remark', e.target.value)} />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col xs={24} md={6}>
            <Form.Item label={t('pages.nodes.scheme')}>
              <Select
                value={form.scheme}
                onChange={(v) => update('scheme', v)}
                options={[
                  { value: 'https', label: 'https' },
                  { value: 'http', label: 'http' },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item label={t('pages.nodes.address')} required>
              <Input
                value={form.address}
                placeholder={t('pages.nodes.addressPlaceholder')}
                onChange={(e) => update('address', e.target.value)}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={6}>
            <Form.Item label={t('pages.nodes.port')} required>
              <InputNumber
                value={form.port}
                min={1}
                max={65535}
                style={{ width: '100%' }}
                onChange={(v) => update('port', Number(v) || 0)}
              />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col xs={24} md={12}>
            <Form.Item label={t('pages.nodes.basePath')}>
              <Input
                value={form.basePath}
                placeholder="/"
                onChange={(e) => update('basePath', e.target.value)}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item label={t('pages.nodes.enable')}>
              <Switch checked={form.enable} onChange={(v) => update('enable', v)} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item label={t('pages.nodes.allowPrivateAddress')}>
          <Switch
            checked={form.allowPrivateAddress}
            onChange={(v) => update('allowPrivateAddress', v)}
          />
          <div className="hint">{t('pages.nodes.allowPrivateAddressHint')}</div>
        </Form.Item>

        <Form.Item label={t('pages.nodes.apiToken')} required>
          <Input.Password
            value={form.apiToken}
            placeholder={t('pages.nodes.apiTokenPlaceholder')}
            onChange={(e) => update('apiToken', e.target.value)}
          />
          <div className="hint">{t('pages.nodes.apiTokenHint')}</div>
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
