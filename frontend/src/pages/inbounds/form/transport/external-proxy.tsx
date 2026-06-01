import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';

import { InputAddon } from '@/components/ui';
import { ALPN_OPTION, UTLS_FINGERPRINT } from '@/schemas/primitives';

export default function ExternalProxyForm({
  toggleExternalProxy,
}: {
  toggleExternalProxy: (on: boolean) => void;
}) {
  const { t } = useTranslation();
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
              <Form.List name={['streamSettings', 'externalProxy']}>
                {(fields, { add, remove }) => (
                  <>
                    <Form.Item label=" " colon={false}>
                      <Button
                        size="small"
                        type="primary"
                        onClick={() => add({
                          forceTls: 'same',
                          dest: '',
                          port: 443,
                          remark: '',
                          sni: '',
                          fingerprint: '',
                          alpn: [],
                        })}
                      >
                        <PlusOutlined />
                      </Button>
                    </Form.Item>
                    <Form.Item wrapperCol={{ span: 24 }}>
                      {fields.map((field) => (
                        <div key={field.key} style={{ margin: '8px 0' }}>
                          <Space.Compact block>
                            <Form.Item name={[field.name, 'forceTls']} noStyle>
                              <Select
                                style={{ width: '20%' }}
                                options={[
                                  { value: 'same', label: t('pages.inbounds.same') },
                                  { value: 'none', label: t('none') },
                                  { value: 'tls', label: 'TLS' },
                                ]}
                              />
                            </Form.Item>
                            <Form.Item name={[field.name, 'dest']} noStyle>
                              <Input style={{ width: '30%' }} placeholder={t('host')} />
                            </Form.Item>
                            <Form.Item name={[field.name, 'port']} noStyle>
                              <InputNumber style={{ width: '15%' }} min={1} max={65535} />
                            </Form.Item>
                            <Form.Item name={[field.name, 'remark']} noStyle>
                              <Input style={{ width: '25%' }} placeholder={t('pages.inbounds.remark')} />
                            </Form.Item>
                            <InputAddon onClick={() => remove(field.name)}>
                              <MinusOutlined />
                            </InputAddon>
                          </Space.Compact>
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
                                <Space.Compact style={{ marginTop: 6 }} block>
                                  <Form.Item name={[field.name, 'sni']} noStyle>
                                    <Input style={{ width: '30%' }} placeholder={t('pages.inbounds.form.sniPlaceholder')} />
                                  </Form.Item>
                                  <Form.Item name={[field.name, 'fingerprint']} noStyle>
                                    <Select
                                      style={{ width: '30%' }}
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
                                  <Form.Item name={[field.name, 'alpn']} noStyle>
                                    <Select
                                      mode="multiple"
                                      style={{ width: '40%' }}
                                      placeholder="ALPN"
                                      options={Object.values(ALPN_OPTION).map((a) => ({
                                        value: a,
                                        label: a,
                                      }))}
                                    />
                                  </Form.Item>
                                </Space.Compact>
                              );
                            }}
                          </Form.Item>
                        </div>
                      ))}
                    </Form.Item>
                  </>
                )}
              </Form.List>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
