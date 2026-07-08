import { useTranslation } from 'react-i18next';
import { Form, Input, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function RawForm() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const headerType = (useWatch({
    control,
    name: 'streamSettings.tcpSettings.header.type',
  }) ?? 'none') as string;
  return (
    <>
      <FormField
        name={['streamSettings', 'tcpSettings', 'acceptProxyProtocol']}
        label={t('pages.inbounds.form.proxyProtocol')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <Form.Item label={`HTTP ${t('camouflage')}`}>
        <Switch
          checked={headerType === 'http'}
          onChange={(v) => {
            setValue(
              'streamSettings.tcpSettings.header',
              v
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
            );
          }}
        />
      </Form.Item>
      {headerType === 'http' && (
        <>
          <FormField
            label={t('pages.inbounds.form.requestVersion')}
            name={['streamSettings', 'tcpSettings', 'header', 'request', 'version']}
          >
            <Input placeholder="1.1" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.requestMethod')}
            name={['streamSettings', 'tcpSettings', 'header', 'request', 'method']}
          >
            <Input placeholder="GET" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.requestPath')}
            name={['streamSettings', 'tcpSettings', 'header', 'request', 'path']}
            transform={{
              input: (v) => (Array.isArray(v) ? v.join(',') : v),
              output: (raw) => {
                const parts = String(raw ?? '')
                  .split(',')
                  .map((s) => s.trim())
                  .filter(Boolean);
                return parts.length > 0 ? parts : ['/'];
              },
            }}
          >
            <Input placeholder="/" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.requestHeaders')}
            name={['streamSettings', 'tcpSettings', 'header', 'request', 'headers']}
          >
            <HeaderMapEditor mode="v2" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.responseVersion')}
            name={['streamSettings', 'tcpSettings', 'header', 'response', 'version']}
          >
            <Input placeholder="1.1" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.responseStatus')}
            name={['streamSettings', 'tcpSettings', 'header', 'response', 'status']}
          >
            <Input placeholder="200" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.responseReason')}
            name={['streamSettings', 'tcpSettings', 'header', 'response', 'reason']}
          >
            <Input placeholder="OK" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.responseHeaders')}
            name={['streamSettings', 'tcpSettings', 'header', 'response', 'headers']}
          >
            <HeaderMapEditor mode="v2" />
          </FormField>
        </>
      )}
    </>
  );
}
