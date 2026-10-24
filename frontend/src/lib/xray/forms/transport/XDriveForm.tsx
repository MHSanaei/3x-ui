import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Typography } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { JsonEditor } from '@/components/form';
import { FormField } from '@/components/form/rhf';
import { XDriveServiceSchema } from '@/schemas/protocols/stream/xdrive';

const BASE = ['streamSettings', 'xdriveSettings'] as const;

const SERVICE_LABEL_KEYS: Partial<Record<string, string>> = {
  local: 'pages.inbounds.form.xdrive.serviceLocal',
  template: 'pages.inbounds.form.xdrive.serviceTemplate',
};
const SERVICE_NAMES: Partial<Record<string, string>> = { 'google-drive': 'Google Drive' };

// Core defaults (transport/internet/xdrive/params.go), shown as placeholders so an
// empty field keeps meaning "use the core's value".
const TUNING_FIELDS = [
  ['segmentBytes', 'segmentBytes', 524288],
  ['flushIntervalMs', 'flushInterval', 20],
  ['pollIntervalMs', 'pollInterval', 50],
  ['maxPollIntervalMs', 'maxPollInterval', 500],
  ['eagerWindowMs', 'eagerWindow', 2000],
  ['holeTimeoutMs', 'holeTimeout', 30000],
  ['sessionTtlSeconds', 'sessionTtl', 300],
  ['concurrency', 'concurrency', 8],
] as const;

const GOOGLE_DRIVE_SECRETS = ['clientId', 'clientSecret', 'refreshToken'] as const;

function TemplateEditor({
  value,
  onChange,
}: {
  value?: Record<string, unknown>;
  onChange?: (next: Record<string, unknown> | undefined) => void;
}) {
  const { t } = useTranslation();
  const [text, setText] = useState(() => (value ? JSON.stringify(value, null, 2) : ''));
  const [invalid, setInvalid] = useState(false);
  return (
    <>
      <JsonEditor
        value={text}
        minHeight="160px"
        onChange={(next) => {
          setText(next);
          if (next.trim() === '') {
            setInvalid(false);
            onChange?.(undefined);
            return;
          }
          try {
            const parsed: unknown = JSON.parse(next);
            const isObject = !!parsed && typeof parsed === 'object' && !Array.isArray(parsed);
            setInvalid(!isObject);
            if (isObject) onChange?.(parsed as Record<string, unknown>);
          } catch {
            setInvalid(true);
          }
        }}
      />
      {invalid && (
        <Typography.Text type="danger">
          {t('pages.inbounds.form.xdrive.templateInvalid')}
        </Typography.Text>
      )}
    </>
  );
}

export default function XDriveForm() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const service = useWatch({ control, name: `${BASE.join('.')}.service` }) as string | undefined;
  const serviceOptions = XDriveServiceSchema.options.map((value) => {
    const labelKey = SERVICE_LABEL_KEYS[value];
    return { value, label: labelKey ? t(labelKey) : (SERVICE_NAMES[value] ?? value) };
  });

  return (
    <>
      <FormField label={t('pages.inbounds.form.xdrive.service')} name={[...BASE, 'service']}>
        <Select options={serviceOptions} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.xdrive.remoteFolder')}
        name={[...BASE, 'remoteFolder']}
        required
      >
        <Input placeholder={service === 'local' ? '/var/lib/xdrive' : 'xray-tunnel'} />
      </FormField>
      {service === 'google-drive' &&
        GOOGLE_DRIVE_SECRETS.map((key, index) => (
          <FormField
            key={key}
            label={t(`pages.inbounds.form.xdrive.${key}`)}
            name={[...BASE, 'secrets', index]}
            required
          >
            {key === 'clientId' ? <Input /> : <Input.Password />}
          </FormField>
        ))}
      {service === 'template' && (
        <>
          <FormField
            label={t('pages.inbounds.form.xdrive.secrets')}
            tooltip={t('pages.inbounds.form.xdrive.secretsDesc')}
            name={[...BASE, 'secrets']}
          >
            <Select mode="tags" tokenSeparators={[',']} open={false} />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.xdrive.template')}
            name={[...BASE, 'template']}
            required
          >
            <TemplateEditor />
          </FormField>
        </>
      )}
      {service !== 'local' && (
        <>
          <FormField
            label={t('pages.inbounds.form.xdrive.front')}
            tooltip={t('pages.inbounds.form.xdrive.frontDesc')}
            name={['streamSettings', 'address']}
            transform={{ output: (raw) => String(raw ?? '').trim() || undefined }}
          >
            <Input placeholder="www.googleapis.com" />
          </FormField>
          <FormField
            label={t('pages.inbounds.form.xdrive.frontPort')}
            name={['streamSettings', 'port']}
          >
            <InputNumber min={1} max={65535} placeholder="443" />
          </FormField>
        </>
      )}
      {TUNING_FIELDS.map(([key, label, placeholder]) => (
        <FormField key={key} label={t(`pages.inbounds.form.xdrive.${label}`)} name={[...BASE, key]}>
          <InputNumber min={0} placeholder={String(placeholder)} style={{ width: '100%' }} />
        </FormField>
      ))}
    </>
  );
}
