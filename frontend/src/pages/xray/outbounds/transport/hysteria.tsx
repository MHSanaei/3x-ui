import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

const MASQ = ['streamSettings', 'hysteriaSettings', 'masquerade'];
const MASQ_DOT = 'streamSettings.hysteriaSettings.masquerade';

export default function HysteriaForm() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const masquerade = useWatch({ control, name: MASQ_DOT }) as { type?: string } | undefined;
  return (
    <>
      <FormField
        label={t('pages.inbounds.form.version')}
        name={['streamSettings', 'hysteriaSettings', 'version']}
      >
        <InputNumber min={2} max={2} disabled style={{ width: '100%' }} />
      </FormField>
      <FormField
        label={t('pages.xray.outboundForm.authPassword')}
        name={['streamSettings', 'hysteriaSettings', 'auth']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.udpIdleTimeout')}
        name={['streamSettings', 'hysteriaSettings', 'udpIdleTimeout']}
      >
        <InputNumber min={2} max={600} style={{ width: '100%' }} />
      </FormField>

      <Form.Item label={t('pages.inbounds.form.masquerade')}>
        <Switch
          checked={!!masquerade}
          onChange={(checked) =>
            setValue(
              MASQ_DOT,
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
      </Form.Item>
      {masquerade && (
        <>
          <FormField label={t('pages.inbounds.form.type')} name={[...MASQ, 'type']}>
            <Select
              options={[
                { value: '', label: 'default (404 page)' },
                { value: 'proxy', label: 'proxy (reverse proxy)' },
                { value: 'file', label: 'file (serve directory)' },
                { value: 'string', label: 'string (fixed body)' },
              ]}
            />
          </FormField>
          {masquerade.type === 'proxy' && (
            <>
              <FormField label={t('pages.inbounds.form.upstreamUrl')} name={[...MASQ, 'url']}>
                <Input placeholder="https://www.example.com" />
              </FormField>
              <FormField
                label={t('pages.inbounds.form.rewriteHost')}
                name={[...MASQ, 'rewriteHost']}
                valueProp="checked"
              >
                <Switch />
              </FormField>
              <FormField
                label={t('pages.inbounds.form.skipTlsVerify')}
                name={[...MASQ, 'insecure']}
                valueProp="checked"
              >
                <Switch />
              </FormField>
            </>
          )}
          {masquerade.type === 'file' && (
            <FormField label={t('pages.inbounds.form.directory')} name={[...MASQ, 'dir']}>
              <Input placeholder="/var/www/html" />
            </FormField>
          )}
          {masquerade.type === 'string' && (
            <>
              <FormField label={t('pages.inbounds.form.statusCode')} name={[...MASQ, 'statusCode']}>
                <InputNumber min={0} max={599} style={{ width: '100%' }} />
              </FormField>
              <FormField label={t('pages.inbounds.form.body')} name={[...MASQ, 'content']}>
                <Input.TextArea autoSize={{ minRows: 3 }} />
              </FormField>
              <FormField label={t('pages.inbounds.form.headers')} name={[...MASQ, 'headers']}>
                <HeaderMapEditor mode="v1" />
              </FormField>
            </>
          )}
        </>
      )}
    </>
  );
}
