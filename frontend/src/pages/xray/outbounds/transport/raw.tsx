import { useTranslation } from 'react-i18next';
import { Form, Input, Switch, type FormInstance } from 'antd';

import { HeaderMapEditor } from '@/components/form';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

export default function RawForm({ form }: { form: FormInstance<OutboundFormValues> }) {
  const { t } = useTranslation();
  return (
    <Form.Item shouldUpdate noStyle>
      {() => {
        const type =
          form.getFieldValue([
            'streamSettings',
            'tcpSettings',
            'header',
            'type',
          ]) ?? 'none';
        return (
          <>
            <Form.Item label={`HTTP ${t('camouflage')}`}>
              <Switch
                checked={type === 'http'}
                onChange={(checked) =>
                  form.setFieldValue(
                    ['streamSettings', 'tcpSettings', 'header'],
                    checked
                      ? {
                        type: 'http',
                        request: {
                          version: '1.1',
                          method: 'GET',
                          path: ['/'],
                          headers: {},
                        },
                        response: {
                          version: '1.1',
                          status: '200',
                          reason: 'OK',
                          headers: {},
                        },
                      }
                      : { type: 'none' },
                  )
                }
              />
            </Form.Item>
            {type === 'http' && (
              <>
                <Form.Item
                  label={t('pages.inbounds.form.requestVersion')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'request', 'version',
                  ]}
                >
                  <Input placeholder="1.1" />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.requestMethod')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'request', 'method',
                  ]}
                >
                  <Input placeholder="GET" />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.requestPath')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'request', 'path',
                  ]}
                  getValueProps={(v) => ({ value: Array.isArray(v) ? v.join(',') : v })}
                  getValueFromEvent={(e) => {
                    const raw = (e?.target?.value ?? '') as string;
                    const parts = raw.split(',').map((s) => s.trim()).filter(Boolean);
                    return parts.length > 0 ? parts : ['/'];
                  }}
                >
                  <Input placeholder="/" />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.requestHeaders')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'request', 'headers',
                  ]}
                >
                  <HeaderMapEditor mode="v2" />
                </Form.Item>

                <Form.Item
                  label={t('pages.inbounds.form.responseVersion')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'response', 'version',
                  ]}
                >
                  <Input placeholder="1.1" />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.responseStatus')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'response', 'status',
                  ]}
                >
                  <Input placeholder="200" />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.responseReason')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'response', 'reason',
                  ]}
                >
                  <Input placeholder="OK" />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.responseHeaders')}
                  name={[
                    'streamSettings', 'tcpSettings', 'header',
                    'response', 'headers',
                  ]}
                >
                  <HeaderMapEditor mode="v2" />
                </Form.Item>
              </>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
