import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';

import { ALPN_OPTION, UTLS_FINGERPRINT } from '@/schemas/primitives';

import './external-proxy.css';

const newEntry = () => ({
  forceTls: 'same',
  dest: '',
  port: 443,
  remark: '',
  sni: '',
  fingerprint: '',
  alpn: [],
  pinnedPeerCertSha256: [],
});

function Field({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="ext-proxy-field">
      <span className="ext-proxy-flabel">{label}</span>
      {children}
    </div>
  );
}

export default function ExternalProxyForm({
  toggleExternalProxy,
}: {
  toggleExternalProxy: (on: boolean) => void;
}) {
  const { t } = useTranslation();
  const form = Form.useFormInstance();

  const generateRandomPin = (name: number) => {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    const hash = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
    const path = ['streamSettings', 'externalProxy', name, 'pinnedPeerCertSha256'];
    const current = (form.getFieldValue(path) as string[] | undefined) ?? [];
    form.setFieldValue(path, [...current, hash]);
  };

  return (
    <Form.Item
      noStyle
      shouldUpdate={(prev, curr) => {
        const a = (prev.streamSettings as { externalProxy?: unknown[] } | undefined)?.externalProxy;
        const b = (curr.streamSettings as { externalProxy?: unknown[] } | undefined)?.externalProxy;
        return (Array.isArray(a) ? a.length : 0) !== (Array.isArray(b) ? b.length : 0);
      }}
    >
      {({ getFieldValue }) => {
        const arr = getFieldValue(['streamSettings', 'externalProxy']);
        const on = Array.isArray(arr) && arr.length > 0;
        return (
          <>
            <Form.Item label={t('pages.inbounds.form.externalProxy')}>
              <Switch checked={on} onChange={toggleExternalProxy} />
            </Form.Item>
            {on && (
              <Form.Item wrapperCol={{ span: 24 }}>
                <Form.List name={['streamSettings', 'externalProxy']}>
                  {(fields, { add, remove }) => (
                    <>
                      <div className="ext-proxy-list">
                        {fields.map((field, idx) => (
                          <div key={field.key} className="ext-proxy-card">
                            <div className="ext-proxy-card__head">
                              <span className="ext-proxy-card__title">#{idx + 1}</span>
                              <Button
                                size="small"
                                type="text"
                                danger
                                icon={<DeleteOutlined />}
                                onClick={() => remove(field.name)}
                              />
                            </div>
                            <div className="ext-proxy-grid ext-proxy-grid--dest">
                              <Field label={t('pages.inbounds.form.forceTls')}>
                                <Form.Item name={[field.name, 'forceTls']} noStyle>
                                  <Select
                                    style={{ width: '100%' }}
                                    options={[
                                      { value: 'same', label: t('pages.inbounds.same') },
                                      { value: 'none', label: t('none') },
                                      { value: 'tls', label: 'TLS' },
                                    ]}
                                  />
                                </Form.Item>
                              </Field>
                              <Field label={t('host')}>
                                <Form.Item name={[field.name, 'dest']} noStyle>
                                  <Input placeholder={t('host')} />
                                </Form.Item>
                              </Field>
                              <Field label={t('pages.inbounds.port')}>
                                <Form.Item name={[field.name, 'port']} noStyle>
                                  <InputNumber style={{ width: '100%' }} min={1} max={65535} />
                                </Form.Item>
                              </Field>
                            </div>
                            <Field label={t('pages.inbounds.remark')}>
                              <Form.Item name={[field.name, 'remark']} noStyle>
                                <Input placeholder={t('pages.inbounds.remark')} />
                              </Form.Item>
                            </Field>
                            <Form.Item
                              noStyle
                              shouldUpdate={(prev, curr) =>
                                prev.streamSettings?.externalProxy?.[field.name]?.forceTls
                                !== curr.streamSettings?.externalProxy?.[field.name]?.forceTls
                              }
                            >
                              {({ getFieldValue }) => {
                                const ft = getFieldValue([
                                  'streamSettings', 'externalProxy', field.name, 'forceTls',
                                ]);
                                if (ft !== 'tls') return null;
                                return (
                                  <div className="ext-proxy-tls">
                                    <div className="ext-proxy-grid ext-proxy-grid--tls">
                                      <Field label="SNI">
                                        <Form.Item name={[field.name, 'sni']} noStyle>
                                          <Input placeholder={t('pages.inbounds.form.sniPlaceholder')} />
                                        </Form.Item>
                                      </Field>
                                      <Field label={t('pages.inbounds.form.fingerprint')}>
                                        <Form.Item name={[field.name, 'fingerprint']} noStyle>
                                          <Select
                                            style={{ width: '100%' }}
                                            placeholder={t('pages.inbounds.form.fingerprint')}
                                            options={[
                                              { value: '', label: t('pages.inbounds.form.defaultOption') },
                                              ...Object.values(UTLS_FINGERPRINT).map((fp) => ({
                                                value: fp,
                                                label: fp,
                                              })),
                                            ]}
                                          />
                                        </Form.Item>
                                      </Field>
                                      <Field label="ALPN">
                                        <Form.Item name={[field.name, 'alpn']} noStyle>
                                          <Select
                                            mode="multiple"
                                            style={{ width: '100%' }}
                                            placeholder="ALPN"
                                            options={Object.values(ALPN_OPTION).map((a) => ({
                                              value: a,
                                              label: a,
                                            }))}
                                          />
                                        </Form.Item>
                                      </Field>
                                    </div>
                                    <Field label={t('pages.inbounds.form.pinnedPeerCertSha256')}>
                                      <Space.Compact block>
                                        <Form.Item name={[field.name, 'pinnedPeerCertSha256']} noStyle>
                                          <Select
                                            mode="tags"
                                            tokenSeparators={[',', ' ']}
                                            placeholder={t('pages.inbounds.form.pinnedPeerCertSha256Placeholder')}
                                            style={{ width: 'calc(100% - 32px)' }}
                                          />
                                        </Form.Item>
                                        <Button
                                          icon={<ReloadOutlined />}
                                          onClick={() => generateRandomPin(field.name)}
                                          title={t('pages.inbounds.form.generateRandomPin')}
                                        />
                                      </Space.Compact>
                                    </Field>
                                  </div>
                                );
                              }}
                            </Form.Item>
                          </div>
                        ))}
                      </div>
                      <Button
                        className="ext-proxy-add"
                        block
                        type="dashed"
                        icon={<PlusOutlined />}
                        onClick={() => add(newEntry())}
                      >
                        {t('add')}
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form.Item>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
