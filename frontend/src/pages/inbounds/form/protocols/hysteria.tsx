import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { HeaderMapEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';

const MASQ_PATH = ['streamSettings', 'hysteriaSettings', 'masquerade'];

export default function HysteriaFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const masq = useWatch({ control, name: 'streamSettings.hysteriaSettings.masquerade' }) as
    | { type?: string }
    | undefined;
  const masqType = useWatch({ control, name: 'streamSettings.hysteriaSettings.masquerade.type' }) as
    | string
    | undefined;
  return (
    <>
      <FormField
        label={t('pages.inbounds.form.version')}
        name={['streamSettings', 'hysteriaSettings', 'version']}
      >
        <InputNumber min={2} max={2} disabled />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.udpIdleTimeout')}
        name={['streamSettings', 'hysteriaSettings', 'udpIdleTimeout']}
      >
        <InputNumber min={2} max={600} style={{ width: '100%' }} />
      </FormField>

      <Form.Item label={t('pages.inbounds.form.masquerade')}>
        <Switch
          checked={!!masq}
          onChange={(checked) =>
            setValue(
              'streamSettings.hysteriaSettings.masquerade',
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
      {masq && (
        <>
          <FormField
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
          </FormField>
          {masqType === 'proxy' && (
            <>
              <FormField
                label={t('pages.inbounds.form.upstreamUrl')}
                name={[...MASQ_PATH, 'url']}
              >
                <Input placeholder="https://www.example.com" />
              </FormField>
              <FormField
                label={t('pages.inbounds.form.rewriteHost')}
                name={[...MASQ_PATH, 'rewriteHost']}
                valueProp="checked"
              >
                <Switch />
              </FormField>
              <FormField
                label={t('pages.inbounds.form.skipTlsVerify')}
                name={[...MASQ_PATH, 'insecure']}
                valueProp="checked"
              >
                <Switch />
              </FormField>
            </>
          )}
          {masqType === 'file' && (
            <FormField
              label={t('pages.inbounds.form.directory')}
              name={[...MASQ_PATH, 'dir']}
            >
              <Input placeholder="/var/www/html" />
            </FormField>
          )}
          {masqType === 'string' && (
            <>
              <FormField
                label={t('pages.inbounds.form.statusCode')}
                name={[...MASQ_PATH, 'statusCode']}
              >
                <InputNumber min={0} max={599} style={{ width: '100%' }} />
              </FormField>
              <FormField
                label={t('pages.inbounds.form.body')}
                name={[...MASQ_PATH, 'content']}
              >
                <Input.TextArea autoSize={{ minRows: 3 }} />
              </FormField>
              <FormField
                label={t('pages.inbounds.form.headers')}
                name={[...MASQ_PATH, 'headers']}
              >
                <HeaderMapEditor mode="v1" />
              </FormField>
            </>
          )}
        </>
      )}
    </>
  );
}
