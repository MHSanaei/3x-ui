import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';

export default function NaiveFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const scheme = (useWatch({ control, name: 'settings.scheme' }) ?? 'https') as string;
  const watched = useWatch({
    control,
    name: [
      'settings.insecureConcurrency',
      'settings.tunnelTimeout',
      'settings.idleTimeout',
      'settings.extraHeaders',
      'settings.hostResolverRules',
      'settings.resolverRange',
      'settings.noPostQuantum',
    ],
  });
  const hasAdvanced = useMemo(
    () => watched.some((value) => value !== undefined && value !== '' && value !== false),
    [watched],
  );

  return (
    <>
      <FormField label={t('protocol')} name={['settings', 'scheme']} required rules={{ required: true }}>
        <Select
          options={[
            { value: 'https', label: 'HTTPS (recommended)' },
            { value: 'quic', label: 'QUIC (UDP, better on bad TCP)' },
            { value: 'http', label: 'HTTP (no encryption)' },
          ]}
        />
      </FormField>
      {scheme === 'http' && (
        <Form.Item wrapperCol={{ offset: 8 }}>
          <span style={{ color: 'orange' }}>{t('pages.xray.naiveForm.httpWarning')}</span>
        </Form.Item>
      )}
      <FormField label={t('username')} name={['settings', 'user']} required rules={{ required: true }}>
        <Input />
      </FormField>
      <FormField label={t('password')} name={['settings', 'pass']} required rules={{ required: true }}>
        <Input.Password />
      </FormField>
      <FormField label={t('pages.inbounds.address')} name={['settings', 'host']} required rules={{ required: true }}>
        <Input placeholder="example.com" />
      </FormField>
      <FormField label={t('pages.inbounds.port')} name={['settings', 'port']} required rules={{ required: true }}>
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </FormField>
      <Form.Item label={t('pages.xray.naiveForm.advancedOptions')}>
        <Switch
          checked={hasAdvanced}
          onChange={(checked) => {
            if (!checked) {
              setValue('settings.insecureConcurrency', undefined);
              setValue('settings.tunnelTimeout', undefined);
              setValue('settings.idleTimeout', undefined);
              setValue('settings.extraHeaders', undefined);
              setValue('settings.hostResolverRules', undefined);
              setValue('settings.resolverRange', undefined);
              setValue('settings.noPostQuantum', undefined);
            } else {
              setValue('settings.resolverRange', '100.64.0.0/10');
            }
          }}
        />
      </Form.Item>
      {hasAdvanced && (
        <>
          <FormField label="insecure-concurrency" name={['settings', 'insecureConcurrency']}>
            <InputNumber min={1} max={8} style={{ width: '100%' }} />
          </FormField>
          <FormField label="tunnel-timeout" name={['settings', 'tunnelTimeout']} tooltip={t('pages.xray.naiveForm.tunnelTimeoutHint')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </FormField>
          <FormField label="idle-timeout" name={['settings', 'idleTimeout']}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </FormField>
          <FormField label="extra-headers" name={['settings', 'extraHeaders']}>
            <Input.TextArea rows={3} />
          </FormField>
          <FormField label="host-resolver-rules" name={['settings', 'hostResolverRules']}>
            <Input />
          </FormField>
          <FormField label="resolver-range" name={['settings', 'resolverRange']}>
            <Input placeholder="100.64.0.0/10" />
          </FormField>
          <FormField label="no-post-quantum" name={['settings', 'noPostQuantum']} valueProp="checked">
            <Switch />
          </FormField>
        </>
      )}
    </>
  );
}