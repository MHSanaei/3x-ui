import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Form, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import {
  NaiveOutboundFormSettingsSchema,
  type OutboundFormValues,
} from '@/schemas/forms/outbound-form';

export default function NaiveFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext<OutboundFormValues>();
  const scheme = useWatch({ control, name: 'settings.scheme' });
  const insecureConcurrency = useWatch({ control, name: 'settings.insecureConcurrency' });
  const tunnelTimeout = useWatch({ control, name: 'settings.tunnelTimeout' });
  const idleTimeout = useWatch({ control, name: 'settings.idleTimeout' });
  const extraHeaders = useWatch({ control, name: 'settings.extraHeaders' });
  const hostResolverRules = useWatch({ control, name: 'settings.hostResolverRules' });
  const resolverRange = useWatch({ control, name: 'settings.resolverRange' });
  const noPostQuantum = useWatch({ control, name: 'settings.noPostQuantum' });
  const hasAdvanced = useMemo(
    () => [
      insecureConcurrency,
      tunnelTimeout,
      idleTimeout,
      extraHeaders,
      hostResolverRules,
      resolverRange,
      noPostQuantum,
    ].some((value) => value !== undefined && value !== '' && value !== false),
    [
      extraHeaders,
      hostResolverRules,
      idleTimeout,
      insecureConcurrency,
      noPostQuantum,
      resolverRange,
      tunnelTimeout,
    ],
  );

  function toggleAdvanced(checked: boolean) {
    if (checked) {
      setValue('settings.resolverRange', '100.64.0.0/10');
      return;
    }
    setValue('settings.insecureConcurrency', undefined);
    setValue('settings.tunnelTimeout', undefined);
    setValue('settings.idleTimeout', undefined);
    setValue('settings.extraHeaders', undefined);
    setValue('settings.hostResolverRules', undefined);
    setValue('settings.resolverRange', undefined);
    setValue('settings.noPostQuantum', undefined);
  }

  return (
    <>
      <FormField
        label={t('transmission')}
        name="settings.scheme"
        required
        rules={{ validate: rhfZodValidate(NaiveOutboundFormSettingsSchema.shape.scheme) }}
      >
        <Select
          options={[
            { value: 'https', label: 'HTTPS' },
            { value: 'quic', label: 'QUIC' },
            { value: 'http', label: 'HTTP' },
          ]}
        />
      </FormField>

      {scheme === 'http' && (
        <Form.Item wrapperCol={{ md: { offset: 8, span: 14 } }}>
          <Alert type="warning" showIcon title={t('pages.xray.naiveForm.httpWarning')} />
        </Form.Item>
      )}

      <FormField
        label={t('username')}
        name="settings.user"
        required
        rules={{ required: 'pages.login.toasts.emptyUsername' }}
      >
        <Input autoComplete="off" />
      </FormField>
      <FormField
        label={t('password')}
        name="settings.pass"
        required
        rules={{ required: 'pages.login.toasts.emptyPassword' }}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField
        label={t('pages.inbounds.address')}
        name="settings.host"
        required
        rules={{ required: 'pages.xray.outboundForm.addressRequired' }}
      >
        <Input placeholder="proxy.example.com" />
      </FormField>
      <FormField
        label={t('pages.inbounds.port')}
        name="settings.port"
        required
        rules={{ required: 'pages.xray.outboundForm.portRequired' }}
      >
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </FormField>

      <Form.Item label={t('pages.xray.naiveForm.advancedOptions')}>
        <Switch checked={hasAdvanced} onChange={toggleAdvanced} />
      </Form.Item>

      {hasAdvanced && (
        <>
          <FormField label="insecure-concurrency" name="settings.insecureConcurrency" tooltip={t('pages.xray.naiveForm.insecureConcurrencyHint')}>
            <InputNumber min={1} max={8} style={{ width: '100%' }} />
          </FormField>
          <FormField
            label="tunnel-timeout"
            name="settings.tunnelTimeout"
            tooltip={t('pages.xray.naiveForm.tunnelTimeoutHint')}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </FormField>
          <FormField label="idle-timeout" name="settings.idleTimeout" tooltip={t('pages.xray.naiveForm.idleTimeoutHint')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </FormField>
          <FormField label="extra-headers" name="settings.extraHeaders" tooltip={t('pages.xray.naiveForm.extraHeadersHint')}>
            <Input.TextArea rows={3} />
          </FormField>
          <FormField label="host-resolver-rules" name="settings.hostResolverRules" tooltip={t('pages.xray.naiveForm.hostResolverRulesHint')}>
            <Input />
          </FormField>
          <FormField label="resolver-range" name="settings.resolverRange" tooltip={t('pages.xray.naiveForm.resolverRangeHint')}>
            <Input placeholder="100.64.0.0/10" />
          </FormField>
          <FormField label="no-post-quantum" name="settings.noPostQuantum" valueProp="checked" tooltip={t('pages.xray.naiveForm.noPostQuantumHint')}>
            <Switch />
          </FormField>
        </>
      )}
    </>
  );
}
