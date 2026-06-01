import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch, type FormInstance } from 'antd';

import { HeaderMapEditor } from '@/components/form';

const MASQ_PATH = ['streamSettings', 'hysteriaSettings', 'masquerade'];

export default function HysteriaFields({ form }: { form: FormInstance }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('pages.inbounds.form.version')}
        name={['streamSettings', 'hysteriaSettings', 'version']}
      >
        <InputNumber min={2} max={2} disabled />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.udpIdleTimeout')}
        name={['streamSettings', 'hysteriaSettings', 'udpIdleTimeout']}
      >
        <InputNumber min={1} style={{ width: '100%' }} />
      </Form.Item>

      <Form.Item label={t('pages.inbounds.form.masquerade')}>
        <Form.Item shouldUpdate noStyle>
          {() => {
            const m = form.getFieldValue(MASQ_PATH);
            return (
              <Switch
                checked={!!m}
                onChange={(checked) =>
                  form.setFieldValue(
                    MASQ_PATH,
                    checked
                      ? {
                        type: '', dir: '', url: '',
                        rewriteHost: false, insecure: false,
                        content: '', headers: {}, statusCode: 0,
                      }
                      : undefined,
                  )
                }
              />
            );
          }}
        </Form.Item>
      </Form.Item>
      <Form.Item shouldUpdate noStyle>
        {() => {
          const m = form.getFieldValue(MASQ_PATH) as { type?: string } | undefined;
          if (!m) return null;
          return (
            <>
              <Form.Item
                label={t('pages.inbounds.form.type')}
                name={[...MASQ_PATH, 'type']}
              >
                <Select
                  options={[
                    { value: '', label: 'default (404 page)' },
                    { value: 'proxy', label: 'proxy (reverse proxy)' },
                    { value: 'file', label: 'file (serve directory)' },
                    { value: 'string', label: 'string (fixed body)' },
                  ]}
                />
              </Form.Item>
              {m.type === 'proxy' && (
                <>
                  <Form.Item
                    label={t('pages.inbounds.form.upstreamUrl')}
                    name={[...MASQ_PATH, 'url']}
                  >
                    <Input placeholder="https://www.example.com" />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.inbounds.form.rewriteHost')}
                    name={[...MASQ_PATH, 'rewriteHost']}
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.inbounds.form.skipTlsVerify')}
                    name={[...MASQ_PATH, 'insecure']}
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                </>
              )}
              {m.type === 'file' && (
                <Form.Item
                  label={t('pages.inbounds.form.directory')}
                  name={[...MASQ_PATH, 'dir']}
                >
                  <Input placeholder="/var/www/html" />
                </Form.Item>
              )}
              {m.type === 'string' && (
                <>
                  <Form.Item
                    label={t('pages.inbounds.form.statusCode')}
                    name={[...MASQ_PATH, 'statusCode']}
                  >
                    <InputNumber min={0} max={599} style={{ width: '100%' }} />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.inbounds.form.body')}
                    name={[...MASQ_PATH, 'content']}
                  >
                    <Input.TextArea autoSize={{ minRows: 3 }} />
                  </Form.Item>
                  <Form.Item
                    label={t('pages.inbounds.form.headers')}
                    name={[...MASQ_PATH, 'headers']}
                  >
                    <HeaderMapEditor mode="v1" />
                  </Form.Item>
                </>
              )}
            </>
          );
        }}
      </Form.Item>
    </>
  );
}
