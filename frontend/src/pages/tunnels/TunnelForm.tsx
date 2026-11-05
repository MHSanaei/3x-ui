import { useEffect } from 'react';
import {
  Button,
  Collapse,
  Form,
  Input,
  InputNumber,
  Modal,
  Switch,
  Alert,
  Row,
  Col,
  Divider,
} from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { NativeSelect } from '@/components/form/NativeSelect';
import './TunnelsPage.css';
import { FormProvider, useFieldArray, useForm, useWatch } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { FormField } from '@/components/form/rhf';
import { HttpUtil } from '@/utils';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import { CapabilitiesSchema, TunnelFormSchema, defaultTunnel } from '@/schemas/tunnel';
import type { TunnelFormValues, TunnelRecord } from '@/schemas/tunnel';

export default function TunnelForm({
  open,
  record,
  onClose,
  onSave,
}: {
  open: boolean;
  record: TunnelRecord | null;
  onClose: () => void;
  onSave: (data: TunnelFormValues) => Promise<void>;
}) {
  const { t } = useTranslation();
  const form = useForm<TunnelFormValues>({
    defaultValues: defaultTunnel(),
    resolver: zodResolver(TunnelFormSchema),
  });
  const { reset, control, handleSubmit, formState, setValue } = form;
  const fields = useFieldArray({ control, name: 'config.ports' });
  const candidates = useFieldArray({ control, name: 'config.candidates' });
  const { nodes } = useNodesQuery();
  const role = useWatch({ control, name: 'config.role' });
  const carrier = useWatch({ control, name: 'config.carrier' });
  const reverse = useWatch({ control, name: 'config.reverse' });
  const nodeId = useWatch({ control, name: 'nodeId' });
  const generateKey = useWatch({ control, name: 'generateKey' });
  const caps = useQuery({
    queryKey: ['tunnels', 'capabilities', nodeId],
    enabled: open,
    queryFn: async () => {
      const suffix = nodeId ? `?nodeId=${nodeId}` : '';
      const msg = await HttpUtil.get(`/panel/api/tunnels/capabilities${suffix}`, undefined, {
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || t('somethingWentWrong'));
      return CapabilitiesSchema.parse(msg.obj);
    },
  });
  useEffect(() => {
    if (!open) return;
    const values = record
      ? TunnelFormSchema.parse({
          ...record,
          nodeId: record.nodeId ?? null,
          secret: '',
          token: '',
          generateKey: false,
        })
      : defaultTunnel();
    reset(values);
  }, [open, record, reset]);
  const carriers = ['auto', 'udp', 'tcp', 'tls', 'wss', 'icmp'] as const;
  const submit = handleSubmit(async (data) => {
    try {
      await onSave(data);
      onClose();
    } catch (error) {
      form.setError('root', {
        message: error instanceof Error ? error.message : t('somethingWentWrong'),
      });
    }
  });
  const carrierOptions = carriers.map((value) => ({
    value,
    label: value.toUpperCase(),
    disabled: value === 'icmp' && !caps.data?.rawSocket,
  }));
  return (
    <Modal
      open={open}
      title={record ? t('edit') : t('menu.tunnels')}
      onCancel={onClose}
      onOk={() => void submit()}
      confirmLoading={formState.isSubmitting}
      okText={t('save')}
      cancelText={t('cancel')}
      mask={{ closable: false }}
      width={760}
      className="tunnel-form-modal"
      styles={{ body: { maxHeight: '70vh', overflowY: 'auto', overflowX: 'hidden' } }}
      destroyOnHidden
    >
      <FormProvider {...form}>
        <Form layout="vertical" component="div" className="tunnel-form">
          <div className="tunnel-form-notices">
            <Alert type="info" showIcon title={t('pages.tunnels.flow')} />
            {caps.error && <Alert type="error" showIcon title={caps.error.message} />}
            {caps.data?.error && (
              <Alert
                type="warning"
                showIcon
                title={
                  caps.data.supported ? caps.data.error : t('pages.tunnels.unsupportedPlatform')
                }
              />
            )}
            {formState.errors.root && (
              <Alert type="error" showIcon title={formState.errors.root.message} />
            )}
          </div>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <FormField name="name" label={t('remark')} required>
                <Input maxLength={128} />
              </FormField>
            </Col>
            <Col xs={24} sm={12}>
              <FormField
                name="nodeId"
                label={t('menu.nodes')}
                transform={{ input: (v) => v ?? '', output: (v) => (v === '' ? null : Number(v)) }}
              >
                <NativeSelect<string | number>
                  disabled={!!record}
                  options={[
                    { value: '', label: t('pages.tunnels.local') },
                    ...nodes
                      .filter((n) => !n.transitive)
                      .map((n) => ({ value: n.id, label: n.name || n.remark || `#${n.id}` })),
                  ]}
                />
              </FormField>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12}>
              <FormField
                name="config.role"
                label={t('pages.tunnels.role')}
                onAfterChange={() => setValue('config.sessions', 0)}
              >
                <NativeSelect
                  options={[
                    { value: 'server', label: 'Server' },
                    { value: 'client', label: 'Client' },
                  ]}
                />
              </FormField>
            </Col>
            <Col xs={24} sm={12}>
              <FormField
                name="config.carrier"
                label={t('pages.tunnels.carrier')}
                onAfterChange={(v) => {
                  setValue('config.sessions', 0);
                  if (v === 'auto' && candidates.fields.length === 0)
                    setValue('config.candidates', [
                      { carrier: 'udp', port: 4443 },
                      { carrier: 'tcp', port: 4443 },
                      { carrier: 'tls', port: 8443 },
                      { carrier: 'wss', port: 2054 },
                      ...(caps.data?.rawSocket ? [{ carrier: 'icmp', port: 0 }] : []),
                    ]);
                }}
              >
                <NativeSelect options={carrierOptions} />
              </FormField>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={carrier === 'icmp' || carrier === 'auto' ? 24 : 16}>
              <FormField name="config.endpoint" label={t('pages.tunnels.endpoint')} required>
                <Input />
              </FormField>
            </Col>
            {carrier !== 'icmp' && carrier !== 'auto' && (
              <Col xs={24} sm={8}>
                <FormField name="config.port" label={t('pages.tunnels.carrierPort')}>
                  <InputNumber min={1} max={65535} />
                </FormField>
              </Col>
            )}
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <FormField
                name="config.reverse"
                label="Reverse"
                tooltip={t('pages.tunnels.reverseHint')}
                valueProp="checked"
                onAfterChange={() => setValue('config.sessions', 0)}
              >
                <Switch />
              </FormField>
            </Col>
            <Col span={12}>
              <FormField name="enabled" label={t('enable')} valueProp="checked">
                <Switch />
              </FormField>
            </Col>
          </Row>
          {reverse && role === 'server' && (
            <FormField name="config.reversePeer" label={t('pages.tunnels.reversePeer')}>
              <Input />
            </FormField>
          )}
          {carrier === 'auto' && (
            <section className="tunnel-form-section" aria-label={t('pages.tunnels.carrier')}>
              <Alert type="info" showIcon title={t('pages.tunnels.autoHint')} />
              {candidates.fields.map((field, i) => (
                <div className="tunnel-candidate-row" key={field.id}>
                  <FormField
                    name={`config.candidates.${i}.carrier`}
                    label={t('pages.tunnels.carrier')}
                  >
                    <NativeSelect
                      options={carrierOptions.filter((option) => option.value !== 'auto')}
                    />
                  </FormField>
                  <FormField
                    name={`config.candidates.${i}.port`}
                    label={t('pages.tunnels.carrierPort')}
                  >
                    <InputNumber min={0} max={65535} />
                  </FormField>
                  <Button
                    className="tunnel-row-remove"
                    danger
                    type="text"
                    icon={<DeleteOutlined aria-hidden />}
                    aria-label={`${t('remove')} ${i + 1}`}
                    onClick={() => candidates.remove(i)}
                  />
                </div>
              ))}
              <Button
                type="dashed"
                block
                icon={<PlusOutlined aria-hidden />}
                disabled={candidates.fields.length >= 5}
                onClick={() => candidates.append({ carrier: 'tcp', port: 4443 })}
              >
                {t('add')}
              </Button>
            </section>
          )}
          <Divider titlePlacement="start">{t('pages.tunnels.ports')}</Divider>
          <section className="tunnel-form-section" aria-label={t('pages.tunnels.ports')}>
            {fields.fields.map((field, i) => (
              <div className="tunnel-mapping-row" key={field.id}>
                <FormField name={`config.ports.${i}.listen`} label={t('pages.tunnels.listenPort')}>
                  <InputNumber min={1} max={65535} />
                </FormField>
                <FormField name={`config.ports.${i}.target`} label={t('pages.tunnels.targetPort')}>
                  <InputNumber min={1} max={65535} />
                </FormField>
                <FormField name={`config.ports.${i}.protocol`} label={t('protocol')}>
                  <NativeSelect
                    options={[
                      { value: 'tcp', label: 'TCP' },
                      { value: 'udp', label: 'UDP' },
                    ]}
                  />
                </FormField>
                <Button
                  className="tunnel-row-remove"
                  danger
                  type="text"
                  icon={<DeleteOutlined aria-hidden />}
                  aria-label={`${t('remove')} ${i + 1}`}
                  disabled={fields.fields.length === 1}
                  onClick={() => fields.remove(i)}
                />
              </div>
            ))}
            <Button
              type="dashed"
              block
              icon={<PlusOutlined aria-hidden />}
              disabled={fields.fields.length >= 128}
              onClick={() => fields.append({ listen: 8443, target: 443, protocol: 'tcp' })}
            >
              {t('add')}
            </Button>
          </section>
          <Divider titlePlacement="start">{t('pages.tunnels.secret')}</Divider>
          <FormField
            name="generateKey"
            label={t('pages.tunnels.generateKey')}
            valueProp="checked"
            onAfterChange={(v) => {
              if (v) {
                setValue('secret', '');
                setValue('token', '');
              }
            }}
          >
            <Switch />
          </FormField>
          {!generateKey && (
            <>
              <FormField
                name="secret"
                label={t('pages.tunnels.secret')}
                extra={t('pages.tunnels.keepSecret')}
              >
                <Input.Password autoComplete="new-password" />
              </FormField>
              <FormField
                name="token"
                label={t('pages.tunnels.token')}
                onAfterChange={(v) => {
                  if (v) setValue('config.role', 'client');
                }}
              >
                <Input.Password autoComplete="off" />
              </FormField>
            </>
          )}
          <Collapse
            items={[
              {
                key: 'advanced',
                label: t('pages.tunnels.advanced'),
                children: (
                  <>
                    <Row gutter={16}>
                      <Col xs={24} sm={12}>
                        <FormField name="config.bind" label={t('pages.tunnels.bind')}>
                          <Input />
                        </FormField>
                      </Col>
                      <Col xs={24} sm={12}>
                        <FormField name="config.listen" label={t('pages.tunnels.listenAddress')}>
                          <Input />
                        </FormField>
                      </Col>
                    </Row>
                    <Row gutter={16}>
                      <Col xs={24} sm={12}>
                        <FormField name="config.mtu" label="MTU">
                          <InputNumber
                            min={0}
                            max={['auto', 'udp', 'icmp'].includes(carrier) ? 1400 : 16384}
                          />
                        </FormField>
                      </Col>
                      <Col xs={24} sm={12}>
                        <FormField name="config.sessions" label={t('pages.tunnels.sessions')}>
                          <InputNumber
                            min={0}
                            max={16}
                            disabled={reverse || role === 'server' || carrier === 'icmp'}
                          />
                        </FormField>
                      </Col>
                      {!['udp', 'icmp'].includes(carrier) && (
                        <Col xs={24} sm={12}>
                          <FormField name="config.conns" label={t('pages.tunnels.conns')}>
                            <InputNumber min={0} max={64} />
                          </FormField>
                        </Col>
                      )}
                      <Col xs={24} sm={12}>
                        <FormField name="config.memoryMB" label={t('pages.tunnels.memory')}>
                          <InputNumber min={0} max={65536} />
                        </FormField>
                      </Col>
                    </Row>
                    <FormField name="config.fec" label="FEC" valueProp="checked">
                      <Switch />
                    </FormField>
                    {['tls', 'wss', 'auto'].includes(carrier) && (
                      <>
                        <Row gutter={16}>
                          <Col xs={24} sm={12}>
                            <FormField name="config.sni" label="SNI">
                              <Input />
                            </FormField>
                          </Col>
                          <Col xs={24} sm={12}>
                            <FormField name="config.path" label={t('pages.tunnels.path')}>
                              <Input />
                            </FormField>
                          </Col>
                        </Row>
                        <FormField name="config.tlsPin" label="TLS SHA-256">
                          <Input />
                        </FormField>
                        {((role === 'server' && !reverse) || (role === 'client' && reverse)) && (
                          <>
                            <FormField name="config.tlsCert" label={t('pages.tunnels.tlsCert')}>
                              <Input />
                            </FormField>
                            <FormField name="config.tlsKey" label={t('pages.tunnels.tlsKey')}>
                              <Input />
                            </FormField>
                          </>
                        )}
                      </>
                    )}
                  </>
                ),
              },
            ]}
          />
        </Form>
      </FormProvider>
    </Modal>
  );
}
