import { useId } from 'react';
import { Form, Input, InputNumber, Switch } from 'antd';
import { useTranslation } from 'react-i18next';
import { useFormContext, useWatch } from 'react-hook-form';

import { NativeSelect } from '@/components/form/NativeSelect';
import { FormField } from '@/components/form/rhf';

const root = 'streamSettings.xhttpSettings.downloadSettings';

export function XhttpDownloadForm({ mode }: { mode?: string }) {
  const { t } = useTranslation();
  const id = useId();
  const { control, getValues, setValue, unregister } = useFormContext();
  const download = useWatch({ control, name: root });
  const security = download?.security;
  const securityBlock = security === 'reality' ? 'realitySettings' : 'tlsSettings';

  return (
    <>
      <Form.Item label={t('pages.inbounds.form.separateDownload')} htmlFor={id}>
        <Switch
          id={id}
          checked={!!download}
          disabled={!download && mode === 'stream-one'}
          onChange={(enabled) => {
            if (enabled) {
              const upload = getValues('streamSettings.xhttpSettings') ?? {};
              const xhttpSettings: Record<string, unknown> = { path: upload.path || '/' };
              for (const key of [
                'host',
                'headers',
                'sessionIDPlacement',
                'sessionIDKey',
                'sessionIDTable',
                'sessionIDLength',
                'seqPlacement',
                'seqKey',
                'xPaddingBytes',
                'xPaddingObfsMode',
                'xPaddingKey',
                'xPaddingHeader',
                'xPaddingPlacement',
                'xPaddingMethod',
              ]) {
                if (upload[key] !== undefined) xhttpSettings[key] = structuredClone(upload[key]);
              }
              setValue(
                root,
                { network: 'xhttp', port: 443, security: 'tls', xhttpSettings },
                { shouldDirty: true },
              );
            } else {
              unregister(root);
            }
          }}
        />
      </Form.Item>
      {download && (
        <>
          <FormField
            name={root + '.address'}
            label={t('pages.inbounds.address')}
            rules={{
              required: 'pages.xray.outboundForm.addressRequired',
              validate: (v) => !!String(v).trim() || 'pages.xray.outboundForm.addressRequired',
            }}
          >
            <Input />
          </FormField>
          <FormField
            name={root + '.port'}
            label={t('pages.inbounds.port')}
            rules={{ required: 'pages.xray.outboundForm.portRequired', min: 1, max: 65535 }}
          >
            <InputNumber min={1} max={65535} precision={0} />
          </FormField>
          <FormField name={root + '.xhttpSettings.host'} label={t('host')}>
            <Input />
          </FormField>
          <FormField name={root + '.xhttpSettings.path'} label={t('path')}>
            <Input placeholder="/" />
          </FormField>
          <FormField
            name={root + '.security'}
            label={t('security')}
            onAfterChange={(value) => {
              unregister(root + '.tlsSettings');
              unregister(root + '.realitySettings');
              if (value === 'reality')
                setValue(root + '.realitySettings', { fingerprint: 'chrome' });
            }}
          >
            <NativeSelect
              options={['none', 'tls', 'reality'].map((value) => ({ value, label: value }))}
            />
          </FormField>
          {(security === 'tls' || security === 'reality') && (
            <FormField name={root + '.' + securityBlock + '.serverName'} label="SNI">
              <Input />
            </FormField>
          )}
          {security === 'reality' && (
            <>
              <FormField
                name={
                  root +
                  '.realitySettings.' +
                  (download.realitySettings?.publicKey !== undefined &&
                  !download.realitySettings?.password
                    ? 'publicKey'
                    : 'password')
                }
                label={t('pages.inbounds.publicKey')}
                rules={{
                  required: true,
                  pattern: {
                    value: /^[A-Za-z0-9_-]{43}$/,
                    message: 'Enter a 32-byte unpadded base64url REALITY public key.',
                  },
                }}
              >
                <Input />
              </FormField>
              <FormField
                name={root + '.realitySettings.fingerprint'}
                label={t('pages.inbounds.form.fingerprint')}
              >
                <Input placeholder="chrome" />
              </FormField>
              <FormField
                name={root + '.realitySettings.shortId'}
                label={t('pages.xray.outboundForm.shortId')}
                rules={{
                  pattern: {
                    value: /^(?:[0-9a-fA-F]{2}){0,8}$/,
                    message: 'Enter up to 8 hexadecimal bytes.',
                  },
                }}
              >
                <Input />
              </FormField>
            </>
          )}
        </>
      )}
    </>
  );
}
